---
status: accepted
date: 2026-10-05
---

# API Gateway connects Cloudflare to the site's Lambda

Browsers reach the site through Cloudflare, which proxies to an AWS API Gateway HTTP API in front of the site's Lambda. It is defined entirely in OpenTofu, costs about $1 per million requests, and puts no code in the path of a request. To stop anyone reaching the site around Cloudflare, Cloudflare adds a secret header that the Go server checks.

## Considered options

- A small Cloudflare Worker that forwards each request to Lambda. $0, but it puts TypeScript in the path of every request and needs a long-lived AWS key stored in Cloudflare.

## Consequences

- API Gateway's HTTP API caps every request at 30 seconds, and Lambda caps request and response bodies at 6 MB each. Nothing the site does may need more.
- Neither vendor documents this exact pairing, so it joins the Access trial as something to prove first.
- Bot Fight Mode stays off for the whole domain and Browser Integrity Check is off for the calendar link addresses, both set in OpenTofu. Calendar apps are not browsers, and on Cloudflare's free plan Bot Fight Mode cannot be skipped for one path.
