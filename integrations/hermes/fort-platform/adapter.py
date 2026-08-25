"""Hermes platform adapter for Fort messaging channels."""

from __future__ import annotations

import asyncio
import json
import os
import uuid
from collections.abc import Mapping
from typing import Any, Dict, Optional

from gateway.config import Platform, PlatformConfig
from gateway.platforms.base import (
    BasePlatformAdapter,
    MessageEvent,
    MessageType,
    SendResult,
)


_REGISTRATION_TIMEOUT_SECONDS = 10
_RECEIPT_TIMEOUT_SECONDS = 30
_MAX_FRAME_BYTES = 64 * 1024


class FortProtocolError(RuntimeError):
    pass


class FortPlatformAdapter(BasePlatformAdapter):
    """One profile-scoped connection from Hermes to Fort."""

    def __init__(
        self,
        config: PlatformConfig,
        *,
        profile_id: str,
        display_name: str,
    ) -> None:
        super().__init__(config=config, platform=Platform("fort"))
        extra = getattr(config, "extra", {}) or {}
        self.profile_id = profile_id
        self.display_name = display_name
        self.url = (_profile_value("FORT_PLATFORM_URL") or extra.get("url", "")).strip()
        self._token = _profile_value("FORT_PLATFORM_TOKEN").strip()
        self._ws = None
        self._reader_task: Optional[asyncio.Task] = None
        self._closing = False
        self._channel_id = ""
        self._conversation_id = ""
        self._write_lock = asyncio.Lock()
        self._pending_receipts: dict[str, asyncio.Future] = {}

    async def connect(self, *, is_reconnect: bool = False) -> bool:
        if self.is_connected:
            return True
        if not self.url or not self._token:
            self._set_fatal_error(
                "config_missing",
                "FORT_PLATFORM_URL and FORT_PLATFORM_TOKEN are required",
                retryable=False,
            )
            return False

        import websockets

        self._closing = False
        try:
            self._ws = await websockets.connect(
                self.url,
                additional_headers={
                    "Authorization": f"Bearer {self._token}",
                    "X-Fort-Hermes-Profile": self.profile_id,
                },
                compression=None,
                max_size=_MAX_FRAME_BYTES,
                open_timeout=_REGISTRATION_TIMEOUT_SECONDS,
                close_timeout=5,
            )
            await self._ws.send(
                json.dumps(
                    {
                        "type": "register",
                        "contract_version": 1,
                        "profile_id": self.profile_id,
                        "display_name": self.display_name,
                    },
                    separators=(",", ":"),
                )
            )
            raw = await asyncio.wait_for(
                self._ws.recv(), timeout=_REGISTRATION_TIMEOUT_SECONDS
            )
            registered = _decode_frame(raw)
            if registered.get("type") != "registered":
                raise FortProtocolError("Fort rejected platform registration")
            self._channel_id = _required_text(registered, "channel_id")
            self._conversation_id = _required_text(registered, "conversation_id")
        except FortProtocolError as exc:
            await self._close_socket()
            self._set_fatal_error(
                "registration_rejected", str(exc), retryable=False
            )
            return False
        except Exception as exc:
            await self._close_socket()
            self._set_fatal_error("connect_failed", str(exc), retryable=True)
            return False

        self._reader_task = asyncio.create_task(self._read_loop())
        self._mark_connected()
        return True

    async def disconnect(self) -> None:
        self._closing = True
        task = self._reader_task
        self._reader_task = None
        if task is not None and task is not asyncio.current_task() and not task.done():
            task.cancel()
            try:
                await task
            except asyncio.CancelledError:
                pass
        self._fail_pending(ConnectionError("Fort platform disconnected"))
        await self._close_socket()
        self._channel_id = ""
        self._conversation_id = ""
        self._mark_disconnected()

    async def send(
        self,
        chat_id: str,
        content: str,
        reply_to: Optional[str] = None,
        metadata: Optional[Dict[str, Any]] = None,
    ) -> SendResult:
        if not self.is_connected or self._ws is None:
            return SendResult(
                success=False,
                error="Fort is not connected",
                retryable=True,
            )
        if chat_id != self._conversation_id:
            return SendResult(
                success=False,
                error="Fort Conversation does not match the registered channel",
                retryable=False,
            )
        if not isinstance(content, str) or not content:
            return SendResult(
                success=False,
                error="Fort outbound text is required",
                retryable=False,
            )

        metadata = metadata or {}
        request_id = metadata.get("request_id") or uuid.uuid4().hex
        if (
            not isinstance(request_id, str)
            or not request_id
            or request_id.strip() != request_id
            or request_id in self._pending_receipts
        ):
            return SendResult(
                success=False,
                error="Fort outbound request ID is invalid or already pending",
                retryable=False,
            )
        if reply_to is not None and not isinstance(reply_to, str):
            return SendResult(
                success=False,
                error="Fort reply target must be text",
                retryable=False,
            )

        future = asyncio.get_running_loop().create_future()
        self._pending_receipts[request_id] = future
        frame = {
            "type": "outbound",
            "request_id": request_id,
            "conversation_id": chat_id,
            "text": content,
            "in_reply_to_message_id": reply_to or "",
        }
        try:
            async with self._write_lock:
                await self._ws.send(json.dumps(frame, separators=(",", ":")))
        except Exception:
            self._pending_receipts.pop(request_id, None)
            return SendResult(
                success=False,
                error="Fort delivery outcome is unknown",
                retryable=False,
            )

        try:
            outcome = await asyncio.wait_for(
                asyncio.shield(future), timeout=_RECEIPT_TIMEOUT_SECONDS
            )
        except (asyncio.TimeoutError, ConnectionError):
            return SendResult(
                success=False,
                error="Fort delivery outcome is unknown",
                retryable=False,
            )
        finally:
            self._pending_receipts.pop(request_id, None)
        if outcome["type"] == "failure":
            return SendResult(
                success=False,
                error=f"Fort rejected message: {outcome['code']}",
                retryable=False,
                raw_response=outcome,
            )
        return SendResult(
            success=True,
            message_id=outcome["message_id"],
            raw_response=outcome,
        )

    async def get_chat_info(self, chat_id: str) -> Dict[str, Any]:
        return {"name": chat_id, "type": "dm"}

    async def _read_loop(self) -> None:
        notify_fatal_error = False
        try:
            async for raw in self._ws:
                frame = _decode_frame(raw)
                if frame.get("type") == "inbound":
                    await self._handle_inbound(frame)
                elif frame.get("type") == "receipt":
                    self._handle_receipt(frame)
                elif frame.get("type") == "failure":
                    self._handle_failure(frame)
                else:
                    raise FortProtocolError("Fort sent an unsupported platform frame")
        except asyncio.CancelledError:
            raise
        except FortProtocolError as exc:
            if not self._closing:
                self._set_fatal_error("protocol_error", str(exc), retryable=False)
                notify_fatal_error = True
        except Exception as exc:
            if not self._closing:
                self._set_fatal_error("connection_lost", str(exc), retryable=True)
                notify_fatal_error = True
        else:
            if not self._closing:
                self._set_fatal_error(
                    "connection_lost",
                    "Fort platform connection closed",
                    retryable=True,
                )
                notify_fatal_error = True
        finally:
            self._fail_pending(ConnectionError("Fort platform connection closed"))
            await self._close_socket()
            self._channel_id = ""
            self._conversation_id = ""
            self._mark_disconnected()
            if self._reader_task is asyncio.current_task():
                self._reader_task = None
        if notify_fatal_error:
            await self._notify_fatal_error()

    async def _handle_inbound(self, frame: dict) -> None:
        _required_text(frame, "request_id")
        message_id = _required_text(frame, "message_id")
        conversation_id = _required_text(frame, "conversation_id")
        if conversation_id != self._conversation_id:
            raise FortProtocolError("Fort inbound conversation does not match registration")
        text = _required_message_text(frame)
        author_id = _required_text(frame, "author_id")
        author_name = _required_text(frame, "author_name")
        source = self.build_source(
            chat_id=conversation_id,
            chat_name="Fort",
            chat_type="dm",
            user_id=author_id,
            user_name=author_name,
            message_id=message_id,
        )
        await self.handle_message(
            MessageEvent(
                text=text,
                message_type=MessageType.TEXT,
                source=source,
                raw_message=frame,
                message_id=message_id,
            )
        )

    def _handle_receipt(self, frame: dict) -> None:
        request_id = _required_text(frame, "request_id")
        message_id = _required_text(frame, "message_id")
        future = self._pending_receipts.get(request_id)
        if future is not None and not future.done():
            future.set_result(
                {
                    "type": "receipt",
                    "request_id": request_id,
                    "message_id": message_id,
                }
            )

    def _handle_failure(self, frame: dict) -> None:
        request_id = _required_text(frame, "request_id")
        code = _required_text(frame, "code")
        future = self._pending_receipts.get(request_id)
        if future is not None and not future.done():
            future.set_result(
                {
                    "type": "failure",
                    "request_id": request_id,
                    "code": code,
                }
            )

    def _fail_pending(self, error: Exception) -> None:
        for future in tuple(self._pending_receipts.values()):
            if not future.done():
                future.set_exception(error)

    async def _close_socket(self) -> None:
        socket = self._ws
        self._ws = None
        if socket is not None:
            try:
                await socket.close()
            except Exception:
                pass


