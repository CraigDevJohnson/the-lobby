import assert from "node:assert/strict";
import { test } from "node:test";
import {
  address,
  announcement,
  askingMessage,
  availability,
  caveat,
  control,
  describe,
  EMPTY,
  forget,
  joinLink,
  key,
  noLink,
  openIsPrimary,
  pageTitle,
  parseStatus,
  pollDelay,
  readAnswer,
  readChange,
  rechecked,
  refusalMessage,
  segments,
  STALE_MS,
  STOP_FAILED,
  take,
  unsureMessage,
  when,
  type Availability,
  type Picture,
} from "./logic.ts";

const JOIN = "https://foundry.example.net:30000/join";
const T0 = Date.parse("2026-10-14T01:00:00Z");
const clock = (at: number) => when(at, T0, "en-US", "UTC");

test("parseStatus keeps the state, the join link and when it was checked", () => {
  assert.deepEqual(
    parseStatus({
      state: "ready",
      join_url: JOIN,
      started_at: "2026-10-14T00:50:00Z",
      checked_at: "2026-10-14T01:00:00Z",
      checks: { host: "running", agent: "online", foundry: "responding" },
    }),
    { state: "ready", joinUrl: JOIN, checkedAt: T0 },
  );
  assert.deepEqual(parseStatus({ state: "stopping" }), { state: "stopping" });
});

test("parseStatus refuses an answer without a known state", () => {
  assert.equal(parseStatus({ state: "running", join_url: JOIN }), null);
  assert.equal(parseStatus({ state: "maintenance" }), null);
  assert.equal(parseStatus(null), null);
  assert.equal(parseStatus("ready"), null);
});

test("only an https web address can stand behind Open Foundry", () => {
  assert.equal(joinLink(JOIN), JOIN);
  assert.equal(joinLink(` ${JOIN} `), JOIN);
  assert.equal(joinLink("http://foundry.example.net/join"), undefined);
  assert.equal(joinLink("javascript:alert(1)"), undefined);
  assert.equal(joinLink("https://player:secret@foundry.example.net/join"), undefined);
  assert.equal(joinLink("foundry.example.net/join"), undefined);
  assert.equal(joinLink(""), undefined);
  assert.equal(joinLink(42), undefined);
  assert.equal(parseStatus({ state: "ready", join_url: "javascript:alert(1)" })?.joinUrl, undefined);
});

test("an unreadable checked_at is left out rather than guessed", () => {
  assert.equal(parseStatus({ state: "ready", checked_at: "yesterday" })?.checkedAt, undefined);
});

test("readAnswer tells access failures apart from Foundry's status", () => {
  assert.deepEqual(readAnswer(401, { code: "signed_out" }), { kind: "signed-out" });
  assert.deepEqual(readAnswer(403, { code: "no_access" }), { kind: "denied" });
  assert.deepEqual(readAnswer(503, { code: "access_unavailable" }), { kind: "access-failed" });
  assert.deepEqual(readAnswer(503, { code: "not_configured" }), { kind: "not-configured" });
  assert.deepEqual(readAnswer(404, { code: "no_server" }), { kind: "no-server" });
  assert.deepEqual(readAnswer(200, { state: "stopped", join_url: JOIN }), {
    kind: "status",
    status: { state: "stopped", joinUrl: JOIN },
  });
});

test("a failed status check still carries the configured join link", () => {
  assert.deepEqual(readAnswer(503, { code: "tool_unavailable", message: "…", join_url: JOIN }), {
    kind: "unavailable",
    joinUrl: JOIN,
  });
  assert.deepEqual(readAnswer(503, { code: "tool_unavailable" }), { kind: "unavailable" });
});

test("anything the page can't read is an unavailable status, never an access answer", () => {
  assert.deepEqual(readAnswer(200, { state: "running" }), { kind: "unavailable" });
  assert.deepEqual(readAnswer(404, null), { kind: "unavailable" });
  assert.deepEqual(readAnswer(404, { code: "gone" }), { kind: "unavailable" });
  assert.deepEqual(readAnswer(502, { code: "no_server" }), { kind: "unavailable" });
  assert.deepEqual(readAnswer(500, { code: "no_access", join_url: JOIN }), { kind: "denied" });
  assert.deepEqual(readAnswer(503, { code: "tool_unavailable", join_url: "ftp://x/join" }), { kind: "unavailable" });
});

