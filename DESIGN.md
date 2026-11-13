---
name: The Lobby
description: A shared place for family and friends, drawn as a local co-op title screen.
colors:
  seafoam: "#74cebf"
  seafoam-light: "#e2f6f1"
  card: "#dcf4ee"
  navy: "#233552"
  navy-soft: "#2e4466"
  custard: "#ffd983"
  custard-hover: "#ffe29c"
  coral: "#ff845c"
  limb: "#b9e7de"
  leaf-deep: "#3a9887"
  leaf-mid: "#4fae9d"
  leaf-soft: "#5fbcab"
  leaf-vein: "#2f8273"
  leaf-vein-soft: "#45a392"
typography:
  display:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "clamp(4.25rem, 1rem + 10.5vw, 10.5rem)"
    fontWeight: 760
    lineHeight: 0.92
    letterSpacing: "-0.025em"
  display-lead:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "clamp(2.25rem, 1.4rem + 3vw, 4.25rem)"
    fontWeight: 650
    lineHeight: 0.95
    letterSpacing: "-0.005em"
  headline:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "clamp(2.375rem, 6.2vw, 4.6rem)"
    fontWeight: 760
    lineHeight: 1.05
    letterSpacing: "-0.02em"
  title:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "1.625rem"
    fontWeight: 700
    letterSpacing: "-0.01em"
  card-title:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "clamp(1.375rem, 1rem + 0.75vw, 1.75rem)"
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: "-0.015em"
  action:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "1.1875rem"
    fontWeight: 600
    lineHeight: 1
  lead:
    fontFamily: "Nunito Sans Variable, Nunito Sans, system-ui, sans-serif"
    fontSize: "clamp(1.25rem, 0.95rem + 1vw, 1.75rem)"
    fontWeight: 600
    lineHeight: 1.3
  body:
    fontFamily: "Nunito Sans Variable, Nunito Sans, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    lineHeight: 1.5
  state:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "clamp(1.75rem, 1.3rem + 0.6vw, 2.125rem)"
    fontWeight: 700
    lineHeight: 1.05
    letterSpacing: "-0.02em"
  value:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "clamp(1.875rem, 1.4rem + 1.2vw, 2.5rem)"
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: "-0.02em"
  hint:
    fontFamily: "Nunito Sans Variable, Nunito Sans, system-ui, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 600
    lineHeight: 1.25
rounded:
  focus: "6px"
  tile: "0.4rem"
  quiet: "0.75rem"
  control: "0.875rem"
  card: "1rem"
  hero: "2rem"
  pill: "999px"
spacing:
  gutter: "clamp(1.25rem, 5vw, 5.5rem)"
  page-max: "84rem"
  lobby-max: "67rem"
  card-gap: "0.875rem"
  card-pad: "clamp(0.875rem, 2vw, 1.5rem)"
  section: "2.5rem"
components:
  button-tactile-hero:
    backgroundColor: "{colors.custard}"
    textColor: "{colors.navy}"
    typography: "{typography.display-lead}"
    rounded: "{rounded.hero}"
    padding: "0.75rem 2rem"
    height: "5.25rem"
    width: "min(100%, 37rem)"
  button-tactile-hero-hover:
    backgroundColor: "{colors.custard-hover}"
    textColor: "{colors.navy}"
  button-tactile-small:
    backgroundColor: "{colors.custard}"
    textColor: "{colors.navy}"
    typography: "{typography.action}"
    rounded: "{rounded.control}"
    padding: "0.5rem 1.5rem"
    height: "2.75rem"
  button-tactile-small-hover:
    backgroundColor: "{colors.custard-hover}"
    textColor: "{colors.navy}"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.navy}"
    rounded: "{rounded.quiet}"
    padding: "0.4rem 1.25rem"
    height: "2.75rem"
  card-tool:
    backgroundColor: "{colors.seafoam}"
    textColor: "{colors.navy}"
    rounded: "{rounded.control}"
    padding: "{spacing.card-pad}"
  panel-notice:
    backgroundColor: "{colors.seafoam-light}"
    textColor: "{colors.navy}"
    rounded: "{rounded.card}"
    padding: "0.75rem 1.25rem"
  panel-problem:
    backgroundColor: "{colors.card}"
    textColor: "{colors.navy}"
    rounded: "{rounded.card}"
    padding: "1.5rem"
  footer-ground:
    backgroundColor: "{colors.navy}"
    textColor: "{colors.seafoam-light}"
  panel-slot:
    backgroundColor: "{colors.card}"
    textColor: "{colors.navy}"
    typography: "{typography.value}"
    rounded: "{rounded.card}"
    padding: "1.125rem 1.5rem"
  tile-step:
    backgroundColor: "{colors.navy}"
    textColor: "{colors.seafoam-light}"
    rounded: "{rounded.tile}"
    height: "1.625rem"
    width: "1.625rem"
  footer-terrace:
    backgroundColor: "{colors.navy}"
    textColor: "{colors.seafoam-light}"
    rounded: "{rounded.tile}"
