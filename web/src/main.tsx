import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import { App, type Initial } from "./App";
import { Foundry } from "./foundry/Foundry";
import { Launcher } from "./minecraft/Launcher";
import { Privacy } from "./Privacy";
import { SignedOut } from "./SignedOut";
import { LetsPlanSoccer } from "./soccer/LetsPlanSoccer";
import "./styles.css";
import "./ToolWorlds.css";

const root = document.getElementById("root")!;
// In `npm run dev` there is no prerendered marker, so the address decides.
const byAddress: Record<string, string> = {
  "/privacy": "privacy",
  "/soccer": "soccer",
  "/minecraft": "minecraft",
  "/foundry": "foundry",
  "/signed-out": "signed-out",
};
const page = byAddress[window.location.pathname] ?? root.dataset.initial;
const initial: Initial = page === "checking" ? "checking" : "welcome";
const tree = (
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
      <App initial={initial} />
    )}
  </StrictMode>
);

// The HTML was rendered at build time for this same page, so React takes it
// over in place (ADR 0003). In `npm run dev` there is no prerender.
if (root.firstElementChild) {
  hydrateRoot(root, tree);
} else {
  createRoot(root).render(tree);
}
