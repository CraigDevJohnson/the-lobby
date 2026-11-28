import { type FormEvent, useEffect, useId, useRef } from "react";
import { limit, type ListInput, type Problem, type Wish, type WishList } from "./api";

export type Edit =
  | { kind: "list"; list: ListInput }
  | { kind: "wish"; list: WishList; wish: Wish }
  | { kind: "remove-list"; list: WishList }
  | { kind: "remove-wish"; list: WishList; wish: Wish };

function text(data: FormData, name: string): string {
  const value = data.get(name);
  return typeof value === "string" ? value.trim() : "";
}

export function applyEdit(edit: Exclude<Edit, { kind: "remove-list" }>, data: FormData): ListInput {
  const { list } = edit;
  switch (edit.kind) {
    case "list":
      return { ...list, name: text(data, "name") };
    case "wish": {
      const wish = {
        id: edit.wish.id,
        title: text(data, "title"),
        url: text(data, "url"),
        notes: text(data, "notes"),
      };
      const exists = list.wishes.some((item) => item.id === wish.id);
      return { ...list, wishes: exists ? list.wishes.map((item) => item.id === wish.id ? wish : item) : [...list.wishes, wish] };
    }
    case "remove-wish":
      return { ...list, wishes: list.wishes.filter((item) => item.id !== edit.wish.id) };
  }
}

function titleOf(edit: Edit): string {
  switch (edit.kind) {
    case "list":
      if (edit.list.revision > 0) return "Rename list";
      return edit.list.kind === "personal" ? "Start your list" : "Add a managed list";
    case "wish":
      return edit.wish.title ? "Edit wish" : "Add a wish";
    case "remove-list":
      return "Remove list?";
    case "remove-wish":
      return "Remove wish?";
  }
}

export function EditWishList({ edit, saving, problem, onSubmit, onCancel, onReload }: {
  edit: Edit;
  saving: boolean;
  problem: Problem | null;
  onSubmit: (form: FormData) => void;
  onCancel: () => void;
  onReload: () => void;
}) {
  const heading = useId();
  const first = useRef<HTMLInputElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    (first.current ?? cancel.current)?.focus();
  }, []);
  const removing = edit.kind === "remove-list" || edit.kind === "remove-wish";
  const conflict = problem?.code === "conflict";

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!saving && !conflict) onSubmit(new FormData(event.currentTarget));
  }

  return (
    <form className="wish-editor" aria-labelledby={heading} onSubmit={submit}>
      <h3 id={heading}>{titleOf(edit)}</h3>
      <fieldset disabled={saving}>
        {edit.kind === "list" && (
          <label>
            Name your family will see
            <input ref={first} name="name" type="text" required maxLength={limit.name} defaultValue={edit.list.name} />
          </label>
        )}
        {edit.kind === "wish" && (
          <>
            <label>
              Wish title
              <input ref={first} name="title" type="text" required maxLength={limit.title} defaultValue={edit.wish.title} />
            </label>
            <label>
              Link (optional)
              <input name="url" type="url" maxLength={limit.url} defaultValue={edit.wish.url} />
            </label>
            <label>
              Notes (optional)
              <textarea name="notes" rows={3} maxLength={limit.notes} defaultValue={edit.wish.notes} />
            </label>
          </>
        )}
        {edit.kind === "remove-list" && <p>Remove the list for {edit.list.name} and all its wishes?</p>}
        {edit.kind === "remove-wish" && <p>Remove “{edit.wish.title}” from this list?</p>}
        {problem && (
          <p className="wish-problem" role="alert">
            {conflict ? "This list changed. Reload the lists to see the saved version before editing again." : problem.message}
          </p>
        )}
        <div className="wish-actions">
          {conflict ? (
            <button className="tactile tactile-small" type="button" onClick={onReload}>Reload lists</button>
          ) : (
            <button className="tactile tactile-small" type="submit">
              {saving ? (removing ? "Removing…" : "Saving…") : removing ? "Remove" : "Save"}
            </button>
          )}
          {!conflict && <button ref={cancel} className="quiet-button" type="button" onClick={onCancel}>Cancel</button>}
        </div>
      </fieldset>
    </form>
  );
}