---

# Design System: The Lobby

## Overview

**Creative North Star: "Local Co-op"**

The Lobby looks like the title screen of a friendly couch co-op game: a matte seafoam field, deep navy lettering and outlines, and one fat custard button sitting on a coral base, waiting to be pressed. Everything is drawn as flat, outlined shapes in a small palette, the way a cartoon game menu is drawn, and the depth that exists is physical (a button you press down) rather than atmospheric.

The public welcome is a single title menu with giant centred lettering, one action, a low custard sun, and layered seafoam foliage. A small robot host waves once beside the navy ground wave. The signed-in VIP Lobby uses **Arcade Cabinet**, selected by Craig on 2026-11-11, and **Attract Mode**, chosen the same day to bring it to life. The title is centred inside a marquee of bulbs on the navy field above a pale seafoam cabinet. Each seafoam Tool tile acts as a launch key, with a navy glyph, a full Tool name, and a custard Open control. Picking a tile closes a navy selector around it and plays its glyph's move; pressing Open grows the tile into a launch screen while the Tool loads. A seafoam control deck with a joystick and two buttons sits under the cabinet. The launcher keeps the robot at 40px in its navy footer.

Tool interiors share a blocky world drawn in those same materials: isometric miniatures, navy number tiles, framed controls and a stepped navy ground. Minecraft has a plot for each world, Soccer a pitch and calendar, and Foundry a tabletop scene. Each drawing belongs to its task and leaves the working controls in reach.

