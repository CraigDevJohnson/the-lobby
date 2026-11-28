import { useCallback, useEffect, useRef, useState } from "react";
import { ToolGlyph } from "../glyphs";
import { SignOut } from "../SignOut";
import { ToolFooter } from "../ToolChrome";
import { applyEdit, EditWishList, type Edit } from "./EditWishList";
import { limit, loadFamily, newID, removeList, saveList, type Answer, type Family, type ListKind, type Problem } from "./api";
import "./wish.css";

type View = { kind: "checking" } | Answer<Family>;

function accessLost(problem: Problem) {
  return ["signed_out", "no_access", "access_unavailable"].includes(problem.code);
}

function WishProblem({ problem, reload }: { problem: Problem; reload: () => void }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { heading.current?.focus(); }, []);
  const signedOut = problem.code === "signed_out";
  const denied = problem.code === "no_access";
  const title = signedOut ? "Sign in to use Wish Lists."
    : denied ? "You don’t have access to Wish Lists."
    : problem.code === "access_unavailable" ? "We couldn’t check your access."
    : "We couldn’t load the wish lists.";

  return (
    <section className="wish-panel wish-message" aria-labelledby="wish-problem-heading">
      <h2 id="wish-problem-heading" ref={heading} tabIndex={-1}>{title}</h2>
      {signedOut && <a className="tactile tactile-small" href="/signin?next=/wish">Sign in</a>}
      {!signedOut && !denied && <button className="tactile tactile-small" type="button" onClick={reload}>Retry</button>}
    </section>
  );
}

export function WishLists() {
  const [view, setView] = useState<View>({ kind: "checking" });
  const [request, setRequest] = useState(0);
  const reload = useCallback(() => {
    setView({ kind: "checking" });
    setRequest((id) => id + 1);
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    void loadFamily(controller.signal).then((answer) => {
      if (!controller.signal.aborted) setView(answer);
    });
    return () => controller.abort();
  }, [request]);
  useEffect(() => {
    function restore(event: PageTransitionEvent) {
      if (event.persisted) reload();
    }
    window.addEventListener("pageshow", restore);
    return () => window.removeEventListener("pageshow", restore);
  }, [reload]);

  return (
    <div className="page wish-page">
      <header className="page-header">
        <div className="page-header-inner">
          <a className="identity identity-compact" href="/">The Lobby</a>
          <nav className="wish-header-nav" aria-label="Account and Tools">
            {(view.kind !== "problem" || view.code !== "signed_out") && <a className="page-back" href="/">Back to Tools</a>}
            {(view.kind === "ok" || (view.kind === "problem" && view.code === "no_access")) && <SignOut />}
          </nav>
        </div>
      </header>
      <main className="wish-main" aria-busy={view.kind === "checking"}>
        <div className="wish-opening">
          <span className="wish-opening-icon"><ToolGlyph glyph="gift" /></span>
          <h1>Wish Lists</h1>
        </div>
        <noscript><p>Turn on JavaScript to open Wish Lists.</p></noscript>
        {view.kind === "checking" && (
          <p className="lobby-status" role="status">
            <span className="lobby-status-dots" aria-hidden="true"><span /><span /><span /></span>
            Checking your access…
          </p>
        )}
        {view.kind === "problem" && <WishProblem problem={view} reload={reload} />}
        {view.kind === "ok" && <FamilyLists initial={view.value} onAccessLost={setView} reload={reload} />}
      </main>
      <ToolFooter />
    </div>
  );
}

