import { ToolGlyph } from "./glyphs";
import { Robot } from "./Robot";
import type { Collection, ToolEntry } from "./tools";

export type LobbyState = { kind: "checking" } | { kind: "failed"; retrying: boolean } | { kind: "ready"; collection: Collection };

export function VipLobby({ state, onRetry }: { state: LobbyState; onRetry?: () => void }) {
  return (
    <div className="lobby">
      <header className="lobby-header">
        <div className="lobby-header-inner">
          <a className="identity identity-compact" href="/">
            The Lobby
          </a>
          <form method="post" action="/signout">
            <button className="quiet-button" type="submit">
              Sign out
            </button>
          </form>
        </div>
      </header>

      <main className="lobby-main" aria-busy={state.kind === "checking" ? true : undefined}>
        <h1 className="lobby-title">The VIP Lobby</h1>
        <p className="lobby-lead">Choose a Tool.</p>

        {state.kind === "checking" && (
          <p className="lobby-status" role="status">
            <span className="lobby-status-dots" aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            Checking your access…
          </p>
        )}

        {state.kind === "failed" && (
          <div className="lobby-problem" role="alert">
            <p className="lobby-problem-title">Your access couldn’t be checked just now.</p>
            <p>
              Your Tools will show once it works. The <a href="/soccer">Schedule Downloader</a> is open to everyone in
              the meantime.
            </p>
            <button className="tactile tactile-small" type="button" onClick={onRetry} disabled={state.retrying}>
              {state.retrying ? "Checking…" : "Retry"}
            </button>
          </div>
        )}

        {state.kind === "ready" && (
          <>
            <ToolSection id="your-tools" title="Your Tools" tools={state.collection.yours}>
              {state.collection.noPrivateGrants && (
                <p className="lobby-note">
                  No private Tools are assigned to you. The Schedule Downloader is open to everyone.
                </p>
              )}
            </ToolSection>
            {state.collection.admin.length > 0 && (
              <ToolSection id="admin-tools" title="Admin Tools" tools={state.collection.admin} />
            )}
          </>
        )}
      </main>

      <footer className="lobby-footer">
        <div className="lobby-footer-inner">
          <Robot className="lobby-robot" />
          <a href="/privacy">Privacy</a>
        </div>
      </footer>
    </div>
  );
}

function ToolSection({
  id,
  title,
  tools,
  children,
}: {
  id: string;
  title: string;
  tools: ToolEntry[];
  children?: React.ReactNode;
}) {
  return (
    <section className="tool-section" aria-labelledby={id}>
      <h2 id={id} className="tool-section-title">
        {title}
      </h2>
      <ul className="tool-grid">
        {tools.map((t) => (
          <li key={t.id} className="tool-card">
            <div className="tool-card-name">
              <ToolGlyph glyph={t.glyph} />
              <h3>{t.name}</h3>
            </div>
            <a className="tactile tactile-small" href={t.href}>
              Open<span className="visually-hidden"> {t.name}</span>
            </a>
          </li>
        ))}
      </ul>
      {children}
    </section>
  );
}
