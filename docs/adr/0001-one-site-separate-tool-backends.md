---
status: accepted
date: 2026-10-05
---

# One site owns the interface and sign-in; each Tool's backend is its own program

The site is a single React app served by its own Go server, and it alone holds sign-in, Sign-in sessions and the record of which Tools each Member may use. Every Tool (the Schedule Downloader first, Minecraft later) is a separate Go backend in its own repo, deployed separately and storing its own data; the site calls it on the Member's behalf.

## Considered options

- Fully separate tools, each with its own interface and sign-in, behind a front page of links. Rejected: sign-in would be built and maintained once per tool.
- One codebase holding everything. Rejected: every Tool's deployment would be coupled to every other's, and the codebase would grow without a boundary.

## Consequences

- A change to a Tool's screen can touch two repos: the site for the screen, the Tool's repo for the logic.
- The site knows only who the Members are and which Tools they may use. Everything a Tool remembers about a Member (for the Schedule Downloader: Remembered teams, the Connected calendar) lives in that Tool's own data.
