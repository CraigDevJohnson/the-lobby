---
status: accepted
date: 2026-11-15
---

# Invite emails go through Fastmail's SMTP server

Adding a Member on Lobby Admin is the whole invite (ADR 0013), but nobody told the new Member. The page showed a line to copy, and Craig pasted it into an email or message himself; the new Member couldn't tell which Tools they had, or how to reach a Minecraft world or Foundry, until they signed in and looked around (#101). Now, when Lobby Admin's add succeeds, the site sends the new Member one plain-text email from `The Lobby <lobby@craigdevjohnson.com>`: how to sign in at that environment's own address, their Tools with links, each Minecraft world's addresses with the `minecraft` grant, and how to join Foundry with the `foundry` grant.

Craig chose on 2026-11-15 to send it through Fastmail, his email provider, which already accepts any address at the domain and delivers its replies to him. The site signs in to `smtp.fastmail.com:465` with an app password limited to SMTP and sends the email itself, over TLS from the first byte. Both environments send, each from its own `SITE_BASE_URL`, since each manages its own Members.

## Considered options

- Amazon SES in the workloads account, the issue's first recommendation. One domain identity with Easy DKIM shared by dev and production, so it would sit in `infra/bootstrap` with its three DKIM records in Cloudflare; the site role would get `ses:SendEmail` on that identity and From address, which the site boundary and the deployer would need to allow, a bootstrap apply with Craig's admin sign-in; the account would need production access out of SES's sandbox, a one-off request AWS reviews; and aws-design.md would gain the identity. It keeps Craig's own mail account out of the site, at the cost of that setup and a new piece to look after.
- No sending: an "Email this invite" button that opens Craig's own mail app with the email filled in (`mailto:`). Nothing to set up, but Craig still presses send for every Member, which is the step the issue set out to remove.

## Consequences

- An app password for Craig's own mail account is in Parameter Store, as the SecureStrings `username` and `password` under `/the-lobby/<env>/smtp/` (`SMTP_PARAMETERS`), which Craig makes in Fastmail and writes by hand (infra/README.md, "Inviting Members"). It is limited to SMTP, never his account password, so it can send mail as him but not read it. He can revoke it in Fastmail at any time; the site reads it at the first add after it starts and keeps it, so a new one takes effect at the next deploy or cold start.
- No DNS change and no AWS permission beyond the site role reading that path (`InviteEmailSettings`, `ssm:GetParameters`), which the bootstrap boundary already allowed for `/the-lobby/<env>/*`. There is no sandbox to leave and no new AWS service.
- Fastmail already signs mail from the domain with DKIM, so the invite passes DMARC as Craig's own mail does, and a reply reaches him.
- A failed send never blocks the add: the record is saved first, the answer says `"invite": "failed"`, and the page says the email didn't send and still shows the line to copy. Unless both `SMTP_PARAMETERS` and `INVITE_FROM` are set, as on a developer's computer, the site sends nothing and the page reads as it did before.
- The Minecraft worlds' addresses come from the Launcher's backend (`GET /server`, naming no Member), the same `connection` values /minecraft shows, so the site keeps no copy; when that read fails the email links to /minecraft instead. The add waits for that read and the SMTP exchange, so its route has 20 seconds rather than 8.
- A server's refusal can quote the Member's address, so a failed send is logged by its stage (settings, dial, auth, mail, rcpt or data) and the server's reply code alone. The log never carries the address, as before.
- Only Lobby Admin's add sends. A later change to a Member's Tools sends nothing, and nor does the Owner command (`site member add`). There is no Resend button: the line to copy covers that. Changed by ADR 0017 (2026-11-19): the daily iCloud update sends a second email, through the same sender and SMTP login, to a Member whose game the league moved.
- The Privacy page says the site emails a new Member their invite through Fastmail.