function FamilyLists({ initial, onAccessLost, reload }: {
  initial: Family;
  onAccessLost: (problem: Problem) => void;
  reload: () => void;
}) {
  const [family, setFamily] = useState(initial);
  const [selected, setSelected] = useState("");
  const [edit, setEdit] = useState<Edit | null>(null);
  const [saving, setSaving] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [status, setStatus] = useState("");
  const opener = useRef<HTMLElement | null>(null);
  const detailTitle = useRef<HTMLHeadingElement>(null);
  const sorted = [...family.lists].sort((a, b) => a.name.localeCompare(b.name));
  const personal = sorted.find((list) => list.editable && list.kind === "personal");
  const list = sorted.find((entry) => entry.id === selected) ?? personal ?? sorted[0];

  useEffect(() => {
    let checking: AbortController | undefined;

    function refreshOnReturn() {
      if (saving || document.visibilityState !== "visible") return;
      checking?.abort();
      const pending = new AbortController();
      checking = pending;
      void loadFamily(pending.signal).then((answer) => {
        if (pending.signal.aborted) return;
        if (answer.kind === "ok") setFamily(answer.value);
        else onAccessLost(answer);
      });
    }

    document.addEventListener("visibilitychange", refreshOnReturn);
    return () => {
      document.removeEventListener("visibilitychange", refreshOnReturn);
      checking?.abort();
    };
  }, [onAccessLost, saving]);

  function begin(next: Edit) {
    opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setStatus("");
    setProblem(null);
    setEdit(next);
  }

  function start(kind: ListKind) {
    begin({ kind: "list", list: { id: newID(), kind, name: "", revision: 0, wishes: [] } });
  }

  function close(restoreFocus: boolean) {
    setEdit(null);
    setProblem(null);
    requestAnimationFrame(() => {
      if (restoreFocus && opener.current?.isConnected) opener.current.focus();
      else detailTitle.current?.focus();
    });
  }

  async function submit(data: FormData) {
    if (!edit || saving) return;
    setSaving(true);
    setProblem(null);
    const answer = await (edit.kind === "remove-list" ? removeList(edit.list) : saveList(applyEdit(edit, data)));
    setSaving(false);

    if (answer.kind === "problem") {
      if (accessLost(answer)) onAccessLost(answer);
      else setProblem(answer);
      return;
    }
    const saved = answer.value;
    if (saved === true) {
      setFamily((current) => ({ ...current, lists: current.lists.filter((entry) => entry.id !== edit.list.id) }));
      setSelected("");
      setStatus("List removed.");
    } else {
      setFamily((current) => ({ ...current, lists: [...current.lists.filter((entry) => entry.id !== saved.id), saved] }));
      setSelected(saved.id);
      setStatus(edit.kind === "remove-wish" ? "Wish removed." : edit.kind === "wish" ? "Wish saved." : "List saved.");
    }
    close(false);
  }

  const creating = edit?.kind === "list" && edit.list.revision === 0;
  return (
    <div className="wish-layout">
      <section className="wish-panel wish-directory" aria-labelledby="wish-family-heading">
        <h2 id="wish-family-heading">Family lists</h2>
        {sorted.length > 0 && (
          <ul className="wish-people">
            {sorted.map((entry) => (
              <li key={entry.id}>
                <button
                  type="button"
                  className="wish-person"
                  aria-pressed={entry.id === list?.id && !creating}
                  aria-controls="wish-detail"
                  disabled={edit !== null}
                  onClick={() => { setSelected(entry.id); setStatus(""); }}
                >
                  <span>{entry.name}</span>
                  {entry.id === personal?.id && <small>Your list</small>}
                </button>
              </li>
            ))}
          </ul>
        )}
        <div className="wish-directory-actions">
          {!personal && <button className="quiet-button" type="button" disabled={edit !== null} onClick={() => start("personal")}>Start your list</button>}
          {family.canManage && <button className="quiet-button" type="button" disabled={edit !== null} onClick={() => start("managed")}>Add a managed list</button>}
        </div>
      </section>
      <section id="wish-detail" className="wish-panel wish-detail" aria-labelledby="wish-detail-heading">
        <div className="wish-detail-header">
          <h2 id="wish-detail-heading" ref={detailTitle} tabIndex={-1}>{list && !creating ? `${list.name}’s list` : "Wishes"}</h2>
          {list?.editable && !edit && (
            <div className="wish-actions">
              {list.wishes.length < limit.wishes && (
                <button
                  className="tactile tactile-small"
                  type="button"
                  onClick={() => begin({ kind: "wish", list, wish: { id: newID(), title: "" } })}
                >
                  Add a wish
                </button>
              )}
              <button className="quiet-button" type="button" onClick={() => begin({ kind: "list", list })}>Rename</button>
            </div>
          )}
        </div>
        {edit && (
          <EditWishList
            edit={edit}
            saving={saving}
            problem={problem}
            onSubmit={(data) => void submit(data)}
            onCancel={() => close(true)}
            onReload={reload}
          />
        )}
        <p className="wish-status" role="status">{status}</p>
        {!list && !edit && <p className="wish-empty">No lists yet. Start your list to add a gift idea.</p>}
        {list && !creating && (
          <>
            {list.wishes.length === 0 && !edit && <p className="wish-empty">This list has no wishes yet.</p>}
            {list.editable && list.wishes.length >= limit.wishes && !edit && (
              <p className="wish-empty">This list has {limit.wishes} wishes. Remove a wish to add another.</p>
            )}
            {list.wishes.length > 0 && (
              <ul className="wish-items">
                {list.wishes.map((wish) => (
                  <li key={wish.id} className="wish-item">
                    <div className="wish-item-copy">
                      <h3>{wish.title}</h3>
                      {wish.notes && <p>{wish.notes}</p>}
                      {wish.url && (
                        <a href={wish.url} target="_blank" rel="noreferrer" aria-label={`Open link for ${wish.title} in a new tab`}>
                          {new URL(wish.url).hostname}
                        </a>
                      )}
                    </div>
                    {list.editable && !edit && (
                      <div className="wish-actions">
                        <button
                          className="quiet-button"
                          type="button"
                          aria-label={`Edit ${wish.title}`}
                          onClick={() => begin({ kind: "wish", list, wish })}
                        >
                          Edit
                        </button>
                        <button
                          className="quiet-button"
                          type="button"
                          aria-label={`Remove ${wish.title}`}
                          onClick={() => begin({ kind: "remove-wish", list, wish })}
                        >
                          Remove
                        </button>
                      </div>
                    )}
                  </li>
                ))}
              </ul>
            )}
            {list.editable && !edit && (
              <button className="quiet-button wish-remove-list" type="button" onClick={() => begin({ kind: "remove-list", list })}>Remove list</button>
            )}
          </>
        )}
      </section>
    </div>
  );
}
