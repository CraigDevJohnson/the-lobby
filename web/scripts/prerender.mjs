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
  [
    "soccer.html",
    "soccer",
    "Put a Boise soccer team’s games in your calendar. Unofficial; not affiliated with Let’s Play Soccer.",
  ],
  ["minecraft.html", "minecraft", "Start a Minecraft world and join it."],
  ["minecraft-admin.html", "minecraft-admin", "Turn mods on or off for each Minecraft world."],
  ["foundry.html", "foundry", "See whether Foundry is up and open it to join your game."],
  ["foundry-admin.html", "foundry-admin", "Start and stop Foundry."],
  ["lobby-admin.html", "lobby-admin", "Invite Members to The Lobby and choose the Tools each can open."],
  ["wish.html", "wish", "Private family wish lists for invited Members."],
  ["signed-out.html", "signed-out", lobby],
  ["admin.html", "devices", "The family’s device profiles and how to install them."],
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
