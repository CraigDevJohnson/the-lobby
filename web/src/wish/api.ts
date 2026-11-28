export type ListKind = "personal" | "managed";

export type Wish = {
  id: string;
  title: string;
  url?: string;
  notes?: string;
};

export type ListInput = {
  id: string;
  kind: ListKind;
  name: string;
  revision: number;
  wishes: Wish[];
};

export type WishList = ListInput & { editable: boolean };
export type Family = { lists: WishList[]; canManage: boolean };
export type Problem = { kind: "problem"; code: string; message: string };
export type Answer<T> = { kind: "ok"; value: T } | Problem;

export const limit = {
  name: 80,
  title: 120,
  notes: 500,
  url: 2048,
  wishes: 50,
};

const unavailable: Problem = {
  kind: "problem",
  code: "unavailable",
  message: "We couldn't reach Wish Lists. Try again.",
};

export function newID(): string {
  return crypto.randomUUID().replaceAll("-", "");
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function opaqueID(value: unknown): value is string {
  return typeof value === "string" && /^[a-f0-9]{32}$/.test(value);
}

function webLink(value: unknown): value is string {
  if (typeof value !== "string" || !/^https?:\/\//i.test(value)) return false;
  try {
    const url = new URL(value);
    return url.username === "" && url.password === "";
  } catch {
    return false;
  }
}

function parseWish(value: unknown): Wish | null {
  if (!record(value)) return null;
  const { id, title, url, notes } = value;
  if (!opaqueID(id) || typeof title !== "string") return null;
  if (notes !== undefined && typeof notes !== "string") return null;
  return { id, title, url: webLink(url) ? url : undefined, notes };
}

function parseList(value: unknown): WishList | null {
  if (!record(value)) return null;
  const { id, kind, name, revision, editable, wishes } = value;
  if (!opaqueID(id) || (kind !== "personal" && kind !== "managed") || typeof name !== "string") return null;
  if (typeof revision !== "number" || !Number.isSafeInteger(revision) || revision < 1) return null;
  if (typeof editable !== "boolean" || !Array.isArray(wishes)) return null;
  const items = wishes.map(parseWish);
  if (items.some((wish) => wish === null)) return null;
  return { id, kind, name, revision, editable, wishes: items.filter((wish) => wish !== null) };
}

function parseFamily(value: unknown): Family | null {
  if (!record(value) || !Array.isArray(value.lists) || typeof value.can_manage !== "boolean") return null;
  const lists = value.lists.map(parseList);
  if (lists.some((list) => list === null)) return null;
  return { lists: lists.filter((list) => list !== null), canManage: value.can_manage };
}

async function request<T>(path: string, options: RequestInit, parse: (value: unknown) => T | null): Promise<Answer<T>> {
  try {
    const response = await fetch(path, { credentials: "same-origin", cache: "no-store", ...options });
    const value: unknown = response.status === 204 ? undefined : await response.json();
    if (response.ok) {
      const parsed = parse(value);
      return parsed === null ? unavailable : { kind: "ok", value: parsed };
    }
    if (record(value) && typeof value.code === "string" && typeof value.message === "string") {
      return { kind: "problem", code: value.code, message: value.message };
    }
  } catch {
    return unavailable;
  }
  return unavailable;
}

export function loadFamily(signal?: AbortSignal) {
  return request("/api/wish", { signal }, parseFamily);
}

export function saveList({ id, kind, name, revision, wishes }: ListInput) {
  const items = wishes.map(({ id, title, url, notes }) => ({ id, title, url: url || undefined, notes: notes || undefined }));
  return request("/api/wish/lists", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, kind, name, revision, wishes: items }),
  }, parseList);
}

export function removeList({ id, kind, revision }: WishList) {
  return request("/api/wish/lists", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, kind, revision }),
  }, (value): true | null => value === undefined ? true : null);
}
