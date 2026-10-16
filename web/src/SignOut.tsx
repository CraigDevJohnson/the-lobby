// Cloudflare Access's logout address on this same site. Cloudflare answers it
// itself: it ends the Member's Access sign-in and clears its cookie, so the
// next Sign in asks for a new code. It never reaches the site's server.
export const ACCESS_LOGOUT = "/cdn-cgi/access/logout";

// The header's Sign out. A plain form, so it works without JavaScript, and
// the signed-out page it lands on ends Access's sign-in (SignedOut.tsx). With
// JavaScript it also starts ending it as the button is pressed, so that still
// happens if the tab is closed before the signed-out page loads.
export function SignOut() {
  return (
    <form method="post" action="/signout" onSubmit={endAccessSignIn}>
      <button className="quiet-button" type="submit">
        Sign out
      </button>
    </form>
  );
}

function endAccessSignIn() {
  fetch(ACCESS_LOGOUT, { credentials: "same-origin", cache: "no-store", keepalive: true }).catch(() => {});
}