def _decode_frame(raw: Any) -> dict:
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8")
    if not isinstance(raw, str):
        raise FortProtocolError("Fort platform frame must be JSON text")
    try:
        frame = json.loads(raw)
    except (TypeError, ValueError) as exc:
        raise FortProtocolError("Fort platform frame is invalid JSON") from exc
    if not isinstance(frame, dict):
        raise FortProtocolError("Fort platform frame must be a JSON object")
    return frame


def _required_text(frame: dict, field: str) -> str:
    value = frame.get(field)
    if not isinstance(value, str) or not value or value.strip() != value:
        raise FortProtocolError(f"Fort platform frame requires {field}")
    return value


def _required_message_text(frame: dict) -> str:
    value = frame.get("text")
    if not isinstance(value, str) or not value.strip():
        raise FortProtocolError("Fort platform frame requires text")
    return value


def _profile_value(name: str) -> str:
    """Read one value from Hermes' active profile secret scope."""
    try:
        from agent.secret_scope import get_secret

        value = get_secret(name, "")
    except ImportError:
        value = os.getenv(name, "")
    return value if isinstance(value, str) else ""


def _check_requirements() -> bool:
    try:
        import websockets  # noqa: F401
    except ImportError:
        return False
    return True


def _validate_config(config: PlatformConfig) -> bool:
    extra = getattr(config, "extra", {}) or {}
    url = _profile_value("FORT_PLATFORM_URL") or extra.get("url", "")
    token = _profile_value("FORT_PLATFORM_TOKEN")
    return isinstance(url, str) and bool(url.strip() and token.strip())


