"use client";

// The authenticated remote Command Deck. All Fort data is decrypted in this
// browser after a Noise IK handshake with the pinned daemon key. Vercel and the
// Worker continue to see only opaque relay frames (specs 028 and 037).

import { decodeBase64, fingerprint } from "@fort/gateway-shared";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { CommandDeckSurface } from "@/components/command-deck-surface";
import { PlaybooksSurface } from "@/components/playbooks-surface";
import {
  chatRequestForRoute,
  DeckLoadGate,
  DeckOperationGate,
  displayAgent,
  meshReachability,
  relayIdentityTrusted,
  relativeAge,
  routePreviewMatchesDraft,
  shouldRefreshPlaybookCatalog,
  type DeckBacklogItem,
  type DeckGate,
  type DeckOperationLease,
  type DeckPayload,
  type DeckPinState,
  type DeckPlaybook,
  type DeckRoutePreview,
} from "@/lib/command-deck";
import { RelayClient, type FetchOptions } from "@/lib/relay-client";

type DeckView = "deck" | "playbooks" | "snapshot" | "activity";
type BusyState = "deck" | "playbooks" | "snapshot" | "mutation" | "direction" | null;
type DeckLoadMode = "foreground" | "poll" | "force";

interface PinVerification {
  identity: string;
  state: DeckPinState;
  pinnedFp: string | null;
}

