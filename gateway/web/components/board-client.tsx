"use client";

// BoardClient — the browser side of the E2E tunnel (spec 028, the crux of the
// web app). What it does, precisely, for v1:
//
//   • TOFU-pins the daemon's static key: on first visit it stores the locally
//     computed key fingerprint in localStorage (keyed by machine_id); on later
//     visits it compares — a CHANGED key raises a loud warning and refuses to
//     connect (defeats a gateway that swaps in its own key to MITM).
//   • Runs the Noise IK initiator handshake through /api/req, deriving a session.
//   • Seals GET /api/summary for a NATIVE status view (proves a sealed JSON
//     round-trip end to end).
//   • "Open board" seals GET / and renders the returned board HTML into a
//     sandboxed srcdoc iframe — a STATIC snapshot (scripts disabled), not a
//     fully-interactive proxied board (see the deferred note in SETUP/report).
//   • "Tail events" seals GET /api/events (SSE) through /api/sse and appends the
//     decoded stream — proves the sealed streaming path end to end.
//
// Everything past decodeBase64(publicKey) is opaque to the gateway: the app's
// /api routes and the worker only ever move Noise/AEAD frames.

import { decodeBase64, fingerprint } from "@fort/gateway-shared";
import { useEffect, useMemo, useRef, useState } from "react";

import { RelayClient } from "@/lib/relay-client";

interface Summary {
  total: number;
  running: number;
  queued: number;
  blocked: number;
  succeeded: number;
  failed: number;
  execution: boolean;
}

type PinState = "checking" | "first" | "pinned" | "mismatch";

const utf8 = new TextDecoder();

