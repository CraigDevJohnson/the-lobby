---
status: accepted
date: 2026-11-27
---

# Keep family wish lists in The Lobby's table

Wish Lists serves one invited family group with The Lobby's Sign-in session and per-Tool grants. On 2026-11-27, Craig chose to keep the lists in the site's existing DynamoDB table. This extends the exception to separate Tool backends in ADR 0001. A separate service would need its own deployment, storage, and access policy for a small collection of lists whose access and removal already belong to The Lobby.

Each list has its own record. The site checks its revision before saving so a stale edit cannot overwrite a newer one. The site checks the `wish` grant for every read and write. A Member edits their personal list, and the Owner edits managed lists for people who do not sign in. Taking away the grant ends access and leaves the personal list visible to the family. Removing the Member deletes their personal list as part of the existing removal process. Managed lists belong to the family and survive removal of a Member.