Navy does every structural job (type, outlines, rules, glyphs, focus rings, the ground). Custard is the action and drawn light (the welcome's sun, the one light on each Minecraft plot). Coral is the tactile base and the robot's body, never text. Seafoam and its lighter tints are the field and the surfaces.

**Key Characteristics:**
- Matte seafoam field with navy type and navy outlines on everything drawn.
- One tactile control: custard face, navy outline, coral base, sinks when pressed.
- Rounded, chunky lettering (Rubik) over a soft, readable text face (Nunito Sans).
- Flat surfaces separated by 2px navy outlines and 1.5px navy rules, not by shadow.
- Hand-drawn SVG scenery and a robot host, all in the token palette, all decorative.
- One task-specific miniature per Tool interior, drawn in the Lobby's own block materials, with shared number tiles and stepped ground.

## Colors

A small, saturated toy palette: a seafoam family for field and surfaces, navy for every line and letter, custard and coral for the one thing you press.

### Primary
- **Title-Screen Custard** (custard): the face of every tactile control, the low sun on the welcome, the one light on each of the Launcher's plots, the lamp on Foundry's tabletop, the VIP marquee's lit bulbs and the control deck's pressed button, text selection, and link colour on the navy ground. Its hover partner, **Lit Custard** (custard-hover), brightens the face on hover and focus.

### Secondary
- **Arcade Coral** (coral): the tactile base under every custard control, and the robot's body, antenna and feet. A material colour only; it never carries text.

### Neutral
- **Co-op Seafoam** (seafoam): the page field, the browser theme colour, and supporting text on navy ("No sign-in needed" on the welcome and "Choose a Tool." in the VIP Lobby). It also draws the VIP marquee's unlit bulb rings, the control deck's ledge and the launch screen's field.
- **Pale Seafoam** (seafoam-light) fills the welcome notice and VIP cabinet. It also colours text on navy regions, the VIP Sign out face, the robot's eyes, the server block's top face, the tick in the ready glyph, the pulse in Foundry's responding mark, the numerals on step tiles, the control deck's joystick washer, ball shine and button caps, and the quiet-button hover wash at 55% alpha.
- **Card Mint** (card) fills the access-problem panel, the Launcher's world columns, and the knock-out centre of the admin gear glyph.
- **Deep Co-op Navy** (navy): text and focus rings on light surfaces, every outline and rule, Tool glyphs, the status dots, the welcome ground wave and footer bar, the Tools' terraces and step tiles, the VIP header, title band and footer, the VIP selector's corners, and the control deck's joystick and button housings. Navy on seafoam reads at 6.6:1; navy on card at 10.7:1; navy on custard at 9.1:1.
- **Soft Navy** (navy-soft): scrollbar thumb only.

### Tertiary (illustration only)
- **Foliage family** (leaf-deep, leaf-mid, leaf-soft, with leaf-vein and leaf-vein-soft for veins): the welcome's edge foliage. Kept inside the seafoam hue so the scenery frames the greeting without competing with it. The same mint family shades the faces of the server block, pitch, calendar, tabletop and die.
- **Robot Limb** (limb): the robot's arms and hands, lighter faces in the Tools' isometric miniatures, and the fill of the picked VIP tile.

### Named Rules
**The Navy Does the Drawing Rule.** Outlines, rules and glyph strokes stay navy. Text and focus rings are navy on light surfaces; on navy regions, text flips to seafoam-light with seafoam supporting copy, footer links and focus rings turn custard.

**The One Thing to Press Rule.** Custard on a coral base means "press me". Do not use custard as a surface fill for cards, panels or banners, and do not use coral for anything that is not a tactile base or the robot. Outside the tactile control, custard appears only as drawn light: the welcome's sun, the Minecraft plots' lights and the tabletop lamp, the VIP marquee's lit bulbs, and the control deck's first button while Open is pressed. The deck's buttons are drawn, never pressable, so they stay pale at rest.

## Typography

**Display Font:** Rubik Variable (with Rubik, ui-rounded, system-ui)
**Body Font:** Nunito Sans Variable (with Nunito Sans, system-ui)

**Character:** Rubik's softened, heavy letterforms are the game-menu lettering; Nunito Sans is the friendly, legible reading voice underneath. Rubik carries headings, the identity, every button label and card names; Nunito Sans carries sentences.

### Hierarchy
- **Display** (760, fluid up to 10.5rem, 0.92): "The Lobby" on the public welcome only.
- **Display lead** (650, fluid up to 4.25rem, 0.95): the "Welcome to" line stacked over the display name; the hero Sign in label uses a near size (clamp(1.75rem, 1.2rem + 1.6vw, 2.5rem), 600).
- **Headline** (760, fluid up to 4.6rem, 1.05) for the centred VIP Lobby title on navy.
- **Title** (700, 1.625rem): the base section-title scale. The VIP Lobby uses smaller 1.25rem, weight 650 labels for "Your Tools" and "Admin Tools", letting the destinations lead. The identity mark uses Rubik 700 at 1.375rem (1.625rem in the compact lobby header).
- **Card title** (700, fluid from 1.375 to 1.75rem, 1.1): Tool names on VIP cards, with -0.015em tracking and a 15ch maximum measure. The VIP launch screen sets the opened Tool's name in the same lettering at twice the size (1.75rem beside its glyph on phones), breaking onto the same lines as the tile's.
- **Action** (Rubik 600, 1.1875rem, line-height 1): small tactile labels and, at 1.0625rem, the quiet button.
- **Kicker** (Rubik 600, 0.8125rem, +0.08em, capitals): the Let's Plan Soccer tagline, Link. Plan. Score, beside its link in the welcome footer and under its name on its Tool card; the one place capitals appear.
- **Lead** (Nunito Sans 600, fluid up to 1.75rem, 1.3, max 34ch): the welcome purpose line; the lobby lead and guidance lines use 600 at 1.1875rem.
- **Body** (Nunito Sans 400, 1.0625rem, 1.5): running text; notes cap at 60ch with pretty wrapping.
- **State** (Rubik 700, fluid 1.75 to 2.125rem, 1.05, balanced): a Minecraft world's state ("Ready to join"), the biggest words in its card, beside a 2.375rem status glyph. Foundry's status card uses the same lettering for its state ("Foundry is responding", or a stale state in the past tense, "Foundry was responding") beside a 2.375rem status mark.
- **Value** (Rubik 600, fluid 1.875 to 2.5rem, 1.75rem on phones, tabular figures): a connection value to copy. It wraps after its dots and colons, so a name stays whole; only a part too long for the line breaks inside it. It is copied exactly as sent.
- **Hint** (Nunito Sans 600, 0.8125rem, 1.25, balanced): where a device tab plays ("Computer, phone, tablet"), under its label.

### Named Rules
**The Heavy Menu Rule.** Headings run tight (negative tracking from -0.005em to -0.025em, line-height at or under 1.05) and balanced. Body text never goes tight.

**The Semibold Voice Rule.** Short functional sentences on the field (purpose, lead, guidance, notices, status) are Nunito Sans 600, so they hold up on saturated seafoam. Long running text stays at 400.

## Layout

The pages use a three-row grid (header, main, footer) filling at least the small viewport height. A fluid gutter (clamp(1.25rem, 5vw, 5.5rem)) sets the side margin everywhere.

The welcome is centred and single-column: identity top left, the stacked title, the purpose line, then the action block with the hero control capped at 37rem. Scenery is absolutely positioned behind the content (sun top left, foliage bottom corners), and the robot stands on the ground wave just right of the control. Under 52rem the sun moves to the top-right corner at 7.5rem, the foliage shrinks to about 85% opacity, and the robot tucks to the right edge.

The VIP Lobby's header, main content, and footer share a 67rem maximum width. The title is centred above the outlined cabinet, with 2.5rem between Tool sections. It sits inside the marquee: a ring of bulbs 0.5 to 0.6875rem across, padded 2.5 bulbs above and below the title and 3.25 bulbs to either side (2 below 22rem, so the title stays on one line). Tiles use an auto-fitting grid with a 14rem minimum column width and 0.875rem gaps. A single tile fills its row. Tiles have a 13rem minimum height above 35rem. At 35rem and below, the glyph and name sit side by side and the tile height follows the content. The control deck is a ledge 3.5 to 4.25rem tall under the cabinet, overhanging it by up to 0.875rem on wide screens and not at all on phones.

The Minecraft Launcher sets its worlds in one board across the page's width: the device choice in a navy top bar, then a column per world in the backend's order, each holding its own state, its Start or Stop and its address for the chosen device, with the columns' rows lined up as a subgrid and 2px of the board's navy between them. The joining steps follow the board, beside "Before you join" (one column under 52rem). Under 56rem the world columns stack, so a phone reads the device choice, each world in turn, then the steps. That order is the markup's as well as the screen's, so keyboard order matches.

Soccer keeps a compact opening with pitch-and-calendar artwork beside the title: 17rem wide on desktop and 6rem on phones. The finder is available in the first phone viewport. The empty finder is capped at 42rem; after Teams are chosen, the flow expands at 64rem to place game review beside the 26rem calendar handoff. The introduction stays in place.

Foundry sets Check availability in a 24rem column (20rem under 64rem) beside Join in Foundry in the wider column. Under 45rem it is one column in that order, which is also the markup and keyboard order.

## Elevation & Depth

The system is flat. Surfaces sit on the field with outlines, not shadows. The single exception is the tactile control, whose depth is a physical part of the object: a solid coral base (a zero-blur offset under the face) plus a soft navy ambient drop beneath it. Hover lifts the face 2px and deepens the base; active sinks the face onto the base in 60ms. The robot and the Tools' isometric miniatures cast a flat ground footprint in navy at 22% alpha. The VIP Lobby's marquee, selector and control deck are flat drawing too: no shadows.

### Shadow Vocabulary
- **Tactile base** (`box-shadow: 0 var(--lift) 0 -1px var(--coral), 0 calc(var(--lift) + 6px) 14px -6px rgb(35 53 82 / 0.35)`): every custard control; lift is 0.75rem hero, 0.5rem default, 0.3rem small.
- **Ground shadow** (ground-shadow, `rgb(35 53 82 / 0.22)`): an ellipse under the robot host, and the footprints under the Tool miniatures.

### Named Rules
**The Only Pressable Thing Has Depth Rule.** Depth means "this can be pressed". Cards, panels, notices and the quiet button stay flat; the coral base and its lift belong to the tactile control alone.

## Shapes

Generous, toy-like rounding with thick navy outlines. Panels use 1rem corners with 2px outlines, and the VIP Lobby's Tool tiles 0.875rem; the notice also uses 1rem with 2px. The hero control is a 2rem-cornered slab with a 4px outline; small tactile controls use 0.875rem corners with 2.5px outlines (0.625rem for the full-width Open inside a VIP tile); the quiet button uses 0.75rem with 2px. Page structure uses thinner 1.5px navy rules (under the lobby header, over its footer, and as the divider between the footer links). The sun is a plain circle; the welcome's ground is a single soft wave (the Tool interiors use the stepped terrace); foliage is broad overlapping leaf shapes with rounded vein strokes.

Tool glyphs share one stroke family on a 32px grid: 2.5px navy strokes, round joins and caps. Their base size is 2.75rem; VIP Lobby cards enlarge them to 3.75rem, reduced to 3rem under 40rem. Admin variants are the ordinary glyph with a solid gear cut into the lower-right corner. The Launcher's device glyphs (monitor, tablet and phone, gamepad) are drawn in the same family.

The block motif adds two shapes: a hexagonal block silhouette for status glyphs, and 0.4rem tile corners for step-number tiles and terrace steps. Foundry's status marks sit on a plain round outline instead (see Cards / Containers).

The VIP selector's corners are 3.5px navy round-capped strokes on 2.25rem boxes, set 8px outside the tile and concentric with its corners (a 22px outer radius around its 14px corner). The calendar glyph is drawn as a binding (its top and rule) and a page below it, so the page can flip over the binding.

## Components

### Buttons
Chunky, tactile and unmistakably pressable.
- **Shape:** hero slab (2rem corners, 4px navy outline, min 5.25rem tall, up to 37rem wide); small (0.875rem corners, 2.5px outline, min 7.5rem by 2.75rem); inside a VIP Tool tile, Open runs the tile's full width with 0.625rem corners and stands at least 3rem tall.
- **Primary (tactile):** custard face, navy Rubik 600 label, coral base. The hero carries a stroked navy arrow that nudges right 0.2em on hover.
- **Hover / Focus:** face brightens to lit custard and lifts 2px with a deeper base. Focus-visible adds a 3px navy ring offset 8px that wraps the face and its coral base together. Active sinks the face onto the base. Disabled drops to 70% opacity with a progress cursor. On the Minecraft Launcher, and for Foundry's Retry and Start Foundry, a request on its way ("Starting…", "Checking…") instead stays sunk onto its base at full strength, keeping focus and a readable label.
- **Quiet:** the base is transparent with a 2px navy outline and navy Rubik label; hover washes in pale seafoam at 55%. In the VIP Lobby's navy header, Sign out uses a solid pale seafoam face, a seafoam hover fill and a custard focus ring. Used for Sign out, a world's Stop and both answers in its confirmation, Check status again, and any Copy that is not the job of the moment. Foundry's Check again is quiet too, and so are Stop Foundry and both answers to its question; while Check again asks it reads "Checking…" with aria-disabled and a progress cursor, and it keeps aria-disabled while a start or stop is on its way.
- **One custard thing on Foundry:** Open Foundry is the one custard control (small tactile corners and outline, full width of its framed slot, min 3.375rem tall). When the backend has confirmed Foundry isn't answering players it drops to the quiet face at the same size, still a working link, with a line under it saying why it may not open. While Foundry is confirmed stopped, Start Foundry in the status card takes the custard face instead (the Launcher's Start server size, full width of the card), so there is still one.

