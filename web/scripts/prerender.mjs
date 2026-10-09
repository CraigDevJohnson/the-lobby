// Writes the HTML documents the Go server serves (ADR 0003): index.html, the
// public welcome rendered in full before any script loads; checking.html,
// served when a Sign-in session cookie is present so a Member never sees the
// public welcome flash before The VIP Lobby; and privacy.html, the public
// privacy note.
import { readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const here = (p) => fileURLToPath(new URL(p, import.meta.url));
const dist = here("../../internal/web/dist/");
const ssr = here("../dist-ssr/entry-server.js");

const { render, titles } = await import(ssr);
const template = await readFile(dist + "index.html", "utf8");

const lobby = "The Lobby is a shared place for family and friends.";
for (const [file, page, description] of [
  ["index.html", "welcome", lobby],
  ["checking.html", "checking", lobby],
  ["privacy.html", "privacy", "What The Lobby keeps, why, and what you control."],
]) {
  const html = template
    .replaceAll("<!--title-->", titles[page])
    .replaceAll("<!--description-->", description)
    .replace("<!--initial-->", page)
    .replace("<!--app-->", render(page));
  await writeFile(dist + file, html);
}

// Keeps the embed directory present in git for builds without the UI.
await writeFile(dist + ".gitkeep", "");
await rm(here("../dist-ssr"), { recursive: true, force: true });
