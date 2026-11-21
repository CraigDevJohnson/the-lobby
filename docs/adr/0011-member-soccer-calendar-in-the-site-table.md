---
status: accepted
date: 2026-11-05
---

# A Member's Let's Plan Soccer calendar is kept in the site's table

A Member with the `soccer` grant gets one Member link that follows their Remembered teams from Session to Session (#53), and later a Google calendar the site writes their games into (#52). What those need to remember about the Member lives in the site's own DynamoDB table (`site-<env>-sessions`), beside the Member records, as items of their own:

- `soccer#<email>`: the Remembered teams, each a name and its Team IDs, newest first, at most two, and the Member link's secret.
- `soccerlink#<secret>`: the Member a Member link belongs to, so a calendar app's fetch, which carries no cookie, finds them.
- `google#<email>` (#52): the Member's Google access, encrypted, and the calendar they picked.
- `icloud#<email>` (ADR 0012): the Member's iCloud access, encrypted, and the calendar they picked.

The soccer backend still stores nothing. The site sends it a Member's Remembered teams on each Member link fetch and keeps the Remembered teams it answers, so a Team the backend moved on to its next Session is kept. The site calls Google itself. Craig chose this on 2026-11-05. Changed by ADR 0015 (2026-11-16): the site also sends them once a day for each Member who has the daily iCloud update on.

This changes ADR 0001's line that everything a Tool remembers about a Member lives in that Tool's own data. For Let's Plan Soccer, it lives in the site's.

## Considered options

- A table in the soccer backend, as ADR 0001 and the baseline planned. The backend's role may only write its logs (`aws-design.md` §6.5), so this needs a new table, a change to the deploy roles applied by hand, and an `aws-design.md` change, for a handful of items per Member. Removing a Member would also have to reach into another Tool's data.
- On the `member#<email>` item. `site member add` writes that item whole (a PutItem), so re-running it would wipe a Member's Teams and link.

## Consequences

- No new AWS resource and no IAM change: the site role already reads and writes single items in its table. The site uses GetItem, PutItem and DeleteItem only.
- Removing a Member (`site member remove`) also deletes their `soccer#` and `soccerlink#` items, so their link stops at once; re-running `site member add` leaves them. The link also stops answering as soon as the Member loses the `soccer` grant.
- Changed on 2026-11-20 (#130): the site puts a Member's `soccer#`, `soccerlink#`, `google#` and `icloud#` items only while their `member#` item is there and not marked as being removed (ADR 0013). The put and that condition are one DynamoDB transaction (TransactWriteItems), so a request already on its way when the Owner removes the Member writes nothing once the removal has marked them, and the removal deletes what was written before. A later Member with the same address therefore starts with no calendar and no connection. There is still no IAM change: a transaction's Put and Update actions are authorised by `dynamodb:PutItem` and `dynamodb:UpdateItem`, which the site role and the bootstrap boundary already allow. So the condition sits on an Update of the `member#` item that sets its `kind` to what it already is. A ConditionCheck action would be the plainer way, but it needs `dynamodb:ConditionCheckItem` added to the site role and to the boundary, which is a bootstrap apply before the Deploy. The condition names no generation, because the calendar routes are told the Member's address and not their generation: it holds against a removal, and not against a removal and a new invitation of the same address that both land inside one request.
- A Member link's secret appears in the item keys and the page that shows it, never in a log line: the site logs the link's route, and its store errors name only the kind of record.
- `aws-design.md` §6.4 lists the new items in the site table (aws-setup, docs only).
