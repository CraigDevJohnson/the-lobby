---
target: the minecraft page is very plain and needs to be stylized more
total_score: 29
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 0
target_identity: "file:/home/user/the-lobby/web/src/minecraft/Launcher.tsx"
target_fingerprint: "sha256:9684d85cf50d4d31bb82ec02f0db33223b0632d6c465aa10fa8f312343431b51"
target_path: /home/user/the-lobby/web/src/minecraft/Launcher.tsx
timestamp: 2026-10-13T05-41-44Z
slug: web-src-minecraft-launcher-tsx
---
Method: dual-agent (A: a525df07c61ff3495 design director, a81f222fab312b42e stylization ceiling, a343a1b7434f690d1 UX state walker · B: a01df522ce4e2df11). Each A reviewer's priority issues were checked by an adversarial verifier (ae60ff8aca32721c3, af3c01192eef8deed, a18a16154cb49aec1); severities below are the verified ones.

Target: web/src/minecraft/Launcher.tsx (with Join.tsx, logic.ts and the .mc-* rules in web/src/styles.css), rendered at /minecraft against a mock backend with made-up data in 15 states, 1440x900 and 390x844, plus 320, 640 and 200% zoom.

## Design Health Score

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 3 | Honest state model with announcements, but the change to Ready alters about 4% of desktop pixels; "Check status again" says nothing when the answer is unchanged; document.title never changes. |
| 2 | Match System / Real World | 3 | Plain, warm copy and exact Minecraft menu paths; the tabs say "Java Edition / Bedrock" with no device hints, so players must know their edition. |
| 3 | User Control and Freedom | 3 | Stop asks in place with "Keep playing"; the device choice is reversible and remembered. |
| 4 | Consistency and Standards | 3 | Matches the Lobby shell; Let's Plan Soccer gives its copy action the custard face, the Launcher never does; the stop confirmation gives the custard face to the destructive Stop. |
| 5 | Error Prevention | 3 | Start is guarded during the request, Stop confirms, a lost request offers a status check instead of a resend; long hostnames wrap mid-token ("example.ne / t"), which invites typing errors. |
| 6 | Recognition Rather Than Recall | 3 | Values and steps sit together and the device is remembered; Console help says "the steps above" when none are above. |
| 7 | Flexibility and Efficiency | 3 | Arrow, Home and End on the tabs, one-tap copy, remembered device; no link to a device tab to send a friend. |
| 8 | Aesthetic and Minimalist Design | 2 | Uncluttered but emphasis is not purposeful: a static 46px title outranks the 20px state, and Ready has no primary control at all. |
| 9 | Error Recovery | 3 | Clear recovery copy throughout; with no connection details, Ready still says "join with the details here". |
| 10 | Help and Documentation | 3 | Device-specific steps and an honest console limit; nothing helps when a join fails inside Minecraft; "Before you join" comes after the steps. |
| **Total** | | **29/40** | **Good** |

## Design Specificity Verdict

**LLM assessment**: The page is authored for The Lobby but not for the Minecraft Launcher. The palette, navy outlines, Rubik over Nunito Sans, the custard tactile control and the 40px footer robot are The Lobby's own; the device tabs, menu paths and the honest unknown, lost-request and refusal copy are this Tool's own. The composition is generic: cover the words and it is any hosting panel's status card plus connection string plus Copy, in a toy palette. The build is a near 1:1 copy of the selected "Status beside setup" stopped-state sketch (h1 46/700 in both, card 352 vs 350px, field 87.1% vs 86.5% of the viewport, custard 1.0% in both). The server card is the VIP Lobby Tool card again with the same cube glyph. Ready, Starting and Failed were never designed: they are the stopped sketch with a different 14px mark and a button swap, and Ready is the plainest frame (0.0% custard, 0.0% coral, 89% flat field).

Two causes, about equal. First, the rules: DESIGN.md keeps sun, foliage and the ground wave on the welcome, makes the print grain welcome-only, allows no second line colour (so no state colour), and reserves custard for the press-me control. Second, the process: the shaping round for this page offered three layouts with identical palette and materials ("Local Co-op, outlined surfaces, tactile action"), so a level of expression was never on the table. Plain was a default, not a choice.

**Deterministic scan**: `impeccable detect --json` found 0 findings on web/src/minecraft, on web/src/Robot.tsx and web/src/glyphs.tsx, and on all three together, exit 0, and the same with `--no-config`. A canary file with deliberate tells returned 3 findings, so the scan works. The detector catalogues anti-patterns; it does not measure richness, so a clean result says the page is plain, not sloppy. No false positives (nothing to be false).