### Cards / Containers
- **Corner Style:** 1rem; 0.875rem for the VIP Lobby's Tool tiles.
- **Background:** seafoam for Tool tiles and card mint for the problem panel.
- **Shadow Strategy:** none (see Elevation & Depth).
- **Border:** 2px navy.
- **Internal padding** is 1.5rem for panels and fluid from 0.875rem to 1.5rem for Tool tiles.
- **Tool tile** pairs the glyph and bold name above a full-width Open control at least 3rem tall. The whole tile is the hit area for the same link. Keyboard focus outlines the tile; once the cabinet's script runs, the selector is the focus cursor instead (see The living cabinet).
- **Problem panel:** same shell, max 40rem, Rubik 600 title, plain sentence, small tactile Retry.
- **Notice:** pale seafoam fill, 2px navy outline, 1rem corners, Nunito Sans 600, max 34rem; used for sign-in outcomes on the welcome, and on the Launcher for a refused stop and the console limitation; on Foundry it says why there is no link to open, or why a start or stop was refused.
- **Foundry status card:** the same shell (1.625rem padding at the bottom), a compact tabletop miniature, the state in State lettering beside a 2.375rem round status mark, then when it was checked, what it means, the one change the state allows (Start Foundry in custard while confirmed stopped; Stop Foundry, quiet, while confirmed on, whose question and two answers take its place), and a quiet Check again. The marks are drawn in the Tool glyph stroke family (32px grid, 2.5px navy strokes, round joins) on a round outline, not the Launcher's hexagonal block glyphs, which belong to its signature: a filled navy disc with a pale pulse line when responding (the only solid mark), a stop square when stopped, a bare turning arrow while starting or shutting down (still under reduced motion), a bang for a problem, and a question on a dashed outline of ten even dashes when unconfirmed. A stale state keeps its own mark, drawn open and still on the dashed outline. Until the first answer the house loading dots sit in the mark's place.