const utf8 = new TextDecoder();
const utf8enc = new TextEncoder();

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
  const daemonPub = useMemo(() => decodeBase64(publicKey), [publicKey]);
  const localFp = useMemo(() => fingerprint(daemonPub), [daemonPub]);
  const identity = `${machineId}:${localFp}`;
  const keyMatchesServer = localFp === serverFingerprint;

  const [pin, setPin] = useState<PinVerification>({
    identity: "",
    state: "checking",
    pinnedFp: null,
  });
  const [deck, setDeck] = useState<DeckPayload | null>(null);
  const [deckLoading, setDeckLoading] = useState(false);
  const [deckLoadGate] = useState(() => new DeckLoadGate());
  const [operationGate] = useState(
    () => new DeckOperationGate<NonNullable<BusyState>>(),
  );
  const [connected, setConnected] = useState(false);
  const [view, setView] = useState<DeckView>("deck");
  const [boardHtml, setBoardHtml] = useState<string | null>(null);
  const [snapshotTime, setSnapshotTime] = useState<Date | null>(null);
  const [tail, setTail] = useState("");
  const [tailing, setTailing] = useState(false);
  const [busy, setBusy] = useState<BusyState>(null);
  const [deckError, setDeckError] = useState<string | null>(null);
  const [operationError, setOperationError] = useState<string | null>(null);
  const [tailError, setTailError] = useState<string | null>(null);
  const [lastLoaded, setLastLoaded] = useState<Date | null>(null);
  const [composerOpen, setComposerOpen] = useState(false);
  const [direction, setDirection] = useState("");
  const [planGate, setPlanGate] = useState(true);
  const [routePreview, setRoutePreview] = useState<DeckRoutePreview | null>(null);
  const [routePreviewDraft, setRoutePreviewDraft] = useState<{
    text: string;
    planGate: boolean;
  } | null>(null);
  const [playbooks, setPlaybooks] = useState<DeckPlaybook[]>([]);
  const [playbooksLoading, setPlaybooksLoading] = useState(false);
  const [selectedPlaybookID, setSelectedPlaybookID] = useState("");
  const [notice, setNotice] = useState<string | null>(null);
  const tailAbort = useRef<AbortController | null>(null);
  const autoLoaded = useRef("");
  const loadGeneration = useRef(0);
  const directionRevision = useRef(0);
  const trustedIdentity = useRef("");

  const pinState: DeckPinState = pin.identity === identity ? pin.state : "checking";
  const pinnedFp = pin.identity === identity ? pin.pinnedFp : null;
  const trusted = relayIdentityTrusted(identity, pin.identity, pinState);
  trustedIdentity.current = trusted ? identity : "";
  const blocked = pinState === "mismatch";

  const beginOperation = useCallback(
    (operation: NonNullable<BusyState>) => {
      const lease = operationGate.begin(operation);
      if (!lease) return null;
      setBusy(operation);
      return lease;
    },
    [operationGate],
  );

  const endOperation = useCallback(
    (lease: DeckOperationLease<NonNullable<BusyState>>) => {
      if (!operationGate.end(lease)) return;
      setBusy((current) => (current === lease.operation ? null : current));
    },
    [operationGate],
  );

  const loadDeck = useCallback(
    async (mode: DeckLoadMode = "foreground") => {
      const requestIdentity = identity;
      if (trustedIdentity.current !== requestIdentity) return;
      const foreground = mode === "foreground";
      const finishLoad = deckLoadGate.begin(mode === "poll");
      if (!finishLoad) return;
      const operationLease = foreground ? beginOperation("deck") : null;
      if (foreground && !operationLease) {
        finishLoad();
        return;
      }
      const generation = ++loadGeneration.current;
      setDeckLoading(true);
      if (foreground) setDeck(null);
      let client: RelayClient | null = null;
      try {
        client = new RelayClient(machineId, daemonPub);
        await client.connect();
        requireTrustedIdentity(trustedIdentity, requestIdentity);
        const summaryRes = await client.fetch("/api/summary");
        requireTrustedIdentity(trustedIdentity, requestIdentity);
        const boardRes = await client.fetch("/api/board");
        requireTrustedIdentity(trustedIdentity, requestIdentity);
        const backlogRes = await client.fetch("/api/backlog");
        requireTrustedIdentity(trustedIdentity, requestIdentity);
        const machinesRes = await client.fetch("/api/machines");
        requireTrustedIdentity(trustedIdentity, requestIdentity);
        ensureOK(summaryRes.status, "/api/summary");
        ensureOK(boardRes.status, "/api/board");
        ensureOK(backlogRes.status, "/api/backlog");
        ensureOK(machinesRes.status, "/api/machines");
        const nextDeck: DeckPayload = {
          summary: decodeJSON<DeckPayload["summary"]>(summaryRes.body),
          board: decodeJSON<DeckPayload["board"]>(boardRes.body),
          backlog: decodeJSON<DeckPayload["backlog"]>(backlogRes.body),
          machines: decodeJSON<DeckPayload["machines"]>(machinesRes.body),
        };
        if (loadGeneration.current !== generation) return;
        requireTrustedIdentity(trustedIdentity, requestIdentity);
        setDeck(nextDeck);
        setConnected(true);
        setDeckError(null);
        setLastLoaded(new Date());
      } catch (cause) {
        if (loadGeneration.current === generation && trustedIdentity.current === requestIdentity) {
          setConnected(false);
          setDeckError(message(cause));
        }
      } finally {
        await client?.close();
        finishLoad();
        if (loadGeneration.current === generation) setDeckLoading(false);
        if (operationLease) endOperation(operationLease);
      }
    },
    [beginOperation, daemonPub, deckLoadGate, endOperation, identity, machineId],
  );

  useEffect(() => {
    const key = `fort.pin.${machineId}`;
    setDeck(null);
    setDeckLoading(false);
    setBoardHtml(null);
    setTail("");
    setTailing(false);
    setDeckError(null);
    setOperationError(null);
    setTailError(null);
    setNotice(null);
    setComposerOpen(false);
    setDirection("");
    setRoutePreview(null);
    setRoutePreviewDraft(null);
    directionRevision.current += 1;
    setPlaybooks([]);
    setPlaybooksLoading(false);
    setSelectedPlaybookID("");
    setLastLoaded(null);
    setSnapshotTime(null);
    setConnected(false);
    operationGate.reset();
    setBusy(null);
    autoLoaded.current = "";
    loadGeneration.current += 1;

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
        // Private mode can proceed for this session without durable pinning.
      }
      setPin({ identity, state: "first", pinnedFp: localFp });
    } else if (stored === localFp) {
      setPin({ identity, state: "pinned", pinnedFp: stored });
    } else {
      setPin({ identity, state: "mismatch", pinnedFp: stored });
    }
    return () => tailAbort.current?.abort();
  }, [identity, localFp, machineId, operationGate]);

  useEffect(() => {
    if (!trusted) return;
    const loadKey = identity;
    if (autoLoaded.current !== loadKey) {
      autoLoaded.current = loadKey;
      void loadDeck();
    }
    const timer = window.setInterval(() => void loadDeck("poll"), 15_000);
    return () => window.clearInterval(timer);
  }, [identity, loadDeck, trusted]);

  async function openSnapshot() {
    const operationLease = beginOperation("snapshot");
    if (!operationLease) return;
    const requestIdentity = identity;
    setOperationError(null);
    setBoardHtml(null);
    try {
      const response = await sealedFetch("/");
      ensureOK(response.status, "/");
      setBoardHtml(utf8.decode(response.body ?? new Uint8Array()));
      setSnapshotTime(new Date());
    } catch (cause) {
      if (trustedIdentity.current === requestIdentity) setOperationError(message(cause));
    } finally {
      endOperation(operationLease);
    }
  }

  async function sealedFetch(path: string, options: FetchOptions = {}) {
    const requestIdentity = identity;
    requireTrustedIdentity(trustedIdentity, requestIdentity);
    const client = new RelayClient(machineId, daemonPub);
    try {
      await client.connect();
      requireTrustedIdentity(trustedIdentity, requestIdentity);
      const response = await client.fetch(path, options);
      requireTrustedIdentity(trustedIdentity, requestIdentity);
      return response;
    } finally {
      await client.close();
    }
  }

  function acceptPlaybookCatalog(items: DeckPlaybook[]) {
    setPlaybooks(items);
    setSelectedPlaybookID((current) => {
      if (items.some((playbook) => playbook.id === current)) return current;
      return items.find((playbook) => playbook.is_default)?.id ?? items[0]?.id ?? "";
    });
  }

  async function loadPlaybooks() {
    const operationLease = beginOperation("playbooks");
    if (!operationLease) return;
    const requestIdentity = identity;
    setPlaybooksLoading(true);
    setOperationError(null);
    try {
      const response = await sealedFetch("/api/playbooks");
      ensureOK(response.status, "/api/playbooks");
      if (trustedIdentity.current !== requestIdentity) return;
      acceptPlaybookCatalog(decodeJSON<DeckPlaybook[]>(response.body));
      setConnected(true);
    } catch (cause) {
      if (trustedIdentity.current === requestIdentity) setOperationError(message(cause));
    } finally {
      if (trustedIdentity.current === requestIdentity) setPlaybooksLoading(false);
      endOperation(operationLease);
    }
  }

  async function savePlaybook(playbook: DeckPlaybook) {
    const operationLease = beginOperation("playbooks");
    if (!operationLease) return;
    const requestIdentity = identity;
    setOperationError(null);
    setNotice(null);
    try {
      const response = await sealedFetch("/api/playbooks", {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: utf8enc.encode(JSON.stringify(playbook)),
      });
      if (response.status === 409) {
        const latestResponse = await sealedFetch("/api/playbooks");
        ensureOK(latestResponse.status, "/api/playbooks");
        if (trustedIdentity.current === requestIdentity) {
          acceptPlaybookCatalog(decodeJSON<DeckPlaybook[]>(latestResponse.body));
        }
        throw new Error("This playbook changed in another edit. Fort reloaded the latest revision.");
      }
      ensureOK(response.status, "/api/playbooks");
      const saved = decodeJSON<DeckPlaybook>(response.body);
      if (trustedIdentity.current !== requestIdentity) return;
      setPlaybooks((current) => {
        const found = current.some((item) => item.id === saved.id);
        return found
          ? current.map((item) => (item.id === saved.id ? saved : item))
          : [...current, saved];
      });
      setSelectedPlaybookID((current) => current || saved.id);
      setNotice(`${saved.name} saved as revision ${saved.revision}.`);
    } catch (cause) {
      if (trustedIdentity.current === requestIdentity) setOperationError(message(cause));
    } finally {
      endOperation(operationLease);
    }
  }

  async function duplicatePlaybook(id: string) {
    const operationLease = beginOperation("playbooks");
    if (!operationLease) return;
    const requestIdentity = identity;
    setOperationError(null);
    setNotice(null);
    const path = `/api/playbooks/${encodeURIComponent(id)}/duplicate`;
    try {
      const response = await sealedFetch(path, { method: "POST" });
      ensureOK(response.status, path);
      const copy = decodeJSON<DeckPlaybook>(response.body);
      if (trustedIdentity.current !== requestIdentity) return;
      setPlaybooks((current) => [...current.filter((item) => item.id !== copy.id), copy]);
      setSelectedPlaybookID(copy.id);
      setNotice(`${copy.name} created.`);
    } catch (cause) {
      if (trustedIdentity.current === requestIdentity) setOperationError(message(cause));
    } finally {
      endOperation(operationLease);
    }
  }

  async function mutate(path: string, body?: unknown) {
    const operationLease = beginOperation("mutation");
    if (!operationLease) {
      throw new Error("Another Fort action is still finishing.");
    }
    const requestIdentity = identity;
    setOperationError(null);
    setNotice(null);
    try {
      const response = await sealedFetch(path, {
        method: "POST",
        headers: { "content-type": "application/json" },
        ...(body === undefined ? {} : { body: utf8enc.encode(JSON.stringify(body)) }),
      });
      ensureOK(response.status, path);
      await loadDeck("force");
      return response;
    } catch (cause) {
      if (trustedIdentity.current === requestIdentity) setOperationError(message(cause));
      throw cause;
    } finally {
      endOperation(operationLease);
    }
  }

  async function decide(gate: DeckGate, decision: "approve" | "reject") {
    const requestIdentity = identity;
    let note: string | undefined;
    if (decision === "reject") {
      const entered = window.prompt("What should change before this checkpoint comes back?");
      if (entered === null) return;
      note = entered.trim();
      if (!note) return;
    }
    try {
      await mutate("/api/gate", {
        run_id: gate.run_id,
        node_id: gate.node_id,
        decision,
        ...(note ? { note } : {}),
      });
      if (trustedIdentity.current === requestIdentity) {
        setNotice(
          decision === "approve" ? "Checkpoint accepted." : "Direction sent back to the crew.",
        );
      }
    } catch {
      // The shared error card already explains the failure.
    }
  }

  async function dispatch(item: DeckBacklogItem) {
    const requestIdentity = identity;
    try {
      await mutate(`/api/backlog/${encodeURIComponent(item.id)}/dispatch`);
      if (trustedIdentity.current === requestIdentity) setNotice(`${item.title} started.`);
    } catch {
      // The shared error card already explains the failure.
    }
  }

  async function previewDirection(selected?: DeckPlaybook) {
    const text = direction.trim();
    if (!text) return;
    const previewRevision = directionRevision.current;
    const previewPlanGate = planGate;
    const operationLease = beginOperation("direction");
    if (!operationLease) return;
    const requestIdentity = identity;
    setOperationError(null);
    setNotice(null);
    setRoutePreview(null);
    setRoutePreviewDraft(null);
    try {
      const routeResponse = await sealedFetch("/api/route", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: utf8enc.encode(
          JSON.stringify({
            text,
            ...(selected
              ? {
                  playbook_id: selected.id,
                  playbook_revision: selected.revision,
                  plan_gate: selected.delivery === "answer" ? false : previewPlanGate,
                }
              : {}),
          }),
        ),
      });
      ensureOK(routeResponse.status, "/api/route");
      let nextPlaybooks = playbooks;
      if (shouldRefreshPlaybookCatalog(selected !== undefined, nextPlaybooks.length)) {
        const playbooksResponse = await sealedFetch("/api/playbooks");
        ensureOK(playbooksResponse.status, "/api/playbooks");
        nextPlaybooks = decodeJSON<DeckPlaybook[]>(playbooksResponse.body);
      }
      if (
        trustedIdentity.current === requestIdentity &&
        directionRevision.current === previewRevision
      ) {
        setRoutePreview(decodeJSON<DeckRoutePreview>(routeResponse.body));
        setRoutePreviewDraft({ text, planGate: previewPlanGate });
        acceptPlaybookCatalog(nextPlaybooks);
      }
    } catch (cause) {
      if (trustedIdentity.current === requestIdentity) setOperationError(message(cause));
    } finally {
      endOperation(operationLease);
    }
  }

  async function handOff() {
    const text = direction.trim();
    const route = routePreview;
    const previewDraft = routePreviewDraft;
    if (
      !text ||
      !route ||
      !previewDraft ||
      !routePreviewMatchesDraft(previewDraft.text, previewDraft.planGate, text, planGate)
    ) {
      setRoutePreview(null);
      setRoutePreviewDraft(null);
      setOperationError("The direction changed. Preview its route again before handoff.");
      return;
    }
    const operationLease = beginOperation("direction");
    if (!operationLease) return;
    const requestIdentity = identity;
    setOperationError(null);
    setNotice(null);
    try {
      const response = await sealedFetch("/api/chat", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: utf8enc.encode(JSON.stringify(chatRequestForRoute(text, route, planGate))),
      });
      ensureOK(response.status, "/api/chat");
      const result = decodeJSON<{ kind?: string; answer?: string; queued?: boolean }>(response.body);
      setDirection("");
      setRoutePreview(null);
      setRoutePreviewDraft(null);
      setComposerOpen(false);
      setNotice(
        result.kind === "answer" && result.answer
          ? result.answer
          : result.queued
            ? "Added to Up next."
            : "Direction handed off.",
      );
      await loadDeck("force");
    } catch (cause) {
      if (trustedIdentity.current === requestIdentity) setOperationError(message(cause));
    } finally {
      endOperation(operationLease);
    }
  }

  async function tailEvents() {
    if (tailAbort.current) {
      tailAbort.current.abort();
      tailAbort.current = null;
      setTailing(false);
      return;
    }
    setTailing(true);
    setTailError(null);
    setTail("");
    const controller = new AbortController();
    tailAbort.current = controller;
    let client: RelayClient | null = null;
    try {
      const requestIdentity = identity;
      requireTrustedIdentity(trustedIdentity, requestIdentity);
      client = new RelayClient(machineId, daemonPub);
      await client.connect();
      requireTrustedIdentity(trustedIdentity, requestIdentity);
      await client.stream(
        "/api/events?since=0",
        (chunk) => {
          if (trustedIdentity.current !== requestIdentity) {
            controller.abort();
            return;
          }
          if (chunk.data && chunk.data.length > 0) {
            setTail((current) => (current + utf8.decode(chunk.data!)).slice(-8000));
          }
        },
        controller.signal,
      );
    } catch (cause) {
      if (!controller.signal.aborted) setTailError(message(cause));
    } finally {
      await client?.close();
      if (tailAbort.current === controller) {
        tailAbort.current = null;
        setTailing(false);
      }
    }
  }

  const activeError = operationError ?? deckError;

  return (
    <section className="gateway-command-deck">
      <div className="deck-toolbar">
        <div className="deck-machine">
          <span className={`status-dot ${connected ? "accepted" : "idle"}`} />
          <div>
            <strong>All machines</strong>
            <span>
              {connected
                ? meshReachability(deck?.machines ?? [])
                : online
                  ? "broker online — verifying"
                  : "offline — retrying"}
            </span>
          </div>
        </div>
        <nav className="deck-tabs" aria-label="Remote Fort views" role="tablist">
          <button
            className={view === "deck" ? "active" : ""}
            onClick={() => setView("deck")}
            disabled={!!busy}
            role="tab"
            aria-selected={view === "deck"}
          >
            Deck
          </button>
          <button
            className={view === "playbooks" ? "active" : ""}
            onClick={() => {
              if (busy) return;
              setView("playbooks");
              void loadPlaybooks();
            }}
            disabled={!!busy || !trusted}
            role="tab"
            aria-selected={view === "playbooks"}
          >
            Playbooks
          </button>
          <button
            className={view === "snapshot" ? "active" : ""}
            onClick={() => {
              if (busy) return;
              setView("snapshot");
              if (boardHtml === null && trusted) void openSnapshot();
            }}
            disabled={!!busy}
            role="tab"
            aria-selected={view === "snapshot"}
          >
            Snapshot
          </button>
          <button
            className={view === "activity" ? "active" : ""}
            onClick={() => setView("activity")}
            disabled={!!busy}
            role="tab"
            aria-selected={view === "activity"}
          >
            Activity
          </button>
        </nav>
        <div className="deck-toolbar-actions">
          {lastLoaded ? <span className="last-sync">synced {relativeAge(lastLoaded.toISOString())}</span> : null}
          <button
            className="btn btn-secondary"
            onClick={() => void (view === "playbooks" ? loadPlaybooks() : loadDeck())}
            disabled={!!busy || !trusted}
          >
            Refresh
          </button>
          <button className="btn btn-primary" onClick={() => setComposerOpen((open) => !open)} disabled={!!busy || !trusted}>
            Give direction
          </button>
        </div>
      </div>

      {blocked ? (
        <div className="attention-banner" role="alert">
          <div>
            <strong>Daemon key changed — connection stopped</strong>
            <p>
              The key for {name} no longer matches this browser&apos;s pin. Verify the new fingerprint
              on the machine before trusting it.
            </p>
            <span className="fingerprint">pinned {pinnedFp}</span>
            <span className="fingerprint">current {localFp}</span>
          </div>
          <button
            className="btn btn-secondary"
            onClick={() => {
              try {
                localStorage.setItem(`fort.pin.${machineId}`, localFp);
              } catch {
                // Keep the in-memory trust decision even in private mode.
              }
              setPin({ identity, state: "pinned", pinnedFp: localFp });
            }}
          >
            Trust verified key
          </button>
        </div>
      ) : null}

      {!keyMatchesServer && !blocked ? (
        <div className="attention-banner compact" role="alert">
          The broker&apos;s displayed fingerprint differs from the key computed in this browser. The
          locally computed fingerprint remains authoritative.
        </div>
      ) : null}

      {composerOpen ? (
        <div className="direction-panel">
          <div>
            <span className="section-label">GIVE DIRECTION</span>
            <h2>Name the outcome; Fort handles the machinery.</h2>
          </div>
          <textarea
            value={direction}
            onChange={(event) => {
              directionRevision.current += 1;
              setDirection(event.target.value);
              setRoutePreview(null);
              setRoutePreviewDraft(null);
            }}
            placeholder="Describe the outcome you want — like briefing an employee."
            aria-label="Direction for Fort"
            disabled={busy === "direction"}
            autoFocus
          />
          {routePreview ? (
            <div className="route-preview">
              <div className="route-preview-heading">
                <span className="section-label">CONFIRM ROUTE</span>
                <span className="route-source">{routePreview.source}</span>
              </div>
              <div className="route-title-row">
                <strong>{routePreview.playbook_name}</strong>
                <span>{routePreview.delivery === "answer" ? "direct answer" : "assignment"}</span>
              </div>
              <label className="route-switcher">
                <span>Change route</span>
                <select
                  value={routePreview.playbook_id}
                  disabled={!!busy}
                  onChange={(event) => {
                    const selected = playbooks.find(
                      (playbook) => playbook.id === event.target.value,
                    );
                    if (selected) void previewDirection(selected);
                  }}
                >
                  {playbooks.map((playbook) => (
                    <option value={playbook.id} key={playbook.id}>
                      {playbook.name}
                    </option>
                  ))}
                </select>
              </label>
              <div className="route-stages">
                {routePreview.stages.map((stage, index) => (
                  <div className="route-stage" key={`${stage.order}/${stage.name}`}>
                    <span>{stage.order}</span>
                    <div>
                      <strong>{stage.name}</strong>
                      <small>
                        {displayAgent(stage.agent)}
                        {stage.model ? ` · ${stage.model}` : ""}
                      </small>
                    </div>
                    {index < routePreview.stages.length - 1 ? <i aria-hidden="true">to</i> : null}
                  </div>
                ))}
              </div>
              <div className="route-foot">
                <span>
                  {routePreview.delivery === "answer"
                    ? "Replies here without creating an assignment."
                    : planGate
                      ? "Plan first — you sign off before work starts."
                      : "Starts immediately after confirmation."}
                </span>
                <button
                  className="link-btn"
                  disabled={busy === "direction"}
                  onClick={() => {
                    setRoutePreview(null);
                    setRoutePreviewDraft(null);
                  }}
                >
                  Change direction
                </button>
              </div>
            </div>
          ) : null}
          <div className="direction-actions">
            <label className="plan-toggle">
              <input
                type="checkbox"
                checked={planGate}
                disabled={busy === "direction"}
                onChange={(event) => {
                  directionRevision.current += 1;
                  setPlanGate(event.target.checked);
                  setRoutePreview(null);
                  setRoutePreviewDraft(null);
                }}
              />
              Propose a plan first — I&apos;ll sign off before work starts
            </label>
            <button
              className="btn btn-primary"
              onClick={() => void (routePreview ? handOff() : previewDirection())}
              disabled={!!busy || !direction.trim() || !trusted}
            >
              {busy === "direction"
                ? routePreview
                  ? "Handing off…"
                  : "Resolving route…"
                : routePreview
                  ? "Hand it off"
                  : "Preview route"}
            </button>
          </div>
        </div>
      ) : null}

      {notice ? (
        <div className="notice" role="status">
          <span>{notice}</span>
          <button onClick={() => setNotice(null)} aria-label="Dismiss notice">
            Dismiss
          </button>
        </div>
      ) : null}

      {activeError ? (
        <div className="error-card" role="alert">
          <strong>{operationError ? "Fort could not finish that action." : "Fort could not refresh this view."}</strong>
          <span>
            {activeError}
            {!operationError && deck && lastLoaded
              ? ` Showing the deck synced ${relativeAge(lastLoaded.toISOString())}.`
              : ""}
          </span>
          {operationError ? (
            <button className="btn btn-secondary" onClick={() => setOperationError(null)}>
              Dismiss
            </button>
          ) : (
            <button className="btn btn-secondary" onClick={() => void loadDeck()} disabled={!!busy || !trusted}>
              Try again
            </button>
          )}
        </div>
      ) : null}

      {view === "deck" ? (
        <CommandDeckSurface
          online={connected}
          brokerOnline={online}
          deck={deck}
          loading={deckLoading}
          busy={!!busy}
          onDecide={(gate, decision) => void decide(gate, decision)}
          onDispatch={(item) => void dispatch(item)}
        />
      ) : null}

      {view === "playbooks" ? (
        <PlaybooksSurface
          playbooks={playbooks}
          selectedID={selectedPlaybookID}
          loading={playbooksLoading}
          busy={!!busy}
          onSelect={setSelectedPlaybookID}
          onReload={() => void loadPlaybooks()}
          onSave={(playbook) => void savePlaybook(playbook)}
          onDuplicate={(id) => void duplicatePlaybook(id)}
        />
      ) : null}

      {view === "snapshot" ? (
        <div className="diagnostic-view">
          <div className="diagnostic-header">
            <div>
              <span className="section-label">FULL BOARD SNAPSHOT</span>
              <p>
                Decrypted directly from {name}. Scripts stay disabled inside this safety sandbox.
              </p>
            </div>
            <div className="diagnostic-actions">
              {snapshotTime ? <span>fetched {relativeAge(snapshotTime.toISOString())}</span> : null}
              <button className="btn btn-secondary" onClick={() => void openSnapshot()} disabled={!!busy || !trusted}>
                {busy === "snapshot" ? "Refreshing…" : boardHtml ? "Refresh snapshot" : "Open snapshot"}
              </button>
            </div>
          </div>
          {boardHtml !== null ? (
            <iframe className="board-frame" sandbox="" srcDoc={boardHtml} title={`${name} board`} />
          ) : (
            <EmptyCard text={busy === "snapshot" ? "Fetching a fresh sealed snapshot…" : "No snapshot loaded."} />
          )}
        </div>
      ) : null}

      {view === "activity" ? (
        <div className="diagnostic-view">
          <div className="diagnostic-header">
            <div>
              <span className="section-label">LIVE ACTIVITY</span>
              <p>Encrypted event frames decoded only in this browser.</p>
            </div>
            <button className="btn btn-secondary" onClick={() => void tailEvents()} disabled={!trusted}>
              {tailing ? "Stop tail" : "Tail events"}
            </button>
          </div>
          {tailError ? (
            <div className="error-card tail-error" role="alert">
              <strong>Activity stream stopped.</strong>
              <span>{tailError}</span>
              <button className="btn btn-secondary" onClick={() => setTailError(null)}>
                Dismiss
              </button>
            </div>
          ) : null}
          {tail ? <pre className="tail">{tail}</pre> : <EmptyCard text="Start the tail to watch live Fort events." />}
        </div>
      ) : null}

      <details className="connection-details">
        <summary>Connection details</summary>
        <div>
          <span>Relay daemon</span>
          <code>{name}</code>
          <span>Daemon fingerprint</span>
          <code>{localFp}</code>
          <span>Gateway record</span>
          <code>{serverFingerprint}</code>
          <span>Pin state</span>
          <code>{pinState}</code>
          <span>Connected mesh</span>
          <code>
            {deck?.machines
              .map((machine) => `${machine.name} (${machine.reachable ? "reachable" : "offline"})`)
              .join(", ") || "not loaded"}
          </code>
        </div>
      </details>
    </section>
  );
}

function EmptyCard({ text }: { text: string }) {
  return <div className="empty-card">{text}</div>;
}

function decodeJSON<T>(body: Uint8Array | undefined): T {
  return JSON.parse(utf8.decode(body ?? new Uint8Array())) as T;
}

function ensureOK(status: number, path: string) {
  if (status < 200 || status >= 300) throw new Error(`${path} returned ${status}`);
}

function message(cause: unknown): string {
  return cause instanceof Error ? cause.message : "The sealed request failed.";
}

function requireTrustedIdentity(trustedIdentity: { current: string }, expected: string) {
  if (trustedIdentity.current !== expected) {
    throw new Error("Verify this Fort's daemon key before opening a sealed connection.");
  }
}
