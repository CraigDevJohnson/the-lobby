import assert from "node:assert/strict";
import { test } from "node:test";
import { findMods, modGrid, parseMods, switched, unswitched } from "./mods.ts";

const fawe = {
  id: "fawe",
  name: "FastAsyncWorldEdit",
  version: "2.16.2",
  summary: "Bulk building tools: copy, paste, fill and brushes.",
  bedrock: "Commands and the wand work. Command suggestions do not show.",
  default: true,
  enabled: false,
};

test("parseMods keeps each world's mods in the backend's order", () => {
  const worlds = parseMods({
    worlds: [
      { id: "survival", name: "Survival", mods: [{ ...fawe, id: "gsit", name: "GSit", enabled: true }] },
      { id: "creative", name: "Creative", mods: [fawe] },
    ],
  })!;
  assert.deepEqual(
    worlds.map((w) => [w.id, w.name, w.mods.map((m) => [m.id, m.enabled, m.default])]),
    [
      ["survival", "Survival", [["gsit", true, true]]],
      ["creative", "Creative", [["fawe", false, true]]],
    ],
  );
  assert.deepEqual(worlds[1].mods[0], fawe);
});

test("parseMods leaves out what it can't read rather than guessing", () => {
  const worlds = parseMods({
    worlds: [
      { id: "Creative", name: "Bad id", mods: [fawe] },
      { name: "No id", mods: [fawe] },
      {
        id: "creative",
        name: "",
        mods: [
          { ...fawe, enabled: "false" },
          { ...fawe, default: undefined },
          { ...fawe, id: "../fawe" },
          fawe,
          { ...fawe, name: "Twice" },
        ],
      },
      { id: "creative", name: "Twice", mods: [] },
      { id: "adventure", name: "Adventure", mods: "none" },
    ],
  })!;
  assert.deepEqual(
    worlds.map((w) => [w.id, w.name, w.mods.map((m) => m.name)]),
    [
      ["creative", "creative", ["FastAsyncWorldEdit"]],
      ["adventure", "Adventure", []],
    ],
  );
  assert.equal(parseMods({}), null);
  assert.equal(parseMods(null), null);
  assert.deepEqual(parseMods({ worlds: [] }), []);
});

test("a switch says what changed and when it applies", () => {
  const creative = { id: "creative", name: "Creative", mods: [fawe] };
  assert.equal(switched(creative, fawe), "FastAsyncWorldEdit is off in Creative from its next start.");
  assert.equal(
    switched(creative, { ...fawe, enabled: true }),
    "FastAsyncWorldEdit is on in Creative from its next start.",
  );
  assert.match(unswitched("unknown_mod", "FastAsyncWorldEdit"), /doesn’t offer it any more/);
  assert.match(unswitched("unknown_world", "FastAsyncWorldEdit"), /doesn’t have that world any more/);
  assert.match(unswitched("unavailable", "FastAsyncWorldEdit"), /couldn’t tell whether FastAsyncWorldEdit was switched/);
});

test("modGrid lists each mod once by name, with a cell per world in the backend's order", () => {
  const gp = { ...fawe, id: "gp", name: "GriefPrevention", version: "16.18", default: true, enabled: true };
  const lines = modGrid([
    { id: "survival", name: "Survival", mods: [gp, { ...fawe, version: "2.15.0" }] },
    { id: "creative", name: "Creative", mods: [fawe, gp] },
    { id: "adventure", name: "Adventure", mods: [gp] },
  ]);
  assert.deepEqual(
    lines.map((l) => [l.id, l.version, l.cells.map((m) => (m ? m.enabled : null))]),
    [
      // The two worlds run different versions, so the row names neither.
      ["fawe", "", [false, false, null]],
      ["gp", "16.18", [true, true, true]],
    ],
  );
  assert.deepEqual(modGrid([]), []);
});

test("modGrid sorts by name whatever the case, numbers in counting order", () => {
  const named = (id: string, name: string) => ({ ...fawe, id, name });
  const lines = modGrid([
    { id: "survival", name: "Survival", mods: [named("z", "voicechat"), named("m10", "Maps 10"), named("g", "GSit")] },
    { id: "creative", name: "Creative", mods: [named("m2", "Maps 2"), named("a", "antiXray")] },
  ]);
  assert.deepEqual(
    lines.map((l) => l.name),
    ["antiXray", "GSit", "Maps 2", "Maps 10", "voicechat"],
  );
});

test("findMods matches every word typed, shows on or off, and can keep just-switched rows", () => {
  const gp = { ...fawe, id: "gp", name: "GriefPrevention", summary: "Claim land others can't break.", enabled: true };
  const gsit = { ...fawe, id: "gsit", name: "GSit", summary: "Sit on stairs.", enabled: true };
  const lines = modGrid([
    { id: "survival", name: "Survival", mods: [gp, fawe, gsit] },
    { id: "creative", name: "Creative", mods: [{ ...gsit, enabled: false }] },
  ]);
  const ids = (q: string, showing: "all" | "on" | "off" = "all", keep?: Set<string>) =>
    findMods(lines, q, showing, keep).map((l) => l.id);
  assert.deepEqual(ids(""), ["fawe", "gp", "gsit"]);
  assert.deepEqual(ids("  claim LAND "), ["gp"]);
  assert.deepEqual(ids("fawe"), ["fawe"]);
  assert.deepEqual(ids("claim brushes"), []);
  // GSit is on in one world and off in the other, so it is in both lists.
  assert.deepEqual(ids("", "on"), ["gp", "gsit"]);
  assert.deepEqual(ids("", "off"), ["fawe", "gsit"]);
  assert.deepEqual(ids("sit", "off"), ["gsit"]);
  assert.deepEqual(ids("", "off", new Set(["gp"])), ["fawe", "gp", "gsit"]);
});
