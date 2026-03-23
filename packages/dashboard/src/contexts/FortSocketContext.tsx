import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  useCallback,
  type ReactNode,
} from "react";
import type { FortState, WSMessage } from "../types";

type Handler = (msg: WSMessage) => void;

interface FortSocketContextValue {
  connected: boolean;
  send: (type: string, payload?: unknown) => void;
  subscribe: (type: string, handler: Handler) => () => void;
  state: FortState | null;
}

const FortSocketContext = createContext<FortSocketContextValue | null>(null);

let idCounter = 0;
function nextId(): string {
  return `msg-${Date.now()}-${++idCounter}`;
}

export function FortSocketProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false);
  const [state, setState] = useState<FortState | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const subscribersRef = useRef(new Map<string, Set<Handler>>());
  const reconnectRef = useRef<ReturnType<typeof setTimeout>>();

  const dispatch = useCallback((msg: WSMessage) => {
    // Update cached state
    if (msg.type === "state" || msg.type === "status.response") {
      setState(msg.payload as FortState);
    }
    // Notify subscribers
    const handlers = subscribersRef.current.get(msg.type);
    if (handlers) {
      for (const h of handlers) h(msg);
    }
    // Also notify wildcard subscribers
    const wildcards = subscribersRef.current.get("*");
    if (wildcards) {
      for (const h of wildcards) h(msg);
    }
  }, []);

  const connect = useCallback(() => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl =
      import.meta.env.VITE_WS_URL || `${protocol}//${window.location.host}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      clearTimeout(reconnectRef.current);
      // Request initial data
      ws.send(JSON.stringify({ id: nextId(), type: "status" }));
      ws.send(JSON.stringify({ id: nextId(), type: "agents" }));
      ws.send(JSON.stringify({ id: nextId(), type: "tasks" }));
    };

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data) as WSMessage;
        dispatch(msg);
      } catch {
        /* ignore parse errors */
      }
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      reconnectRef.current = setTimeout(connect, 3000);
    };

    ws.onerror = () => ws.close();
  }, [dispatch]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectRef.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const send = useCallback((type: string, payload?: unknown) => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ id: nextId(), type, payload }));
    }
  }, []);

  const subscribe = useCallback((type: string, handler: Handler) => {
    const subs = subscribersRef.current;
    if (!subs.has(type)) subs.set(type, new Set());
    subs.get(type)!.add(handler);
    return () => {
      subs.get(type)?.delete(handler);
    };
  }, []);

  return (
    <FortSocketContext.Provider value={{ connected, send, subscribe, state }}>
      {children}
    </FortSocketContext.Provider>
  );
}

export function useFortSocket(): FortSocketContextValue {
  const ctx = useContext(FortSocketContext);
  if (!ctx) throw new Error("useFortSocket must be used within FortSocketProvider");
  return ctx;
}
