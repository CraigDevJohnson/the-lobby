import { ACCESS_LOGOUT } from "./SignOut";
import { Welcome } from "./Welcome";

// Where sign-out lands (POST /signout), and where a Sign in in the minute
// after goes back to. The site has already ended its own Sign-in session; the
// hidden frame opens Cloudflare Access's logout address (ACCESS_LOGOUT) to
// end Access's sign-in too. Access may still accept that sign-in for up to 30
// seconds, so the site holds Sign in for a minute (internal/web, the
// signed-out mark). A frame rather than a script, so it works without
// JavaScript, and the person stays on The Lobby.
export function SignedOut() {
  return (
    <>
      <Welcome notice="signed-out" />
      <iframe src={ACCESS_LOGOUT} title="Ending Cloudflare’s sign-in" hidden tabIndex={-1} />
    </>
  );
}
