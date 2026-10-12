import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { App, titles as appTitles } from "./App";
import { Launcher } from "./minecraft/Launcher";
import { Privacy } from "./Privacy";
import { LetsPlanSoccer } from "./soccer/LetsPlanSoccer";

// Each prerendered page, keyed by the value main.tsx reads from data-initial.
export type Page = "welcome" | "checking" | "privacy" | "soccer" | "minecraft";

export const titles: Record<Page, string> = {
  ...appTitles,
  privacy: "Privacy · The Lobby",
  soccer: "Let’s Plan Soccer · The Lobby",
  minecraft: "Minecraft Launcher · The Lobby",
};

export function render(page: Page): string {
  return renderToString(
    <StrictMode>
      {page === "privacy" ? (
        <Privacy />
      ) : page === "soccer" ? (
        <LetsPlanSoccer />
      ) : page === "minecraft" ? (
        <Launcher />
      ) : (
        <App initial={page} />
      )}
    </StrictMode>,
  );
}
