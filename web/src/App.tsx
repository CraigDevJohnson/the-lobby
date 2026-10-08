import { useCallback, useEffect, useState } from "react";
import { collectionFor } from "./tools";
import { VipLobby, type LobbyState } from "./VipLobby";
import { Welcome, type SignInNotice } from "./Welcome";

// The server picks the first state (ADR 0003): the public welcome, or
// "checking" when the request carried a Sign-in session cookie. Only the
// answer from /api/me ever puts private Tool names on screen.
export type Initial = "welcome" | "checking";

export const titles: Record<Initial, string> = {
  welcome: "Welcome to The Lobby",
  checking: "The VIP Lobby · The Lobby",
};

type View = { kind: "welcome"; notice?: SignInNotice } | { kind: "lobby"; lobby: LobbyState };

type Me = { role: "visitor" } | { role: "member"; email: string; tools?: string[] };

const NOTICES: readonly string[] = ["not-invited", "failed", "unavailable"];

async function fetchMe(): Promise<Me> {
  const res = await fetch("/api/me", { credentials: "same-origin", headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(`access check: ${res.status}`);
  return (await res.json()) as Me;
}

export function App({ initial }: { initial: Initial }) {
  const [view, setView] = useState<View>(
    initial === "checking" ? { kind: "lobby", lobby: { kind: "checking" } } : { kind: "welcome" },
  );

  const check = useCallback(async () => {
    try {
      const me = await fetchMe();
      if (me.role === "member") {
        setView({ kind: "lobby", lobby: { kind: "ready", collection: collectionFor(me.tools ?? []) } });
      } else {
        // Signed out or expired: no private content stays on screen.
        setView({ kind: "welcome" });
      }
    } catch {
      // A failed check is never shown as an empty collection, and earlier
      // results are dropped rather than presented as still confirmed.
      setView({ kind: "lobby", lobby: { kind: "failed", retrying: false } });
    }
  }, []);

  const retry = useCallback(() => {
    setView({ kind: "lobby", lobby: { kind: "failed", retrying: true } });
    void check();
  }, [check]);

  // First load: the welcome reads any sign-in outcome from the address; the
  // lobby asks the server who this is.
  useEffect(() => {
    if (initial === "checking") {
      void check();
      return;
    }
    const url = new URL(window.location.href);
    const notice = url.searchParams.get("signin");
    if (notice && NOTICES.includes(notice)) {
      setView({ kind: "welcome", notice: notice as SignInNotice });
    }
    if (url.searchParams.has("signin")) {
      url.searchParams.delete("signin");
      window.history.replaceState(null, "", url.pathname + url.search + url.hash);
    }
  }, [initial, check]);

  // Grants can change while the page sits open; re-check when it is shown again.
  const inLobby = view.kind === "lobby";
  useEffect(() => {
    if (!inLobby) return;
    const onVisible = () => {
      if (document.visibilityState === "visible") void check();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [inLobby, check]);

  useEffect(() => {
    document.title = titles[view.kind === "lobby" ? "checking" : "welcome"];
  }, [view.kind]);

  return view.kind === "welcome" ? (
    <Welcome notice={view.notice} />
  ) : (
    <VipLobby state={view.lobby} onRetry={retry} />
  );
}
