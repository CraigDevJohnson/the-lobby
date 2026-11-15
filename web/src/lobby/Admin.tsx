import { useCallback, useEffect, useId, useRef, useState } from "react";
import { changePlayer, EDITION, nameProblem, playerOutcome, type Edition, type Player } from "../minecraft/adminTools";
import { SignOut } from "../SignOut";
import { ToolFooter } from "../ToolChrome";
import { TOOLS } from "../tools";
import {
  addedLine,
  addMember,
  addOutcome,
  emailProblem,
  getRoster,
  grantOutcome,
  inviteText,
  lastHereLine,
  MINECRAFT,
  offersAllowlist,
  otherGrants,
  OWN_GRANT,
  playerLabel,
  removeMember,
  removeOutcome,
  setGrant,
  signOutMember,
  signOutOutcome,
  unsureNote,
  type Answer,
  type Member,
  type Roster,
} from "./members";

// Lobby Admin (docs/design/vip-lobby-brief.md, "Lobby Admin, 2026-11-15"):
// the Owner's screen for who can sign in and which Tools each Member can
// open, for Members with the lobby-admin grant. It arrives as a checking
// state with nothing private in it; the Members appear only once the site
// has confirmed the grant. It uses Minecraft Admin's cards and rows.
//
// For someone who also holds minecraft-admin, a Member given Minecraft
// Launcher can have their player put on the Minecraft allowlist through
// Minecraft Admin's route (#100). The site keeps no player name, so taking
// a player off the allowlist stays on Minecraft Admin.

type View =
  | { kind: "checking" }
  | { kind: "signed-out" }
  | { kind: "denied" }
  | { kind: "access-failed"; retrying: boolean; retried?: boolean }
  // note: a change the page couldn't confirm, kept until the list can be
  // read again.
  | { kind: "unavailable"; retrying: boolean; retried?: boolean; note?: string }
  | { kind: "roster"; roster: Roster };

// The one change on its way, so every control waits for it, and which
// control it came from.
type Pending =
  | { kind: "grant"; email: string; tool: string; granted: boolean }
  | { kind: "remove"; email: string }
  | { kind: "sign-out"; email: string }
  // An add, with its player's trip to the allowlist when one was typed.
  | { kind: "add" }
  // A row's player on its way to the allowlist.
  | { kind: "allowlist"; email: string };

const GRANTS = TOOLS.map((t) => ({ id: t.id, name: t.name }));
const KNOWN = GRANTS.map((g) => g.id);
const nameOf = (id: string) => GRANTS.find((g) => g.id === id)?.name ?? id;

const STILL_NO_ACCESS = "Checked just now. We still can’t check your access.";
const STILL_UNAVAILABLE = "Checked just now. We still can’t load the Members.";

