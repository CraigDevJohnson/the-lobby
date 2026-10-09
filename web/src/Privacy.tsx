import type { ReactNode } from "react";
import { Robot } from "./Robot";

// The public privacy note (docs/design/privacy-brief.md). It is the same page
// for Visitors and Members and is fully readable without JavaScript. It
// describes the agreed first release; check each row against the behavior
// actually released before publishing a change to it.

// The date of the last change to what the note says.
const UPDATED = { iso: "2026-10-09", label: "9 October 2026" };

type Topic = {
  id: string;
  title: string;
  keep: ReactNode;
  control: { title: string; body: ReactNode };
};

const TOPICS: Topic[] = [
  {
    id: "signing-in",
    title: "Signing in",
    keep: (
      <>
        <p>
          Craig invites each Member by email address. For each Member, The Lobby keeps that address and which Tools
          they can use.
        </p>
        <p>
          Signing in sets a cookie that keeps you signed in for up to 90 days. It renews while you keep using the
          site.
        </p>
      </>
    ),
    control: {
      title: "Signing out and leaving",
      body: (
        <>
          <p>Signing out ends that sign-in. You stay a Member.</p>
          <p>
            You can ask for your membership to be removed. That deletes your access record and your Remembered teams.
          </p>
        </>
      ),
    },
  },
  {
    id: "teams-and-links",
    title: "Remembered teams and calendar links",
    keep: (
      <>
        <p>
          Members can save Remembered teams, so the Schedule Downloader can follow them from one Soccer Session to
          the next without hunting for new Team IDs.
        </p>
        <p>
          A Session link covers one Session’s games and works for anyone who has it. A Member link is private to you
          and follows your Remembered teams. Any calendar app holding a working link can fetch its games without
          signing in.
        </p>
      </>
    ),
    control: {
      title: "Replacing your link",
      body: (
        <>
          <p>
            You can replace your Member link whenever you like. The old one stops working, which withdraws it from
            anyone you shared it with.
          </p>
          <p>Dropping a Remembered team stops following it and removes its upcoming games.</p>
        </>
      ),
    },
  },
  {
    id: "google-calendar",
    title: "Connected Google calendars",
    keep: (
      <>
        <p>
          When you connect Google Calendar, Google asks you to let The Lobby see your list of calendars, so you can
          pick where games go, and to view and edit events across your calendars.
        </p>
        <p>
          That permission is broader than what The Lobby does with it. It adds games to the calendar you pick and
          changes only the events it added there, updating times, places and scores once a day while leaving your
          notes, reminders and colours alone.
        </p>
      </>
    ),
    control: {
      title: "Disconnecting",
      body: (
        <>
          <p>
            Disconnecting removes the upcoming games The Lobby added. Games already played stay in your calendar.
          </p>
          <p>
            It also withdraws The Lobby’s access at Google, so Google asks for your agreement again the next time you
            sign in with it.
          </p>
        </>
      ),
    },
  },
];

export function Privacy() {
  return (
    <div className="page">
      <header className="page-header">
        <div className="page-header-inner">
          <a className="identity identity-compact" href="/">
            The Lobby
          </a>
          <a className="page-back" href="/">
            Back to The Lobby
          </a>
        </div>
      </header>

      <main className="privacy-ground">
        <div className="privacy">
          <div className="privacy-opening">
            <h1 className="privacy-title">Privacy</h1>
            <p className="privacy-lead">How The Lobby uses information when you visit, sign in, and use its Tools.</p>
            <p className="privacy-visiting">
              <strong>Visiting</strong> The Schedule Downloader is open to everyone, with no sign-in needed.
            </p>
          </div>

          <div className="privacy-pairs">
            <div className="privacy-columns" aria-hidden="true">
              <span />
              <span>What we keep</span>
              <span>What you control</span>
            </div>

            {TOPICS.map((t) => (
              <section key={t.id} className="privacy-pair" aria-labelledby={t.id}>
                <h2 id={t.id} className="privacy-topic">
                  {t.title}
                </h2>
                <div className="privacy-keep">
                  <h3 className="privacy-side">What we keep</h3>
                  {t.keep}
                </div>
                <div className="privacy-control">
                  <h3 className="privacy-side">
                    <span className="visually-hidden">What you control: </span>
                    {t.control.title}
                  </h3>
                  {t.control.body}
                </div>
              </section>
            ))}
          </div>

          <section className="privacy-running" aria-labelledby="running-the-site">
            <h2 id="running-the-site" className="privacy-topic">
              Running the site
            </h2>
            <div className="privacy-running-body">
              <p>
                The Lobby runs on Amazon Web Services, with Cloudflare in front to deliver and protect it, so both handle
                your requests.
              </p>
              <p>
                Like most websites, it writes service logs of each request: the page asked for, how your browser
                describes itself, and whether it worked. They are there to find and fix problems. Counts of which
                download buttons and calendar apps get used come from those logs, not from an analytics service.
              </p>
            </div>
          </section>

          <section className="privacy-running" aria-labelledby="questions">
            <h2 id="questions" className="privacy-topic">
              Questions
            </h2>
            <div className="privacy-running-body">
              <p>
                To ask about this note or to have your membership removed, ask Craig, who invited you. There is no
                self-service form.
              </p>
              <p className="privacy-updated">
                Updated <time dateTime={UPDATED.iso}>{UPDATED.label}</time>
              </p>
            </div>
          </section>
        </div>
      </main>

      <footer className="page-footer">
        <div className="page-footer-inner">
          <Robot className="lobby-robot" />
          <a href="/">Back to The Lobby</a>
        </div>
      </footer>
    </div>
  );
}