**Visual overlays**: detect.js injected successfully into a headless browser on 5 views (ready 1440, stopped 390, ready-long 390, signed-out 1440, unknown 1440); each logged "No anti-patterns found." The browser ran headless in a cloud container, so no user-visible overlay exists.

## Overall Impression

An honest, well-engineered Operate tool that answers "Can I join? Do I need to start it? How do I connect?" in the right order and handles the hard states (unknown with last-known, a lost start, a refused stop, a slow start) better than most real server panels. It looks like a settings screen in Lobby colours. The single biggest opportunity: make the server's state the thing the page is about, and give the Ready moment, the reason anyone opens the page, a look of its own.

## What's Working

- **Honest, recoverable state model.** "Last known: ready to join, not confirmed now"; "We couldn't tell whether the server got the start request. Check its status before trying again."; a refused stop says why and changes nothing. This is PRODUCT.md principle 4 done properly.
- **Stop asks in place.** The question replaces the button ("Stop the server? Everyone's progress is saved first."), focus goes to it and returns to Stop server on "Keep playing". No modal.
- **Real accessibility plumbing.** Shape-coded marks (never colour alone), aria-live announcements without stealing focus, a true tablist with arrow/Home/End and a remembered choice, no horizontal overflow at 320, 390, 640 or 200% zoom.

## Priority Issues

**[P2] Ready, the payoff, is the plainest screen and never makes the connection details the focus**
- Why it matters: the brief says at Ready "connection details become the focus". Instead the custard Start disappears and nothing replaces it; Copy address stays a quiet outline button; the first button under "Ready to join" is Stop server. On a phone the visible change in the guide is a 14px chip disappearing and a 27px shift. By the peak-end rule the remembered moment is "it went quiet".
- Fix: when ready, Copy address takes the custard tactile (on Bedrock, address custard and port quiet) and the stop confirmation's Stop goes quiet so only one thing is custard; show the address in a framed value slot; on phones render Stop server after the guide (the brief only requires readiness and Start first); make the lead line follow the state; one short settle on a live change to Ready, off under reduced motion; set document.title from the state.
- Suggested command: /impeccable bolder

**[P2] "Can I join?" is answered in small type while a static title gets the biggest**
- Why it matters: the h1 "Minecraft Launcher" is 46px; the state is 20px Nunito Sans beside a 14px CSS ring. The server name (24/600) is smaller than the guide heading beside it (26/700). Sizes step about 1.07 apart, so nothing below the h1 leads. The busy arc reads as a letter "C", and under reduced motion it is always a "C".
- Fix: set the state in Rubik at the title token or larger; redraw the five marks as drawn glyphs in the 2.5px navy stroke family (hollow, travelling arc, filled with check, struck, dashed); snap remaining sizes to DESIGN.md's tokens, which the brief asked for. Keep the h1 for parity with the VIP Lobby.
- Suggested command: /impeccable typeset

**[P2] No signature move: the world shows up only as palette**
- Why it matters: this is the owner's complaint. The page is 87-89% untouched seafoam (the welcome is 68%), with a 352x238px dead rectangle under the server card at 1440 (410px tall at 1920). Its footer is identical to the VIP Lobby's. The one move the brief names, the tactile Start, is shared by every page and absent in 4 of 7 states. Making Start hero-size is not a free win: at hero size "Start server" wraps to two lines in the 22rem column.
- Fix: needs a DESIGN.md decision. Options: bring the welcome's navy ground and the robot host into Tool interiors (the host waves once when the server becomes ready); or an original block motif drawn in the Lobby's own line; or stay inside today's rules and accept a sharper but still quiet page.
- Suggested command: /impeccable bolder (after a DESIGN.md amendment), or /impeccable shape for a look round

**[P2] Recovery checks drop focus and stay silent; status changes have side effects**
- Why it matters: "Check status again" and access Retry use the disabled attribute, so focus falls to the page; an unchanged answer is not announced and nothing visible says it checked. The 4-minute slow-start note is never announced. On Android Chromium, if focus stayed on Start, the page jumps from the guide to the top at Ready.
- Fix: aria-disabled with a click guard, as Start already does; announce unchanged answers and show "Checked just now"; send the slow note through the live region; keep the jump only if the guide gets its own visible ready line.
- Suggested command: /impeccable harden

**[P2] The joining guide misses edge cases and phone basics**
- Why it matters: with no connection details, Ready says "Open Minecraft and join with the details here" beside "Connection details unavailable"; the tabs need edition knowledge (the brief lists "computer, phone, tablet" and "Xbox, PlayStation, Switch" hints the build dropped); a long hostname breaks as "example.ne / t" and after its own hyphens; Back to Tools (92x23) and See console help (126x24) are under the brief's 44px.
- Fix: a no-details message with a next step; device hints under each tab label; break values only after "." and ":" with display-only `<wbr>`, keeping the copied text exact; 44px hit areas at phone width.
- Suggested command: /impeccable clarify, then /impeccable adapt