### Navigation
- **Identity:** "The Lobby" in Rubik 700, no underline, top left on both surfaces; navy on the public welcome and seafoam-light on the VIP Lobby's navy header.
- **Links:** inherit navy, underlined with a 0.09em line offset 0.22em that thickens to 0.14em on hover. On the navy ground, links are custard 700 and focus rings turn custard.
- **Lobby header and footer** sit on the same navy field as the title and surround the pale cabinet. The header carries the identity and secondary Sign out. The footer is at least 5rem tall with the 40px robot and a custard Privacy link.

### Loading status
Three navy dots (0.55rem) that pulse and rise 3px in sequence beside a Nunito Sans 600 sentence; the area reserves 11rem so the page does not jump when Tools arrive. The VIP launch screen shows them under the Tool's name once a page takes more than 0.9s.

### The Ground and the Host (signature)
The welcome footer stands on a navy ground wave that runs edge to edge into a navy bar holding the public link and Privacy. The robot host (coral body, navy outlines, custard cheeks and chest light, limb-coloured arms, smiling seafoam eyes) stands on that ground and waves once, 0.5s after arrival, over 1.6s. In the VIP Lobby it stands at 40px in the footer; it hops and waves at the first pick after 12 seconds without one, and jumps when Open is pressed. On each Tool interior it stands at 6rem on the shared terrace's landing, or 4.75rem on phones. It is always decorative and hidden from assistive technology.

