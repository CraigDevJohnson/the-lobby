import assert from "node:assert/strict";
import { test } from "node:test";
import { STATES } from "./logic.ts";
import { artFor, cueFor, hash, lit, VARIANTS, variants, waiting } from "./plot.ts";

test("a plot is lit only while its world is ready", () => {
  for (const state of STATES) assert.equal(lit(state), state === "ready", state);
});

test("the three worlds have their own drawings, and any other gets the block", () => {
  assert.equal(artFor("survival"), "camp");
  assert.equal(artFor("creative"), "build");
  assert.equal(artFor("adventure"), "gate");
  assert.equal(artFor("skyblock"), "block");
  // The one server of a backend that lists no worlds.
  assert.equal(artFor(undefined), "block");
  // An id is never read as a property the registry merely inherits.
  assert.equal(artFor("constructor"), "block");
  assert.equal(artFor("toString"), "block");
});

test("pressing Start strikes, and only a start that ends ready wakes the plot", () => {
  assert.equal(cueFor("stopped", "starting"), "strike");
  assert.equal(cueFor("failed", "starting"), "strike");
  assert.equal(cueFor("starting", "ready"), "wake");
  assert.equal(cueFor("starting", "failed"), "fizzle");
});

test("ready by any other road only turns the light on", () => {
  for (const before of STATES) {
    if (before === "ready" || before === "starting") continue;
    assert.equal(cueFor(before, "ready"), "light", before);
  }
});

test("stopping packs up from ready and rests once stopped", () => {
  assert.equal(cueFor("ready", "stopping"), "pack");
  assert.equal(cueFor("stopping", "stopped"), "rest");
});

test("a change the page has no road for cuts to the new picture", () => {
  assert.equal(cueFor("ready", "ready"), null);
  assert.equal(cueFor("ready", "stopped"), null);
  assert.equal(cueFor("ready", "unknown"), null);
  assert.equal(cueFor("unknown", "starting"), null);
  assert.equal(cueFor("unknown", "stopped"), null);
  assert.equal(cueFor("starting", "stopped"), null);
  assert.equal(cueFor("stopped", "maintenance"), null);
  assert.equal(cueFor("stopped", "failed"), null);
  for (const before of STATES) {
    assert.equal(cueFor(before, "unknown"), null, before);
    assert.equal(cueFor(before, "maintenance"), null, before);
  }
});

test("no move ever plays into a state it does not show", () => {
  // Every move that lights something ends at ready, and every move away from
  // ready puts the light out.
  for (const before of STATES) {
    for (const after of STATES) {
      const cue = cueFor(before, after);
      if (cue === "wake" || cue === "light") assert.equal(after, "ready");
      if (cue === "pack") assert.equal(before, "ready");
    }
  }
});

test("a status answer gets a beat only while a world is on its way", () => {
  for (const state of STATES) assert.equal(waiting(state), state === "starting" || state === "stopping", state);
});

test("an undrawn world's variant comes from its id and never changes", () => {
  assert.equal(VARIANTS.length, 12);
  assert.equal(new Set(VARIANTS.map((v) => `${v.flag} ${v.pole} ${v.slots}`)).size, 12);
  const once = variants(["skyblock"]).get("skyblock");
  assert.ok(once);
  for (let i = 0; i < 5; i++) assert.deepEqual(variants(["skyblock"]).get("skyblock"), once);
  assert.deepEqual(once, VARIANTS[hash("skyblock") % VARIANTS.length]);
});

test("drawn worlds and the one unnamed server take no variant", () => {
  const got = variants(["survival", "creative", "adventure", undefined, "skyblock"]);
  assert.deepEqual([...got.keys()], ["skyblock"]);
});

test("undrawn worlds on one page never share a flag while there are flags left", () => {
  // Ids chosen so several start from the same variant.
  const ids = Array.from({ length: 400 }, (_, i) => `w${i}`);
  const clash = ids.filter((id) => hash(id) % VARIANTS.length === hash(ids[0]) % VARIANTS.length).slice(0, 5);
  assert.equal(clash.length, 5);
  const got = variants(clash);
  assert.equal(new Set([...got.values()]).size, 5);
  // The first keeps its own; a list's order is the backend's, so it is stable.
  assert.deepEqual(got.get(clash[0]), VARIANTS[hash(clash[0]) % VARIANTS.length]);

  const twelve = variants(ids.slice(0, 12));
  assert.equal(new Set([...twelve.values()]).size, 12);
  // Past twelve there are no flags left to be distinct with; nothing breaks.
  assert.equal(variants(ids.slice(0, 30)).size, 30);
});

test("the ids spread across every variant", () => {
  const seen = new Set<number>();
  for (let i = 0; i < 1000; i++) seen.add(hash(`world-${i}`) % VARIANTS.length);
  assert.equal(seen.size, VARIANTS.length);
});