export function LobbyAdmin() {
  const [view, setView] = useState<View>({ kind: "checking" });
  const [pending, setPending] = useState<Pending | null>(null);
  // What the last change to the list did, and what the last add did.
  const [listSaid, setListSaid] = useState<React.ReactNode>("");
  const [added, setAdded] = useState<{ text: string; player?: string; invite?: string } | null>(null);
  // The address whose Remove is asking first.
  const [removing, setRemoving] = useState("");
  // The address just given Minecraft Launcher whose row asks for their
  // player, until it is answered or another change starts.
  const [offering, setOffering] = useState("");
  // Today, as of the newest list, so "last here" is said against the
  // reader's own calendar; nothing reads it before the first list.
  const [now, setNow] = useState(() => new Date(0));
  const latest = useRef(0);
  const problemTitle = useRef<HTMLParagraphElement>(null);
  const title = useRef<HTMLHeadingElement>(null);
  const listStatus = useRef<HTMLParagraphElement>(null);
  // Said aloud only: a Retry that came back with the same problem.
  const [said, setSaid] = useState("");

  // show puts the newest answer on screen; an older one arriving late is
  // dropped. A refusal leaves the view as it is.
  const show = useCallback((answer: Answer, id: number) => {
    if (id !== latest.current) return false;
    switch (answer.kind) {
      case "roster":
        setView({ kind: "roster", roster: answer.roster });
        setNow(new Date());
        break;
      case "access-failed":
      case "unavailable":
        setView({ kind: answer.kind, retrying: false });
        break;
      case "signed-out":
      case "denied":
        setView({ kind: answer.kind });
        break;
    }
    return true;
  }, []);

  const load = useCallback(async () => {
    const id = ++latest.current;
    const answer = await getRoster();
    return { answer, shown: show(answer, id) };
  }, [show]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    document.title = "Lobby Admin · The Lobby";
  }, []);

  // Retry keeps focus where it is and says so when nothing changed; when
  // the Members come back, focus goes to the page's first heading.
  const retry = useCallback(async () => {
    if ((view.kind !== "access-failed" && view.kind !== "unavailable") || view.retrying) return;
    const kind = view.kind;
    const note = view.kind === "unavailable" ? view.note : undefined;
    setView({ kind, retrying: true, note });
    setSaid("");
    const { answer, shown } = await load();
    if (shown && answer.kind === kind) {
      setView({ kind, retrying: false, retried: true, note });
      requestAnimationFrame(() => setSaid(kind === "access-failed" ? STILL_NO_ACCESS : STILL_UNAVAILABLE));
    }
    else if (shown) requestAnimationFrame(() => (problemTitle.current ?? title.current)?.focus());
  }, [view, load]);

  // change sends one change and shows the list it answers with. A refusal
  // or an answer the page can't read is followed by a fresh read, so the
  // page never shows something the table doesn't have; if that read fails
  // too, the page says it can't tell whether the change was made.
  const change = useCallback(
    async (p: Pending, send: () => Promise<Answer>) => {
      if (pending) return "busy";
      setPending(p);
      setOffering("");
      if (p.kind === "add") setAdded(null);
      else setListSaid("");
      const id = ++latest.current;
      const answer = await send();
      if (answer.kind === "roster") {
        setPending(null);
        show(answer, id);
        return "";
      }
      if (answer.kind !== "refused" && answer.kind !== "unavailable") {
        setPending(null);
        show(answer, id);
        return answer.kind;
      }
      const code = answer.kind === "refused" ? answer.code : "unavailable";
      const { answer: reread, shown } = await load();
      setPending(null);
      if (shown && reread.kind === "unavailable" && code === "unavailable") {
        setView({ kind: "unavailable", retrying: false, note: unsureNote(p.kind === "add") });
      }
      return code;
    },
    [pending, show, load],
  );

  // Only someone who can change the allowlist is offered it.
  const allowlist = view.kind === "roster" && offersAllowlist(view.roster);

  const grant = async (m: Member, tool: string, granted: boolean) => {
    const code = await change({ kind: "grant", email: m.email, tool, granted }, () => setGrant(m.email, tool, granted));
    if (code === "busy") return;
    const said = grantOutcome(m.email, nameOf(tool), granted, code);
    if (code !== "" || tool !== MINECRAFT || !allowlist) setListSaid(said);
    else if (granted) {
      setListSaid(said);
      // The row asks one question at a time.
      setRemoving((r) => (r === m.email ? "" : r));
      setOffering(m.email);
    } else {
      // The site never knew their player, so it can't take them off.
      setListSaid(
        <>
          {said} Their player name stays on the allowlist until you remove it on{" "}
          <a href="/minecraft/admin">Minecraft Admin</a>.
        </>,
      );
    }
  };

  // putOnAllowlist sends a Member's player to the allowlist and keeps
  // nothing. It is the one change on its way until the answer, which the
  // site waits up to 23 seconds for while a world is running.
  const putOnAllowlist = async (m: Member, p: Player) => {
    if (pending) return;
    setPending({ kind: "allowlist", email: m.email });
    setListSaid("");
    const code = await changePlayer("add", p);
    setPending(null);
    setOffering("");
    setListSaid(playerOutcome("add", p, code, false));
    // The question and its buttons are gone, so focus goes to what happened.
    requestAnimationFrame(() => listStatus.current?.focus());
  };

  const remove = async (m: Member) => {
    const code = await change({ kind: "remove", email: m.email }, () => removeMember(m.email));
    if (code === "busy") return;
    setRemoving("");
    setListSaid(removeOutcome(m.email, code));
    // The row and its buttons are gone, so focus goes to what happened.
    requestAnimationFrame(() => listStatus.current?.focus());
  };

  const signOut = async (m: Member, self: boolean) => {
    const code = await change({ kind: "sign-out", email: m.email }, () => signOutMember(m.email));
    if (code === "busy") return;
    setListSaid(signOutOutcome(m.email, self, code));
    requestAnimationFrame(() => listStatus.current?.focus());
  };

  // add adds a Member and then, only once they are one, sends their player
  // to the allowlist, so both are the one change and Add Member waits for
  // them together.
  const add = async (email: string, tools: string[], player?: Player) => {
    const typed = email.trim().toLowerCase();
    let joined = "";
    const code = await change({ kind: "add" }, async () => {
      const answer = await addMember(typed, tools);
      if (answer.kind === "roster" && player) joined = playerOutcome("add", player, await changePlayer("add", player), false);
      return answer;
    });
    if (code === "busy") return false;
    setAdded({
      text: addOutcome(typed, code),
      player: joined || undefined,
      invite: code === "" ? inviteText(window.location.origin, typed) : undefined,
    });
    return code === "";
  };

  const member = view.kind !== "checking" && view.kind !== "signed-out";

  return (
    <div className="page lobby-admin">
      <header className="page-header">
        <div className="page-header-inner">
          <a className="identity identity-compact" href="/">
            The Lobby
          </a>
          <nav className="la-header-nav" aria-label="Account and Tools">
            {view.kind !== "signed-out" && (
              <a className="page-back" href="/">
                Back to Tools
              </a>
            )}
            {member && <SignOut />}
          </nav>
        </div>
      </header>

      <main className="la-main" aria-busy={view.kind === "checking" ? true : undefined}>
        <h1 ref={title} className="la-title" tabIndex={-1}>
          Lobby Admin
        </h1>
        {(view.kind === "checking" || view.kind === "roster") && (
          <p className="la-lead">Who can sign in to The Lobby, and which Tools each Member can open.</p>
        )}

        <noscript>
          <p className="la-problem-inline">Lobby Admin needs JavaScript to load and change the Members.</p>
        </noscript>

        {view.kind === "checking" && (
          <p className="lobby-status la-checking" role="status">
            <span className="lobby-status-dots" aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            Checking your access…
          </p>
        )}

        {view.kind === "signed-out" && (
          <Problem
            titleRef={problemTitle}
            title="Sign in to use Lobby Admin."
            detail="You’ll come back here after signing in."
          >
            <a className="tactile tactile-small" href="/signin?next=/lobby/admin">
              Sign in
            </a>
          </Problem>
        )}

        {view.kind === "denied" && (
          <Problem titleRef={problemTitle} title="You don’t have access to this Tool.">
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </Problem>
        )}

        {(view.kind === "access-failed" || view.kind === "unavailable") && (
          <Problem
            titleRef={problemTitle}
            title={view.kind === "access-failed" ? "We couldn’t check your access." : "We can’t load the Members right now."}
            detail={
              view.kind === "access-failed"
                ? "This doesn’t mean your access has changed."
                : view.note
                  ? `${view.note} Retry to see what the site has.`
                  : "Nothing has been changed."
            }
          >
            <button
              className="tactile tactile-small"
              type="button"
              aria-disabled={view.retrying ? true : undefined}
              onClick={() => void retry()}
            >
              {view.retrying ? "Checking…" : "Retry"}
            </button>
            {view.retried && !view.retrying && (
              <p className="la-rechecked">{view.kind === "access-failed" ? STILL_NO_ACCESS : STILL_UNAVAILABLE}</p>
            )}
          </Problem>
        )}

        {view.kind === "roster" && (
          <>
            <Members
              roster={view.roster}
              now={now}
              pending={pending}
              removing={removing}
              offering={offering}
              said={listSaid}
              statusRef={listStatus}
              onGrant={(m, tool, granted) => void grant(m, tool, granted)}
              onAskRemove={setRemoving}
              onRemove={(m) => void remove(m)}
              onSignOut={(m, self) => void signOut(m, self)}
              onAllowlist={(m, p) => void putOnAllowlist(m, p)}
              onNotNow={() => setOffering("")}
            />
            <Invite
              pending={pending}
              added={added}
              allowlist={allowlist}
              onAdd={add}
              onTyped={() => setAdded(null)}
            />
          </>
        )}

        <p className="visually-hidden" role="status">
          {said}
        </p>
      </main>

      <ToolFooter />
    </div>
  );
}

