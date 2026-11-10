import assert from "node:assert/strict";
import { test } from "node:test";
import {
  allUnconfirmed,
  announcement,
  canStart,
  canStop,
  describe,
  hasDetails,
  javaAddress,
  lead,
  names,
  pageTitle,
  parseRefusal,
  parseStatus,
  parseWorlds,
  pollDelay,
  refusalMessage,
  segments,
  unconfirmed,
  UNNAMED,
  worldKey,
  type World,
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

test("parseRefusal keeps the backend's reason and drops the player count", () => {
  assert.deepEqual(parseRefusal({ code: "players_online", players: 2 }), { code: "players_online" });
  assert.deepEqual(parseRefusal({ code: "players_online" }), { code: "players_online" });
  assert.deepEqual(parseRefusal({ code: "players_unknown" }), { code: "players_unknown" });
  assert.deepEqual(parseRefusal({ code: "starting", players: 3 }), { code: "starting" });
  assert.deepEqual(parseRefusal({ code: "unknown_world" }), { code: "unknown_world" });
  assert.equal(parseRefusal({ code: "maintenance" }), null);
  assert.equal(parseRefusal({ code: "no_server" }), null);
  assert.equal(parseRefusal(null), null);
});

test("a refusal says what is in the way, never how many are playing", () => {
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

test("lead says what to do now, and admits when there is no address to join with", () => {
  const conn = { java: { address: "mc.example.net" } };
  assert.equal(lead(null), "Start the server, then join from Minecraft.");
  assert.equal(lead([{ name: "s", state: "stopped", connection: conn }]), "Start the server, then join from Minecraft.");
  assert.equal(
    lead([{ name: "s", state: "ready", connection: conn }]),
    "The server is up. Copy the address and join from Minecraft.",
  );
  assert.equal(
    lead([{ name: "s", state: "ready", connection: {} }]),
    "The server is up, but there’s no address to join with yet.",
  );
  assert.equal(hasDetails({}), false);
  assert.equal(hasDetails({ bedrock: { address: "mc.example.net", port: 19132 } }), true);
});

test("describe never points at details that aren't there", () => {
  const ready = describe({ name: "s", state: "ready", connection: {} });
  assert.match(ready.message, /nothing to copy/);
  assert.doesNotMatch(ready.message, /details here/);
});

test("pageTitle puts the server's state first", () => {
  assert.equal(pageTitle(null), "Minecraft Launcher · The Lobby");
  assert.equal(
    pageTitle([{ name: "s", state: "ready", connection: {} }]),
    "Ready to join · Minecraft Launcher · The Lobby",
  );
});

test("segments break a value only after its dots and colons, and lose nothing", () => {
  assert.deepEqual(segments("play.example.net"), ["play.", "example.", "net"]);
  assert.deepEqual(segments("a-very-long-name.example.net"), ["a-very-long-name.", "example.", "net"]);
  for (const v of ["[2001:db8:85a3::8a2e:370:7334]:25570", "192.0.2.10", "19132", "mc.example.net.", ""]) {
    assert.equal(segments(v).join(""), v);
  }
});

// Three worlds as the backend lists them (docs/minecraft-backend.md).
const conn = (host: string) => ({ java: { address: host }, bedrock: { address: host, port: 19132 } });
const listed = {
  name: "Survival",
  state: "stopped",
  connection: conn("survival.mc.example.net"),
  worlds: [
    { id: "survival", name: "Survival", state: "stopped", connection: conn("survival.mc.example.net") },
    { id: "creative", name: "Creative", state: "ready", connection: conn("creative.mc.example.net") },
    { id: "adventure", name: "Adventure", state: "starting", connection: conn("adventure.mc.example.net") },
  ],
};

test("parseWorlds keeps every world, in the backend's order, with its own address", () => {
  const worlds = parseWorlds(listed)!;
  assert.deepEqual(
    worlds.map((w) => [w.id, w.name, w.state, w.connection.java?.address]),
    [
      ["survival", "Survival", "stopped", "survival.mc.example.net"],
      ["creative", "Creative", "ready", "creative.mc.example.net"],
      ["adventure", "Adventure", "starting", "adventure.mc.example.net"],
    ],
  );
  assert.deepEqual(worlds.map(worldKey), ["survival", "creative", "adventure"]);
});

test("without a worlds list the page shows the one server, with no id", () => {
  const worlds = parseWorlds({ name: "Family server", state: "ready", connection: conn("mc.example.net") })!;
  assert.equal(worlds.length, 1);
  assert.equal(worlds[0].id, undefined);
  assert.equal(worlds[0].name, "Family server");
  assert.equal(worldKey(worlds[0]), "");
  assert.equal(parseWorlds({ state: "running" }), null);
  assert.equal(parseWorlds(null), null);
});

test("parseWorlds leaves out a world it could not start or stop, and never guesses a state", () => {
  const worlds = parseWorlds({
    ...listed,
    worlds: [
      { id: "Creative!", name: "Bad id", state: "ready" },
      { name: "No id", state: "ready" },
      { id: "survival", name: "Survival", state: "running" },
      { id: "survival", name: "Twice", state: "ready" },
      "creative",
      { id: "creative", name: "Creative", state: "ready" },
    ],
  })!;
  assert.deepEqual(
    worlds.map((w) => [w.id, w.name, w.state]),
    [
      ["survival", "Survival", "unknown"],
      ["creative", "Creative", "ready"],
    ],
  );
  // A list with nothing usable falls back to the top-level status, which
  // repeats the first world.
  const fallback = parseWorlds({ ...listed, worlds: [{ id: "BAD", state: "ready" }] })!;
  assert.deepEqual(
    fallback.map((w) => [w.id, w.name]),
    [[undefined, "Survival"]],
  );
  assert.deepEqual(parseWorlds({ ...listed, worlds: [] })!.length, 1);
});

test("a world's card speaks of the world by name, the one server as the server", () => {
  const ready: World = { id: "creative", name: "Creative", state: "failed", connection: {} };
  assert.equal(describe(ready, "Creative").message, "Creative didn’t start. You can try starting it again.");
  assert.equal(describe(ready).message, "The server didn’t start. You can try starting it again.");
  const unknown = unconfirmed({ ...ready, state: "stopped" });
  assert.equal(
    describe(unknown, "Creative").message,
    "We can’t confirm Creative’s status. Last known: stopped, not confirmed now.",
  );
  assert.equal(announcement("starting", { ...ready, state: "ready" }, "Creative"), "Creative is ready to join.");
  assert.equal(announcement("ready", { ...ready, state: "stopped" }, "Creative"), "Creative has stopped.");
  assert.equal(
    refusalMessage({ code: "players_online" }, "Creative"),
    "Someone is still playing in Creative. Ask them to leave first.",
  );
  assert.match(refusalMessage({ code: "unknown_world" }), /doesn’t have this world any more/);
});

test("the page asks often while any world is on its way somewhere", () => {
  const worlds = parseWorlds(listed)!;
  assert.equal(pollDelay(worlds), 5_000);
  assert.equal(pollDelay(worlds.filter((w) => w.state !== "starting")), 30_000);
  assert.equal(pollDelay([{ ...worlds[0], state: "stopping" }]), 5_000);
});

test("with several worlds, the lead and the tab title name the ones that are up", () => {
  const worlds = parseWorlds(listed)!;
  assert.equal(lead(worlds), "Creative is up. Copy its address and join from Minecraft.");
  assert.equal(pageTitle(worlds), "Creative is ready · Minecraft Launcher · The Lobby");
  const twoUp = worlds.map((w) => (w.id === "survival" ? { ...w, state: "ready" as const } : w));
  assert.equal(lead(twoUp), "Survival and Creative are up. Copy an address and join from Minecraft.");
  assert.equal(pageTitle(twoUp), "Survival and Creative are ready · Minecraft Launcher · The Lobby");
  const noneUp = worlds.map((w) => (w.id === "creative" ? { ...w, state: "stopped" as const } : w));
  assert.equal(lead(noneUp), "Get set up to join while Adventure starts.");
  assert.equal(pageTitle(noneUp), "Adventure starting · Minecraft Launcher · The Lobby");
  const allStopped = worlds.map((w) => ({ ...w, state: "stopped" as const }));
  assert.equal(lead(allStopped), "Each world runs on its own. Start the one you want, then join it from Minecraft.");
  assert.equal(pageTitle(allStopped), "Minecraft Launcher · The Lobby");
  assert.equal(lead(allUnconfirmed(worlds)), "Check the worlds’ status before starting one or joining.");
});

test("allUnconfirmed keeps every world and its last confirmed state", () => {
  const worlds = allUnconfirmed(parseWorlds(listed));
  assert.deepEqual(
    worlds.map((w) => [w.id, w.state, w.lastKnown]),
    [
      ["survival", "unknown", "stopped"],
      ["creative", "unknown", "ready"],
      ["adventure", "unknown", "starting"],
    ],
  );
  assert.deepEqual(allUnconfirmed(null), [{ name: UNNAMED, state: "unknown", connection: {} }]);
});

test("names lists worlds in prose", () => {
  assert.equal(names([]), "");
  assert.equal(names([{ name: "Creative" }]), "Creative");
  assert.equal(names([{ name: "Survival" }, { name: "Creative" }]), "Survival and Creative");
  assert.equal(names([{ name: "Survival" }, { name: "Creative" }, { name: "Adventure" }]), "Survival, Creative and Adventure");
});
