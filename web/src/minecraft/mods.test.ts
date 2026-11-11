import assert from "node:assert/strict";
import { test } from "node:test";
import { parseMods, switched, unswitched } from "./mods.ts";

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
