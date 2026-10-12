// Calls to the site's /api/minecraft routes. The site checks the Member's
// access first, then asks the Minecraft backend; every status shown on the
// page comes from there.
import { parseRefusal, parseStatus, type Refusal, type ServerStatus } from "./logic";

export type Answer =
  | { kind: "status"; status: ServerStatus }
  | ({ kind: "refused" } & Refusal) // the backend would not change the server, and said why
  | { kind: "signed-out" } // no Sign-in session, or it expired
  | { kind: "denied" } // a Member without the Launcher's grant
  | { kind: "access-failed" } // the site couldn't check access
  | { kind: "not-configured" } // no server is set up for the Launcher
  | { kind: "unavailable" }; // the backend didn't answer, or not readably

async function call(path: string, init?: RequestInit): Promise<Answer> {
  let res: Response;
  try {
    res = await fetch(`/api/minecraft${path}`, {
      credentials: "same-origin",
      ...init,
      headers: { Accept: "application/json", ...init?.headers },
    });
  } catch {
    return { kind: "unavailable" };
  }
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    // Not one of the site's own answers.
  }
  if (res.ok) {
    const status = parseStatus(body);
    return status ? { kind: "status", status } : { kind: "unavailable" };
  }
  if (res.status === 409) {
    const refusal = parseRefusal(body);
    return refusal ? { kind: "refused", ...refusal } : { kind: "unavailable" };
  }
  const code = (body as { code?: unknown } | null)?.code;
  switch (code) {
    case "signed_out":
      return { kind: "signed-out" };
    case "no_access":
      return { kind: "denied" };
    case "access_unavailable":
      return { kind: "access-failed" };
    case "not_configured":
    case "no_server":
      return { kind: "not-configured" };
    default:
      return { kind: "unavailable" };
  }
}

// Every answer but a refusal, which only Stop server can get.
export type StatusAnswer = Exclude<Answer, { kind: "refused" }>;

// Only stopping can be refused; a refusal anywhere else is an answer the page
// can't act on, so it counts as the backend not answering readably.
function unrefused(answer: Answer): StatusAnswer {
  return answer.kind === "refused" ? { kind: "unavailable" } : answer;
}

export const getStatus = async () => unrefused(await call("/server"));

// Starting is idempotent at the backend: a second request while the server
// starts or runs gets the same status back.
export const startServer = async () => unrefused(await call("/server/start", { method: "POST" }));

// Stopping is refused, with why, while anyone is playing; nothing changes
// then. A second request while the server stops gets the same status back.
export const stopServer = () => call("/server/stop", { method: "POST" });