const status = (state: string, extra: object = {}) =>
  readAnswer(200, { state, join_url: JOIN, checked_at: "2026-10-14T00:59:58Z", ...extra });

function seen(...answers: [ReturnType<typeof readAnswer>, number][]): Picture | null {
  let p: Picture | null = EMPTY;
  for (const [a, at] of answers) p = take(p ?? EMPTY, a, at);
  return p;
}

test("a confirmed state shows the backend's own time", () => {
  const p = seen([status("ready"), T0])!;
  const a = availability(p, T0)!;
  assert.equal(a.kind, "confirmed");
  const d = describe(a, true, clock);
  assert.equal(d.label, "Foundry is responding");
  assert.equal(d.mark, "on");
  assert.equal(d.when, "Checked at 12:59 AM");
  assert.equal(d.at, Date.parse("2026-10-14T00:59:58Z"));
});

test("nothing on the page claims a game is open", () => {
  const d = describe(availability(seen([status("ready"), T0])!, T0)!, true, clock);
  assert.match(d.message, /Whether a game is open yet/);
  const claim = /\bready\b|opens? a game|game is open\b(?! yet)|world is/i;
  for (const state of ["ready", "stopped", "starting", "stopping", "unresponsive"]) {
    const confirmed = availability(seen([status(state), T0])!, T0)!;
    const stale = availability(seen([status(state), T0], [readAnswer(503, {}), T0])!, T0)!;
    for (const a of [confirmed, stale]) {
      const s = describe(a, true, clock);
      assert.doesNotMatch(`${s.label} ${s.message} ${caveat(a)} ${announcement(null, a)}`, claim, state);
    }
  }
});

test("stopped, starting and shutting down are confirmed states that aren't responding", () => {
  for (const [state, label, mark] of [
    ["stopped", "Foundry is stopped", "off"],
    ["starting", "Foundry is starting", "busy"],
    ["stopping", "Foundry is shutting down", "busy"],
  ] as const) {
    const a = availability(seen([status(state), T0])!, T0)!;
    assert.equal(a.kind, "confirmed");
    const d = describe(a, true, clock);
    assert.equal(d.label, label);
    assert.equal(d.mark, mark);
    assert.equal(openIsPrimary(a), false, state);
    assert.notEqual(caveat(a), "", state);
  }
});

test("not answering and no server are their own confirmed states", () => {
  const unresponsive = availability(seen([status("unresponsive"), T0])!, T0)!;
  assert.equal(describe(unresponsive, true, clock).label, "Foundry isn’t answering");
  const gone = availability(seen([readAnswer(404, { code: "no_server" }), T0])!, T0)!;
  assert.deepEqual(gone, { kind: "confirmed", reading: { state: "gone", checkedAt: T0, receivedAt: T0 } });
  assert.equal(describe(gone, false, clock).label, "Foundry has no server right now");
});

test("a failed check after a confirmed one keeps it, labelled as the last check", () => {
  const p = seen([status("ready"), T0 - 60_000], [readAnswer(503, { code: "tool_unavailable" }), T0])!;
  const a = availability(p, T0)!;
  assert.deepEqual(a.kind, "stale");
  const d = describe(a, true, clock);
  // The last state, in the past tense, with its own mark drawn stale.
  assert.equal(d.label, "Foundry was responding");
  assert.equal(d.mark, "on");
  assert.equal(d.stale, true);
  assert.equal(d.when, "Last checked at 12:59 AM");
  assert.match(d.message, /^This may be out of date\. We couldn’t check again/);
  // The way in stays open, and stays the thing to press.
  assert.equal(p.link, JOIN);
  assert.equal(openIsPrimary(a), true);
});

test("the backend's unknown counts as a failed check, not a state", () => {
  const p = seen([status("stopped"), T0 - 60_000], [status("unknown"), T0])!;
  assert.equal(availability(p, T0)?.kind, "stale");
  assert.equal(p.last?.state, "stopped");
});

test("a confirmed state goes stale once nothing has confirmed it for a while", () => {
  const p = seen([status("ready"), T0])!;
  assert.equal(availability(p, T0 + STALE_MS)?.kind, "confirmed");
  const old = availability(p, T0 + STALE_MS + 1)!;
  assert.equal(old.kind, "stale");
  assert.match(describe(old, true, clock).message, /^This may be out of date\. .*more than five minutes ago/);
});