### The living cabinet (VIP Lobby)
Attract Mode, chosen by Craig on 2026-11-11, brings the Arcade Cabinet to life in the same flat materials. Every motion answers the Member (the Tools arriving, a pick, a press) and settles within 2.5 seconds; nothing moves at rest, so no motion that starts on its own runs past five seconds (WCAG 2.2.2). The Open links stay ordinary links: nothing delays or replaces a navigation. Under prefers-reduced-motion nothing animates: the selector still marks the picked tile without travelling, and Open simply navigates.
- **Marquee:** 36 bulbs ring the title, 16 a row and 2 a side, numbered clockwise. Unlit bulbs are 2px seafoam rings; lit bulbs are custard discs, and every other one is lit at rest. A pick runs a three-phase chase clockwise (105ms a step) for 1.3s, and the Tools' arrival runs it for 2.4s; pressing Open flashes every bulb twice, 340ms a flash. The option's idle chase was dropped so nothing loops at rest.
- **Selector:** four navy corners close around the picked tile. A pick is a pointer hover, keyboard focus or an arrow key; touch never picks by hovering. Hover picks only while the pointer leads: focus that no pointer press caused (a key, assistive technology) puts the keyboard in charge, and a page scrolled under a resting pointer then never takes the cursor from the focused tile, until the pointer really moves or scrolls. Each corner glides in 480ms on an easing sampled for its own move, so it settles at most 4px past the tile however far it travels, even when a move cuts into the last one; the corners on the side it travels toward lead by up to 50ms. It appears by closing in from 14px further out, and lets go outward as it fades once the pointer leaves the cabinet and no tile has focus. When an access re-check adds or removes a Tool, it follows the picked tile to its new place, or lets go if the tile is gone. While the script runs, only the picked tile fills with limb (without it, hover and focus do), and the selector is the focus cursor: the picked, focused tile's own ring steps aside, and it returns without script or in forced colours.
- **Arrow keys:** left and right move along the row and stop at its ends; up and down move to the nearest tile that way, preferring one that shares the column. An arrow with nothing that way scrolls as usual. Tab order is unchanged.
- **Glyph moves** play once on each pick: the calendar's page flips over its binding to the next one (520ms) and its days settle one by one after it (about 760ms in all); the block squats, hops while turning around and lands squashed (720ms); the F lifts and stamps (560ms); an admin gear turns two teeth (760ms). A move already running finishes first.
- **Control deck:** a navy ball-top joystick on a pale washer leans up to 22° toward each move (a first pick leans from the middle of the cabinet) and springs back (620ms, about 13% overshoot). Two buttons with navy housings and pale caps are drawn, not pressable; the first sinks 3.5px and lights custard when Open is pressed. With Admin Tools at laptop heights, the deck and the host sit below the first view: they are a bottom-of-cabinet flourish, and the feedback in view (the selector, the glyph move, the marquee, the Open sink and the launch screen) answers every pick and press on its own.
- **Press and launch:** pressing Open (pointer down with a mouse or pen, Enter, or a touch tap) flashes the marquee, presses the deck's button and makes the host jump, while the Open control sinks onto its base. 90ms after the click, a navy edge and a field, each clipped to the tile's rounded box, open out until they pass the screen's edges (560ms, expo ease-out), so the growing shape keeps the tile's 2px outline and 14px corners on every frame; the field starts in the tile's own fill and settles to seafoam. The Tool's glyph and name each grow out of the tile's own (glyph beside name on phones, as in the tile) and stay readable throughout. The browser follows the link at once: the cabinet's drawing waits for the next frame, after the navigation has started, and the launch screen only decorates the wait. The screen takes the pointer and the focus, which assistive technology reads as "Opening" and the Tool's name; its drawing is hidden from assistive technology, and the lobby underneath goes inert, so neither the pointer nor the keyboard reaches what it hides. A second Open only changes which Tool it names. Escape, a back/forward restore, the opened Tool disappearing (an access re-check), or 8 seconds without a navigation hand the lobby back, with focus where it was, scrolled into view. A click that opens a new tab or window skips it.
- **Arrival:** when the Member's Tools arrive, the tiles rise into place in reading order (420ms, 45ms apart), fully drawn from the first frame, and the marquee chases.
- **Motion tokens:** `--glide-tl`, `--glide-tr`, `--glide-bl` and `--glide-br` are the selector corners' easings, each sampled by the script for its own corner's move; `--spring-loose` is the joystick's sampled spring (about 13% overshoot). Both fall back to cubic-bezier curves where `linear()` is not supported. The cabinet's other transitions use the house expo ease-out; the glyph and host moves carry their own keyframed curves, and the marquee steps from bulb to bulb.

The behaviour lives in `web/src/useCabinet.ts`, the drawing in `web/src/CabinetArt.tsx`, and the arrow-key geometry, bulb numbering and the selector's glide in `web/src/cabinet.ts` (tested in `cabinet.test.ts`). The cabinet's event handlers only take notes; its drawing waits for the next animation frame. Each state change is set on the part it affects (the marquee, the deck), so it restyles only that part. There are no new dependencies or rendering loops.

