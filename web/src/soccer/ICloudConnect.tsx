import { useState, type FormEvent } from "react";
import { connectICloud, ToolError, type CalendarStatus } from "./api";
import { Icon, ToolDown } from "./parts";

export function ICloudConnect({
  onStatus,
  working,
  onWorking,
}: {
  onStatus: (s: CalendarStatus) => void;
  working: boolean;
  onWorking: (busy: boolean) => void;
}) {
  const [appleID, setAppleID] = useState("");
  const [password, setPassword] = useState("");
  const [failure, setFailure] = useState<"denied" | "tool" | null>(null);
  const denied = failure === "denied";

  async function connect(e: FormEvent) {
    e.preventDefault();
    if (working || !appleID.trim() || !password.trim()) return;
    onWorking(true);
    setFailure(null);
    try {
      const now = await connectICloud(appleID.trim(), password.trim());
      setPassword("");
      onStatus(now);
    } catch (err) {
      setFailure(err instanceof ToolError && err.kind === "denied" ? "denied" : "tool");
    } finally {
      onWorking(false);
    }
  }

  return (
    <form className="soccer-act" onSubmit={(e) => void connect(e)}>
      <div className="soccer-requires">
        <p className="soccer-requires-title">This needs an app-specific password</p>
        <p>
          Apple has no sign-in screen for this. You make a separate password in your Apple Account and give it to The
          Lobby. Apple doesn’t limit it to calendars: it also opens the other iCloud data outside apps can reach, such
          as mail and contacts. The Lobby uses it only to list your calendars and add games: the ones you add, and new
          ones once a day if you turn the daily update on, which also moves a game the league has moved.{" "}
          <a href="/privacy#icloud-calendar">What The Lobby keeps</a>
        </p>
      </div>
      <ol className="soccer-steps" role="list">
        <li>
          Sign in at{" "}
          <a href="https://account.apple.com/" target="_blank" rel="noreferrer">
            account.apple.com
          </a>
          . Your account needs two-factor authentication.
        </li>
        <li>
          Open <b>Sign-In and Security</b> → <b>App-Specific Passwords</b> and make one named <b>The Lobby</b>.
        </li>
        <li>Enter it below with your Apple Account email. Never enter your Apple Account’s own password here.</li>
      </ol>

      <div className="soccer-icloud-field">
        <label className="soccer-label" htmlFor="icloud-apple-id">
          Apple Account email
        </label>
        <input
          id="icloud-apple-id"
          className="soccer-input"
          type="email"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          required
          value={appleID}
          onChange={(e) => setAppleID(e.target.value)}
        />
      </div>
      <div className="soccer-icloud-field">
        <label className="soccer-label" htmlFor="icloud-password">
          App-specific password
        </label>
        <input
          id="icloud-password"
          className="soccer-input"
          type="password"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          required
          aria-invalid={denied || undefined}
          aria-describedby={denied ? "icloud-denied" : undefined}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </div>
      {denied && (
        <p id="icloud-denied" className="soccer-blocked" role="alert">
          Apple didn’t accept that email and password. Check both, and that it’s an app-specific password, not your
          Apple Account’s own.
        </p>
      )}
      <button type="submit" className="tactile soccer-action soccer-icloud-connect" disabled={working}>
        <span>{working ? "Checking with Apple…" : "Connect iCloud Calendar"}</span>
        <Icon name="arrow" />
      </button>
      {failure === "tool" && <ToolDown what="iCloud Calendar wasn’t connected." />}
    </form>
  );
}