export default function BoardClient({
  machineId,
  name,
  publicKey,
  serverFingerprint,
  online,
}: {
  machineId: string;
  name: string;
  publicKey: string;
  serverFingerprint: string;
  online: boolean;
}) {
  // The daemon static key + its fingerprint, computed HERE from the raw key so
  // the pin is on the exact key we will handshake against — not on whatever the
  // gateway claims the fingerprint is.
  const daemonPub = useMemo(() => decodeBase64(publicKey), [publicKey]);
  const localFp = useMemo(() => fingerprint(daemonPub), [daemonPub]);
  const keyMatchesServer = localFp === serverFingerprint;

  const [pinState, setPinState] = useState<PinState>("checking");
  const [pinnedFp, setPinnedFp] = useState<string | null>(null);

  const [summary, setSummary] = useState<Summary | null>(null);
  const [boardHtml, setBoardHtml] = useState<string | null>(null);
  const [tail, setTail] = useState<string>("");
  const [busy, setBusy] = useState<null | "status" | "board" | "tail">(null);
  const [error, setError] = useState<string | null>(null);
  const tailAbort = useRef<AbortController | null>(null);

  // TOFU pin on mount.
  useEffect(() => {
    const key = `fort.pin.${machineId}`;
    let stored: string | null = null;
    try {
      stored = localStorage.getItem(key);
    } catch {
      stored = null;
    }
    if (stored === null) {
      try {
        localStorage.setItem(key, localFp);
      } catch {
        /* private mode: proceed unpinned */
      }
      setPinnedFp(localFp);
      setPinState("first");
    } else if (stored === localFp) {
      setPinnedFp(stored);
      setPinState("pinned");
    } else {
      setPinnedFp(stored);
      setPinState("mismatch");
    }
    return () => tailAbort.current?.abort();
  }, [machineId, localFp]);

  const blocked = pinState === "mismatch";

  async function fetchStatus() {
    setBusy("status");
    setError(null);
    try {
      const client = new RelayClient(machineId, daemonPub);
      await client.connect();
      const res = await client.fetch("/api/summary");
      if (res.status !== 200) throw new Error(`daemon returned ${res.status}`);
      setSummary(JSON.parse(utf8.decode(res.body ?? new Uint8Array())) as Summary);
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed");
    } finally {
      setBusy(null);
    }
  }

  async function openBoard() {
    setBusy("board");
    setError(null);
    try {
      const client = new RelayClient(machineId, daemonPub);
      await client.connect();
      const res = await client.fetch("/");
      if (res.status !== 200) throw new Error(`daemon returned ${res.status}`);
      setBoardHtml(utf8.decode(res.body ?? new Uint8Array()));
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed");
    } finally {
      setBusy(null);
    }
  }

  async function tailEvents() {
    if (tailAbort.current) {
      tailAbort.current.abort();
      tailAbort.current = null;
      setBusy(null);
      return;
    }
    setBusy("tail");
    setError(null);
    setTail("");
    const ctrl = new AbortController();
    tailAbort.current = ctrl;
    try {
      const client = new RelayClient(machineId, daemonPub);
      await client.connect();
      await client.stream(
        "/api/events?since=0",
        (chunk) => {
          if (chunk.data && chunk.data.length > 0) {
            setTail((t) => (t + utf8.decode(chunk.data!)).slice(-8000));
          }
        },
        ctrl.signal,
      );
    } catch (e) {
      if (!ctrl.signal.aborted) setError(e instanceof Error ? e.message : "stream failed");
    } finally {
      if (tailAbort.current === ctrl) tailAbort.current = null;
      setBusy(null);
    }
  }

  return (
    <div>
      {/* --- key pin state ---------------------------------------------------- */}
      {blocked ? (
        <div className="warn-banner" role="alert">
          <h3>⚠ Daemon key changed — refusing to connect</h3>
          <p>
            The static key for <strong>{name}</strong> does not match the one this browser pinned
            on first connect. A compromised gateway may be trying to impersonate the machine, or the
            machine was re-joined with a fresh identity.
          </p>
          <p className="fingerprint">pinned: {pinnedFp}</p>
          <p className="fingerprint">now: {localFp}</p>
          <p className="hint">
            If you deliberately re-joined this machine, clear the pin to trust the new key:
          </p>
          <button
            className="btn"
            onClick={() => {
              try {
                localStorage.setItem(`fort.pin.${machineId}`, localFp);
              } catch {
                /* ignore */
              }
              setPinnedFp(localFp);
              setPinState("pinned");
            }}
          >
            Trust the new key
          </button>
        </div>
      ) : (
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <div>
              <div className="hint">daemon key fingerprint (verify against `fort relay join`)</div>
              <div className="fingerprint">{localFp}</div>
            </div>
            <span className="pill">
              {pinState === "first" ? "pinned (first use)" : pinState === "pinned" ? "pin ok" : "…"}
            </span>
          </div>
          {!keyMatchesServer ? (
            <p className="err" style={{ marginTop: 8 }}>
              Note: the gateway&apos;s reported fingerprint ({serverFingerprint}) differs from the
              one computed here from the key. The locally computed value above is authoritative.
            </p>
          ) : null}
        </div>
      )}

      {/* --- actions ---------------------------------------------------------- */}
      <div className="card">
        <div className="row">
          <button className="btn btn-primary" onClick={fetchStatus} disabled={!!busy || blocked}>
            {busy === "status" ? "Connecting…" : "Fetch status (sealed)"}
          </button>
          <button className="btn" onClick={openBoard} disabled={!!busy || blocked}>
            {busy === "board" ? "Loading…" : "Open board (sealed GET /)"}
          </button>
          <button className="btn" onClick={tailEvents} disabled={(!!busy && busy !== "tail") || blocked}>
            {busy === "tail" ? "Stop tail" : "Tail events (sealed SSE)"}
          </button>
        </div>
        {!online ? (
          <p className="hint" style={{ marginTop: 8 }}>
            This machine is currently offline; requests will fail until `fort serve` reconnects.
          </p>
        ) : null}
        {error ? (
          <p className="err" style={{ marginTop: 8 }}>
            {error}
          </p>
        ) : null}
      </div>

      {/* --- native status view ---------------------------------------------- */}
      {summary ? (
        <div className="card">
          <div className="hint">
            board summary {summary.execution ? "(execution plane attached)" : "(control only)"}
          </div>
          <div className="stat-grid">
            <div className="stat">
              <div className="n">{summary.total}</div>
              <div className="l">total</div>
            </div>
            <div className="stat">
              <div className="n">{summary.running}</div>
              <div className="l">running</div>
            </div>
            <div className="stat">
              <div className="n">{summary.queued}</div>
              <div className="l">queued</div>
            </div>
            <div className="stat">
              <div className="n">{summary.blocked}</div>
              <div className="l">blocked</div>
            </div>
            <div className="stat">
              <div className="n">{summary.succeeded}</div>
              <div className="l">succeeded</div>
            </div>
            <div className="stat">
              <div className="n">{summary.failed}</div>
              <div className="l">failed</div>
            </div>
          </div>
        </div>
      ) : null}

      {/* --- board snapshot --------------------------------------------------- */}
      {boardHtml !== null ? (
        <div className="card">
          <div className="hint" style={{ marginBottom: 8 }}>
            Board snapshot (static — scripts disabled inside the sandbox; open a native client for a
            fully interactive board).
          </div>
          <iframe className="board-frame" sandbox="" srcDoc={boardHtml} title={`${name} board`} />
        </div>
      ) : null}

      {/* --- sealed event tail ------------------------------------------------ */}
      {tail ? (
        <div className="card">
          <div className="hint" style={{ marginBottom: 8 }}>
            Live events (sealed SSE over the tunnel)
          </div>
          <pre className="tail">{tail}</pre>
        </div>
      ) : null}
    </div>
  );
}
