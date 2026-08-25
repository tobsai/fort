from __future__ import annotations

import asyncio
import json
import os
import sys
import unittest
from pathlib import Path
from unittest import mock

import websockets
import yaml


PLUGIN_DIR = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(PLUGIN_DIR))

import adapter as fort_platform
from gateway.config import PlatformConfig


class RecordingPluginContext:
    def __init__(self, profile_name: str, display_name: str):
        self.profile_name = profile_name
        self.profile_identity = {
            "profile_id": profile_name,
            "display_name": display_name,
        }
        self.registration = None

    def register_platform(self, **kwargs):
        self.registration = kwargs
        from gateway.platform_registry import PlatformEntry, platform_registry

        platform_registry.register(PlatformEntry(**kwargs))


class IdentityPluginContext:
    def __init__(self):
        self.profile_name = "writing"
        self.profile_identity = {
            "profile_id": "writing",
            "display_name": "Grace",
        }
        self.registration = None

    def register_platform(self, **kwargs):
        self.registration = kwargs
        from gateway.platform_registry import PlatformEntry, platform_registry

        platform_registry.register(PlatformEntry(**kwargs))


class FortPlatformRegistrationTests(unittest.TestCase):
    def test_package_manifest_declares_a_hermes_platform_plugin(self):
        manifest_path = PLUGIN_DIR / "plugin.yaml"
        package_path = PLUGIN_DIR / "__init__.py"

        manifest = yaml.safe_load(manifest_path.read_text(encoding="utf-8"))

        self.assertTrue(package_path.is_file())
        self.assertEqual(manifest["name"], "fort-platform")
        self.assertEqual(manifest["label"], "Fort")
        self.assertEqual(manifest["kind"], "platform")
        self.assertEqual(
            [entry["name"] for entry in manifest["requires_env"]],
            ["FORT_PLATFORM_URL", "FORT_PLATFORM_TOKEN"],
        )

    def test_registration_captures_public_profile_identity_and_hermes_auth_hooks(self):
        context = RecordingPluginContext("research", "Ada")

        fort_platform.register(context)

        registration = context.registration
        self.assertEqual(registration["name"], "fort")
        self.assertEqual(registration["label"], "Fort")
        self.assertEqual(registration["allowed_users_env"], "FORT_ALLOWED_USERS")
        self.assertEqual(registration["allow_all_env"], "FORT_ALLOW_ALL_USERS")

        old_url = os.environ.get("FORT_PLATFORM_URL")
        old_token = os.environ.get("FORT_PLATFORM_TOKEN")
        try:
            os.environ["FORT_PLATFORM_URL"] = "ws://127.0.0.1:4187/platforms/hermes"
            os.environ["FORT_PLATFORM_TOKEN"] = "test-profile-token"
            instance = registration["adapter_factory"](PlatformConfig(enabled=True))
        finally:
            if old_url is None:
                os.environ.pop("FORT_PLATFORM_URL", None)
            else:
                os.environ["FORT_PLATFORM_URL"] = old_url
            if old_token is None:
                os.environ.pop("FORT_PLATFORM_TOKEN", None)
            else:
                os.environ["FORT_PLATFORM_TOKEN"] = old_token

        self.assertEqual(instance.profile_id, "research")
        self.assertEqual(instance.display_name, "Ada")

    def test_registration_accepts_public_profile_identity_accessor(self):
        context = IdentityPluginContext()

        fort_platform.register(context)

        old_url = os.environ.get("FORT_PLATFORM_URL")
        old_token = os.environ.get("FORT_PLATFORM_TOKEN")
        try:
            os.environ["FORT_PLATFORM_URL"] = "ws://127.0.0.1:4187/platforms/hermes"
            os.environ["FORT_PLATFORM_TOKEN"] = "test-profile-token"
            instance = context.registration["adapter_factory"](
                PlatformConfig(enabled=True)
            )
        finally:
            if old_url is None:
                os.environ.pop("FORT_PLATFORM_URL", None)
            else:
                os.environ["FORT_PLATFORM_URL"] = old_url
            if old_token is None:
                os.environ.pop("FORT_PLATFORM_TOKEN", None)
            else:
                os.environ["FORT_PLATFORM_TOKEN"] = old_token

        self.assertEqual(instance.profile_id, "writing")
        self.assertEqual(instance.display_name, "Grace")

    def test_configuration_reads_the_active_hermes_profile_scope(self):
        from agent.secret_scope import reset_secret_scope, set_secret_scope

        context = RecordingPluginContext("research", "Ada")
        fort_platform.register(context)
        scope_token = set_secret_scope(
            {
                "FORT_PLATFORM_URL": "ws://127.0.0.1:4187/platforms/hermes",
                "FORT_PLATFORM_TOKEN": "research-profile-token",
            }
        )
        try:
            with mock.patch.dict(
                os.environ,
                {"FORT_PLATFORM_URL": "", "FORT_PLATFORM_TOKEN": ""},
                clear=False,
            ):
                self.assertTrue(
                    context.registration["validate_config"](
                        PlatformConfig(enabled=True)
                    )
                )
                self.assertEqual(
                    context.registration["env_enablement_fn"](),
                    {"url": "ws://127.0.0.1:4187/platforms/hermes"},
                )
                instance = context.registration["adapter_factory"](
                    PlatformConfig(enabled=True)
                )
        finally:
            reset_secret_scope(scope_token)

        self.assertEqual(
            instance.url, "ws://127.0.0.1:4187/platforms/hermes"
        )

    def test_configuration_fails_closed_without_a_multiplex_profile_scope(self):
        from agent.secret_scope import (
            UnscopedSecretError,
            is_multiplex_active,
            set_multiplex_active,
        )

        context = RecordingPluginContext("research", "Ada")
        fort_platform.register(context)
        previous_multiplex = is_multiplex_active()
        try:
            set_multiplex_active(True)
            with mock.patch.dict(
                os.environ,
                {
                    "FORT_PLATFORM_URL": "ws://wrong-profile.example/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "wrong-profile-token",
                },
                clear=False,
            ):
                with self.assertRaises(UnscopedSecretError):
                    context.registration["validate_config"](
                        PlatformConfig(enabled=True)
                    )
        finally:
            set_multiplex_active(previous_multiplex)

    def test_registration_fails_without_a_public_profile_name_accessor(self):
        class ContextWithoutPublicName:
            profile_name = "research"
            ui_meta = {"hermes-bots": {"title": "Private UI title"}}

            def register_platform(self, **kwargs):
                raise AssertionError("registration must not occur")

        with self.assertRaisesRegex(RuntimeError, "public profile identity"):
            fort_platform.register(ContextWithoutPublicName())

    def test_registration_rejects_legacy_display_name_without_profile_identity(self):
        class LegacyDisplayNameOnlyContext:
            profile_name = "research"
            profile_display_name = "A possibly stale generic profile name"

            def register_platform(self, **kwargs):
                raise AssertionError("registration must not occur")

        with self.assertRaisesRegex(RuntimeError, "public profile identity"):
            fort_platform.register(LegacyDisplayNameOnlyContext())


