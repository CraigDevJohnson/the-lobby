// What the Foundry page shows, worked out from what the site and the Foundry
// backend say (docs/foundry-backend.md). Nothing here guesses: "ready" means
// only that Foundry's join page answered, so the page says Foundry is
// responding and never that a game is open.

export const STATES = ["stopped", "starting", "ready", "stopping", "unresponsive", "unknown"] as const;
export type BackendState = (typeof STATES)[number];

export type Status = {
  state: BackendState;
  joinUrl?: string; // the configured join page, https only
  checkedAt?: number; // when the backend finished looking, in ms
};

// What one call to /api/foundry/server came back with.
export type Answer =
  | { kind: "status"; status: Status }
  | { kind: "no-server" } // the backend found no host, as in a rebuild
  | { kind: "unavailable"; joinUrl?: string } // no readable status; the site may still send the link
  | { kind: "signed-out" } // no Sign-in session, or it expired
  | { kind: "denied" } // a Member without Foundry's grant
  | { kind: "access-failed" } // the site couldn't check access
  | { kind: "not-configured" }; // the site has no Foundry backend

function isState(v: unknown): v is BackendState {
  return typeof v === "string" && (STATES as readonly string[]).includes(v);
}

// joinLink accepts only an https address, so nothing but a web page can be
// put behind Open Foundry.
export function joinLink(v: unknown): string | undefined {
  if (typeof v !== "string") return undefined;
  const raw = v.trim();
  if (!raw || raw.length > 2048) return undefined;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return undefined;
  }
  if (url.protocol !== "https:" || url.username || url.password || !url.hostname) return undefined;
  return url.href;
}

function time(v: unknown): number | undefined {
  if (typeof v !== "string") return undefined;
  const t = Date.parse(v);
  return Number.isFinite(t) ? t : undefined;
}

// parseStatus reads the backend's status. Anything without a known state is
// null, which the page treats like a backend that did not answer.
export function parseStatus(raw: unknown): Status | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as { state?: unknown; join_url?: unknown; checked_at?: unknown };
  if (!isState(r.state)) return null;
  const status: Status = { state: r.state };
  const link = joinLink(r.join_url);
  if (link) status.joinUrl = link;
  const checked = time(r.checked_at);
  if (checked !== undefined) status.checkedAt = checked;
  return status;
}

// readAnswer turns the site's reply into an Answer. Only the site's own codes
// mean something about access; anything else is a status the page can't read.
export function readAnswer(httpStatus: number, body: unknown): Answer {
  if (httpStatus === 200) {
    const status = parseStatus(body);
    return status ? { kind: "status", status } : { kind: "unavailable" };
  }
  const b = (body && typeof body === "object" ? body : {}) as { code?: unknown; join_url?: unknown };
  switch (b.code) {
    case "signed_out":
      return { kind: "signed-out" };
    case "no_access":
      return { kind: "denied" };
    case "access_unavailable":
      return { kind: "access-failed" };
    case "not_configured":
      return { kind: "not-configured" };
    case "no_server":
      if (httpStatus === 404) return { kind: "no-server" };
  }
  // The site sends the configured link with a failed status check to a
  // Member who passed the access check, so the check never blocks joining.
  const link = joinLink(b.join_url);
  return link ? { kind: "unavailable", joinUrl: link } : { kind: "unavailable" };
}

// A state the backend confirmed. "gone" is the backend's no_server: it found
// no Foundry host at all.
export type Confirmed = Exclude<BackendState, "unknown"> | "gone";

export type Reading = {
  state: Confirmed;
  checkedAt: number; // when it was checked, as shown on the page
  receivedAt: number; // when this page got it, by this browser's clock
};

// Everything the page knows about Foundry on this visit.
export type Picture = {
  last: Reading | null; // the newest confirmed state
  failedAt: number | null; // when the newest check failed, if it came after `last`
  failedCheckedAt?: number; // the backend's own time for that check, when it answered "unknown"
  link: string | null; // the join page, once the site has sent it
};

export const EMPTY: Picture = { last: null, failedAt: null, link: null };

// forget is what the page still knows after an answer about access or setup.
// A failed access check is not a change of access, so what this visit has
// learned waits, out of sight, for the answer after Retry; signing out,
// losing the grant, or a site without Foundry clears it.
export function forget(p: Picture, a: Answer): Picture {
  return a.kind === "access-failed" ? p : EMPTY;
}

