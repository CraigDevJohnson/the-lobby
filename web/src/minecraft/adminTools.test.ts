import assert from "node:assert/strict";
import { test } from "node:test";
import {
  nameProblem,
  operatorOutcome,
  parseArchives,
  parseOperators,
  parsePlayers,
  playerOutcome,
  restoreOutcome,
  sizeOf,
} from "./adminTools.ts";

test("parsePlayers lists readable players once, A to Z", () => {
  const got = parsePlayers({
    players: [
      { edition: "java", name: "zed" },
      { edition: "bedrock", name: "Blocky Kid" },
      { edition: "java", name: "Zed" },
      { edition: "xbox", name: "Nope" },
      { edition: "java" },
      null,
    ],
  });
  assert.deepEqual(got, [
    { edition: "bedrock", name: "Blocky Kid" },
    { edition: "java", name: "zed" },
  ]);
  assert.equal(parsePlayers({}), null);
  assert.equal(parsePlayers(null), null);
  assert.deepEqual(parsePlayers({ players: [] }), []);
});

test("parseOperators keeps each world with a usable id", () => {
  const got = parseOperators({
    worlds: [
      { id: "survival", operators: [{ edition: "java", name: "Steve" }] },
      { id: "creative" },
      { id: "Bad Id", operators: [] },
    ],
  });
  assert.deepEqual(got, [
    { id: "survival", operators: [{ edition: "java", name: "Steve" }] },
    { id: "creative", operators: [] },
  ]);
  assert.equal(parseOperators({ players: [] }), null);
});

test("parseArchives reads the time from the name, newest first", () => {
  const got = parseArchives({
    world: "survival",
    archives: [
      { name: "20261016T031500Z.tar.gz", size: 1500000 },
      { name: "20261101T120000Z.tar.gz", size: "big" },
      { name: "20261016T031500Z.tar.gz.sha256", size: 64 },
      { name: "../20261016T031500Z.tar.gz" },
    ],
  })!;
  assert.deepEqual(
    got.map((a) => [a.name, a.size, a.made.toISOString()]),
    [
      ["20261101T120000Z.tar.gz", 0, "2026-11-01T12:00:00.000Z"],
      ["20261016T031500Z.tar.gz", 1500000, "2026-10-16T03:15:00.000Z"],
    ],
  );
  assert.equal(parseArchives({}), null);
});

test("sizeOf keeps the number short", () => {
  assert.equal(sizeOf(0), "");
  assert.equal(sizeOf(400), "1 KB");
  assert.equal(sizeOf(1_500_000), "2 MB");
  assert.equal(sizeOf(2_340_000_000), "2.3 GB");
});

test("nameProblem applies the server's rules for each edition", () => {
  assert.equal(nameProblem("java", "Steve_1"), "");
  assert.notEqual(nameProblem("java", "ab"), "");
  assert.notEqual(nameProblem("java", "has space"), "");
  assert.equal(nameProblem("bedrock", "Blocky Kid 7"), "");
  assert.notEqual(nameProblem("bedrock", "semi;colon"), "");
  assert.notEqual(nameProblem("bedrock", "x".repeat(17)), "");
  assert.notEqual(nameProblem("bedrock", ""), "");
});

test("outcomes say what happened, or that the page can't tell", () => {
  const steve = { edition: "java" as const, name: "Steve" };
  assert.match(playerOutcome("add", steve, ""), /^Steve is on the allowlist\./);
  assert.equal(playerOutcome("remove", steve, ""), "Steve is off the allowlist.");
  assert.equal(playerOutcome("add", steve, "player_not_found"), "No Java account is named Steve.");
  assert.match(playerOutcome("add", { edition: "bedrock", name: "Kid" }, "linked_account"), /Java account/);
  assert.match(playerOutcome("remove", steve, "unavailable"), /couldn’t tell whether Steve was removed/);
  assert.equal(operatorOutcome("add", steve, "Survival", ""), "Steve is an operator in Survival.");
  assert.match(operatorOutcome("add", steve, "Survival", "not_allowlisted"), /isn’t on the allowlist/);
  assert.match(restoreOutcome("Creative", ""), /^Restoring Creative\./);
  assert.match(restoreOutcome("Creative", "not_stopped"), /has to be stopped first/);
  assert.match(restoreOutcome("Creative", "unavailable"), /couldn’t tell/);
});