## Persona Red Flags

**Casey (phone, distracted)**: after tapping Start and scrolling to the steps, Ready changes almost nothing in view; the first button under "Ready to join" is Stop server; the tab title never says Ready; Back to Tools is 23px tall.

**Sam (keyboard and screen reader)**: Check status again and Retry drop focus to the page and stay silent on an unchanged answer; the slow-start note is never announced. What works for Sam (tablist, focus moves, announcements, shape-coded marks) must survive any restyle.

**Jordan (first-timer)**: must choose "Java Edition" or "Bedrock" without knowing which they own; Java is preselected even on a tablet; the address shows at full size with an active Copy while stopped, guarded only by a 14px "Join after the server is ready" chip.

**A younger family member on a tablet or Switch**: nothing says "it's ready, go"; the most tappable thing at Ready is Stop server (the confirmation and the backend's refusal while anyone plays prevent harm); Console help ends at "play from a computer, phone or tablet" with no way to ask Craig.

**An invited friend reading on a phone while the game runs on a console or PC**: has to retype the address on another device and a long hostname wraps mid-token; Bedrock's address and port are separate copy targets whose feedback lines pile up and never clear.

## Minor Observations

- The lead "Start the server, then join from Minecraft." stays on the denied, not-configured, maintenance and ready screens. Keep a neutral version when signed out.
- In the stop confirmation the destructive Stop wears the custard face; "Keep playing" is quiet.
- Copy feedback ("Address copied.") never clears, and Address and Port messages stack.
- The refusal "Someone is still playing. Ask them to leave first." looks like body text, with no notice treatment.
- Maintenance and failed share the struck ring; unknown's dashed ring and the busy arc look alike at 14px.
- While the Start request is in flight the label still says "Stopped" beside a "Starting…" button.
- The disabled "Starting…" label is about 4.0:1 at 70% opacity (exempt as inactive, per DESIGN.md's disabled spec).
- The checking state floats one line in an empty field; problem states are one panel over about 70% empty field.
- "Let Craig know" in the slow-start note has no route to Craig.
- Console help's tab panel has no focusable content; Tab skips to Privacy.
- Escape does not dismiss the inline stop confirmation (a nicety, not a standard).
- At 320px the tab labels wrap to two lines.

## Questions to Consider

- What if "Ready to join" were the biggest words on the page when it is true?
- If copying the address is the job once the server is ready, why is Copy address the quietest button, when Let's Plan Soccer gives its copy action the custard face?
- Was the Ready state ever designed, or only inherited from a stopped-state sketch?
- Has "scenery belongs to the welcome" quietly become "Tools get no character at all"? Should DESIGN.md grant each Tool interior exactly one signature move?
- What would this page look like designed for a 10-year-old holding a controller, rather than the adult who runs the server?

## Ways to Stylize It (from the stylization review, verified)

| Direction | What changes | DESIGN.md change | Effort | Main risk |
|---|---|---|---|---|
| Sharpen only | State-first type and drawn status glyphs; Copy address takes the custard face at Ready; Stop moves below the guide on phones; state-aware lead | None, or one line if the state label moves to Rubik | S | Still reads quiet: field share drops to about 82% only by stretching the card |
| Ground and host | Sharpen only, plus the welcome's navy ground and footer bar; the robot host (about 6-7rem) stands beside the server card and waves once when the server becomes ready | A Tool-interior tier: ground and host allowed, no sun or foliage | M | Desktop gains about 60px and scrolls at 900px tall; robot must not collide with copy on phones; VIP Lobby and Soccer look barer unless they get it too; static per-state robot poses would be new artwork |
| Blocky look | An original isometric "server block" illustration heading the card, a stepped navy terrace for the ground, block-shaped status marks and step tiles, device glyphs on tabs; Rubik stays | Shapes, illustration colours, scenery boundary, per-Tool motif policy | L | Trademark and trade dress (no Mojang art, grass-block colours, pixel fonts), and the page already shares its name with Mojang's own launcher app; costume risk on a working screen |

Refuse-list hazards any direction must avoid: kickers or eyebrows ("MINECRAFT SERVER"), gradient text, coloured side stripes for state, hard offset "block" shadows, emoji or Unicode icons, pixel or monospace display fonts, custard surfaces as a ready indicator, progress rings or percentages while starting, player-count chips, looping robot or scenery motion.