// take folds an answer into what the page knows. Null means the answer was
// about access or setup, and nothing about Foundry stays on screen.
export function take(p: Picture, a: Answer, at: number): Picture | null {
  switch (a.kind) {
    case "status": {
      const link = a.status.joinUrl ?? p.link;
      if (a.status.state === "unknown") {
        return { last: p.last, failedAt: at, failedCheckedAt: a.status.checkedAt, link };
      }
      const last = { state: a.status.state, checkedAt: a.status.checkedAt ?? at, receivedAt: at };
      return { last, failedAt: null, link };
    }
    case "no-server":
      return { last: { state: "gone", checkedAt: at, receivedAt: at }, failedAt: null, link: p.link };
    case "unavailable":
      return { last: p.last, failedAt: at, link: a.joinUrl ?? p.link };
    default:
      return null;
  }
}

// After this long without a newer confirmation (the tab slept, say), a
// confirmed state is shown as possibly out of date.
export const STALE_MS = 5 * 60_000;

export type Availability =
  | { kind: "confirmed"; reading: Reading }
  | { kind: "stale"; last: Reading; cause: "failed" | "old" }
  | { kind: "unavailable"; at: number; checked: boolean };

// availability is what the page can honestly say now. Null until the first
// answer about Foundry.
export function availability(p: Picture, now: number): Availability | null {
  if (p.last && p.failedAt === null) {
    return now - p.last.receivedAt > STALE_MS
      ? { kind: "stale", last: p.last, cause: "old" }
      : { kind: "confirmed", reading: p.last };
  }
  if (p.last) return { kind: "stale", last: p.last, cause: "failed" };
  if (p.failedAt === null) return null;
  return p.failedCheckedAt !== undefined
    ? { kind: "unavailable", at: p.failedCheckedAt, checked: true }
    : { kind: "unavailable", at: p.failedAt, checked: false };
}

// key names the status on screen, so the page speaks only when it changes. A
// stale state's cause is part of it: a check failing after the state had
// already aged changes what the card says. The join link's address is not:
// it is the configured join page rather than a status, and when it moves,
// Open Foundry shows the new address.
export function key(a: Availability | null): string {
  if (!a) return "";
  if (a.kind === "confirmed") return `confirmed:${a.reading.state}`;
  return a.kind === "stale" ? `stale:${a.cause}` : a.kind;
}

export type Mark = "on" | "off" | "busy" | "problem" | "unknown";

const LABELS: Record<Confirmed, string> = {
  ready: "Foundry is responding",
  stopped: "Foundry is stopped",
  starting: "Foundry is starting",
  stopping: "Foundry is shutting down",
  unresponsive: "Foundry isn’t answering",
  gone: "Foundry has no server right now",
};

const MARKS: Record<Confirmed, Mark> = {
  ready: "on",
  stopped: "off",
  starting: "busy",
  stopping: "busy",
  unresponsive: "problem",
  gone: "problem",
};

const MEANINGS: Record<Confirmed, string> = {
  ready: "Its join page is answering. Whether a game is open yet, you’ll see in Foundry.",
  stopped: "It isn’t running, so there’s no game to join yet.",
  starting: "It’s on its way up. This page checks again every few seconds.",
  stopping: "There’s no game to join while it shuts down.",
  unresponsive: "Its server is on, but Foundry’s join page isn’t answering. If this lasts, let Craig know.",
  gone: "It may be in the middle of a rebuild. Check back later.",
};

// What a stale state was, in the past tense.
const WAS: Record<Confirmed, string> = {
  ready: "Foundry was responding",
  stopped: "Foundry was stopped",
  starting: "Foundry was starting",
  stopping: "Foundry was shutting down",
  unresponsive: "Foundry wasn’t answering",
  gone: "Foundry had no server",
};

export type Description = {
  label: string; // the state in a few words, beside its mark
  mark: Mark;
  stale: boolean; // the mark is drawn as no longer confirmed
  when: string; // when it was checked
  at: number; // the same moment, for a <time> element
  message: string; // what it means for joining
};

// describe says the state, when it was checked, and what it means. `clock`
// writes a moment as "at 7:42 PM" (see when).
export function describe(a: Availability, hasLink: boolean, clock: (at: number) => string): Description {
  switch (a.kind) {
    case "confirmed": {
      const s = a.reading.state;
      return {
        label: LABELS[s],
        mark: MARKS[s],
        stale: false,
        when: `Checked ${clock(a.reading.checkedAt)}`,
        at: a.reading.checkedAt,
        message: MEANINGS[s],
      };
    }
    case "stale":
      // The last state the backend confirmed, in the past tense, with its own
      // mark drawn open on a dashed outline, so it reads apart from both a
      // confirmed state and a status the page couldn't check.
      return {
        label: WAS[a.last.state],
        mark: MARKS[a.last.state],
        stale: true,
        when: `Last checked ${clock(a.last.checkedAt)}`,
        at: a.last.checkedAt,
        message:
          a.cause === "failed"
            ? "This may be out of date. We couldn’t check again just now."
            : "This may be out of date. That check was more than five minutes ago.",
      };
    case "unavailable":
      return {
        label: "We can’t check Foundry right now",
        mark: "unknown",
        stale: false,
        when: `${a.checked ? "Checked" : "Tried"} ${clock(a.at)}`,
        at: a.at,
        message: hasLink
          ? "That doesn’t mean it’s down. You can still try opening it."
          : "That doesn’t mean it’s down. Try checking again in a moment.",
      };
  }
}

