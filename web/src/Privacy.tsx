import type { ReactNode } from "react";
import { Robot } from "./Robot";

// The public privacy note (docs/design/privacy-brief.md). It is the same page
// for Visitors and Members and is fully readable without JavaScript. It
// describes what the site does today, checked against the code (issue #5).
// Remembered teams, Member links and Connected calendars get their own topic
// here when they ship, not before.

// Where privacy questions and membership-removal requests go.
const CONTACT = "privacy@craigdevjohnson.com";

// The date of the last change to what the note says.
const UPDATED = { iso: "2026-10-16", label: "16 October 2026" };

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
          Craig invites each Member by email address. For each Member, The Lobby keeps that address, which Tools they
          can use, and when Craig added them or last changed their Tools.
        </p>
        <p>
          Cloudflare emails you a one-time code and sets a sign-in cookie of its own. The Lobby then sets a cookie
          holding a random code, not your address, and keeps a sign-in record with your address. You stay signed in
          for up to 90 days, renewed while you keep using the site.
        </p>
      </>
    ),
    control: {
      title: "Signing out and leaving",
      body: (
        <>
          <p>
            Signing out ends your sign-in on that browser and deletes its record. Other devices stay signed in. You
            stay a Member.
          </p>
          <p>
            You can ask for your membership to be removed. That deletes your Member record and ends your access at
            once. Any sign-in records left stop working and are deleted later. Your address stays on the invite list
            Cloudflare checks until Craig takes it off, which is a separate step.
          </p>
        </>
      ),
    },
  },
  {
    id: "session-links",
    title: "Session links and downloads",
    keep: (
      <>
        <p>
          Let’s Plan Soccer has no accounts and saves no Teams. A Session link is the Team IDs you chose, written into
          the address, and carries nothing about you. A downloaded calendar file is made when you ask and isn’t kept.
        </p>
        <p>
          To show games, Let’s Plan Soccer sends the league only those Team IDs. Picking a calendar on the page only
          changes the steps it shows. The Lobby gets no access to your calendar or your Google account.
        </p>
      </>
    ),
    control: {
      title: "Sharing a link",
      body: (
        <>
          <p>
            Anyone who has a Session link can use it, and calendar apps fetch it without signing in. It doesn’t expire
            and can’t be switched off, so a link you’ve shared can’t be taken back.
          </p>
          <p>Removing a Team on the page changes only the next link or download you make.</p>
        </>
      ),
    },
  },
  {
    id: "game-servers",
    title: "Minecraft and Foundry",
    keep: (
      <>
        <p>
          The Minecraft Launcher and Foundry pass your email address to the program that runs each game server. When a
          Member starts or stops a server, that program logs their address so Craig can see who asked. Other Members
          aren’t shown who it was.
        </p>
        <p>
          Craig adds your Minecraft account and Foundry player name on the game servers himself. The Lobby never asks
          for them. The game servers keep their own lists, logs and backups.
        </p>
      </>
    ),
    control: {
      title: "Your device and game accounts",
      body: (
        <>
          <p>
            The Minecraft Launcher’s joining help remembers the device you pick, in that browser only. Signing out
            doesn’t clear it.
          </p>
          <p>
            Removing your membership doesn’t remove your Minecraft account or Foundry player name. Ask Craig for those
            too.
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
              <strong>Visiting</strong> Let’s Plan Soccer is open to everyone, with no sign-in needed.
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
                Like most websites, it writes service logs. For each request that reaches it, they record when it
                happened, the address asked for, how your browser or calendar app describes itself, and whether it
                worked. If someone who isn’t a Member gets through Cloudflare’s sign-in, the address they used is logged.
              </p>
              <p>
                The logs are there to find and fix problems and to show who started or stopped a game server. Downloads,
                new Session links and the calendar apps that fetch them are counted from these logs. The site adds no
                analytics or tracking code to its pages.
              </p>
              <p>
                The Lobby and its Tools keep these logs for 30 days. Cloudflare and Amazon Web Services may keep their
                own records of the requests they handle, under their own policies.
              </p>
            </div>
          </section>

          <section className="privacy-running" aria-labelledby="questions">
            <h2 id="questions" className="privacy-topic">
              Questions
            </h2>
            <div className="privacy-running-body">
              <p>
                To ask about this note or to have your membership removed, email{" "}
                <a href={`mailto:${CONTACT}`}>{CONTACT}</a>. There is no self-service form.
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
