import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import { App, type Initial } from "./App";
import { FamilyDevices } from "./Devices";
import { Foundry, FoundryAdmin } from "./foundry/Foundry";
import { MinecraftAdmin } from "./minecraft/Admin";
import { Launcher } from "./minecraft/Launcher";
import { Privacy } from "./Privacy";
import { SignedOut } from "./SignedOut";
import { LetsPlanSoccer } from "./soccer/LetsPlanSoccer";
import "./styles.css";
import "./ToolWorlds.css";
import "./minecraft/plots.css";

const root = document.getElementById("root")!;
// In `npm run dev` there is no prerendered marker, so the address decides.
const byAddress: Record<string, string> = {
  "/privacy": "privacy",
  "/soccer": "soccer",
  "/minecraft": "minecraft",
  "/minecraft/admin": "minecraft-admin",
  "/foundry": "foundry",
  "/foundry/admin": "foundry-admin",
  "/signed-out": "signed-out",
  "/admin": "devices",
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