### The block motif (shared Tool world)
The Tool pages share an original isometric drawing family, number tiles and stepped ground. Keep the mint palette, navy outlines, clear task order and tactile controls consistent. Each miniature stays decorative; words and live controls carry the information.

#### Minecraft Launcher
The Launcher's server miniature, drawn from scratch in the Lobby's palette and stroke family. It nods to blocks without borrowing anything of Minecraft's: no logo, textures, grass or dirt colours, block art or pixel fonts.
- **The board:** one navy-framed panel (2.5px navy, 1.25rem corners, a soft drop shadow) whose navy shows through 2px gaps between the world columns as their rules. Its top bar holds the device tabs.
- **World columns:** one per world, card mint, each with its plot beside the world's name (under it where a column is too narrow for both) (Rubik 760, clamp(1.5rem, 1.2rem + 0.6vw, 1.875rem)), the state below in State lettering, then the one action the state allows: Start in custard at the column's full width, or a quiet Stop whose question takes its place. Every column is on the same ground, ready or not; the plot's light, the state and the custard Copy say which is up. A backend that lists no worlds gets one column for its one server.
- **Pocket Worlds:** chosen by Craig on 2026-11-12 to bring the page to life and make each world its own. At the head of each column stands the world's plot, a small isometric place on a slab six grid steps square, drawn in the house materials with 3.25-unit navy outlines on a 112 by 118 view: 6.5 to 8.5rem wide in side-by-side columns and 5.75rem when the columns stack. Survival is the Camp (a pine, a tent and a fire on the darkest ground), Creative the Build Site (an arch going up on a drafting sheet, with a site lamp) and Adventure the Gatehouse (a tower with a portcullis, a pole and a pennant, at the end of a dotted trail). A world the page has no drawing for gets the server block on a plot of its own, told apart by its initial on the block's top and a flag whose shape, pole and drive slots come from its id; the one unnamed server of a backend that lists no worlds gets the plain block.
- **A plot is a picture of the state the words show.** Each state has its own still drawing: packed up and unlit when stopped, at work while starting, lit and open when ready, packing up while stopping, a start that didn't take when failed, the stopped picture behind a pale striped barrier for maintenance, and a dashed, drained outline of itself when the state is unconfirmed. Each plot has exactly one custard light (the camp's flame, the site lamp, the lit doorway, the block's lamp), drawn only while the world is ready, never from a last known state. Inside the art, dashes mean unconfirmed and nothing else; a trail is dots.
- **Plot moves:** one-shots between those pictures, each answering something that happened: Start pressed (the plot strikes up as the label turns to Starting), the world turning ready (the one big moment, about 2 to 2.5 seconds, as the host waves), a stop beginning and ending, a failed start, and the ready world's address reaching the clipboard (its light flares once). While a world is on its way up or down, each status answer the page shows gets one small beat of 420ms or less, so the only thing that moves during a start is the page hearing back: nothing loops, and nothing suggests measured progress. A newer state cuts any move still running. Moves are skipped off screen, in a hidden tab and under reduced motion, where the plot simply shows its new picture. When the worlds arrive the plots rise 0.5rem into place in the board's order (420ms, 60ms apart), fully drawn in their true state; no control moves.
- **Status glyphs:** a 2.375rem glyph beside the state, saying it again in shape. Most sit on a block silhouette: filled navy with a pale tick when ready, a resting moon when stopped, a bang for a problem, a question on a dashed outline when unconfirmed. While busy it is a bare turning arrow, the Launcher's only looping motion, stopped under reduced motion. One mark turns into the next over about a quarter of a second: the block's outline unwinds as the arrow takes over, and the tick draws itself on. The state's words always sit beside it.
- **Device tabs:** in the board's navy bar, each tab carries its device glyph (1.75rem), its label and a hint saying where it plays, in pale seafoam lettering. The selected tab is marked by a 4px seafoam underline under its words and a bolder label; focus rings are pale seafoam. When the board is narrower than 40rem the tabs share the bar evenly, each glyph over its name and hint.
- **Connection values:** at the foot of each world column, under a 1.5px navy rule, the world's values for the chosen device (Rubik 600, 1.375rem), each with its Copy at the column's full width under it. Once a world is ready, its Copy address takes the custard face (on Bedrock the address's Copy, not the port's); every other Copy is quiet. One feedback line sits just under the values, with its space kept so a longer message never moves the column's other rows; "copied" clears after a few seconds.
- **Step tiles:** step numbers on 1.625rem navy tiles with 0.4rem corners and pale seafoam numerals.
- **Shared terrace:** the footer's ground is navy steps in whole 1.5rem block heights instead of the welcome's wave, with only exposed top corners rounded. The landing under the board is the highest step, and the host stands on it, waving once each time a world turns ready while the page is open (never on arrival, never looping). A navy bar below holds Privacy in custard.

