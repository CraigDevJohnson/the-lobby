import { Welcome } from "./Welcome";

// Where sign-out lands (POST /signout). The site has already ended its own
// Sign-in session; the hidden frame opens Cloudflare Access's logout address
// on this same site, which ends the Member's Access sign-in and clears its
// cookie, so the next Sign in asks for a new code. Cloudflare answers that
// address itself; it never reaches the site's server. A frame rather than a
// script, so it works without JavaScript, and the person stays on The Lobby.
export function SignedOut() {
  return (
    <>
      <Welcome notice="signed-out" />
      <iframe src="/cdn-cgi/access/logout" title="Ending Cloudflare’s sign-in" hidden tabIndex={-1} />
    </>
  );
}