function Members({
  roster,
  now,
  pending,
  removing,
  offering,
  said,
  statusRef,
  onGrant,
  onAskRemove,
  onRemove,
  onSignOut,
  onAllowlist,
  onNotNow,
}: {
  roster: Roster;
  now: Date;
  pending: Pending | null;
  removing: string;
  offering: string;
  said: React.ReactNode;
  statusRef: React.RefObject<HTMLParagraphElement | null>;
  onGrant: (m: Member, tool: string, granted: boolean) => void;
  onAskRemove: (email: string) => void;
  onRemove: (m: Member) => void;
  onSignOut: (m: Member, self: boolean) => void;
  onAllowlist: (m: Member, p: Player) => void;
  onNotNow: () => void;
}) {
  const busy = pending !== null || undefined;
  return (
    <section className="mcx la-members" aria-labelledby="la-members-title">
      <h2 id="la-members-title" className="mcx-title">
        Members
      </h2>
      <p className="mcx-lead">
        Everyone who can sign in to {window.location.host}, which keeps its own list. A change counts from the
        Member’s next page.
      </p>
      {roster.members.length === 0 ? (
        <p className="mcx-quiet">Nobody is a Member yet.</p>
      ) : (
        <ul className="mcx-list">
          {roster.members.map((m) => {
            const self = m.email === roster.you;
            const extra = otherGrants(m, KNOWN);
            const added = addedLine(m, now);
            return (
              <li key={m.email} className="mcx-item la-member">
                <div className="la-who">
                  <p className="mcx-name">
                    {m.email}
                    {self && <span className="mcx-tag la-you">You</span>}
                  </p>
                  <p className="la-meta">
                    {added && `${added} · `}
                    {lastHereLine(m, now)}
                  </p>
                </div>

                <fieldset className="la-tools">
                  <legend className="la-tools-label">
                    Tools<span className="visually-hidden"> {m.email} can open</span>
                  </legend>
                  <div className="la-tool-grid">
                    {GRANTS.map((g) => {
                      const mine = self && g.id === OWN_GRANT;
                      const asking = pending?.kind === "grant" && pending.email === m.email && pending.tool === g.id;
                      const on = asking ? pending.granted : m.tools.includes(g.id);
                      return (
                        <label key={g.id} className="la-tool" data-mine={mine || undefined}>
                          <input
                            type="checkbox"
                            checked={on}
                            disabled={mine}
                            aria-disabled={(!mine && busy) || undefined}
                            aria-describedby={mine ? `la-own-${g.id}` : undefined}
                            onChange={() => {
                              if (!pending && !mine) onGrant(m, g.id, !on);
                            }}
                          />
                          {g.name}
                        </label>
                      );
                    })}
                  </div>
                  {self && (
                    <p id={`la-own-${OWN_GRANT}`} className="la-note">
                      Your own Lobby Admin stays on, so you can’t lock yourself out.
                    </p>
                  )}
                  {extra.length > 0 && <p className="la-note">Also holds {extra.join(", ")}, which this page doesn’t offer.</p>}
                </fieldset>

                <div className="la-acts" data-confirm={removing === m.email || offering === m.email || undefined}>
                  {removing === m.email ? (
                    <span className="mcx-confirm">
                      <span>Remove {m.email}? Their Tools, calendar link and saved teams go too.</span>
                      <button className="quiet-button" type="button" aria-disabled={busy} onClick={() => onRemove(m)}>
                        {pending?.kind === "remove" ? "Removing…" : "Yes, remove"}
                      </button>
                      <button className="quiet-button" type="button" onClick={() => onAskRemove("")}>
                        Keep
                      </button>
                    </span>
                  ) : offering === m.email ? (
                    <AllowlistAsk pending={pending} onAdd={(p) => onAllowlist(m, p)} onNotNow={onNotNow} />
                  ) : (
                    <>
                      {(self ? m.devices > 1 : m.devices > 0) && (
                        <button
                          className="quiet-button"
                          type="button"
                          aria-disabled={busy}
                          onClick={() => !pending && onSignOut(m, self)}
                        >
                          {pending?.kind === "sign-out" && pending.email === m.email
                            ? "Signing out…"
                            : self
                              ? "Sign out my other devices"
                              : "Sign out everywhere"}
                          {!self && <span className="visually-hidden"> {m.email}</span>}
                        </button>
                      )}
                      {!self && (
                        <button className="quiet-button" type="button" onClick={() => onAskRemove(m.email)}>
                          Remove<span className="visually-hidden"> {m.email}</span>
                        </button>
                      )}
                    </>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      <p ref={statusRef} className="mcx-said" role="status" tabIndex={-1}>
        {said}
      </p>
    </section>
  );
}

// The question a Member's row asks once they can open Minecraft Launcher:
// their player, for the allowlist. Not now closes it and sends nothing.
function AllowlistAsk({
  pending,
  onAdd,
  onNotNow,
}: {
  pending: Pending | null;
  onAdd: (p: Player) => void;
  onNotNow: () => void;
}) {
  const [edition, setEdition] = useState<Edition>("java");
  const [name, setName] = useState("");
  const [problem, setProblem] = useState("");
  const ask = useId();
  const busy = pending !== null || undefined;

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (pending) return;
    const typed = name.trim();
    const p = nameProblem(edition, typed);
    setProblem("");
    // Cleared first, so pressing Add to allowlist again says it again.
    if (p) requestAnimationFrame(() => setProblem(p));
    else onAdd({ edition, name: typed });
  };

  return (
    <form className="la-ask" aria-labelledby={ask} onSubmit={submit} noValidate>
      <p id={ask} className="la-ask-line">
        Add their player name to the allowlist?
      </p>
      <PlayerFields
        edition={edition}
        name={name}
        problem={problem}
        optional={false}
        onEdition={(e) => {
          setEdition(e);
          setProblem("");
        }}
        onName={(n) => {
          setName(n);
          setProblem("");
        }}
      />
      <span className="mcx-confirm">
        <button className="quiet-button" type="submit" aria-disabled={busy}>
          {pending?.kind === "allowlist" ? "Adding…" : "Add to allowlist"}
        </button>
        <button className="quiet-button" type="button" aria-disabled={busy} onClick={() => !pending && onNotNow()}>
          Not now
        </button>
      </span>
    </form>
  );
}

// A player's edition and name, as Minecraft Admin's Players card asks for
// them, with what is wrong with the name under it.
function PlayerFields({
  edition,
  name,
  problem,
  optional,
  onEdition,
  onName,
}: {
  edition: Edition;
  name: string;
  problem: string;
  optional: boolean;
  onEdition: (e: Edition) => void;
  onName: (name: string) => void;
}) {
  const field = useId();
  return (
    <>
      <fieldset className="mcx-edition">
        <legend className="mca-find-label">Edition</legend>
        {(["java", "bedrock"] as const).map((e) => (
          <label key={e}>
            <input type="radio" name={`${field}-edition`} checked={edition === e} onChange={() => onEdition(e)} />
            {EDITION[e]}
          </label>
        ))}
      </fieldset>
      <label className="mca-search la-player-name" htmlFor={field}>
        <span className="mca-find-label">{playerLabel(edition, optional)}</span>
        <input
          id={field}
          type="text"
          value={name}
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
          maxLength={32}
          aria-invalid={problem ? true : undefined}
          aria-describedby={problem ? `${field}-problem` : undefined}
          onChange={(e) => onName(e.target.value)}
        />
      </label>
      {problem && (
        <p id={`${field}-problem`} className="la-problem-line" role="alert">
          {problem}
        </p>
      )}
    </>
  );
}

function Invite({
  pending,
  added,
  allowlist,
  onAdd,
  onTyped,
}: {
  pending: Pending | null;
  added: { text: string; player?: string; invite?: string } | null;
  allowlist: boolean;
  onAdd: (email: string, tools: string[], player?: Player) => Promise<boolean>;
  onTyped: () => void;
}) {
  const [email, setEmail] = useState("");
  const [tools, setTools] = useState<string[]>([]);
  const [problem, setProblem] = useState("");
  const [edition, setEdition] = useState<Edition>("java");
  const [name, setName] = useState("");
  const [nameSaid, setNameSaid] = useState("");
  const [copied, setCopied] = useState(false);
  const field = useId();
  const adding = pending?.kind === "add";
  // The player is asked for, and sent, only while Minecraft Launcher is
  // ticked.
  const asking = allowlist && tools.includes(MINECRAFT);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (pending) return;
    const typed = asking ? name.trim() : "";
    const p = emailProblem(email);
    const q = typed ? nameProblem(edition, typed) : "";
    setProblem("");
    setNameSaid("");
    setCopied(false);
    if (p || q) {
      // Cleared first, so pressing Add Member again says it again.
      requestAnimationFrame(() => {
        setProblem(p);
        setNameSaid(q);
      });
      return;
    }
    if (await onAdd(email, tools, typed ? { edition, name: typed } : undefined)) {
      setEmail("");
      setTools([]);
      setEdition("java");
      setName("");
    }
  };

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return (
    <section className="mcx la-invite" aria-labelledby="la-invite-title">
      <h2 id="la-invite-title" className="mcx-title">
        Invite a Member
      </h2>
      <p className="mcx-lead">Their email address is how they sign in. Tick the Tools they can open; you can change them later.</p>
      <form className="la-add" onSubmit={(e) => void submit(e)} noValidate>
        <label className="mca-search la-email" htmlFor={field}>
          <span className="mca-find-label">Email address</span>
          <input
            id={field}
            type="email"
            value={email}
            autoComplete="off"
            autoCapitalize="off"
            spellCheck={false}
            maxLength={254}
            aria-invalid={problem ? true : undefined}
            aria-describedby={problem ? `${field}-problem` : undefined}
            onChange={(e) => {
              setEmail(e.target.value);
              setProblem("");
              if (added) onTyped();
            }}
          />
        </label>
        {problem && (
          <p id={`${field}-problem`} className="la-problem-line" role="alert">
            {problem}
          </p>
        )}
        <fieldset className="la-tools">
          <legend className="la-tools-label">Tools</legend>
          <div className="la-tool-grid">
            {GRANTS.map((g) => (
              <label key={g.id} className="la-tool">
                <input
                  type="checkbox"
                  checked={tools.includes(g.id)}
                  onChange={(e) => setTools((t) => (e.target.checked ? [...t, g.id] : t.filter((x) => x !== g.id)))}
                />
                {g.name}
              </label>
            ))}
          </div>
        </fieldset>
        {asking && (
          <fieldset className="la-tools la-player">
            <legend className="la-tools-label">Minecraft player</legend>
            <PlayerFields
              edition={edition}
              name={name}
              problem={nameSaid}
              optional
              onEdition={(e) => {
                setEdition(e);
                setNameSaid("");
              }}
              onName={(n) => {
                setName(n);
                setNameSaid("");
                if (added) onTyped();
              }}
            />
          </fieldset>
        )}
        <button className="tactile tactile-small" type="submit" aria-disabled={pending !== null || undefined}>
          {adding ? "Adding…" : "Add Member"}
        </button>
      </form>
      <div className="mcx-said la-added" role="status">
        {added && <p>{added.text}</p>}
        {added?.invite && (
          <>
            <p className="la-invite-text">{added.invite}</p>
            <button className="quiet-button" type="button" onClick={() => void copy(added.invite!)}>
              {copied ? "Copied" : "Copy invite"}
            </button>
          </>
        )}
        {/* After the invite, so "Send them this:" leads straight to it. */}
        {added?.player && <p>{added.player}</p>}
      </div>
    </section>
  );
}

function Problem({
  title,
  detail,
  titleRef,
  children,
}: {
  title: string;
  detail?: string;
  titleRef: React.RefObject<HTMLParagraphElement | null>;
  children: React.ReactNode;
}) {
  return (
    <div className="lobby-problem">
      <div className="la-problem-says" role="alert">
        <p ref={titleRef} className="lobby-problem-title" tabIndex={-1}>
          {title}
        </p>
        {detail && <p>{detail}</p>}
      </div>
      {children}
    </div>
  );
}
