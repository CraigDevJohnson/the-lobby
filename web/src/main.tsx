import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import { App, type Initial } from "./App";
import { Privacy } from "./Privacy";
import "./styles.css";

const root = document.getElementById("root")!;
const page = root.dataset.initial;
const initial: Initial = page === "checking" ? "checking" : "welcome";
const tree = <StrictMode>{page === "privacy" ? <Privacy /> : <App initial={initial} />}</StrictMode>;

// The HTML was rendered at build time for this same page, so React takes it
// over in place (ADR 0003). In `npm run dev` there is no prerender.
if (root.firstElementChild) {
  hydrateRoot(root, tree);
} else {
  createRoot(root).render(tree);
}
