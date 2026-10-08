import { Robot } from "./Robot";
import { Ground, LeftFoliage, RightFoliage } from "./Scenery";

export type SignInNotice = "not-invited" | "failed" | "unavailable";

const NOTICES: Record<SignInNotice, string> = {
  "not-invited": "That address hasn’t been invited to The Lobby. Try signing in with the address your invitation went to.",
  failed: "Sign-in didn’t finish. Please try again.",
  unavailable: "Sign-in isn’t available right now. Please try again later.",
};

export function Welcome({ notice }: { notice?: SignInNotice }) {
  return (
    <div className="welcome">
      <div className="welcome-sun" aria-hidden="true" />
      <LeftFoliage className="welcome-leaves welcome-leaves-left" />
      <RightFoliage className="welcome-leaves welcome-leaves-right" />

      <header className="welcome-header">
        <a className="identity" href="/">
          The Lobby
        </a>
      </header>

      <main className="welcome-main">
        <h1 className="welcome-title">
          <span className="welcome-title-lead">Welcome to</span> <span className="welcome-title-name">The Lobby</span>
        </h1>
        <p className="welcome-purpose">A shared place for family and friends.</p>

        {notice && (
          <p className="welcome-notice" role="status">
            {NOTICES[notice]}
          </p>
        )}

        <div className="welcome-action">
          <a className="tactile tactile-hero" href="/signin">
            <span>Sign in</span>
            <svg className="tactile-arrow" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
              <path d="M4 12h15M13 5.5 19.5 12 13 18.5" />
            </svg>
          </a>
          <p className="welcome-guidance">Access is by invitation.</p>
          <Robot className="welcome-robot" wave />
        </div>
      </main>

      <footer className="welcome-footer">
        <Ground className="welcome-ground" />
        <div className="welcome-footer-bar">
          <p className="welcome-public">
            <a href="/soccer">Schedule Downloader</a>
            <span className="welcome-public-note">No sign-in needed</span>
          </p>
          <a href="/privacy">Privacy</a>
        </div>
      </footer>
    </div>
  );
}
