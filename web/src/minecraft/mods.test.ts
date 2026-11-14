import assert from "node:assert/strict";
import { test } from "node:test";
import {
  changedSince,
  firstSeen,
  countLine,
  findMods,
  launcherCue,
  modGrid,
  noneFound,
  notOffered,
  parseMods,
  stackUnder,
  switched,
  unswitched,
} from "./mods.ts";

const fawe = {
  id: "fawe",
  name: "FastAsyncWorldEdit",
  version: "2.16.2",
  summary: "Bulk building tools: copy, paste, fill and brushes.",
  source: "https://modrinth.com/mod/fawe",
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

test("parseMods links a mod only to an https page on Modrinth or GitHub", () => {
  const read = (source: unknown) =>
    parseMods({ worlds: [{ id: "creative", name: "Creative", mods: [{ ...fawe, source }] }] })![0].mods[0].source;
  assert.equal(read("https://modrinth.com/mod/tFLdoQMh"), "https://modrinth.com/mod/tFLdoQMh");
  assert.equal(read("https://github.com/example/mod"), "https://github.com/example/mod");
  for (const bad of [undefined, 7, "", "http://modrinth.com/mod/x", "https://modrinth.com.example.net/x", "javascript:alert(1)", "modrinth.com/mod/x"]) {
    assert.equal(read(bad), "");
  }
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

test("modGrid gives a mod with a build per loader one row", () => {
  const flan = { ...fawe, id: "flan", name: "Flan", enabled: true };
  const lines = modGrid([
    { id: "survival", name: "Survival", mods: [flan] },
    { id: "adventure", name: "Adventure", mods: [{ ...flan, id: "flan-neoforge", enabled: false }] },
  ]);
  assert.deepEqual(
    lines.map((l) => [l.id, l.cells.map((m) => m?.id)]),
    [["flan", ["flan", "flan-neoforge"]]],
  );
});

test("modGrid keeps two mods of one name in a world apart, and from a mod named like an id", () => {
  const named = (id: string, name: string) => ({ ...fawe, id, name });
  const lines = modGrid([
    { id: "survival", name: "Survival", mods: [named("a", "Shared"), named("b", "Shared")] },
    { id: "creative", name: "Creative", mods: [named("c", "B"), named("b", "Shared")] },
  ]);
  assert.deepEqual(
    lines.map((l) => [l.name, l.cells.map((m) => m?.id)]),
    [
      ["B", [undefined, "c"]],
      ["Shared", ["a", undefined]],
      ["Shared", ["b", "b"]],
    ],
  );
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

test("changedSince marks a cell only while it differs from the settings first loaded", () => {
  const gsit = { ...fawe, id: "gsit", name: "GSit", enabled: true };
  const first = firstSeen(null, [
    { id: "survival", name: "Survival", mods: [fawe, gsit] },
    { id: "creative", name: "Creative", mods: [fawe] },
  ]);
  assert.equal(changedSince(first, "survival", fawe), false);
  assert.equal(changedSince(first, "survival", { ...fawe, enabled: true }), true);
  // The same mod in another world is its own cell.
  assert.equal(changedSince(first, "creative", fawe), false);
  assert.equal(changedSince(first, "survival", { ...gsit, enabled: false }), true);
  // Switched back to what it was, the mark goes.
  assert.equal(changedSince(first, "survival", gsit), false);
  // A cell the page hasn't shown is never marked, nor is anything before the first load.
  assert.equal(changedSince(first, "creative", gsit), false);
  assert.equal(changedSince(first, "adventure", fawe), false);
  assert.equal(changedSince(null, "survival", { ...fawe, enabled: true }), false);
});

test("firstSeen starts from a list with nothing in it and marks a later switch", () => {
  const none = firstSeen(null, []);
  assert.equal(none.size, 0);
  const first = firstSeen(none, [{ id: "survival", name: "Survival", mods: [fawe] }]);
  assert.equal(changedSince(first, "survival", fawe), false);
  assert.equal(changedSince(first, "survival", { ...fawe, enabled: !fawe.enabled }), true);
});

test("firstSeen never replaces a setting it already has", () => {
  const flipped = { ...fawe, enabled: !fawe.enabled };
  const first = firstSeen(null, [{ id: "survival", name: "Survival", mods: [fawe] }]);
  // The answer to a switch shows the cell again with its new setting.
  const later = firstSeen(first, [{ id: "survival", name: "Survival", mods: [flipped] }]);
  assert.equal(later, first);
  assert.equal(changedSince(later, "survival", flipped), true);
  assert.equal(changedSince(later, "survival", fawe), false);
});

test("firstSeen takes a world or mod that appears later as it first showed", () => {
  const gsit = { ...fawe, id: "gsit", name: "GSit", enabled: true };
  const first = firstSeen(null, [{ id: "survival", name: "Survival", mods: [fawe] }]);
  const later = firstSeen(first, [
    { id: "survival", name: "Survival", mods: [fawe, gsit] },
    { id: "creative", name: "Creative", mods: [gsit] },
  ]);
  assert.notEqual(later, first);
  assert.equal(first.size, 1);
  assert.equal(later.size, 3);
  assert.equal(changedSince(later, "survival", gsit), false);
  assert.equal(changedSince(later, "survival", { ...gsit, enabled: false }), true);
  assert.equal(changedSince(later, "creative", { ...gsit, enabled: false }), true);
});

test("firstSeen keeps a world and a mod apart whatever hyphens their ids hold", () => {
  const first = firstSeen(null, [
    { id: "a", name: "A", mods: [{ ...fawe, id: "b-c", enabled: true }] },
    { id: "a-b", name: "A B", mods: [{ ...fawe, id: "c", enabled: false }] },
  ]);
  assert.equal(first.size, 2);
  assert.equal(changedSince(first, "a", { ...fawe, id: "b-c", enabled: false }), true);
  assert.equal(changedSince(first, "a-b", { ...fawe, id: "c", enabled: false }), false);
});

test("countLine says how many mods show and what On or Off means", () => {
  const line = (matching: number, query = "", showing: "all" | "on" | "off" = "all", kept = 0, worlds = 3) =>
    countLine({ total: 10, matching, kept, query, showing, worlds });
  assert.equal(line(10), "10 mods");
  assert.equal(countLine({ total: 1, matching: 1, kept: 0, query: " ", showing: "all", worlds: 1 }), "1 mod");
  assert.equal(line(7, "", "on"), "7 of 10 mods are on in at least one world");
  assert.equal(line(6, "", "off"), "6 of 10 mods are off in at least one world");
  assert.equal(line(1, "", "off"), "1 of 10 mods is off in at least one world");
  assert.equal(line(0, "", "on"), "0 of 10 mods are on in at least one world");
  assert.equal(line(3, "map"), "3 of 10 mods match your search");
  assert.equal(line(1, "map"), "1 of 10 mods matches your search");
  assert.equal(line(2, "map", "on"), "2 of 10 mods match your search and are on in at least one world");
  assert.equal(line(1, "map", "off"), "1 of 10 mods matches your search and is off in at least one world");
  // With one world there is no "at least one" to explain.
  assert.equal(line(7, "", "on", 0, 1), "7 of 10 mods are on");
  // A row kept only because it was just switched is named, not counted as matching.
  assert.equal(line(5, "", "off", 1), "5 of 10 mods are off in at least one world, plus 1 you just switched");
});

test("noneFound names the filter only when the filter alone left no rows", () => {
  assert.equal(noneFound("zzz", "all"), "No mods match that.");
  assert.equal(noneFound("zzz", "on"), "No mods match that.");
  assert.equal(noneFound(" ", "on"), "No mods are on.");
  assert.equal(noneFound("", "off"), "No mods are off.");
});

test("launcherCue links to the Launcher without promising a restart or naming a running world", () => {
  const cue = launcherCue();
  assert.equal(cue.href, "/minecraft");
  const said = cue.before + cue.link + cue.after;
  assert.equal(said, "Worlds start and stop on the Launcher.");
  assert.doesNotMatch(said, /restart|running|ready|your/i);
});

test("notOffered folds the worlds without a mod into one line", () => {
  assert.equal(notOffered([]), "");
  assert.equal(notOffered(["Creative"]), "Not offered in Creative");
  assert.equal(notOffered(["Creative", "Skyblock Summer Camp"]), "Not offered in Creative or Skyblock Summer Camp");
  assert.equal(notOffered(["A", "B", "C"]), "Not offered in A, B or C");
});

test("stackUnder widens with the number of worlds and never drops under a phone's width", () => {
  assert.equal(stackUnder(1), 48);
  // From three worlds the columns' own width sets the mark, with room for a scrollbar.
  assert.equal(stackUnder(3), 49);
  assert.equal(stackUnder(4), 60);
  assert.equal(stackUnder(5), 70);
  // More columns than the page's widest can hold: always stacked.
  assert.equal(stackUnder(8), 9999);
});
