---
status: accepted
date: 2026-11-17
---

# Private input files live in the state bucket

Four files that OpenTofu reads are gitignored: `infra/bootstrap/bootstrap.auto.tfvars`, `infra/zero-trust/zero-trust.auto.tfvars`, and `secrets.auto.tfvars` in `infra/environments/dev` and `infra/environments/prod`. They hold the origin secrets (ADR 0008) and personal data such as the budget's address and the two adults' addresses (ADR 0009, ADR 0010), so they are never committed, and they have existed only on Craig's computer. A cloud thread signs in to AWS with a device code Craig approves on his phone and can stand in for his computer on other operator steps (`infra/README.md`), but a fresh checkout has none of these files, so it could not plan `bootstrap` or `zero-trust`.

On 2026-11-17 Craig chose where the files live: in S3, beside the state. Their place is the state bucket that `bootstrap` already owns, under `the-lobby/inputs/` followed by each file's path from the repository root. `task infra:inputs:pull` copies the ones the bucket holds into a checkout, and `task infra:inputs:push` copies a checkout's files up; `infra/inputs.sh` does both. One last job on Craig's computer uploads them, with his go-ahead, and after that his computer is not needed for them again. `infra/README.md` says whether that upload has run.

The bucket fits because it is already private, versioned and encrypted, the state objects in it already hold the same values, and the sign-in a cloud thread needs for the state is the one that reads the files. Nothing new is created, and no `.tf` file changes.

## Considered options

- Rebuilding the files from OpenTofu state. Every value is in a root's state, but reading them back copies secret values out of state inside the thread, and it needs the `tofu state` command, which aws-setup's rules count among the commands that need Craig's approval.
- Leaving them on Craig's computer. Every plan of `bootstrap` or `zero-trust` would still need that computer.

## Consequences

- An admin session in the workloads account can read and write under `the-lobby/inputs/`. The three pipeline roles can read there, because each carries `ReadOnlyAccess`. That opens nothing new to them: they already read the state objects, which hold the same values. They cannot write there. The planner writes nothing and each deployer writes only under `the-lobby/dev/` or `the-lobby/prod/`, so an input file is never put under those two. No Deny is added.
- The origin secrets are still in `bootstrap.auto.tfvars` and the environment files, and still in state. This decision moves no secret out of either.
- So a pull writes files only their owner can read (mode 600), `infra/inputs.sh` prints file names and object versions and never a file's content, and a cloud thread never opens or prints a pulled file. It deletes the pulled files and any saved plan before it signs out.
- A pull is a read and needs no approval. A push changes AWS and needs Craig's go-ahead for that push. The natural moment is right after an approved apply, so the bucket holds what was last applied.
- A pull does not replace a local file that differs from the bucket's copy unless `FORCE=1`. The bucket keeps every version of a file, as it does of the state, and each command prints the version id it read or wrote.
- The two environment files are optional copies of what GitHub holds for the pipeline (`docs/pipeline.md`) and may not exist anywhere. A pull names what the bucket lacks and goes on.
- Not tried on the day of this decision: a pull or a plan of `bootstrap` or `zero-trust` in a cloud thread, and whether the Cloudflare token a cloud container is given is enough for either plan. `zero-trust` needs Zero Trust Edit (ADR 0009).
- aws-setup's `aws-design.md` is where the AWS arrangement is written down, so the prefix, the approval rule for a pull and a push, and what a cloud thread does with a pulled file go there too, in a separate change to that repository.
