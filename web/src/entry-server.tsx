import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { App, titles as appTitles } from "./App";
import { Launcher } from "./minecraft/Launcher";
import { Privacy } from "./Privacy";
import { ScheduleDownloader } from "./soccer/ScheduleDownloader";

// Each prerendered page, keyed by the value main.tsx reads from data-initial.
export type Page = "welcome" | "checking" | "privacy" | "soccer" | "minecraft";

export const titles: Record<Page, string> = {
  ...appTitles,
  privacy: "Privacy · The Lobby",
  soccer: "Schedule Downloader · The Lobby",
  minecraft: "Minecraft Launcher · The Lobby",
};

export function render(page: Page): string {
  return renderToString(
    <StrictMode>
      {page === "privacy" ? (
        <Privacy />
      ) : page === "soccer" ? (
        <ScheduleDownloader />
      ) : page === "minecraft" ? (
        <Launcher />
      ) : (
        <App initial={page} />
      )}
    </StrictMode>,
  );
}
