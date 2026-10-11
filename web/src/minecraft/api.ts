// Calls to the site's /api/minecraft routes. The site checks the Member's
// access first, then asks the Minecraft backend; every status shown on the
// page comes from there.
import { parseStatus, type ServerStatus } from "./logic";

export type Answer =
  | { kind: "status"; status: ServerStatus }
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

export const getStatus = () => call("/server");

// Starting is idempotent at the backend: a second request while the server
// starts or runs gets the same status back.
export const startServer = () => call("/server/start", { method: "POST" });
