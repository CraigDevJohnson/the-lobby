---
status: accepted
date: 2026-10-05
---

# Go backends run on AWS Lambda

The site and every Tool backend are Go programs with Chi routers, run on AWS Lambda in Oregon, in the AWS account Craig already uses for his other workloads, through a small adapter so that the same program also runs as an ordinary server on a developer's computer. Craig ranked idle cost above everything else, ahead of a language he knows and infrastructure defined as code: at this traffic Lambda costs about nothing, while a container that runs all day (ECS Fargate) costs roughly $9 a month for the smallest task before any way in from the internet.

## Considered options

- Google Cloud Run: takes any container, but its custom domains were not production-ready.
- Fly.io: simpler, about $2 a month. Rejected: $2 is more than $0, and it sits outside AWS, where Craig's other infrastructure and OpenTofu already live.
- A container on ECS: no cold starts, but a process that runs 24 hours a day for a site nobody visits most of the time.

## Consequences

- Every piece of work is either a request or a scheduled run. The daily Connected calendar update is started by EventBridge Scheduler. Nothing may rely on a process staying up.
- A cold start of a few hundred milliseconds after a quiet spell.
- Every function gets a low concurrency cap as a cost brake, because an AWS budget can alert, or act on IAM, EC2 and RDS, but cannot stop a Lambda being invoked.
