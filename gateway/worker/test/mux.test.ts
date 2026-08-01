// Unit tests for the pure multiplexing logic (no workerd runtime needed). These
// pin the (stream, kind) routing, the buffered relay <-> subscribe behaviours,
// and the fail-fast on daemon disconnect.

import { describe, expect, it } from "vitest";

import type { Frame } from "@fort/gateway-shared";
import { Multiplexer, OfflineError, TimeoutError, relayTimeoutMs, replyExpected } from "../src/mux";

const f = (stream: string, kind: string, b64 = "x"): Frame => ({ stream, kind, b64 });

describe("Multiplexer", () => {
  it("relay(hs1) resolves with the daemon's next frame on that stream", async () => {
    const sent: Frame[] = [];
    const mux = new Multiplexer((fr) => sent.push(fr));

    const p = mux.relay(f("s1", "hs1"), { expectReply: true, timeoutMs: 1000 });
    expect(sent).toEqual([f("s1", "hs1")]); // forwarded to daemon
    mux.deliver(f("s1", "hs2", "reply"));
    expect(await p).toEqual(f("s1", "hs2", "reply"));
  });

  it("routes strictly by stream id, never by payload", async () => {
    const mux = new Multiplexer(() => {});
    const p1 = mux.relay(f("a", "req"), { expectReply: true, timeoutMs: 1000 });
    const p2 = mux.relay(f("b", "req"), { expectReply: true, timeoutMs: 1000 });
    mux.deliver(f("b", "res", "for-b"));
    mux.deliver(f("a", "res", "for-a"));
    expect(await p1).toEqual(f("a", "res", "for-a"));
    expect(await p2).toEqual(f("b", "res", "for-b"));
  });

  it("passes the sealed b64 through byte-for-byte (opaque relay)", async () => {
    const sent: Frame[] = [];
    const mux = new Multiplexer((fr) => sent.push(fr));
    const sealed = "c2VhbGVkLWNpcGhlcnRleHQ=";
    const p = mux.relay(f("s", "req", sealed), { expectReply: true, timeoutMs: 1000 });
    expect(sent[0]!.b64).toBe(sealed); // forwarded unmodified
    mux.deliver(f("s", "res", "cmVzcG9uc2U="));
    expect((await p)!.b64).toBe("cmVzcG9uc2U="); // returned unmodified
  });

  it("end/bye are fire-and-forget (no reply awaited)", async () => {
    const sent: Frame[] = [];
    const mux = new Multiplexer((fr) => sent.push(fr));
    expect(await mux.relay(f("s", "end"), { expectReply: false, timeoutMs: 1000 })).toBeNull();
    expect(await mux.relay(f("s", "bye"), { expectReply: false, timeoutMs: 1000 })).toBeNull();
    expect(sent.map((x) => x.kind)).toEqual(["end", "bye"]);
  });

  it("a reply that arrives before the relay registers is still delivered", async () => {
    // Models a daemon that replies synchronously inside send().
    const mux = new Multiplexer((fr) => {
      if (fr.kind === "hs1") mux.deliver(f(fr.stream, "hs2", "fast"));
    });
    const reply = await mux.relay(f("s", "hs1"), { expectReply: true, timeoutMs: 1000 });
    expect(reply).toEqual(f("s", "hs2", "fast"));
  });

  it("subscribe streams every daemon frame on the stream in order", () => {
    const mux = new Multiplexer(() => {});
    const got: Frame[] = [];
    let ended = false;
    mux.subscribe("s", { push: (fr) => got.push(fr), end: () => (ended = true) });
    mux.deliver(f("s", "res", "1"));
    mux.deliver(f("s", "chunk", "2"));
    mux.deliver(f("s", "chunk", "3"));
    expect(got.map((x) => x.b64)).toEqual(["1", "2", "3"]);
    expect(ended).toBe(false);
    mux.unsubscribe("s");
  });

  it("subscribe flushes frames that were parked before it attached", () => {
    const mux = new Multiplexer(() => {});
    mux.deliver(f("s", "res", "early"));
    const got: Frame[] = [];
    mux.subscribe("s", { push: (fr) => got.push(fr), end: () => {} });
    expect(got.map((x) => x.b64)).toEqual(["early"]);
  });

  it("fail() rejects in-flight relays and ends subscriptions", async () => {
    const mux = new Multiplexer(() => {});
    const p = mux.relay(f("s", "req"), { expectReply: true, timeoutMs: 5000 });
    let ended = false;
    mux.subscribe("s2", { push: () => {}, end: () => (ended = true) });
    mux.fail();
    await expect(p).rejects.toBeInstanceOf(OfflineError);
    expect(ended).toBe(true);
    expect(mux.isOnline()).toBe(false);
    await expect(mux.relay(f("s", "req"), { expectReply: true, timeoutMs: 5000 })).rejects.toBeInstanceOf(
      OfflineError,
    );
  });

  it("relay times out if the daemon never answers", async () => {
    const mux = new Multiplexer(() => {});
    await expect(mux.relay(f("s", "req"), { expectReply: true, timeoutMs: 10 })).rejects.toBeInstanceOf(
      TimeoutError,
    );
  });
});

describe("replyExpected", () => {
  it("only hs1 and req expect a single daemon reply", () => {
    expect(replyExpected("hs1")).toBe(true);
    expect(replyExpected("req")).toBe(true);
    expect(replyExpected("end")).toBe(false);
    expect(replyExpected("bye")).toBe(false);
    expect(replyExpected("hs2")).toBe(false);
  });

  it("keeps handshakes bounded while allowing a durable application ack to clear cold-start work", () => {
    expect(relayTimeoutMs("hs1")).toBe(10_000);
    expect(relayTimeoutMs("req")).toBe(30_000);
    expect(relayTimeoutMs("bye")).toBe(10_000);
  });
});
