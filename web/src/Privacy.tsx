import type { ReactNode } from "react";
import { Robot } from "./Robot";

// The public privacy note (docs/design/privacy-brief.md). It is the same page
// for Visitors and Members and is fully readable without JavaScript. It
// describes what the site does today, checked against the code (issue #5).
// Add to Google Calendar has its own topic (#52, ADR 0007).

// Where privacy questions and membership-removal requests go.
const CONTACT = "privacy@craigdevjohnson.com";

// The date of the last change to what the note says.
const UPDATED = { iso: "2026-11-05", label: "5 November 2026" };

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
            Signing out ends your sign-in to The Lobby on that browser and deletes its record. It also ends
            Cloudflare’s sign-in, so signing in again needs a new code. Your other devices stay signed in to The
            Lobby. You stay a Member.
          </p>
          <p>
            You can ask for your membership to be removed. That deletes your Member record and ends your access at
            once. It also deletes the Teams in My calendar, stops your Member link and deletes your Google connection.
            Any sign-in records left stop
            working and are deleted later. Your address stays on the invite list
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
          A Session link or a download saves no Teams. A Session link is the Team IDs you chose, written into the
          address, and carries nothing about you. A downloaded calendar file is made when you ask and isn’t kept.
        </p>
        <p>
          To show games, Let’s Plan Soccer sends the league only those Team IDs. Picking a calendar on the page only
          changes the steps it shows. A Session link or a download gives The Lobby no access to your calendar or your
          Google account.
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
    id: "my-calendar",
    title: "My calendar",
    keep: (
      <>
        <p>
          If Craig gives you Let’s Plan Soccer, The Lobby keeps the Teams you add to My calendar with your address, and
          a Member link: a secret address only your calendar app needs.
        </p>
        <p>
          Each time your calendar app fetches the Member link, Let’s Plan Soccer sends the league those Team IDs. When a
          Team moves on to its next Session, The Lobby keeps the new Team ID and the one before it.
        </p>
      </>
    ),
    control: {
      title: "Your Member link",
      body: (
        <>
          <p>
            Anyone who has your Member link can see your Teams’ games, and calendar apps fetch it without signing in.
            Replace link gives you a new one and stops the old one at once.
          </p>
          <p>Removing a Team from My calendar takes its games out on the link’s next update.</p>
        </>
      ),
    },
  },
  {
    id: "google-calendar",
    title: "Add to Google Calendar",
    keep: (
      <>
        <p>
          If Craig gives you Let’s Plan Soccer, you can connect a Google account to add games to it. Google then lets The
          Lobby see your list of calendars and see and change events in all of them; Google doesn’t limit it to one
          calendar. The Lobby only adds and updates the games you add, in the calendar you pick, and only when you press
          Add.
        </p>
        <p>
          The Lobby keeps, with your address, your Google account’s address, the calendar you picked, and Google’s
          access for The Lobby, encrypted with a key kept apart from it. Let’s Plan Soccer sees only your Teams and
          games, never your Google account.
        </p>
      </>
    ),
    control: {
      title: "Disconnecting",
      body: (
        <>
          <p>
            Disconnect ends The Lobby’s access at Google and deletes what it kept about your Google account. Games
            already added stay in your calendar; delete them there if you don’t want them.
          </p>
          <p>
            You can also remove The Lobby in your Google account’s third-party access settings. Removing your
            membership deletes the connection too.
          </p>
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
                happened, the address asked for (leaving out a Member link’s secret part), how your browser or calendar
                app describes itself, and whether it worked. If someone who isn’t a Member gets through Cloudflare’s sign-in, the address they used is logged.
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
