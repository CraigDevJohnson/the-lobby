---
status: accepted
date: 2026-10-05
---

# Cloudflare Access proves identity; the site keeps its own Sign-in session and access record

Members sign in through Cloudflare Access, by emailed one-time code or Google, from an invite list that Craig maintains. Access only proves who someone is. The site then issues its own 90-day Sign-in session cookie, renewed on each visit, and checks its own record of which Tools the Member may use on every request, so removing a Member takes effect at once. Access covers email delivery and identity for $0 and is defined in OpenTofu.

## Considered options

- WorkOS AuthKit in the same role. Its sign-in page sits on a WorkOS domain unless you pay $99 a month.
- Amazon Cognito. Everything in AWS, but it needs an approved email-sending service and custom code for Google invitations. It remains the fallback if the Access trial fails.
- Writing sign-in in Go. Code expiry, attempt limits, Sign-in sessions and email delivery would all be Craig's to maintain, and sign-in is the one part where a mistake hurts other people.

## Consequences

- All site traffic passes through Cloudflare's proxy.
- Adding a Member takes two steps: the Access invite list, then Tool access in the site.
- This pairing was designed for this site and is not a documented recipe, so it is proven in a small trial before anything is built on it.
- Only sign-in paths sit behind Access; everything else is guarded by the site's own Sign-in session. Visitors never meet Access, and the calendar link addresses must sit outside it, because calendar apps cannot sign in.
