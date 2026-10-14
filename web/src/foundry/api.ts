// Calls to the site's /api/foundry route. The site checks the Member's
// access first, then asks the Foundry backend; every status shown on the page
// comes from there.
import { readAnswer, type Answer } from "./logic";

export async function getStatus(): Promise<Answer> {
  let res: Response;
  try {
    res = await fetch("/api/foundry/server", {
      credentials: "same-origin",
      headers: { Accept: "application/json" },
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
  return readAnswer(res.status, body);
}
