---
status: accepted
date: 2026-11-17
---

# Private input files live in the state bucket

Four files that OpenTofu reads are gitignored: `infra/bootstrap/bootstrap.auto.tfvars`, `infra/zero-trust/zero-trust.auto.tfvars`, and `secrets.auto.tfvars` in `infra/environments/dev` and `infra/environments/prod`. They hold the origin secrets (ADR 0008) and personal data such as the budget's address and the two adults' addresses (ADR 0009, ADR 0010), so they are never committed, and they have existed only on Craig's computer. A cloud thread signs in to AWS with a device code Craig approves on his phone and can stand in for his computer on other operator steps (`infra/README.md`), but a fresh checkout has none of these files, so it could not plan `bootstrap` or `zero-trust`.

On 2026-11-17 Craig chose where the files live: in S3, beside the state. Their place is the state bucket that `bootstrap` already owns, under `the-lobby/inputs/` followed by each file's path from the repository root. `task infra:inputs:pull` copies the ones the bucket holds into a checkout, and `task infra:inputs:push` copies a checkout's files up; `infra/inputs.sh` does both. One last job uploads them from Craig's computer, with his go-ahead, and after that his computer is not needed for them again. That upload ran the same day (`infra/README.md`).

The bucket fits because it is already private, versioned and encrypted, the state objects in it already hold the same values, and the sign-in a cloud thread needs for the state is the one that reads the files. Nothing new is created, and no `.tf` file changes.

## Considered options

- Rebuilding the files from OpenTofu state. Every value is in a root's state, but reading them back copies secret values out of state inside the thread, and it needs the `tofu state` command, which aws-setup's rules count among the commands that need Craig's approval.
- Leaving them on Craig's computer. Every plan of `bootstrap` or `zero-trust` would still need that computer.

## Consequences

- An admin session in the workloads account can read and write under `the-lobby/inputs/`. The three pipeline roles can read there, because each carries `ReadOnlyAccess`. That opens nothing new to them: they already read the state objects, which hold the same values. They cannot write there. The planner writes nothing and each deployer writes only under `the-lobby/dev/` or `the-lobby/prod/`, so an input file is never put under those two. No Deny is added. Changed by #113 (2026-11-17): each pipeline role is now denied every S3 object outside its own environment's state prefix, so none of the three can read under `the-lobby/inputs/`, nor the other roots' state (`infra/README.md`, What GitHub Actions may do).
- The origin secrets are still in `bootstrap.auto.tfvars` and the environment files, and still in state. This decision moves no secret out of either.
- So a pull writes files only their owner can read (mode 600), `infra/inputs.sh` prints file names and object versions and never a file's content, and a cloud thread never opens or prints a pulled file. It deletes the pulled files and any saved plan before it signs out.
- A pull is a read and needs no approval. A push changes AWS and needs Craig's go-ahead for that push. The natural moment is right after an approved apply, so the bucket holds what was last applied.
- A pull does not replace a local file that differs from the bucket's copy unless `FORCE=1`. The bucket keeps every version of a file, as it does of the state, and each command prints the version id it read or wrote.
- The two environment files are optional copies of what GitHub holds for the pipeline (`docs/pipeline.md`) and may not exist anywhere. A pull names what the bucket lacks and goes on.
- Tried later the same day, once a cloud thread had uploaded all four files with Craig's go-ahead: it pulled them back and planned `bootstrap`, which showed no changes, and `zero-trust`, which showed only the in-place update of the two DNS locations that `infra/zero-trust/locations.tf` describes. The Cloudflare token a cloud container is given was enough for both plans.
- aws-setup's `aws-design.md` is where the AWS arrangement is written down, so the prefix, the approval rule for a pull and a push, and what a cloud thread does with a pulled file go there too, in a separate change to that repository.