def _env_enablement() -> Optional[dict]:
    url = _profile_value("FORT_PLATFORM_URL").strip()
    token = _profile_value("FORT_PLATFORM_TOKEN").strip()
    if not (url and token):
        return None
    return {"url": url}


def register(ctx) -> None:
    """Register Fort through Hermes' documented platform-plugin interface."""
    raw_profile_id = ctx.profile_name
    if (
        not isinstance(raw_profile_id, str)
        or not raw_profile_id
        or raw_profile_id.strip() != raw_profile_id
    ):
        raise RuntimeError("Fort platform requires a canonical Hermes profile ID")
    profile_id = raw_profile_id

    missing = object()
    identity = getattr(ctx, "profile_identity", missing)
    if identity is missing:
        raise RuntimeError("Fort platform requires Hermes' public profile identity")
    if callable(identity):
        identity = identity()
    if isinstance(identity, Mapping):
        identity_profile_id = identity.get("profile_id")
        display_name = identity.get("display_name")
    else:
        identity_profile_id = getattr(identity, "profile_id", None)
        display_name = getattr(identity, "display_name", None)
    if identity_profile_id != profile_id:
        raise RuntimeError(
            "Fort platform profile identity does not match the active profile"
        )

    if not isinstance(display_name, str):
        raise RuntimeError("Fort platform requires a textual Hermes display name")
    display_name = display_name.strip()
    if not display_name:
        display_name = profile_id
    if len(display_name) > 64:
        raise RuntimeError("Fort platform Hermes display name exceeds 64 characters")

    ctx.register_platform(
        name="fort",
        label="Fort",
        adapter_factory=lambda cfg: FortPlatformAdapter(
            cfg,
            profile_id=profile_id,
            display_name=display_name,
        ),
        check_fn=_check_requirements,
        validate_config=_validate_config,
        required_env=["FORT_PLATFORM_URL", "FORT_PLATFORM_TOKEN"],
        env_enablement_fn=_env_enablement,
        allowed_users_env="FORT_ALLOWED_USERS",
        allow_all_env="FORT_ALLOW_ALL_USERS",
        max_message_length=4096,
        emoji="🏰",
        platform_hint="You are chatting through Fort.",
    )
