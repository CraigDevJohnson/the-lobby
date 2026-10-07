---
status: accepted
date: 2026-10-05
---

# The site calls Tool backends with signed requests

The site calls a Tool backend by sending an AWS-signed web request to the backend's own Lambda address, which AWS lets only the site's role call. The site names the Member in a header, and the backend trusts that header because nothing but the site can reach it. This costs nothing, stores no keys, and leaves the backend an ordinary web server, so the same code runs on a developer's computer.

## Considered options

- Direct Lambda invocation through the AWS SDK. Rejected: the backend stops being a web server and can no longer run locally as one.
- A second API Gateway in front of each backend. Rejected: more to define and more to pay for, for no gain.

## Consequences

- A Tool backend never does its own sign-in. It believes the Member named in the header, so the site's access check is the only one.
- Each backend's address is private to the site; Visitors and Members never call it directly.
