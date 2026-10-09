import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { App, titles, type Initial } from "./App";

export { titles };

export function render(initial: Initial): string {
  return renderToString(
    <StrictMode>
      <App initial={initial} />
    </StrictMode>,
  );
}
