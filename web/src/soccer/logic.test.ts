import assert from "node:assert/strict";
import { test } from "node:test";
import {
  afterTeamIDs,
  allGames,
  checkTeamID,
  divisionLabels,
  downloadChoice,
  earlierName,
  fieldName,
  gameWhen,
  matchup,
  MAX_TEAMS,
  nextSummary,
  NO_PICKS,
  reduce,
  teamsKey,
  webcal,
  type Action,
  type Game,
  type NextSessionAnswer,
  type Picks,
} from "./logic.ts";

const game = (id: string, remaining: boolean, start = "2026-10-14T19:15:00-06:00"): Game => ({
  id,
  start,
  end: start,
  field: "Field 1",
  home: "Boise Comets",
  away: "River City",
  remaining,
});

const run = (...actions: Action[]): Picks => actions.reduce(reduce, NO_PICKS);

const comets = { id: "111111", name: "Boise Comets" };
const owls = { id: "222222", name: "Evening Owls" };

test("remaining games start checked and played games do not", () => {
  const picks = run(
    { type: "add", team: comets },
    { type: "loaded", id: comets.id, team: comets, games: [game("played", false), game("next", true)] },
  );
  assert.deepEqual([...picks.checked], ["next"]);
  assert.equal(picks.teams[0].state, "ready");
});

test("adding another Team keeps the choices already made", () => {
  const picks = run(
    { type: "add", team: comets },
    { type: "loaded", id: comets.id, team: comets, games: [game("a", true), game("b", true), game("old", false)] },
    { type: "toggle", gameId: "a", on: false },
    { type: "toggle", gameId: "old", on: true },
    { type: "add", team: owls },
    // "a" is a game between the two Teams: it stays unchecked.
    { type: "loaded", id: owls.id, team: owls, games: [game("a", true), game("c", true)] },
  );
  assert.deepEqual([...picks.checked].sort(), ["b", "c", "old"]);
});

test("the same Team is never added twice, and no more than the backend combines", () => {
  assert.equal(run({ type: "add", team: comets }, { type: "add", team: comets }).teams.length, 1);
  const many = Array.from({ length: MAX_TEAMS + 2 }, (_, i): Action => ({
    type: "add",
    team: { id: String(100000 + i), name: `Team ${i}` },
  }));
  assert.equal(run(...many).teams.length, MAX_TEAMS);
});

test("removing a Team takes its games out, except one another Team plays in", () => {
  const picks = run(
    { type: "add", team: comets },
    { type: "loaded", id: comets.id, team: comets, games: [game("shared", true), game("comets-only", true)] },
    { type: "add", team: owls },
    { type: "loaded", id: owls.id, team: owls, games: [game("shared", true), game("owls-only", true)] },
    { type: "remove", id: comets.id },
  );
  assert.deepEqual(picks.teams.map((t) => t.id), [owls.id]);
  assert.deepEqual([...picks.checked].sort(), ["owls-only", "shared"]);
});

test("a failed Team stays chosen and can be retried", () => {
  let picks = run({ type: "add", team: comets }, { type: "failed", id: comets.id, problem: "league" });
  assert.equal(picks.teams[0].state, "failed");
  assert.equal(picks.teams[0].problem, "league");
  picks = reduce(picks, { type: "retry", id: comets.id });
  assert.equal(picks.teams[0].state, "loading");
  // Answers for a Team removed meanwhile are dropped.
  picks = reduce(reduce(picks, { type: "remove", id: comets.id }), {
    type: "loaded",
    id: comets.id,
    team: comets,
    games: [game("late", true)],
  });
  assert.deepEqual(picks, { teams: [], checked: new Set() });
});

test("a download asks for exactly the checked games", () => {
  const picks = run(
    { type: "add", team: comets },
    { type: "loaded", id: comets.id, team: comets, games: [game("old", false), game("a", true), game("b", true)] },
    { type: "add", team: owls },
    { type: "loaded", id: owls.id, team: owls, games: [game("a", true), game("c", true)] },
    { type: "toggle", gameId: "b", on: false },
  );
  const choice = downloadChoice(picks);
  assert.deepEqual(choice.team_ids, [comets.id, owls.id]);
  // Everything not checked is excluded, once, including the played game.
  assert.deepEqual(choice.excluded.sort(), ["b", "old"]);
  assert.ok(new Date(choice.since) < new Date("2001-01-01"));
  assert.equal(allGames(picks.teams).length, 4);
});

test("a Session link's Teams do not depend on the order they were added", () => {
  assert.equal(teamsKey([owls, comets]), teamsKey([comets, owls]));
  assert.notEqual(teamsKey([comets]), teamsKey([comets, owls]));
});

test("a Team ID is six digits, kept as text", () => {
  assert.deepEqual(checkTeamID(" 012345 "), { ok: true, id: "012345" });
  assert.deepEqual(checkTeamID("12a456"), { ok: false, message: "Enter a Team ID using numbers." });
  assert.deepEqual(checkTeamID(""), { ok: false, message: "Enter a Team ID using numbers." });
  assert.deepEqual(checkTeamID("1234"), { ok: false, message: "A Team ID has 6 numbers." });
});

