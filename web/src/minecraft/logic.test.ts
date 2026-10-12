import assert from "node:assert/strict";
import { test } from "node:test";
import {
  announcement,
  canStart,
  canStop,
  describe,
  javaAddress,
  parseRefusal,
  parseStatus,
  refusalMessage,
  unconfirmed,
  UNNAMED,
} from "./logic.ts";

test("parseStatus keeps what the backend sent and nothing else", () => {
  const s = parseStatus({
    name: " Family server ",
    state: "ready",
    connection: { java: { address: "mc.example.net" }, bedrock: { address: "mc.example.net", port: 19132 } },
  });
  assert.deepEqual(s, {
    name: "Family server",
    state: "ready",
    connection: { java: { address: "mc.example.net" }, bedrock: { address: "mc.example.net", port: 19132 } },
  });
});

test("parseStatus refuses an answer without a known state", () => {
  assert.equal(parseStatus({ name: "Family server", state: "running" }), null);
  assert.equal(parseStatus(null), null);
  assert.equal(parseStatus("ready"), null);
});

test("parseStatus drops connection values it can't use rather than guessing", () => {
  const s = parseStatus({
    name: "",
    state: "stopped",
    connection: { java: { address: "  " }, bedrock: { address: "mc.example.net" } },
  });
  assert.deepEqual(s, { name: UNNAMED, state: "stopped", connection: {} });
  const badPort = parseStatus({ state: "stopped", connection: { java: { address: "mc.example.net", port: 70000 } } });
  assert.deepEqual(badPort?.connection, { java: { address: "mc.example.net" } });
});

test("a last-known state is kept only beside unknown", () => {
  assert.equal(parseStatus({ state: "unknown", last_known: "ready" })?.lastKnown, "ready");
  assert.equal(parseStatus({ state: "stopped", last_known: "ready" })?.lastKnown, undefined);
  assert.equal(parseStatus({ state: "unknown", last_known: "unknown" })?.lastKnown, undefined);
});

test("unconfirmed keeps the last confirmed state, labelled as such", () => {
  const ready = parseStatus({
    name: "Family server",
    state: "ready",
    connection: { java: { address: "mc.example.net" } },
  })!;
  const u = unconfirmed(ready);
  assert.equal(u.state, "unknown");
  assert.equal(u.lastKnown, "ready");
  assert.equal(u.connection.java?.address, "mc.example.net");
  assert.match(describe(u).message, /Last known: ready to join, not confirmed now/);
  // Twice in a row still points at the last confirmed state.
  assert.equal(unconfirmed(u).lastKnown, "ready");
  assert.deepEqual(unconfirmed(null), { name: UNNAMED, state: "unknown", connection: {} });
});

test("javaAddress adds a port only when there is one", () => {
  assert.equal(javaAddress({ address: "mc.example.net" }), "mc.example.net");
  assert.equal(javaAddress({ address: "mc.example.net", port: 25570 }), "mc.example.net:25570");
  assert.equal(javaAddress({ address: "2001:db8::1", port: 25570 }), "[2001:db8::1]:25570");
});

test("only a stopped or failed server offers Start", () => {
  assert.deepEqual(
    (["stopped", "starting", "ready", "stopping", "maintenance", "failed", "unknown"] as const).filter(canStart),
    ["stopped", "failed"],
  );
});

test("only a ready server offers Stop", () => {
  assert.deepEqual(
    (["stopped", "starting", "ready", "stopping", "maintenance", "failed", "unknown"] as const).filter(canStop),
    ["ready"],
  );
});

test("parseRefusal keeps the backend's reason and a player count it can use", () => {
  assert.deepEqual(parseRefusal({ code: "players_online", players: 2 }), { code: "players_online", players: 2 });
  assert.deepEqual(parseRefusal({ code: "players_online", players: 0 }), { code: "players_online" });
  assert.deepEqual(parseRefusal({ code: "players_online", players: "2" }), { code: "players_online" });
  assert.deepEqual(parseRefusal({ code: "players_unknown" }), { code: "players_unknown" });
  assert.deepEqual(parseRefusal({ code: "starting", players: 3 }), { code: "starting" });
  assert.equal(parseRefusal({ code: "maintenance" }), null);
  assert.equal(parseRefusal({ code: "no_server" }), null);
  assert.equal(parseRefusal(null), null);
});

test("a refusal says who is in the way, counting players naturally", () => {
  assert.equal(
    refusalMessage({ code: "players_online", players: 2 }),
    "Someone is still playing. 2 players are online. Ask them to leave first.",
  );
  assert.equal(
    refusalMessage({ code: "players_online", players: 1 }),
    "Someone is still playing. 1 player is online. Ask them to leave first.",
  );
  assert.equal(refusalMessage({ code: "players_online" }), "Someone is still playing. Ask them to leave first.");
  assert.equal(
    refusalMessage({ code: "players_unknown" }),
    "The server could not be asked who is playing. Try again in a moment.",
  );
  assert.equal(refusalMessage({ code: "starting" }), "The server is still starting. Wait for it to be ready.");
});

test("announcements speak changes only", () => {
  const ready = parseStatus({ state: "ready" })!;
  assert.equal(announcement("starting", ready), "The server is ready to join.");
  assert.equal(announcement("ready", ready), "");
  assert.equal(announcement(undefined, parseStatus({ state: "stopped" })!), "");
});
