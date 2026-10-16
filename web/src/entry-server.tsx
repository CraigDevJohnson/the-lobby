import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { App, titles as appTitles } from "./App";
import { Foundry } from "./foundry/Foundry";
import { Launcher } from "./minecraft/Launcher";
import { Privacy } from "./Privacy";
import { SignedOut } from "./SignedOut";
import { LetsPlanSoccer } from "./soccer/LetsPlanSoccer";

// Each prerendered page, keyed by the value main.tsx reads from data-initial.
export type Page = "welcome" | "checking" | "privacy" | "soccer" | "minecraft" | "foundry" | "signed-out";

export const titles: Record<Page, string> = {
  ...appTitles,
  privacy: "Privacy · The Lobby",
  soccer: "Let’s Plan Soccer · The Lobby",
  minecraft: "Minecraft Launcher · The Lobby",
  foundry: "Foundry · The Lobby",
  "signed-out": "Signed out · The Lobby",
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
      ) : page === "foundry" ? (
        <Foundry />
      ) : page === "signed-out" ? (
        <SignedOut />
      ) : (
        <App initial={page} />
      )}
    </StrictMode>,
  );
}
