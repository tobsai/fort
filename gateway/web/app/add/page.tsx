"use client";

// /add — Add machine. A button POSTs to /api/invite (which calls the worker's
// invite endpoint server-side) and shows the paste-ready `fort relay join`
// line for the machine's terminal. Single-use, short-TTL code.

import Link from "next/link";
import { useState } from "react";

export default function AddMachinePage() {
  const [command, setCommand] = useState<string | null>(null);
  const [code, setCode] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function mint() {
    setBusy(true);
    setErr(null);
    setCopied(false);
    try {
      const res = await fetch("/api/invite", { method: "POST" });
      if (!res.ok) throw new Error(`invite failed (${res.status})`);
      const body = (await res.json()) as { code: string; command: string };
      setCode(body.code);
      setCommand(body.command);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "invite failed");
    } finally {
      setBusy(false);
    }
  }

  async function copy() {
    if (!command) return;
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div>
      <h1>Add machine</h1>
      <p className="subtitle">
        Mint a single-use join code, then run the line below on the machine.
      </p>

      <div className="card">
        <button className="btn btn-primary" onClick={mint} disabled={busy}>
          {busy ? "Minting…" : command ? "Mint another code" : "Generate join code"}
        </button>
        {err ? <p className="err">{err}</p> : null}

        {command ? (
          <>
            <p className="hint" style={{ marginTop: 16 }}>
              Run this in the machine&apos;s terminal (code expires shortly):
            </p>
            <div className="mono-block">{command}</div>
            <div className="row" style={{ marginTop: 10 }}>
              <button className="btn" onClick={copy}>
                {copied ? "Copied ✓" : "Copy"}
              </button>
              {code ? <span className="pill">code {code}</span> : null}
            </div>
            <p className="hint" style={{ marginTop: 12 }}>
              After it joins, verify the printed fingerprint matches the one on the{" "}
              <Link href="/">Machines</Link> page.
            </p>
          </>
        ) : null}
      </div>
    </div>
  );
}
