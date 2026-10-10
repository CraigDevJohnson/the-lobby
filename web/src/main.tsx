import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import { App, type Initial } from "./App";
import { Privacy } from "./Privacy";
import { ScheduleDownloader } from "./soccer/ScheduleDownloader";
import "./styles.css";

const root = document.getElementById("root")!;
// In `npm run dev` there is no prerendered marker, so the address decides.
const byAddress: Record<string, string> = { "/privacy": "privacy", "/soccer": "soccer" };
const page = byAddress[window.location.pathname] ?? root.dataset.initial;
const initial: Initial = page === "checking" ? "checking" : "welcome";
const tree = (
  <StrictMode>
    {page === "privacy" ? <Privacy /> : page === "soccer" ? <ScheduleDownloader /> : <App initial={initial} />}
  </StrictMode>
);

// The HTML was rendered at build time for this same page, so React takes it
// over in place (ADR 0003). In `npm run dev` there is no prerender.
if (root.firstElementChild) {
  hydrateRoot(root, tree);
} else {
  createRoot(root).render(tree);
}