test("with nothing confirmed, a failed check is an unavailable status", () => {
  const tried = availability(seen([readAnswer(503, { code: "tool_unavailable", join_url: JOIN }), T0])!, T0)!;
  assert.deepEqual(tried, { kind: "unavailable", at: T0, checked: false });
  assert.equal(describe(tried, true, clock).when, "Tried at 1:00 AM");
  assert.match(describe(tried, true, clock).message, /still try opening it/);
  assert.match(describe(tried, false, clock).message, /checking again/);

  // The backend did look, and says when, but couldn't read the host.
  const checked = availability(seen([status("unknown"), T0])!, T0)!;
  assert.deepEqual(checked, { kind: "unavailable", at: Date.parse("2026-10-14T00:59:58Z"), checked: true });
  assert.equal(describe(checked, true, clock).when, "Checked at 12:59 AM");
});

test("the join link is kept for the visit and comes back with a failure", () => {
  const fromStatus = seen([status("stopped"), T0], [readAnswer(503, { code: "tool_unavailable" }), T0 + 1])!;
  assert.equal(fromStatus.link, JOIN);
  const fromFailure = seen([readAnswer(503, { code: "tool_unavailable", join_url: JOIN }), T0])!;
  assert.equal(fromFailure.link, JOIN);
  const none = seen([readAnswer(503, { code: "tool_unavailable" }), T0])!;
  assert.equal(none.link, null);
});

test("an access answer clears everything about Foundry", () => {
  for (const a of [
    readAnswer(401, { code: "signed_out" }),
    readAnswer(403, { code: "no_access" }),
    readAnswer(503, { code: "access_unavailable" }),
    readAnswer(503, { code: "not_configured" }),
  ]) {
    assert.equal(take(seen([status("ready"), T0])!, a, T0 + 1), null);
  }
});

test("only a failed access check keeps what the visit learned, for after Retry", () => {
  const known = seen([status("ready"), T0])!;
  for (const code of ["signed_out", "no_access", "not_configured"]) {
    assert.equal(forget(known, readAnswer(code === "signed_out" ? 401 : 403, { code })), EMPTY, code);
  }
  const kept = forget(known, readAnswer(503, { code: "access_unavailable" }));
  assert.equal(kept, known);
  // Retry gets past the access check, but the status check fails: the last
  // state and the link are still there.
  const after = take(kept, readAnswer(502, null), T0 + 1)!;
  assert.equal(availability(after, T0 + 1)?.kind, "stale");
  assert.equal(after.link, JOIN);
});

test("nothing is shown before the first answer about Foundry", () => {
  assert.equal(availability(EMPTY, T0), null);
  assert.equal(pageTitle(null), "Foundry · The Lobby");
});

test("without a link, Join in Foundry says why", () => {
  const gone = availability(seen([readAnswer(404, { code: "no_server" }), T0])!, T0)!;
  assert.match(noLink(gone), /no Foundry server/);
  assert.match(noLink({ kind: "unavailable", at: T0, checked: false }), /until a check gets through/);
});

test("Open Foundry is the thing to press when responding or unconfirmed", () => {
  const ready = availability(seen([status("ready"), T0])!, T0)!;
  assert.equal(openIsPrimary(ready), true);
  assert.equal(caveat(ready), "");
  const unavailable: Availability = { kind: "unavailable", at: T0, checked: false };
  assert.equal(openIsPrimary(unavailable), true);
  assert.match(caveat(unavailable), /may not open/);
});

test("the page speaks only when what it shows changes", () => {
  const ready = availability(seen([status("ready"), T0])!, T0)!;
  const stopped = availability(seen([status("stopped"), T0])!, T0)!;
  assert.equal(announcement(null, ready), "Foundry is responding.");
  assert.equal(announcement(ready, ready), "");
  assert.equal(announcement(ready, stopped), "Foundry is stopped.");
  const stale = availability(seen([status("ready"), T0], [readAnswer(503, {}), T0])!, T0)!;
  assert.match(announcement(ready, stale), /couldn’t check Foundry again/);
  // A state that had already aged, then a failed check: the card changes, so
  // the page says so.
  const later = T0 + STALE_MS + 1;
  const aged = availability(seen([status("ready"), T0])!, later)!;
  const failed = availability(seen([status("ready"), T0], [readAnswer(503, {}), later])!, later)!;
  assert.match(announcement(aged, failed), /couldn’t check Foundry again/);
  assert.equal(rechecked(ready), "Checked just now. The status is the same.");
  assert.equal(rechecked(stale), "Tried again just now. Still no answer.");
});

