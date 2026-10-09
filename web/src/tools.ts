// The Tools The VIP Lobby can show, in their fixed display order. A Member's
// grants come from /api/me; the server checks access again on every Member
// action, so this list decides presentation only (ADR 0003).
//
// `ready` keeps a Tool out of The VIP Lobby until its destination exists:
// a grant alone never shows a card that leads nowhere.

export type Section = "yours" | "admin";

export type ToolEntry = {
  id: string; // the grant name used by `site member add <email> <tool ...>`
  name: string;
  href: string;
  section: Section;
  glyph: "calendar" | "cube" | "foundry" | "cube-admin" | "foundry-admin";
  ready: boolean;
};

export const TOOLS: readonly ToolEntry[] = [
  { id: "soccer", name: "Schedule Downloader", href: "/soccer", section: "yours", glyph: "calendar", ready: true },
  { id: "minecraft", name: "Minecraft Launcher", href: "/minecraft", section: "yours", glyph: "cube", ready: false },
  { id: "foundry", name: "Foundry", href: "/foundry", section: "yours", glyph: "foundry", ready: false },
  { id: "minecraft-admin", name: "Minecraft Admin", href: "/minecraft/admin", section: "admin", glyph: "cube-admin", ready: false },
  { id: "foundry-admin", name: "Foundry Admin", href: "/foundry/admin", section: "admin", glyph: "foundry-admin", ready: false },
];

// Schedule Downloader is public, so every Member sees it whatever their grants.
const ALWAYS_SHOWN = "soccer";

export type Collection = {
  yours: ToolEntry[];
  admin: ToolEntry[];
  // True when the Member holds no grant beyond the public Tool, which is
  // different from holding grants for Tools that are not ready yet.
  noPrivateGrants: boolean;
};

export function collectionFor(grants: readonly string[]): Collection {
  const granted = new Set(grants);
  const visible = TOOLS.filter((t) => t.ready && (t.id === ALWAYS_SHOWN || granted.has(t.id)));
  return {
    yours: visible.filter((t) => t.section === "yours"),
    admin: visible.filter((t) => t.section === "admin"),
    noPrivateGrants: !TOOLS.some((t) => t.id !== ALWAYS_SHOWN && granted.has(t.id)),
  };
}
