---
status: accepted
date: 2026-10-05
---

# A React app served by the site's own Go server

The site's screens are a React and TypeScript single-page app, built by Vite into static files that the site's Go server serves beside its API. The landing page's HTML is generated at build time, so it renders before any script loads and shared links get a real preview. Craig wanted React on show, not pages rendered by Go, and this shape gives React a live server behind it while keeping all server logic in the language he knows.

## Considered options

- Pages rendered by Go with htmx for the interactive parts. Cheapest and simplest (one language, no JavaScript build), but Craig did not want Go-rendered pages.
- A server-rendered React framework such as Next.js. Rejected: sign-in and Sign-in sessions would then be written in TypeScript, and a Node runtime would have to be hosted on Lambda through a community-maintained adapter with no official OpenTofu module.
- A static landing page with small plain-JavaScript widgets per Tool. Rejected: too little for the Minecraft screens to come.

## Consequences

- TypeScript is confined to the screens. Sign-in sessions, access checks and calls to Tool backends are Go.
- The app files are the same for everyone and are cached by Cloudflare; the reactivity runs in the browser. The browser is never trusted for access: it shows Member sections because the server told it the role, and the server checks the Sign-in session again on every Member action.
