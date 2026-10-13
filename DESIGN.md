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
    fontSize: "clamp(2.25rem, 1.6rem + 2vw, 2.875rem)"
    fontWeight: 700
    lineHeight: 1.05
    letterSpacing: "-0.02em"
  title:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "1.625rem"
    fontWeight: 700
    letterSpacing: "-0.01em"
  card-title:
    fontFamily: "Rubik Variable, Rubik, ui-rounded, system-ui, sans-serif"
    fontSize: "clamp(1.25rem, 1.1rem + 0.4vw, 1.4375rem)"
    fontWeight: 500
    lineHeight: 1.2
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
  card-gap: "1.25rem"
  card-pad: "1.5rem"
  section: "clamp(2.25rem, 4.5vh, 3rem)"
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
    backgroundColor: "{colors.card}"
    textColor: "{colors.navy}"
    rounded: "{rounded.card}"
    padding: "1.5rem 1.5rem 1.25rem"
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

The world has two densities. The public welcome is a single title menu: giant centred lettering, one action, a low custard sun, layered seafoam foliage at the edges, and a navy ground wave the footer stands on, with a small robot host waving once on arrival. The signed-in VIP Lobby is the same materials worked down to a compact launcher: a flat seafoam shell, thin navy rules, pale outlined cards with navy line glyphs, and the same custard tactile control at small size. Scenery belongs to the welcome; the VIP Lobby keeps only the robot, at 40px, in its footer. A Tool interior may carry one signature move of its own, drawn in the same materials and no louder than the welcome: the Minecraft Launcher's is the block motif (see Components). The other Tools have none yet.

Navy does every structural job (type, outlines, rules, glyphs, focus rings, the ground). Custard is the action and drawn light (the welcome's sun, the server block's lamp). Coral is the tactile base and the robot's body, never text. Seafoam and its lighter tints are the field and the surfaces.

**Key Characteristics:**
- Matte seafoam field with navy type and navy outlines on everything drawn.
- One tactile control: custard face, navy outline, coral base, sinks when pressed.
- Rounded, chunky lettering (Rubik) over a soft, readable text face (Nunito Sans).
- Flat surfaces separated by 2px navy outlines and 1.5px navy rules, not by shadow.
- Hand-drawn SVG scenery and a robot host, all in the token palette, all decorative.
- At most one signature per Tool interior; the Minecraft Launcher's is the block motif, the Lobby's own drawing, never Minecraft's.

## Colors

A small, saturated toy palette: a seafoam family for field and surfaces, navy for every line and letter, custard and coral for the one thing you press.

### Primary
- **Title-Screen Custard** (custard): the face of every tactile control, the low sun on the welcome, the lit lamp on the Launcher's server block, text selection, and link colour on the navy ground. Its hover partner, **Lit Custard** (custard-hover), brightens the face on hover and focus.

### Secondary
- **Arcade Coral** (coral): the tactile base under every custard control, and the robot's body, antenna and feet. A material colour only; it never carries text.

### Neutral
- **Co-op Seafoam** (seafoam): the page field on every surface, the browser theme colour, and the quiet text colour on the navy ground ("No sign-in needed").
- **Pale Seafoam** (seafoam-light): the notice surface, the footer text colour on the navy ground, the robot's eyes, the server block's top face, the tick in the ready glyph and the numerals on step tiles, and (at 55% alpha) the quiet-button hover wash.
- **Card Mint** (card): the fill of Tool cards, the access-problem panel, and the Launcher's server card and connection slot; also the knock-out centre of the admin gear glyph.
- **Deep Co-op Navy** (navy): all text, every outline and rule, Tool glyphs, focus rings, the status dots, the ground wave, the Launcher's terrace and step tiles, and the footer bar. Navy on seafoam reads at 6.6:1; navy on card at 10.7:1; navy on custard at 9.1:1.
- **Soft Navy** (navy-soft): scrollbar thumb only.

### Tertiary (illustration only)
- **Foliage family** (leaf-deep, leaf-mid, leaf-soft, with leaf-vein and leaf-vein-soft for veins): the welcome's edge foliage. Kept inside the seafoam hue so the scenery frames the greeting without competing with it. Leaf-soft also shades the server block's right face.
- **Robot Limb** (limb): the robot's arms and hands, and the server block's left face.

