# VIP Lobby review captures

Captured on 2026-11-07 from the local Vite render of `web/src/VipLobby.tsx`, after integrating Navy Frame with current main.

- `vip-lobby-navy-frame-desktop.png`: 1440 × 1000 viewport.
- `vip-lobby-navy-frame-phone.png`: 390 × 844 viewport, full-page capture.

The local fixture grants the three ready Tools: Let's Plan Soccer, Minecraft Launcher and Foundry. These captures contain no Member information and do not establish live deployment or access to an actual account.

## Arcade Cabinet

Captured on 2026-11-11 from the local Vite render of `web/src/VipLobby.tsx` after integrating Arcade Cabinet with main. Both images show the full page.

- `vip-lobby-arcade-cabinet-desktop.png` uses a 1440 × 1000 viewport.
- `vip-lobby-arcade-cabinet-phone.png` uses a 390 × 844 viewport.

The all-Tools fixture includes the three entries under Your Tools and both entries under Admin Tools. Foundry Admin is included to review the full layout and remains preview-only. The captures use fixture data.

## Attract Mode

Captured on 2026-11-11 with headless Chrome from the local Vite render of `web/src/VipLobby.tsx`, after bringing Arcade Cabinet to life with Attract Mode. They use the same all-Tools fixture and contain no Member information.

- `vip-lobby-attract-mode-desktop.png`: 1440 wide, full page, taken with the viewport set to the page's 1214px height so the capture does not resize it (a resize re-fires hover and restarts the marquee's chase). The pointer rests on Minecraft Launcher, so the selector has closed around it; the marquee is at rest, every other bulb lit.
- `vip-lobby-attract-mode-phone.png`: 390 × 844 viewport, full page, at rest.
- `vip-lobby-attract-mode-launch.png`: 1440 × 1000 viewport, 160ms into Open on Minecraft Launcher, frozen by pausing the page's animations. The navigation was held back for the capture.

## Pocket Worlds

Captured on 2026-11-12 with headless Chrome from the local Vite render of `web/src/minecraft/Launcher.tsx`, after giving each Minecraft world its own plot. Both images show the full page on made-up worlds with example addresses: Survival ready, Creative starting and Adventure stopped. No real server was started, and the captures contain no Member information.

- `minecraft-pocket-worlds-desktop.png`: 1440 × 1000 viewport.
- `minecraft-pocket-worlds-phone.png`: 390 × 844 viewport at twice the pixel density.

## Board moves

Captured on 2026-11-13 with headless Chrome from the local Vite render of `web/src/minecraft/Launcher.tsx`, after giving the Launcher's board its moves. Each image is a strip of frames from one move: the page's animations were paused the instant the change arrived and stepped to the time under each frame. They use the same made-up worlds with example addresses. No real server was started, and the captures contain no Member information.

- `minecraft-board-moves-desktop.png`: 1440 × 1000 viewport, the board as the device changes from Java Edition to Bedrock. The words change at once, the rows open for the port, and the underline travels to the new tab.
- `minecraft-board-moves-ready.png`: the same viewport, Creative's column as its world turns ready. Its rows close, and Copy address turns custard and then rises onto its coral base.
- `minecraft-board-moves-phone.png`: 390 × 844 viewport at twice the pixel density, Creative turning ready with the worlds stacked. The world under it rides up as the rows close.

## Add to iCloud Calendar

Captured on 2026-11-14 with headless Chrome from the local Vite render of `web/src/soccer/LetsPlanSoccer.tsx`, after adding Add to iCloud Calendar (ADR 0012). The phone images are the Get your calendar section alone; the desktop image is the page for the height of that section. All are at twice the pixel density. Every answer the page asked for was made up inside the test browser: a Member with the `soccer` grant, one Team with three games, an example Apple Account address and two calendars. Nothing was sent to Apple, and the captures contain no Member information.

- `soccer-icloud-connect-phone.png`: 390 wide, not connected: the note, the steps and the form.
- `soccer-icloud-added-phone.png`: 390 wide, connected, after pressing Add 3 games.
- `soccer-icloud-connected-desktop.png`: 1280 wide, connected, after the same press.
