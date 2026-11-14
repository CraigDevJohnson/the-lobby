import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { App, titles as appTitles } from "./App";
import { FamilyDevices } from "./Devices";
import { Foundry, FoundryAdmin } from "./foundry/Foundry";
import { MinecraftAdmin } from "./minecraft/Admin";
import { Launcher } from "./minecraft/Launcher";
import { Privacy } from "./Privacy";
import { SignedOut } from "./SignedOut";
import { LetsPlanSoccer } from "./soccer/LetsPlanSoccer";

// Each prerendered page, keyed by the value main.tsx reads from data-initial.
export type Page =
  | "welcome"
  | "checking"
  | "privacy"
  | "soccer"
  | "minecraft"
  | "minecraft-admin"
  | "foundry"
  | "foundry-admin"
  | "signed-out"
  | "devices";

export const titles: Record<Page, string> = {
  ...appTitles,
  privacy: "Privacy · The Lobby",
  soccer: "Let’s Plan Soccer · The Lobby",
  minecraft: "Minecraft Launcher · The Lobby",
  "minecraft-admin": "Minecraft Admin · The Lobby",
  foundry: "Foundry · The Lobby",
  "foundry-admin": "Foundry Admin · The Lobby",
  "signed-out": "Signed out · The Lobby",
  devices: "Family devices · The Lobby",
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
      ) : page === "minecraft-admin" ? (
        <MinecraftAdmin />
      ) : page === "foundry" ? (
        <Foundry />
      ) : page === "foundry-admin" ? (
        <FoundryAdmin />
      ) : page === "signed-out" ? (
        <SignedOut />
      ) : page === "devices" ? (
        <FamilyDevices />
      ) : (
        <App initial={page} />
      )}
    </StrictMode>,
  );
}
