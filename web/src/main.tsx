import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import { App, type Initial } from "./App";
import "./styles.css";

const root = document.getElementById("root")!;
const initial: Initial = root.dataset.initial === "checking" ? "checking" : "welcome";

// The HTML was rendered at build time for this same initial state, so React
// takes it over in place (ADR 0003). In `npm run dev` there is no prerender.
if (root.firstElementChild) {
  hydrateRoot(
    root,
    <StrictMode>
      <App initial={initial} />
    </StrictMode>,
  );
} else {
  createRoot(root).render(
    <StrictMode>
      <App initial={initial} />
    </StrictMode>,
  );
}
