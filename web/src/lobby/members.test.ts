import assert from "node:assert/strict";
import { test } from "node:test";
import {
  addOutcome,
  emailProblem,
  grantOutcome,
  inviteText,
  lastHereLine,
  otherGrants,
  parseRoster,
  readAnswer,
  removeOutcome,
  signOutOutcome,
  unsureNote,
  type Member,
} from "./members.ts";

const member = (over: Partial<Member>): Member => ({
  email: "kid@example.com",
  tools: [],
  addedAt: null,
  lastHere: null,
  devices: 0,
  ...over,
});

test("parseRoster keeps each readable Member once, A to Z", () => {
  const got = parseRoster({
    you: "owner@example.com",
    members: [
      { email: "zed@example.com", tools: ["soccer", 7], added_at: "2026-11-06T00:00:00Z", devices: 2, last_here: "2026-11-14T10:00:00Z" },
      { email: "amy@example.com", tools: "soccer", last_here: "not a date" },
      { email: "zed@example.com", tools: [] },
      { email: "no-at-sign", tools: [] },
      null,
    ],
  });
  assert.ok(got);
  assert.equal(got.you, "owner@example.com");
  assert.deepEqual(
    got.members.map((m) => [m.email, m.tools, m.devices, m.lastHere?.toISOString() ?? null]),
    [
      ["amy@example.com", [], 0, null],
      ["zed@example.com", ["soccer"], 2, "2026-11-14T10:00:00.000Z"],
    ],
  );
  assert.equal(got.members[1].addedAt?.toISOString(), "2026-11-06T00:00:00.000Z");
  assert.equal(parseRoster({ members: [] }), null);
  assert.equal(parseRoster({ you: "a@b.co" }), null);
  assert.equal(parseRoster(null), null);
});

test("readAnswer tells the list, a refusal and each problem apart", () => {
  assert.equal(readAnswer(200, { you: "a@b.co", members: [] }).kind, "roster");
  assert.equal(readAnswer(200, { members: [] }).kind, "unavailable");
  assert.equal(readAnswer(401, { code: "signed_out" }).kind, "signed-out");
  assert.equal(readAnswer(403, { code: "no_access" }).kind, "denied");
  assert.equal(readAnswer(503, { code: "access_unavailable" }).kind, "access-failed");
  assert.equal(readAnswer(503, { code: "unavailable" }).kind, "unavailable");
  assert.deepEqual(readAnswer(409, { code: "already_member" }), { kind: "refused", code: "already_member" });
  assert.deepEqual(readAnswer(409, { code: "self" }), { kind: "refused", code: "self" });
  assert.deepEqual(readAnswer(404, { code: "not_member" }), { kind: "refused", code: "not_member" });
  // A refusal the page can't explain, or another site's page, is trouble.
  assert.equal(readAnswer(403, { code: "cross_site" }).kind, "unavailable");
  assert.equal(readAnswer(502, null).kind, "unavailable");
});

test("readAnswer reads whether an add's invite email went", () => {
  const roster = { you: "a@b.co", members: [] };
  for (const invite of ["sent", "failed"] as const) {
    const answer = readAnswer(200, { ...roster, invite });
    assert.ok(answer.kind === "roster");
    assert.equal(answer.invite, invite);
  }
  for (const body of [roster, { ...roster, invite: "queued" }, { ...roster, invite: true }]) {
    const answer = readAnswer(200, body);
    assert.ok(answer.kind === "roster");
    assert.equal(answer.invite, undefined);
  }
});

test("addOutcome says whether the invite went by email", () => {
  assert.equal(
    addOutcome("new@example.com", "", "sent"),
    "new@example.com is a Member. Their invite is on its way by email. To send it another way too:",
  );
  assert.equal(addOutcome("new@example.com", "", "failed"), "new@example.com is added, but the email didn’t send. Send them this:");
  assert.equal(addOutcome("new@example.com", ""), "new@example.com is a Member. Send them this:");
  // A refused add says why, whatever the answer said about email.
  assert.match(addOutcome("kid@example.com", "already_member", "sent"), /already a Member/);
});

test("lastHereLine counts whole days of the reader's calendar", () => {
  const now = new Date(2026, 10, 15, 9, 0);
  assert.equal(lastHereLine(member({}), now), "Not signed in anywhere");
  assert.equal(
    lastHereLine(member({ lastHere: new Date(2026, 10, 15, 1, 0), devices: 1 }), now),
    "Last here today, signed in on 1 device",
  );
  assert.equal(
    lastHereLine(member({ lastHere: new Date(2026, 10, 14, 23, 0), devices: 2 }), now),
    "Last here yesterday, signed in on 2 devices",
  );
  assert.equal(
    lastHereLine(member({ lastHere: new Date(2026, 10, 12, 12, 0), devices: 1 }), now),
    "Last here 3 days ago, signed in on 1 device",
  );
  assert.match(lastHereLine(member({ lastHere: new Date(2026, 9, 1), devices: 1 }), now), /^Last here on .+, signed in on 1 device$/);
});

test("emailProblem follows the site's rule for an address", () => {
  assert.equal(emailProblem("  "), "Type their email address.");
  for (const ok of ["kid@example.com", " Kid@Example.COM "]) assert.equal(emailProblem(ok), "");
  for (const bad of ["kid", "@example.com", "kid@example", "kid@ex ample.com", "a@b@example.com", "kid@.com."]) {
    assert.equal(emailProblem(bad), "That doesn’t look like an email address.", bad);
  }
});

test("otherGrants keeps the grants the page has no box for", () => {
  assert.deepEqual(otherGrants(member({ tools: ["soccer", "retired-tool"] }), ["soccer", "foundry"]), ["retired-tool"]);
});

test("each change says what happened, and what to do when it didn't", () => {
  assert.equal(grantOutcome("kid@example.com", "Foundry", true, ""), "kid@example.com can open Foundry now.");
  assert.equal(grantOutcome("kid@example.com", "Foundry", false, ""), "kid@example.com can’t open Foundry any more.");
  assert.match(grantOutcome("owner@example.com", "Lobby Admin", false, "self"), /can’t take away your own Lobby Admin/);
  assert.match(grantOutcome("kid@example.com", "Foundry", true, "unavailable"), /couldn’t tell/);
  assert.equal(addOutcome("new@example.com", ""), "new@example.com is a Member. Send them this:");
  assert.match(addOutcome("kid@example.com", "already_member"), /already a Member/);
  assert.equal(removeOutcome("kid@example.com", ""), "kid@example.com is no longer a Member and is signed out everywhere.");
  assert.equal(signOutOutcome("owner@example.com", true, ""), "Your other devices are signed out.");
  assert.match(signOutOutcome("kid@example.com", false, ""), /can sign in again/);
  assert.equal(
    inviteText("https://next.example", "new@example.com"),
    "You’re invited to The Lobby. Go to https://next.example, choose Sign in and enter new@example.com. You’ll get a code by email to finish signing in.",
  );
});

test("the note for a change nobody could confirm doesn't claim the list was read", () => {
  for (const adding of [true, false]) {
    assert.match(unsureNote(adding), /^We couldn’t tell whether/);
    assert.doesNotMatch(unsureNote(adding), /read again/);
  }
});
