// Calls to the site's /api/foundry routes. The site checks the Member's
// access first, then asks the Foundry backend; every status shown on the page
// comes from there, and so does the answer to a start or stop.
import { readAnswer, readChange, type Answer, type Change, type ChangeAnswer } from "./logic";

async function fetchJSON(path: string, init?: RequestInit): Promise<{ status: number; body: unknown } | null> {
  let res: Response;
  try {
    res = await fetch(`/api/foundry${path}`, {
      credentials: "same-origin",
      ...init,
      headers: { Accept: "application/json", ...init?.headers },
    });
  } catch {
    return null;
  }
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    // Not one of the site's own answers.
  }
  return { status: res.status, body };
}

export async function getStatus(): Promise<Answer> {
  const res = await fetchJSON("/server");
  return res ? readAnswer(res.status, res.body) : { kind: "unavailable" };
}

async function change(of: Change, body?: string): Promise<ChangeAnswer> {
  const res = await fetchJSON(`/server/${of}`, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : {},
    body,
  });
  return res ? readChange(of, res.status, res.body) : { kind: "unavailable" };
}

// Starting is idempotent at the backend: a second request while Foundry
// starts or runs gets the current status back.
export const startServer = () => change("start");

// Stopping sends the Member's confirmation that nobody is playing, which the
// page asks for first; the site refuses a stop without it and the backend
// can't see who is playing. A second request while Foundry stops gets the
// current status back.
export const stopServer = () => change("stop", JSON.stringify({ nobody_playing: true }));