### Named Rules
**The Navy Does the Drawing Rule.** Every outline, rule, glyph stroke, focus ring and letter is navy. No second line colour exists; on the navy ground the roles flip to seafoam-light text and custard links and focus.

**The One Thing to Press Rule.** Custard on a coral base means "press me". Do not use custard as a surface fill for cards, panels or banners, and do not use coral for anything that is not a tactile base or the robot. Outside the tactile control, custard appears only as drawn light: the welcome's sun and the server block's lamp.

## Typography

**Display Font:** Rubik Variable (with Rubik, ui-rounded, system-ui)
**Body Font:** Nunito Sans Variable (with Nunito Sans, system-ui)

**Character:** Rubik's softened, heavy letterforms are the game-menu lettering; Nunito Sans is the friendly, legible reading voice underneath. Rubik carries headings, the identity, every button label and card names; Nunito Sans carries sentences.

### Hierarchy
- **Display** (760, fluid up to 10.5rem, 0.92): "The Lobby" on the public welcome only.
- **Display lead** (650, fluid up to 4.25rem, 0.95): the "Welcome to" line stacked over the display name; the hero Sign in label uses a near size (clamp(1.75rem, 1.2rem + 1.6vw, 2.5rem), 600).
- **Headline** (700, fluid up to 2.875rem, 1.05): the VIP Lobby page title.
- **Title** (700, 1.625rem): section titles ("Your Tools", "Admin Tools"). The identity mark uses Rubik 700 at 1.375rem (1.625rem in the compact lobby header).
- **Card title** (500, fluid around 1.25 to 1.44rem, 1.2): Tool names on cards; the lighter weight keeps a grid of names calm next to the bold section title.
- **Action** (Rubik 600, 1.1875rem, line-height 1): small tactile labels and, at 1.0625rem, the quiet button.
- **Kicker** (Rubik 600, 0.8125rem, +0.08em, capitals): the Let's Plan Soccer tagline, Link. Plan. Score, beside its link in the welcome footer and under its name on its Tool card; the one place capitals appear.
- **Lead** (Nunito Sans 600, fluid up to 1.75rem, 1.3, max 34ch): the welcome purpose line; the lobby lead and guidance lines use 600 at 1.1875rem.
- **Body** (Nunito Sans 400, 1.0625rem, 1.5): running text; notes cap at 60ch with pretty wrapping.
- **State** (Rubik 700, fluid 1.75 to 2.125rem, 1.05, balanced): the Minecraft Launcher's server state ("Ready to join"), the biggest words in its card, beside a 2.375rem status glyph.
- **Value** (Rubik 600, fluid 1.875 to 2.5rem, 1.75rem on phones, tabular figures): a connection value to copy. It wraps after its dots and colons, so a name stays whole; only a part too long for the line breaks inside it. It is copied exactly as sent.
- **Hint** (Nunito Sans 600, 0.8125rem, 1.25, balanced): where a device tab plays ("Computer, phone, tablet"), under its label.

### Named Rules
**The Heavy Menu Rule.** Headings run tight (negative tracking from -0.005em to -0.025em, line-height at or under 1.05) and balanced. Body text never goes tight.

**The Semibold Voice Rule.** Short functional sentences on the field (purpose, lead, guidance, notices, status) are Nunito Sans 600, so they hold up on saturated seafoam. Long running text stays at 400.

## Layout

Both surfaces are a three-row grid (header, main, footer) filling at least the small viewport height. A fluid gutter (clamp(1.25rem, 5vw, 5.5rem)) sets the side margin everywhere.

The welcome is centred and single-column: identity top left, the stacked title, the purpose line, then the action block with the hero control capped at 37rem. Scenery is absolutely positioned behind the content (sun top left, foliage bottom corners), and the robot stands on the ground wave just right of the control. Under 52rem the sun moves to the top-right corner at 7.5rem, the foliage shrinks to about 85% opacity, and the robot tucks to the right edge.

The VIP Lobby is left-aligned in a container capped at 84rem. Tools sit in a three-column grid with a 1.25rem gap; it drops to two columns under 64rem and one under 40rem, where cards lose their minimum height. Sections stack with fluid spacing (about 2.25 to 3.75rem between them). Header and footer inner rows match the main container width.

The Minecraft Launcher sets a 24rem server column beside the joining help (20rem under 64rem), with Stop server under the server card. Under 45rem it is one column in the order card, joining help, Stop server, so Copy address comes before Stop. That order is the markup's as well as the screen's, so keyboard order matches.

