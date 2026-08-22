"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

const cursorStorageKey = "fort.v2.events.cursor";

// Keeps a rendered chat projection in step with Fort's durable ledger. The
// browser reconnects EventSource with Last-Event-ID; session storage preserves
// that cursor when navigation creates a new stream.
export default function DurableChatRefresh() {
  const router = useRouter();

  useEffect(() => {
    const cursor = storedCursor();
    const source = new EventSource(`/api/v2/events?cursor=${encodeURIComponent(cursor)}`);
    let refreshTimer: ReturnType<typeof setTimeout> | null = null;

    const remember = (event: Event) => {
      if (event instanceof MessageEvent && validCursor(event.lastEventId)) {
        try {
          sessionStorage.setItem(cursorStorageKey, event.lastEventId);
        } catch {
          // A blocked browser storage policy must not disable live chat.
        }
      }
    };
    const refresh = (event: Event) => {
      remember(event);
      if (refreshTimer !== null) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        router.refresh();
      }, 100);
    };

    source.addEventListener("fort.event", refresh);
    source.addEventListener("fort.reconnect", remember);
    return () => {
      source.removeEventListener("fort.event", refresh);
      source.removeEventListener("fort.reconnect", remember);
      source.close();
      if (refreshTimer !== null) clearTimeout(refreshTimer);
    };
  }, [router]);

  return null;
}

function storedCursor(): string {
  try {
    const cursor = sessionStorage.getItem(cursorStorageKey) ?? "cursor-0";
    return validCursor(cursor) ? cursor : "cursor-0";
  } catch {
    return "cursor-0";
  }
}

function validCursor(value: string): boolean {
  return /^cursor-(0|[1-9][0-9]{0,18})$/.test(value);
}
