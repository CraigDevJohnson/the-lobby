// Lobby Admin's Site card, read from the site's /api/lobby/status, which
// checks the lobby-admin grant first (internal/members/status.go): which
// environment this is, the version it runs, whether Google and iCloud
// calendars are offered, and whether each Tool's backend answered a quick
// check just now. It answers "is it the site or the Tool?". It imports
// nothing, so Node's test runner can load it on its own.

// The Tools with a backend of their own, in the order the card lists them.
const BACKENDS = ["soccer", "minecraft", "foundry"] as const;

// Each yes or no is null when the site's answer didn't say.
export type Backend = { tool: string; configured: boolean | null; answered: boolean | null };

// commit is empty when the build recorded none, as on a developer's
// computer. committed is when that commit was made.
export type Version = { commit: string; committed: Date | null; modified: boolean };

export type Site = {
  environment: string;
  version: Version;
  calendars: boolean | null;
  backends: Backend[];
};

const yesNo = (raw: unknown) => (typeof raw === "boolean" ? raw : null);

function date(raw: unknown): Date | null {
  if (typeof raw !== "string" || raw === "") return null;
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? null : d;
}

const object = (raw: unknown) =>
  (typeof raw === "object" && raw !== null && !Array.isArray(raw) ? raw : {}) as Record<string, unknown>;

// parseSite reads the site's answer. A Tool the card doesn't know is left
// out, and so is a second row for the same Tool; anything missing is shown
// as not known. Only an answer that isn't an object at all is null.
export function parseSite(raw: unknown): Site | null {
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) return null;
  const r = raw as Record<string, unknown>;
  const v = object(r.version);
  const listed = Array.isArray(r.backends) ? r.backends.map(object) : [];
  const backends: Backend[] = [];
  for (const tool of BACKENDS) {
    const b = listed.find((e) => e.tool === tool);
    if (b) backends.push({ tool, configured: yesNo(b.configured), answered: yesNo(b.answered) });
  }
  return {
    environment: typeof r.environment === "string" ? r.environment : "",
    version: {
      commit: typeof v.commit === "string" ? v.commit.trim() : "",
      committed: date(v.committed),
      modified: v.modified === true,
    },
    calendars: yesNo(r.calendars),
    backends,
  };
}

// siteLead says which site this is, by its address and its environment.
export function siteLead(host: string, environment: string): string {
  switch (environment) {
    case "prod":
      return `This is ${host}, the production site.`;
    case "dev":
      return `This is ${host}, the dev site.`;
    case "local":
      return `This is ${host}, running on a developer’s computer.`;
    default:
      return `This is ${host}.`;
  }
}

export function backendWords(b: Backend): string {
  if (b.answered === true) return "Answering";
  if (b.configured === false) return "Not set up";
  if (b.configured === true && b.answered === false) return "Not answering";
  return "Not known";
}

export function calendarWords(offered: boolean | null): string {
  return offered === null ? "Not known" : offered ? "Offered" : "Not set up";
}

// versionWords names the commit by its short form, with when it was made in
// the reader's own time.
export function versionWords(v: Version): string {
  if (v.commit === "") return "Not known";
  const when = v.committed ? `, committed ${v.committed.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })}` : "";
  return `${v.commit.slice(0, 7)}${when}${v.modified ? ", with local changes" : ""}`;
}

// getSite asks the site; null when it couldn't say.
export async function getSite(): Promise<Site | null> {
  try {
    const res = await fetch("/api/lobby/status", { credentials: "same-origin", headers: { Accept: "application/json" } });
    if (!res.ok) return null;
    return parseSite(await res.json());
  } catch {
    return null;
  }
}
