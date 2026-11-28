import type { ReactNode } from "react";
import { Robot } from "./Robot";

// The public privacy note (docs/design/privacy-brief.md). It is the same page
// for Visitors and Members and is fully readable without JavaScript. It
// describes what the site does today, checked against the code (issue #5).
// Add to Google Calendar has its own topic (#52, ADR 0007), and so does Add
// to iCloud Calendar (ADR 0012), which covers its daily update (ADR 0015) and
// the email when the league moves a game (ADR 0017).

// Where privacy questions and membership-removal requests go.
const CONTACT = "privacy@craigdevjohnson.com";

// The date of the last change to what the note says.
const UPDATED = { iso: "2026-11-27", label: "27 November 2026" };

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
          can use, and when Craig added them or last changed their Tools. When Craig adds a Member on the site, The
          Lobby emails them their invite from lobby@craigdevjohnson.com, through Fastmail, Craig’s email provider.
          When a Member turns on daily calendar updates, The Lobby emails them what changed when the league moves a
          game it tracks in their Google or iCloud calendar. It sends no other email to that address.
        </p>
        <p>
          Cloudflare emails you a one-time code and sets a sign-in cookie of its own. The Lobby then sets a cookie
          holding a random code, not your address, and keeps a sign-in record with your address. You stay signed in
          for up to 90 days, renewed while you keep using the site. Craig can see the day you were last here and how
          many devices you’re signed in on, and can sign those devices out.
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
            once. It also deletes your remembered teams, stops any existing Member link and deletes your Google and iCloud connections.
            Any sign-in records left stop
            working and are deleted later.
          </p>
        </>
      ),
    },
  },
  {
    id: "wish-lists",
    title: "Wish Lists",
    keep: (
      <p>
        The Lobby keeps the names shown on wish lists and each wish’s title, link, and notes. Members Craig gives
        Wish Lists access can see the family’s lists. The Lobby opens a product link only when you choose it.
      </p>
    ),
    control: {
      title: "Changing or removing a list",
      body: (
        <p>
          You can edit or remove your personal list. Craig manages lists for people who don’t sign in. If Craig takes
          away your Wish Lists access, the family can still see your list. Contact Craig if you want it removed.
          Removing your membership deletes your personal list. Managed lists stay until Craig removes them.
        </p>
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
    title: "Remembered teams",
    keep: (
      <>
        <p>
          If Craig gives you Let’s Plan Soccer, The Lobby keeps the teams you remember with your address.
        </p>
        <p>
          Let’s Plan Soccer checks those Team IDs with the league for a daily update or when a calendar app fetches an
          existing Member link. When a team moves to its next Session, The Lobby keeps the new Team ID and the one before it.
        </p>
      </>
    ),
    control: {
      title: "Existing Member links",
      body: (
        <>
          <p>
            If you already have a Member link, anyone who has it can see those teams’ games without signing in.
            Choose Stop old Member link under Remembered teams to turn off that address.
          </p>
          <p>Forgetting a team takes its games out of the link on the next fetch and stops future daily updates for that team. Games already copied stay in your connected calendar.</p>
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
          calendar. The Lobby writes your checked games when you press Add. If you turn on daily updates, it also adds
          upcoming games for your remembered teams, moves games when the league changes their time or field, and emails
          you what changed. Both use the calendar you pick.
        </p>
        <p>
          The Lobby keeps, with your address, your Google account’s address, the calendar you picked, and Google’s
          access for The Lobby, encrypted with a key kept apart from it. For daily updates it keeps your switch setting,
          event IDs and each tracked game’s time and field. The soccer backend sees only your teams and games.
        </p>
      </>
    ),
    control: {
      title: "Disconnecting",
      body: (
        <>
          <p>Daily updates start off. Turn them off to stop scheduled writes and emails for this calendar.</p>
          <p>
            Disconnect ends The Lobby’s access at Google and deletes what it kept about your Google account. Games
            already added stay in your calendar; delete them there if you don’t want them.
          </p>
          <p>
            You can also remove The Lobby in your Google account’s third-party access settings. Removing your
            membership deletes the connection too. The earlier Let’s Plan Soccer on craigdevjohnson.com uses the same
            Google app, so disconnecting also ends a connection you made there.
          </p>
        </>
      ),
    },
  },
  {
    id: "icloud-calendar",
    title: "Add to iCloud Calendar",
    keep: (
      <>
        <p>
          If Craig gives you Let’s Plan Soccer, you can connect an iCloud account to add games to it. Apple has no
          sign-in screen for this, so you give The Lobby your Apple Account email and an app-specific password you make
          at Apple. Apple doesn’t limit that password to calendars or to The Lobby’s games: it also opens the other
          iCloud data outside apps can reach, such as mail and contacts.
        </p>
        <p>
          The Lobby uses it only to list your calendars and to add and update the games you add, in the calendar you
          pick, when you press Add. If you turn the daily update on, The Lobby also uses it once a day, whether or not
          you’re here, to add new games of your remembered teams to that calendar and to move a game there when the
          league moves it. It then emails you what changed, at the address you sign in to The Lobby with.
        </p>
        <p>
          The Lobby keeps, with your address, your Apple Account email, the calendar you picked, where Apple keeps your
          calendars, and the app-specific password, encrypted with a key kept apart from it. For the daily update it
          also keeps whether it’s on and a list of what it has added (game IDs and Team IDs, and each game’s time and
          field, no more), so it never adds a game twice and can tell when the league moves one. It never asks for or
          keeps your Apple Account’s own password. Let’s Plan Soccer sees only your Teams and games, never your Apple
          Account.
        </p>
      </>
    ),
    control: {
      title: "The daily update and disconnecting",
      body: (
        <>
          <p>
            The daily update is off until you turn it on, and you can turn it off again. Off, The Lobby adds games
            only when you press Add, and sends no email about moved games. The list of what it added stays until you
            disconnect or change calendars.
          </p>
          <p>
            Disconnect deletes what The Lobby kept about your Apple Account, the password and that list included, which
            ends the daily update. Games already added stay in your calendar; delete them there if you don’t want them.
          </p>
          <p>
            The Lobby can’t end the password at Apple; only you can. Remove it at account.apple.com under Sign-In and
            Security, App-Specific Passwords, and it stops working everywhere at once. Removing your membership deletes
            the connection too.
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