## Elevation & Depth

The system is flat. Surfaces sit on the field with outlines, not shadows. The single exception is the tactile control, whose depth is a physical part of the object: a solid coral base (a zero-blur offset under the face) plus a soft navy ambient drop beneath it. Hover lifts the face 2px and deepens the base; active sinks the face onto the base in 60ms. The robot and the server block cast a soft ground shadow in navy at 22% alpha.

### Shadow Vocabulary
- **Tactile base** (`box-shadow: 0 var(--lift) 0 -1px var(--coral), 0 calc(var(--lift) + 6px) 14px -6px rgb(35 53 82 / 0.35)`): every custard control; lift is 0.75rem hero, 0.5rem default, 0.3rem small.
- **Ground shadow** (ground-shadow, `rgb(35 53 82 / 0.22)`): an ellipse under the robot host, and the footprint under the server block.

### Named Rules
**The Only Pressable Thing Has Depth Rule.** Depth means "this can be pressed". Cards, panels, notices and the quiet button stay flat; the coral base and its lift belong to the tactile control alone.

## Shapes

Generous, toy-like rounding with thick navy outlines. Tool cards and panels use 1rem corners with 2px outlines; the notice also uses 1rem with 2px. The hero control is a 2rem-cornered slab with a 4px outline; small tactile controls use 0.875rem corners with 2.5px outlines; the quiet button uses 0.75rem with 2px. Page structure uses thinner 1.5px navy rules (under the lobby header, over its footer, and as the divider between the footer links). The sun is a plain circle; the welcome's ground is a single soft wave (the Minecraft Launcher's is the stepped terrace); foliage is broad overlapping leaf shapes with rounded vein strokes.

Tool glyphs share one stroke family on a 32px grid: 2.5px navy strokes, round joins and caps, drawn at 2.75rem. Admin variants are the ordinary glyph with a solid gear cut into the lower-right corner. The Launcher's device glyphs (monitor, tablet and phone, gamepad) are drawn in the same family.

The block motif adds two shapes: a hexagonal block silhouette for status glyphs, and 0.4rem tile corners for step-number tiles and terrace steps.

## Components

### Buttons
Chunky, tactile and unmistakably pressable.
- **Shape:** hero slab (2rem corners, 4px navy outline, min 5.25rem tall, up to 37rem wide); small (0.875rem corners, 2.5px outline, min 7.5rem by 2.75rem).
- **Primary (tactile):** custard face, navy Rubik 600 label, coral base. The hero carries a stroked navy arrow that nudges right 0.2em on hover.
- **Hover / Focus:** face brightens to lit custard and lifts 2px with a deeper base. Focus-visible adds a 3px navy ring offset 8px that wraps the face and its coral base together. Active sinks the face onto the base. Disabled drops to 70% opacity with a progress cursor. On the Minecraft Launcher a request on its way ("Starting…", "Checking…") instead stays sunk onto its base at full strength, keeping focus and a readable label.
- **Quiet:** transparent with a 2px navy outline and navy Rubik label; hover washes in pale seafoam at 55%. Used for Sign out, Stop server and both answers in its confirmation, Check status again, and any Copy that is not the job of the moment.

### Cards / Containers
- **Corner Style:** 1rem.
- **Background:** card mint.
- **Shadow Strategy:** none (see Elevation & Depth).
- **Border:** 2px navy.
- **Internal Padding:** 1.5rem (1.25rem at the bottom of Tool cards).
- **Tool card:** glyph and Tool name in a row at the top, a small tactile Open at the bottom, min 10.5rem tall.
- **Problem panel:** same shell, max 40rem, Rubik 600 title, plain sentence, small tactile Retry.
- **Notice:** pale seafoam fill, 2px navy outline, 1rem corners, Nunito Sans 600, max 34rem; used for sign-in outcomes on the welcome, and on the Launcher for a refused stop and the console limitation.

### Navigation
- **Identity:** "The Lobby" in Rubik 700, navy, no underline, top left on both surfaces.
- **Links:** inherit navy, underlined with a 0.09em line offset 0.22em that thickens to 0.14em on hover. On the navy ground, links are custard 700 and focus rings turn custard.
- **Lobby header and footer:** thin 1.5px navy rules separate them from the main area; the header carries the identity and quiet Sign out, the footer the 40px robot and Privacy.