#### Minecraft Admin
- **World panels:** one per world, the card shell (card mint, 2px navy, 1rem corners) at most 52rem wide, the world's name as its heading (Rubik 700, 1.625rem). Mods are rows inside it, divided by 1.5px navy rules: the name (Rubik 650, 1.25rem) with its version in small tabular figures, what it does, how it behaves on Bedrock, and the default in small semibold.
- **Switch:** a 3.25rem by 1.875rem track with a 2px navy outline and a navy knob; on, the track fills navy and the knob turns pale and moves right. The word On, Off or "Switching…" sits beside it in Rubik 600, in a space wide enough for the longest so nothing moves. Navy only, never custard. Its focus ring is 3px navy, 3px out. The knob's 120ms slide stops under reduced motion. While a switch is on its way, the others dim to 70%.
- **Outcome:** what the last switch did sits at the top of its world's panel on a pale seafoam note with a 2px navy outline.

#### Let's Plan Soccer
- **Pitch and calendar:** an original isometric pitch slab with goals and a standing calendar, drawn in mint faces with navy outlines. The calendar gains a navy check after a Team is chosen and loses it when the selection is empty. The check reflects Team selection, never successful calendar delivery. Its single 200ms stamp settles from scale 1.2 to 1 and is disabled under reduced motion.
- **Task tiles:** Find a team, Games in your download and Get your calendar carry 1.75rem navy tiles in their actual task order. Selected teams stays an ordinary heading. Instruction lists use the smaller 1.625rem tiles.

#### Foundry
- **Tabletop:** a folded game screen, map and cubic die on an isometric table, 12rem wide inside the availability card and 10rem on phones. The tiny edge lamp is custard only while the current reading freshly confirms Foundry is responding. Stale or unavailable status leaves it navy. The drawing never claims that a game is open.
- **Check, then join:** numbered task tiles lead the two columns. Open Foundry, its destination and any caveat sit in one mint connection frame with 2px navy outline and 1rem corners. Joining instructions use the shared smaller number tiles. A status failure preserves an authorized configured join link.

The shared terrace and host are implemented once in `web/src/ToolChrome.tsx`. Tool-specific details live in `web/src/ToolWorlds.css`; base materials and Minecraft's layout remain in `web/src/styles.css`. The Launcher's plots are drawn in `web/src/minecraft/plots/` on the grid in `iso.ts`, mounted and moved by `plots/View.tsx`, styled in `web/src/minecraft/plots.css`, and what each state shows and which change gets a move is decided in `web/src/minecraft/plot.ts` (tested in `plot.test.ts`). Apart from the plots' one-shots, Soccer's stamp is the only decorative movement. There are no new dependencies or rendering loops.

## Do's and Don'ts

### Do:
- **Do** draw every line, letter, outline and glyph in navy; flip to seafoam-light text and custard links and focus rings on the navy ground.
- **Do** make the primary action the custard tactile control with its coral base, and keep one per view at hero size.
- **Do** separate cards and panels with 2px navy outlines on 1rem corners (0.875rem for the VIP Lobby's Tool tiles), and page regions with 1.5px navy rules.
- **Do** draw new Tool glyphs in the existing stroke family: 32px grid, 2.5px navy stroke, round joins; admin variants add the corner gear.
- **Do** keep sun, foliage and ground wave on the public welcome. Give Tool interiors their shared block world and task-specific miniature. Keep every illustration and the robot decorative and aria-hidden.
- **Do** give a copy action the custard face once copying is the job of the moment, and drop every other control in view to quiet. On the Launcher each ready world's Copy address and each stopped world's Start are their own job, so each takes the custard face.
- **Do** honour reduced motion: every animation and transition is switched off under prefers-reduced-motion.
- **Do** keep the VIP cabinet's motion answering the Member (the Tools arriving, a pick, a press) and settling within a few seconds; nothing moves at rest.

### Don't:
- **Don't** give cards, panels, notices or the quiet button a shadow or a coral base; depth belongs to the pressable control.
- **Don't** set text in coral or custard on the seafoam field.
- **Don't** fill cards or banners with custard; custard is the action and drawn light (the sun, the Minecraft plots' lights and the tabletop lamp, the VIP marquee's bulbs and the deck's pressed button).
- **Don't** introduce a second line colour, gradients, or backdrop-blur glass; the world is matte and flat-drawn (the welcome's faint print grain is the only texture).
- **Don't** borrow Minecraft's own art for the block motif or the plots: no logo, textures, grass or dirt colours, block faces, tools, creatures, hard offset "block" shadows or pixel fonts.
- **Don't** let a plot say more than its words: no light before the backend says ready, no loop, and nothing that reads as progress while a world starts.
- **Don't** use colour alone or a custard surface to say a server's state; the state is said in words, with a drawn glyph beside them.
- **Don't** give the control deck's drawn buttons a custard face at rest, or anything to do; the custard Open control is the one thing to press.
