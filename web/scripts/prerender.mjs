// Writes the two HTML documents the Go server chooses between (ADR 0003):
// index.html, the public welcome rendered in full before any script loads,
// and checking.html, served when a Sign-in session cookie is present so a
// Member never sees the public welcome flash before The VIP Lobby.
import { readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const here = (p) => fileURLToPath(new URL(p, import.meta.url));
const dist = here("../../internal/web/dist/");
const ssr = here("../dist-ssr/entry-server.js");

const { render, titles } = await import(ssr);
const template = await readFile(dist + "index.html", "utf8");

for (const [file, initial] of [
  ["index.html", "welcome"],
  ["checking.html", "checking"],
]) {
  const html = template
    .replace("<!--title-->", titles[initial])
    .replace("<!--initial-->", initial)
    .replace("<!--app-->", render(initial));
  await writeFile(dist + file, html);
}

// Keeps the embed directory present in git for builds without the UI.
await writeFile(dist + ".gitkeep", "");
await rm(here("../dist-ssr"), { recursive: true, force: true });