test("game times read in Boise time on any device", () => {
  // Daylight time (UTC-6), then standard time (UTC-7) after 1 November 2026.
  assert.deepEqual(gameWhen("2026-10-14T19:15:00-06:00"), { date: "Wed 14 Oct", time: "7:15 pm" });
  assert.deepEqual(gameWhen("2026-11-04T20:00:00-07:00"), { date: "Wed 4 Nov", time: "8:00 pm" });
  // The same instant written in another zone is still shown as Boise has it.
  assert.deepEqual(gameWhen("2026-11-05T03:00:00Z"), { date: "Wed 4 Nov", time: "8:00 pm" });
  assert.equal(gameWhen("not a time"), null);
});

test("a game names the other side", () => {
  const g = game("a", true);
  assert.equal(matchup(g, "boise comets"), "vs River City");
  assert.equal(matchup(g, "River City"), "vs Boise Comets");
  assert.equal(matchup(g, "Someone Else"), "Boise Comets vs River City");
  assert.equal(matchup({ ...g, home: "", away: "" }, "Boise Comets"), "Opponent not posted");
});

test("Apple Calendar opens a webcal address", () => {
  assert.equal(webcal("https://lobby.example/soccer/link/abc.ics"), "webcal://lobby.example/soccer/link/abc.ics");
  assert.equal(webcal("http://127.0.0.1:8080/soccer/link/abc.ics"), "webcal://127.0.0.1:8080/soccer/link/abc.ics");
});

test("a field given as a bare number is named", () => {
  assert.equal(fieldName("3"), "Field 3");
  assert.equal(fieldName("Field 2"), "Field 2");
  assert.equal(fieldName("North Court"), "North Court");
  assert.equal(fieldName(" "), "Field not posted");
});

test("an ended Session's Teams come from repeated after parameters", () => {
  assert.deepEqual(afterTeamIDs("?after=111111&after=222222"), ["111111", "222222"]);
  // Each once, in the link's order; anything not a six-digit Team ID is dropped.
  assert.deepEqual(afterTeamIDs("?after=222222&after=111111&after=222222"), ["222222", "111111"]);
  assert.deepEqual(afterTeamIDs("?after=12345&after=1234567&after=12ab56&after=&after=%20333333%20"), ["333333"]);
  assert.deepEqual(afterTeamIDs("?q=111111&after"), []);
  assert.deepEqual(afterTeamIDs(""), []);
  // No more than one calendar combines.
  const many = Array.from({ length: MAX_TEAMS + 3 }, (_, i) => `after=${100000 + i}`).join("&");
  assert.equal(afterTeamIDs(`?${many}`).length, MAX_TEAMS);
  assert.equal(afterTeamIDs(`?${many}`)[0], "100000");
});

test("what was found for an ended Session's Team is said in one line", () => {
  const old = { id: "111111", name: "Boise Comets", season: "172" };
  const next = (id: string, same_coach: boolean) => ({ id, name: "Boise Comets", season: "173", same_coach });
  const answer = (match: NextSessionAnswer["match"], teams: NextSessionAnswer["teams"]): NextSessionAnswer => ({
    team: old,
    match,
    teams,
    similar: [],
  });
  assert.equal(nextSummary(answer("follow", [next("222222", true)])), "Same name and coach as last Session");
  assert.equal(nextSummary(answer("ask", [next("222222", false)])), "Same name, different coach");
  assert.equal(nextSummary(answer("ask", [next("222222", true), next("333333", false)])), "More than one possible match");
  assert.equal(
    nextSummary(answer("none", [])),
    "Not signed up for next Session yet. Teams show up here once they sign up.",
  );
  // A suggestion with no Team in it is nothing to add.
  assert.equal(nextSummary(answer("follow", [])), nextSummary(answer("none", [])));
});

test("an ended Session's Team is called by its name once read, by its Team ID until then", () => {
  assert.equal(earlierName("111111"), "Team ID 111111");
  assert.equal(earlierName("111111", null), "Team ID 111111");
  assert.equal(earlierName("111111", { id: "111111", name: " Boise Comets " }), "Boise Comets");
  // A blank name says nothing, so the Team ID stands in.
  assert.equal(earlierName("111111", { id: "111111", name: "  " }), "Team ID 111111");
});

test("division names carry their Session only when the list holds more than one", () => {
  const thisSession = [
    { id: "1", name: "Coed Friday", season: "173" },
    { id: "2", name: "Open Sunday", season: "173" },
  ];
  assert.deepEqual(divisionLabels(thisSession), ["Coed Friday", "Open Sunday"]);
  assert.deepEqual(divisionLabels([...thisSession, { id: "3", name: "Coed Friday", season: "175" }]), [
    "Coed Friday · Session 173",
    "Open Sunday · Session 173",
    "Coed Friday · Session 175",
  ]);
  // A backend that gives no Sessions lists plain names.
  assert.deepEqual(divisionLabels([{ id: "1", name: "Coed Friday" }]), ["Coed Friday"]);
});
