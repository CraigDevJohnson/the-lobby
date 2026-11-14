import { useCallback, useEffect, useId, useState } from "react";
import {
  changeOperator,
  changePlayer,
  EDITION,
  getArchives,
  getOperators,
  getPlayers,
  madeAt,
  nameProblem,
  operatorOutcome,
  playerOutcome,
  restoreArchive,
  restoreOutcome,
  samePlayer,
  sizeOf,
  type Archive,
  type Edition,
  type Player,
  type WorldOperators,
} from "./adminTools";

// Minecraft Admin's sections under the mods (docs/design/minecraft-brief.md,
// "Players, operators and backups, 2026-11-14"): the allowlist every world
// shares, each world's operators, and each world's backups with a restore.
// Each section reads its own list and says so when it can't. One change is
// sent at a time in a section, and the list is read again after it, so the
// page shows what the server has rather than what was asked.

type World = { id: string; name: string };

const key = (p: Player) => `${p.edition}:${p.name.toLowerCase()}`;

export function AdminTools({ worlds }: { worlds: World[] }) {
  // The allowlist is read here because the operators section offers its
  // players too. undefined: not read yet; null: couldn't be read.
  const [players, setPlayers] = useState<Player[] | null | undefined>(undefined);
  const readPlayers = useCallback(async () => setPlayers(await getPlayers()), []);
  useEffect(() => {
    void readPlayers();
  }, [readPlayers]);

  return (
    <>
      <Players players={players} read={readPlayers} />
      <Operators worlds={worlds} players={players ?? []} />
      <Backups worlds={worlds} />
    </>
  );
}

function Section({ title, lead, children }: { title: string; lead: string; children: React.ReactNode }) {
  const id = useId();
  return (
    <section className="mcx" aria-labelledby={id}>
      <h2 id={id} className="mcx-title">
        {title}
      </h2>
      <p className="mcx-lead">{lead}</p>
      {children}
    </section>
  );
}

// A list that couldn't be read, with a way to read it again.
function Unread({ what, retry }: { what: string; retry: () => void }) {
  return (
    <div className="mcx-unread" role="alert">
      <p>We can’t load {what} right now.</p>
      <button className="quiet-button" type="button" onClick={retry}>
        Retry
      </button>
    </div>
  );
}

function Said({ text }: { text: string }) {
  return (
    <p className="mcx-said" role="status">
      {text}
    </p>
  );
}