// when writes a moment for "Checked …": the time alone on the same day,
// with the weekday within a week, and with the date beyond that.
export function when(at: number, now: number, locale?: string, timeZone?: string): string {
  const t = new Intl.DateTimeFormat(locale, { hour: "numeric", minute: "2-digit", timeZone }).format(at);
  const day = (ms: number) => new Intl.DateTimeFormat("en-CA", { dateStyle: "short", timeZone }).format(ms);
  if (day(at) === day(now)) return `at ${t}`;
  const d =
    Math.abs(now - at) < 6 * 24 * 60 * 60_000
      ? new Intl.DateTimeFormat(locale, { weekday: "long", timeZone }).format(at)
      : new Intl.DateTimeFormat(locale, { month: "short", day: "numeric", timeZone }).format(at);
  return `on ${d} at ${t}`;
}

// Open Foundry takes the custard face when opening it is the sensible next
// step: Foundry is responding, or its status can't be confirmed. When the
// backend has confirmed Foundry isn't answering players, it stays a working
// link with a quiet face and says why it may not open.
export function openIsPrimary(a: Availability): boolean {
  return a.kind !== "confirmed" || a.reading.state === "ready";
}

// Each says only whether the link may open, never that a game will be there.
const CAVEATS: Record<Confirmed, string> = {
  ready: "",
  stopped: "It won’t open while Foundry is stopped.",
  starting: "It may not open until Foundry is responding.",
  stopping: "It may not open while Foundry shuts down.",
  unresponsive: "It may not open until Foundry answers.",
  gone: "It won’t open while there’s no server.",
};

// caveat is the line under Open Foundry, or "" when it needs none.
export function caveat(a: Availability): string {
  if (a.kind === "confirmed") return CAVEATS[a.reading.state];
  return "We can’t confirm Foundry is up, so it may not open.";
}

// noLink is what Join in Foundry says when the page doesn't know the link:
// only a failed check or no server at all comes without it.
export function noLink(a: Availability): string {
  return a.kind === "confirmed"
    ? "There’s no Foundry server to open right now."
    : "Foundry’s link comes with its status, so it can’t be shown until a check gets through.";
}

// announcement is what a screen reader hears when what the page shows
// changes, or nothing when it did not.
export function announcement(before: Availability | null, after: Availability | null): string {
  if (!after || key(before) === key(after)) return "";
  switch (after.kind) {
    case "confirmed":
      return `${LABELS[after.reading.state]}.`;
    case "stale":
      return after.cause === "failed"
        ? "We couldn’t check Foundry again. The last check is shown."
        : "Foundry’s status may be out of date.";
    case "unavailable":
      return "We can’t check Foundry right now.";
  }
}

// rechecked is what Check again says when its answer left the status as it
// was, so pressing it is never met with silence. It speaks only of the
// status, as that is all key compares.
export function rechecked(a: Availability | null): string {
  return a?.kind === "confirmed"
    ? "Checked just now. The status is the same."
    : "Tried again just now. Still no answer.";
}

// STILL_NO_ACCESS is what the page says when Retry still can't check access.
export const STILL_NO_ACCESS = "Checked just now. We still couldn’t check your access.";

// How long the page waits before asking again: often while Foundry is on its
// way up or down, about once a minute otherwise.
export function pollDelay(a: Availability): number {
  return a.kind === "confirmed" && (a.reading.state === "starting" || a.reading.state === "stopping")
    ? 10_000
    : 60_000;
}

const SHORT: Record<Confirmed, string> = {
  ready: "Responding",
  stopped: "Stopped",
  starting: "Starting",
  stopping: "Shutting down",
  unresponsive: "Not answering",
  gone: "No server",
};

// pageTitle puts the state first, so a player waiting in another tab sees it.
export function pageTitle(a: Availability | null): string {
  const page = "Foundry · The Lobby";
  if (!a) return page;
  const state = a.kind === "confirmed" ? SHORT[a.reading.state] : a.kind === "stale" ? "Out of date" : "Can’t check";
  return `${state} · ${page}`;
}

// address is what Open Foundry opens, as a person would read it: the host,
// with its port when it has one.
export function address(link: string): string {
  return new URL(link).host;
}

// segments splits a value after each ".", ":" and "/", so on screen it wraps
// only between its parts. Joined, the parts are the value exactly.
export function segments(value: string): string[] {
  return value.match(/[^.:/]*[.:/]|[^.:/]+$/g) ?? [value];
}