test("a state that aged unseen is said as out of date, and the next answer as itself", () => {
  const p = seen([status("ready"), T0])!;
  const before = availability(p, T0);
  const back = T0 + STALE_MS + 60_000;
  const aged = availability(p, back);
  assert.equal(announcement(before, aged), "Foundry’s status may be out of date.");
  const fresh = availability(take(p, status("ready"), back + 500)!, back + 500);
  assert.equal(announcement(aged, fresh), "Foundry is responding.");
  // Measured against what was said before the state aged, the same state says
  // nothing. So the page has to record the aged state before the next answer
  // lands (Foundry.tsx's age, on coming back to the tab), or both changes go
  // unsaid. That wiring isn't covered here.
  assert.equal(announcement(before, fresh), "");
});

test("a re-confirmed state keeps its key whatever its join link, so Check again says only that the status held", () => {
  const moved = "https://play.example.org/join";
  const before = seen([status("ready"), T0])!;
  const after = take(before, status("ready", { join_url: moved }), T0 + 5_000)!;
  // Open Foundry now opens the new link, but the status is the same.
  assert.equal(after.link, moved);
  const a = availability(after, T0 + 5_000);
  assert.equal(key(a), key(availability(before, T0)));
  assert.equal(rechecked(a), "Checked just now. The status is the same.");
});

test("the page asks often only while Foundry is on its way up or down", () => {
  assert.equal(pollDelay(availability(seen([status("starting"), T0])!, T0)!), 10_000);
  assert.equal(pollDelay(availability(seen([status("stopping"), T0])!, T0)!), 10_000);
  assert.equal(pollDelay(availability(seen([status("ready"), T0])!, T0)!), 60_000);
  assert.equal(pollDelay({ kind: "unavailable", at: T0, checked: false }), 60_000);
});

test("the tab's title leads with the state", () => {
  assert.equal(pageTitle(availability(seen([status("ready"), T0])!, T0)), "Responding · Foundry · The Lobby");
  assert.equal(pageTitle({ kind: "unavailable", at: T0, checked: false }), "Can’t check · Foundry · The Lobby");
});

test("when names the day once it isn't today", () => {
  assert.equal(when(T0, T0, "en-US", "UTC"), "at 1:00 AM");
  assert.equal(when(T0 - 24 * 60 * 60_000, T0, "en-US", "UTC"), "on Tuesday at 1:00 AM");
  assert.equal(when(T0 - 10 * 24 * 60 * 60_000, T0, "en-US", "UTC"), "on Oct 4 at 1:00 AM");
});

test("the address shows the host and port, wrapping only between its parts", () => {
  assert.equal(address(JOIN), "foundry.example.net:30000");
  assert.deepEqual(segments("foundry.example.net:30000"), ["foundry.", "example.", "net:", "30000"]);
  assert.equal(segments("a.b/c").join(""), "a.b/c");
});

// Starting and stopping.

const confirmed = (state: "stopped" | "starting" | "ready" | "stopping" | "unresponsive" | "gone", extra = {}): Availability => ({
  kind: "confirmed",
  reading: { state, checkedAt: T0, receivedAt: T0, ...extra },
});

test("the card offers Start only while Foundry is confirmed stopped, and Stop only while its server is confirmed on", () => {
  assert.equal(control(confirmed("stopped")), "start");
  assert.equal(control(confirmed("ready")), "stop");
  assert.equal(control(confirmed("unresponsive")), "stop");
  for (const s of ["starting", "stopping", "gone"] as const) assert.equal(control(confirmed(s)), null, s);
  assert.equal(control(null), null);
  assert.equal(control({ kind: "stale", last: confirmed("stopped").reading as never, cause: "old" }), null);
  assert.equal(control({ kind: "unavailable", at: T0, checked: false }), null);
});