function Players({ players, read }: { players: Player[] | null | undefined; read: () => Promise<void> }) {
  const [edition, setEdition] = useState<Edition>("java");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [said, setSaid] = useState("");
  const [removing, setRemoving] = useState("");
  const field = useId();

  const change = async (operation: "add" | "remove", p: Player) => {
    if (busy) return;
    setBusy(true);
    setSaid("");
    const code = await changePlayer(operation, p);
    await read();
    setBusy(false);
    setRemoving("");
    setSaid(playerOutcome(operation, p, code));
    if (operation === "add" && code === "") setName("");
  };

  const add = (e: React.FormEvent) => {
    e.preventDefault();
    const typed = name.trim();
    const problem = nameProblem(edition, typed);
    if (problem) setSaid(problem);
    else void change("add", { edition, name: typed });
  };

  return (
    <Section title="Players" lead="Who can join. Every world shares this one allowlist.">
      {players === undefined && <p className="mcx-quiet">Loading the players…</p>}
      {players === null && <Unread what="the players" retry={() => void read()} />}
      {players && players.length === 0 && <p className="mcx-quiet">Nobody is on the allowlist yet.</p>}
      {players && players.length > 0 && (
        <ul className="mcx-list">
          {players.map((p) => (
            <li key={key(p)} className="mcx-item">
              <span className="mcx-name">
                {p.name} <span className="mcx-tag">{EDITION[p.edition]}</span>
              </span>
              {removing === key(p) ? (
                <span className="mcx-confirm">
                  <span>Remove {p.name}?</span>
                  <button
                    className="quiet-button"
                    type="button"
                    aria-disabled={busy || undefined}
                    onClick={() => void change("remove", p)}
                  >
                    {busy ? "Removing…" : "Yes, remove"}
                  </button>
                  <button className="quiet-button" type="button" onClick={() => setRemoving("")}>
                    Keep
                  </button>
                </span>
              ) : (
                <button className="quiet-button" type="button" onClick={() => setRemoving(key(p))}>
                  Remove<span className="visually-hidden"> {p.name}</span>
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {players !== undefined && players !== null && (
        <form className="mcx-add" onSubmit={add}>
          <fieldset className="mcx-edition">
            <legend className="mca-find-label">Edition</legend>
            {(["java", "bedrock"] as const).map((e) => (
              <label key={e}>
                <input type="radio" name={`${field}-edition`} checked={edition === e} onChange={() => setEdition(e)} />
                {EDITION[e]}
              </label>
            ))}
          </fieldset>
          <label className="mca-search" htmlFor={field}>
            <span className="mca-find-label">{edition === "java" ? "Java account name" : "Bedrock gamertag"}</span>
            <input
              id={field}
              type="text"
              value={name}
              autoComplete="off"
              autoCapitalize="off"
              spellCheck={false}
              maxLength={32}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <button className="tactile tactile-small" type="submit" aria-disabled={busy || undefined}>
            {busy && !removing ? "Adding…" : "Add player"}
          </button>
        </form>
      )}
      <Said text={said} />
    </Section>
  );
}

function Operators({ worlds, players }: { worlds: World[]; players: Player[] }) {
  const [listed, setListed] = useState<WorldOperators[] | null | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const [said, setSaid] = useState("");
  const [picked, setPicked] = useState<Record<string, string>>({});

  const read = useCallback(async () => setListed(await getOperators()), []);
  useEffect(() => {
    void read();
  }, [read]);

  const change = async (world: World, operation: "add" | "remove", p: Player) => {
    if (busy) return;
    setBusy(true);
    setSaid("");
    const code = await changeOperator(world.id, operation, p);
    await read();
    setBusy(false);
    setPicked((k) => ({ ...k, [world.id]: "" }));
    setSaid(operatorOutcome(operation, p, world.name, code));
  };

  return (
    <Section
      title="Operators"
      lead="An operator can change game rules, give items and stop the game from inside it. Only players on the allowlist can be one."
    >
      {listed === undefined && <p className="mcx-quiet">Loading the operators…</p>}
      {listed === null && <Unread what="the operators" retry={() => void read()} />}
      {listed &&
        worlds.map((w) => {
          const ops = listed.find((l) => l.id === w.id)?.operators ?? [];
          const offer = players.filter((p) => !ops.some((o) => samePlayer(o, p)));
          const choice = offer.find((p) => key(p) === picked[w.id]);
          return (
            <div key={w.id} className="mcx-world">
              <h3 className="mcx-world-name">{w.name}</h3>
              {ops.length === 0 ? (
                <p className="mcx-quiet">No operators.</p>
              ) : (
                <ul className="mcx-list">
                  {ops.map((p) => (
                    <li key={key(p)} className="mcx-item">
                      <span className="mcx-name">
                        {p.name} <span className="mcx-tag">{EDITION[p.edition]}</span>
                      </span>
                      <button
                        className="quiet-button"
                        type="button"
                        aria-disabled={busy || undefined}
                        onClick={() => void change(w, "remove", p)}
                      >
                        Remove<span className="visually-hidden"> {p.name} as an operator in {w.name}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
              {offer.length > 0 && (
                <div className="mcx-pick">
                  <label>
                    <span className="visually-hidden">Player to make an operator in {w.name}</span>
                    <select
                      value={picked[w.id] ?? ""}
                      onChange={(e) => setPicked((k) => ({ ...k, [w.id]: e.target.value }))}
                    >
                      <option value="">Choose a player</option>
                      {offer.map((p) => (
                        <option key={key(p)} value={key(p)}>
                          {p.name} ({EDITION[p.edition]})
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    className="quiet-button"
                    type="button"
                    aria-disabled={busy || !choice || undefined}
                    onClick={() => choice && void change(w, "add", choice)}
                  >
                    Make operator
                  </button>
                </div>
              )}
            </div>
          );
        })}
      <Said text={said} />
    </Section>
  );
}

// How many backups a world shows before "Show all".
const FIRST = 5;

function Backups({ worlds }: { worlds: World[] }) {
  return (
    <Section
      title="Backups"
      lead="Every stop saves a backup of the world. Restoring one replaces the world as it is now, and only works while the world is stopped."
    >
      {worlds.map((w) => (
        <WorldBackups key={w.id} world={w} />
      ))}
    </Section>
  );
}

function WorldBackups({ world }: { world: World }) {
  // "closed": not asked for yet, so a visit that never opens it asks nothing.
  const [archives, setArchives] = useState<Archive[] | null | "closed" | "loading">("closed");
  const [all, setAll] = useState(false);
  const [confirming, setConfirming] = useState("");
  const [busy, setBusy] = useState(false);
  const [said, setSaid] = useState("");

  const read = async () => {
    setArchives("loading");
    setArchives(await getArchives(world.id));
  };

  const restore = async (a: Archive) => {
    if (busy) return;
    setBusy(true);
    setSaid("");
    const code = await restoreArchive(world.id, a.name);
    setBusy(false);
    setConfirming("");
    setSaid(restoreOutcome(world.name, code));
  };

  const shown = Array.isArray(archives) ? (all ? archives : archives.slice(0, FIRST)) : [];

  return (
    <div className="mcx-world">
      <h3 className="mcx-world-name">{world.name}</h3>
      {archives === "closed" && (
        <button className="quiet-button" type="button" onClick={() => void read()}>
          Show backups<span className="visually-hidden"> of {world.name}</span>
        </button>
      )}
      {archives === "loading" && <p className="mcx-quiet">Loading the backups…</p>}
      {archives === null && <Unread what={`${world.name}’s backups`} retry={() => void read()} />}
      {Array.isArray(archives) && archives.length === 0 && <p className="mcx-quiet">No backups yet.</p>}
      {shown.length > 0 && (
        <ul className="mcx-list">
          {shown.map((a) => (
            <li key={a.name} className="mcx-item">
              <span className="mcx-name">
                {madeAt(a.made)} {sizeOf(a.size) && <span className="mcx-tag">{sizeOf(a.size)}</span>}
              </span>
              {confirming === a.name ? (
                <span className="mcx-confirm">
                  <span>Replace {world.name} with this backup? The world as it is now is replaced.</span>
                  <button
                    className="quiet-button"
                    type="button"
                    aria-disabled={busy || undefined}
                    onClick={() => void restore(a)}
                  >
                    {busy ? "Asking…" : "Yes, restore"}
                  </button>
                  <button className="quiet-button" type="button" onClick={() => setConfirming("")}>
                    Cancel
                  </button>
                </span>
              ) : (
                <button className="quiet-button" type="button" onClick={() => setConfirming(a.name)}>
                  Restore<span className="visually-hidden"> the backup from {madeAt(a.made)}</span>
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      {Array.isArray(archives) && archives.length > FIRST && !all && (
        <button className="quiet-button" type="button" onClick={() => setAll(true)}>
          Show all {archives.length}
        </button>
      )}
      {said && (
        <p className="mcx-said" role="status">
          {said} <a href="/minecraft">Open the Launcher</a>
        </p>
      )}
    </div>
  );
}