class FortPlatformConnectionTests(unittest.IsolatedAsyncioTestCase):
    async def test_connect_authenticates_and_waits_for_registration_ack(self):
        registration_received = asyncio.Event()
        allow_registration_ack = asyncio.Event()
        observed = {}

        async def fort_endpoint(socket):
            observed["authorization"] = socket.request.headers["Authorization"]
            observed["profile"] = socket.request.headers[
                "X-Fort-Hermes-Profile"
            ]
            observed["registration"] = json.loads(await socket.recv())
            registration_received.set()
            await allow_registration_ack.wait()
            await socket.send(
                json.dumps(
                    {
                        "type": "registered",
                        "channel_id": "messaging-channel:hermes:v1:abc123",
                        "conversation_id": "conversation:hermes:research",
                    }
                )
            )
            await socket.wait_closed()

        async with websockets.serve(fort_endpoint, "127.0.0.1", 0) as server:
            from agent.secret_scope import reset_secret_scope, set_secret_scope

            port = server.sockets[0].getsockname()[1]
            context = RecordingPluginContext("research", "Ada")
            fort_platform.register(context)
            scope_token = set_secret_scope(
                {
                    "FORT_PLATFORM_URL": f"ws://127.0.0.1:{port}/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "profile-secret",
                }
            )
            try:
                with mock.patch.dict(
                    os.environ,
                    {
                        "FORT_PLATFORM_URL": "ws://127.0.0.1:1/wrong-profile",
                        "FORT_PLATFORM_TOKEN": "wrong-profile-secret",
                    },
                    clear=False,
                ):
                    instance = context.registration["adapter_factory"](
                        PlatformConfig(enabled=True)
                    )
            finally:
                reset_secret_scope(scope_token)

            connect_task = asyncio.create_task(instance.connect())
            await asyncio.wait_for(registration_received.wait(), timeout=1)
            self.assertFalse(instance.is_connected)
            allow_registration_ack.set()

            self.assertTrue(await asyncio.wait_for(connect_task, timeout=1))
            self.assertTrue(instance.is_connected)
            self.assertEqual(observed["authorization"], "Bearer profile-secret")
            self.assertEqual(observed["profile"], "research")
            self.assertEqual(
                observed["registration"],
                {
                    "type": "register",
                    "contract_version": 1,
                    "profile_id": "research",
                    "display_name": "Ada",
                },
            )
            await instance.disconnect()
            self.assertFalse(instance.is_connected)

    async def test_connect_fails_closed_on_incomplete_registration_ack(self):
        async def fort_endpoint(socket):
            await socket.recv()
            await socket.send(
                json.dumps(
                    {
                        "type": "registered",
                        "channel_id": "messaging-channel:hermes:v1:abc123",
                    }
                )
            )
            await socket.wait_closed()

        async with websockets.serve(fort_endpoint, "127.0.0.1", 0) as server:
            port = server.sockets[0].getsockname()[1]
            context = RecordingPluginContext("research", "Ada")
            fort_platform.register(context)
            with mock.patch.dict(
                os.environ,
                {
                    "FORT_PLATFORM_URL": f"ws://127.0.0.1:{port}/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "profile-secret",
                },
                clear=False,
            ):
                instance = context.registration["adapter_factory"](
                    PlatformConfig(enabled=True)
                )

            self.assertFalse(await instance.connect())
            self.assertFalse(instance.is_connected)
            self.assertEqual(instance.fatal_error_code, "registration_rejected")
            self.assertFalse(instance.fatal_error_retryable)

    async def test_inbound_fort_message_becomes_a_hermes_message_event(self):
        send_inbound = asyncio.Event()

        async def fort_endpoint(socket):
            await socket.recv()
            await socket.send(
                json.dumps(
                    {
                        "type": "registered",
                        "channel_id": "messaging-channel:hermes:v1:abc123",
                        "conversation_id": "conversation:hermes:research",
                    }
                )
            )
            await send_inbound.wait()
            await socket.send(
                json.dumps(
                    {
                        "type": "inbound",
                        "request_id": "fort-request-1",
                        "message_id": "fort-message-1",
                        "conversation_id": "conversation:hermes:research",
                        "text": "  Please summarize this.\n",
                        "author_id": "fort-user-7",
                        "author_name": "Toby",
                    }
                )
            )
            await socket.wait_closed()

        async with websockets.serve(fort_endpoint, "127.0.0.1", 0) as server:
            port = server.sockets[0].getsockname()[1]
            context = RecordingPluginContext("research", "Ada")
            fort_platform.register(context)
            with mock.patch.dict(
                os.environ,
                {
                    "FORT_PLATFORM_URL": f"ws://127.0.0.1:{port}/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "profile-secret",
                },
                clear=False,
            ):
                instance = context.registration["adapter_factory"](
                    PlatformConfig(enabled=True)
                )

            delivered = asyncio.get_running_loop().create_future()

            async def observe_message(event):
                if not delivered.done():
                    delivered.set_result(event)

            instance.set_message_handler(observe_message)
            self.assertTrue(await instance.connect())
            send_inbound.set()

            event = await asyncio.wait_for(delivered, timeout=1)
            self.assertEqual(event.text, "  Please summarize this.\n")
            self.assertEqual(event.message_id, "fort-message-1")
            self.assertEqual(event.source.platform.value, "fort")
            self.assertEqual(event.source.chat_id, "conversation:hermes:research")
            self.assertEqual(event.source.chat_type, "dm")
            self.assertEqual(event.source.user_id, "fort-user-7")
            self.assertEqual(event.source.user_name, "Toby")
            await instance.disconnect()

    async def test_send_waits_for_fort_acceptance_receipt(self):
        outbound_received = asyncio.Event()
        allow_receipt = asyncio.Event()
        observed = {}

        async def fort_endpoint(socket):
            await socket.recv()
            await socket.send(
                json.dumps(
                    {
                        "type": "registered",
                        "channel_id": "messaging-channel:hermes:v1:abc123",
                        "conversation_id": "conversation:hermes:research",
                    }
                )
            )
            observed["outbound"] = json.loads(await socket.recv())
            outbound_received.set()
            await allow_receipt.wait()
            await socket.send(
                json.dumps(
                    {
                        "type": "receipt",
                        "request_id": "hermes-request-1",
                        "message_id": "fort-message-9",
                    }
                )
            )
            await socket.wait_closed()

        async with websockets.serve(fort_endpoint, "127.0.0.1", 0) as server:
            port = server.sockets[0].getsockname()[1]
            context = RecordingPluginContext("research", "Ada")
            fort_platform.register(context)
            with mock.patch.dict(
                os.environ,
                {
                    "FORT_PLATFORM_URL": f"ws://127.0.0.1:{port}/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "profile-secret",
                },
                clear=False,
            ):
                instance = context.registration["adapter_factory"](
                    PlatformConfig(enabled=True)
                )

            self.assertTrue(await instance.connect())
            send_task = asyncio.create_task(
                instance.send(
                    "conversation:hermes:research",
                    "Here is the summary.",
                    reply_to="fort-message-1",
                    metadata={"request_id": "hermes-request-1"},
                )
            )
            await asyncio.wait_for(outbound_received.wait(), timeout=1)
            self.assertFalse(send_task.done())
            self.assertEqual(
                observed["outbound"],
                {
                    "type": "outbound",
                    "request_id": "hermes-request-1",
                    "conversation_id": "conversation:hermes:research",
                    "text": "Here is the summary.",
                    "in_reply_to_message_id": "fort-message-1",
                },
            )
            allow_receipt.set()

            result = await asyncio.wait_for(send_task, timeout=1)
            self.assertTrue(result.success)
            self.assertEqual(result.message_id, "fort-message-9")
            await instance.disconnect()

    async def test_send_does_not_claim_retryable_success_after_ambiguous_close(self):
        outbound_frames = []

        async def fort_endpoint(socket):
            await socket.recv()
            await socket.send(
                json.dumps(
                    {
                        "type": "registered",
                        "channel_id": "messaging-channel:hermes:v1:abc123",
                        "conversation_id": "conversation:hermes:research",
                    }
                )
            )
            outbound_frames.append(json.loads(await socket.recv()))
            await socket.close()

        async with websockets.serve(fort_endpoint, "127.0.0.1", 0) as server:
            port = server.sockets[0].getsockname()[1]
            context = RecordingPluginContext("research", "Ada")
            fort_platform.register(context)
            with mock.patch.dict(
                os.environ,
                {
                    "FORT_PLATFORM_URL": f"ws://127.0.0.1:{port}/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "profile-secret",
                },
                clear=False,
            ):
                instance = context.registration["adapter_factory"](
                    PlatformConfig(enabled=True)
                )

            self.assertTrue(await instance.connect())
            result = await asyncio.wait_for(
                instance.send(
                    "conversation:hermes:research",
                    "An ambiguous response.",
                    metadata={"request_id": "hermes-request-ambiguous"},
                ),
                timeout=1,
            )

            self.assertFalse(result.success)
            self.assertFalse(result.retryable)
            self.assertEqual(result.error, "Fort delivery outcome is unknown")
            self.assertEqual(len(outbound_frames), 1)
            await instance.disconnect()

    async def test_send_surfaces_fort_failure_without_disconnect(self):
        async def fort_endpoint(socket):
            await socket.recv()
            await socket.send(
                json.dumps(
                    {
                        "type": "registered",
                        "channel_id": "messaging-channel:hermes:v1:abc123",
                        "conversation_id": "conversation:hermes:research",
                    }
                )
            )
            await socket.recv()
            await socket.send(
                json.dumps(
                    {
                        "type": "failure",
                        "request_id": "already-finished-request",
                        "code": "stale_request",
                    }
                )
            )
            await socket.send(
                json.dumps(
                    {
                        "type": "failure",
                        "request_id": "hermes-request-denied",
                        "code": "recipient_not_allowed",
                    }
                )
            )
            await socket.wait_closed()

        async with websockets.serve(fort_endpoint, "127.0.0.1", 0) as server:
            port = server.sockets[0].getsockname()[1]
            context = RecordingPluginContext("research", "Ada")
            fort_platform.register(context)
            with mock.patch.dict(
                os.environ,
                {
                    "FORT_PLATFORM_URL": f"ws://127.0.0.1:{port}/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "profile-secret",
                },
                clear=False,
            ):
                instance = context.registration["adapter_factory"](
                    PlatformConfig(enabled=True)
                )

            self.assertTrue(await instance.connect())
            result = await asyncio.wait_for(
                instance.send(
                    "conversation:hermes:research",
                    "A denied response.",
                    metadata={"request_id": "hermes-request-denied"},
                ),
                timeout=1,
            )

            self.assertFalse(result.success)
            self.assertFalse(result.retryable)
            self.assertEqual(
                result.error, "Fort rejected message: recipient_not_allowed"
            )
            self.assertTrue(instance.is_connected)
            await instance.disconnect()

    async def test_fatal_protocol_error_closes_socket_and_clears_registration(self):
        server_observed_close = asyncio.Event()

        async def fort_endpoint(socket):
            await socket.recv()
            await socket.send(
                json.dumps(
                    {
                        "type": "registered",
                        "channel_id": "messaging-channel:hermes:v1:abc123",
                        "conversation_id": "conversation:hermes:research",
                    }
                )
            )
            await socket.send(json.dumps({"type": "unsupported"}))
            await socket.wait_closed()
            server_observed_close.set()

        async with websockets.serve(fort_endpoint, "127.0.0.1", 0) as server:
            port = server.sockets[0].getsockname()[1]
            context = RecordingPluginContext("research", "Ada")
            fort_platform.register(context)
            with mock.patch.dict(
                os.environ,
                {
                    "FORT_PLATFORM_URL": f"ws://127.0.0.1:{port}/platforms/hermes",
                    "FORT_PLATFORM_TOKEN": "profile-secret",
                },
                clear=False,
            ):
                instance = context.registration["adapter_factory"](
                    PlatformConfig(enabled=True)
                )

            self.assertTrue(await instance.connect())
            await asyncio.wait_for(server_observed_close.wait(), timeout=1)

            self.assertFalse(instance.is_connected)
            self.assertIsNone(instance._ws)
            self.assertEqual(instance._channel_id, "")
            self.assertEqual(instance._conversation_id, "")
            self.assertEqual(instance.fatal_error_code, "protocol_error")


if __name__ == "__main__":
    unittest.main()