### Loading status
Three navy dots (0.55rem) that pulse and rise 3px in sequence beside a Nunito Sans 600 sentence; the area reserves 11rem so the page does not jump when Tools arrive.

### The Ground and the Host (signature)
The welcome footer stands on a navy ground wave that runs edge to edge into a navy bar holding the public link and Privacy. The robot host (coral body, navy outlines, custard cheeks and chest light, limb-coloured arms, smiling seafoam eyes) stands on that ground and waves once, 0.5s after arrival, over 1.6s. In the VIP Lobby it appears static at 40px in the footer. On the Minecraft Launcher it stands at 6rem on the terrace's landing (below). It is always decorative and hidden from assistive technology.

### The block motif (Minecraft Launcher signature)
The Launcher's one signature move, drawn from scratch in the Lobby's palette and stroke family. It nods to blocks without borrowing anything of Minecraft's: no logo, textures, grass or dirt colours, block art or pixel fonts.
- **Server block:** an isometric block in the server card (7.5rem; 4rem beside the server's name on phones). Pale seafoam top, limb left face, leaf-soft right face, 3.5px navy outlines, two drive slots and a grille. Its lamp is custard with short navy rays while the server is ready, navy otherwise.
- **Status glyphs:** a 2.375rem glyph beside the state, saying it again in shape. Most sit on a block silhouette: filled navy with a pale tick when ready, a resting moon when stopped, a bang for a problem, a question on a dashed outline when unconfirmed. While busy it is a bare turning arrow, the only looping motion, stopped under reduced motion. The state's words always sit beside it.
- **Device tabs:** each joining tab carries its device glyph (1.75rem), its label and a hint saying where it plays. The selected tab is marked by a 4px navy underline under its words and a bolder label. When the joining column is narrower than 40rem the tabs share the row evenly, each glyph over its name and hint.
- **Connection slot:** a card-mint frame (2px navy, 1rem corners) around the values to copy. Once the server is ready, Copy address takes the custard face (on Bedrock the address's Copy, not the port's) and every other control in view is quiet, so one custard thing shows at a time. One feedback line sits just under the frame, with its space kept so a longer message pushes the steps down; "copied" clears after a few seconds.
- **Step tiles:** step numbers on 1.625rem navy tiles with 0.4rem corners and pale seafoam numerals.
- **The terrace:** the footer's ground is navy steps in whole 1.5rem block heights instead of the welcome's wave, with only exposed top corners rounded. The landing under the server card is the highest step, and the host stands on it, waving once each time the server turns ready while the page is open (never on arrival, never looping). A navy bar below holds Privacy in custard.

## Do's and Don'ts

### Do:
- **Do** draw every line, letter, outline and glyph in navy; flip to seafoam-light text and custard links and focus rings on the navy ground.
- **Do** make the primary action the custard tactile control with its coral base, and keep one per view at hero size.
- **Do** separate cards and panels with 2px navy outlines on 1rem corners, and page regions with 1.5px navy rules.
- **Do** draw new Tool glyphs in the existing stroke family: 32px grid, 2.5px navy stroke, round joins; admin variants add the corner gear.
- **Do** keep scenery (sun, foliage, ground wave) on the public welcome and keep the robot decorative and aria-hidden. A Tool interior gets at most one signature move (the Minecraft Launcher's block motif and terrace), recorded here before it ships.
- **Do** give a copy action the custard face once copying is the job of the moment, and drop every other control in view to quiet.
- **Do** honour reduced motion: every animation and transition is switched off under prefers-reduced-motion.

### Don't:
- **Don't** give cards, panels, notices or the quiet button a shadow or a coral base; depth belongs to the pressable control.
- **Don't** set text in coral or custard on the seafoam field.
- **Don't** fill cards or banners with custard; custard is the action and drawn light (the sun, the server block's lamp).
- **Don't** introduce a second line colour, gradients, or backdrop-blur glass; the world is matte and flat-drawn (the welcome's faint print grain is the only texture).
- **Don't** borrow Minecraft's own art for the block motif: no logo, textures, grass or dirt colours, block faces, hard offset "block" shadows or pixel fonts.
- **Don't** use colour alone or a custard surface to say a server's state; the state is said in words, with a drawn glyph beside them.
