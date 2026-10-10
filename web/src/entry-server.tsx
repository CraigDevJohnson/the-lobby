import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { App, titles as appTitles } from "./App";
import { Privacy } from "./Privacy";
import { ScheduleDownloader } from "./soccer/ScheduleDownloader";

// Each prerendered page, keyed by the value main.tsx reads from data-initial.
export type Page = "welcome" | "checking" | "privacy" | "soccer";

export const titles: Record<Page, string> = {
  ...appTitles,
  privacy: "Privacy · The Lobby",
  soccer: "Schedule Downloader · The Lobby",
};

export function render(page: Page): string {
  return renderToString(
    <StrictMode>
      {page === "privacy" ? <Privacy /> : page === "soccer" ? <ScheduleDownloader /> : <App initial={page} />}
    </StrictMode>,
  );
}