test("a change's status comes back whether it began or there was nothing to do", () => {
  const began = readChange("start", 202, { state: "starting", join_url: JOIN, checked_at: "2026-10-14T01:00:00Z" });
  assert.equal(began.kind, "status");
  if (began.kind === "status") assert.equal(began.status.state, "starting");
  const nothing = readChange("stop", 200, { state: "stopped", join_url: JOIN });
  assert.equal(nothing.kind, "status");
  if (nothing.kind === "status") assert.equal(nothing.status.state, "stopped");
  assert.deepEqual(readChange("start", 202, { state: "elsewhere" }), { kind: "unavailable" });
});

test("a refusal is read only for a reason that change can be refused for", () => {
  assert.deepEqual(readChange("start", 409, { code: "stopping" }), { kind: "refused", of: "start", code: "stopping" });
  assert.deepEqual(readChange("stop", 409, { code: "starting" }), { kind: "refused", of: "stop", code: "starting" });
  assert.deepEqual(readChange("stop", 409, { code: "agent_offline" }), { kind: "refused", of: "stop", code: "agent_offline" });
  // A start's reason on a stop, a stop's on a start, or no reason at all.
  assert.deepEqual(readChange("stop", 409, { code: "stopping" }), { kind: "unavailable" });
  assert.deepEqual(readChange("start", 409, { code: "agent_offline" }), { kind: "unavailable" });
  assert.deepEqual(readChange("start", 409, "no"), { kind: "unavailable" });
});

test("the site's own answers to a change are read as they are for a check", () => {
  assert.deepEqual(readChange("stop", 401, { code: "signed_out" }), { kind: "signed-out" });
  assert.deepEqual(readChange("start", 403, { code: "no_access" }), { kind: "denied" });
  assert.deepEqual(readChange("start", 503, { code: "access_unavailable" }), { kind: "access-failed" });
  assert.deepEqual(readChange("stop", 503, { code: "not_configured" }), { kind: "not-configured" });
  assert.deepEqual(readChange("start", 404, { code: "no_server" }), { kind: "no-server" });
  // The page never sends a stop without its confirmation, so this is the
  // backend not answering as far as the page can tell; and a failed change
  // carries no join link.
  assert.deepEqual(readChange("stop", 400, { code: "confirmation_required" }), { kind: "unavailable" });
  assert.deepEqual(readChange("stop", 503, { code: "tool_unavailable" }), { kind: "unavailable" });
  assert.deepEqual(readChange("start", 403, { code: "cross_site" }), { kind: "unavailable" });
});

test("every refusal says what to do instead, and a change on its way or unsure is said as such", () => {
  assert.match(refusalMessage({ of: "start", code: "stopping" }), /shutting down/);
  assert.match(refusalMessage({ of: "stop", code: "starting" }), /starting up/);
  assert.match(refusalMessage({ of: "stop", code: "agent_offline" }), /in a minute/);
  assert.equal(askingMessage("start"), "Asking Foundry to start…");
  assert.equal(askingMessage("stop"), "Asking Foundry to stop…");
  assert.match(unsureMessage("stop"), /stop request/);
  assert.match(unsureMessage("start"), /Check again/);
});

test("a stop that failed is read from the backend's checks, said, and shown with the state", () => {
  const status = parseStatus({ state: "ready", join_url: JOIN, checks: { host: "running", stop: "failed" } });
  assert.equal(status?.stopFailed, true);
  assert.equal(parseStatus({ state: "ready", join_url: JOIN, checks: { stop: "succeeded" } })?.stopFailed, undefined);
  assert.equal(parseStatus({ state: "ready", join_url: JOIN })?.stopFailed, undefined);

  const p = take(EMPTY, { kind: "status", status: status! }, T0)!;
  assert.equal(p.last?.stopFailed, true);
  const a = availability(p, T0)!;
  const d = describe(a, true, clock);
  assert.equal(d.label, "Foundry is responding");
  assert.ok(d.message.endsWith(STOP_FAILED));
  // It still offers Stop, so the Member can try again.
  assert.equal(control(a), "stop");
  // It is a change worth saying when it appears, and the key tells it apart.
  assert.notEqual(key(a), key(confirmed("ready")));
  assert.match(announcement(confirmed("ready"), a), /didn’t finish/);
  // Once the backend reports it no longer, the plain state is back.
  const q = take(p, { kind: "status", status: { state: "ready", joinUrl: JOIN } }, T0 + 1)!;
  assert.equal(q.last?.stopFailed, undefined);
  assert.equal(describe(availability(q, T0 + 1)!, true, clock).message.includes(STOP_FAILED), false);
});
