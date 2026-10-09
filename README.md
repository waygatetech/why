# why

`why` records decisions as markdown files in your repo — what was asked, what
was ruled, why, who ruled it, and what it supersedes — and refuses to let an
agent quietly claim a human made a call.

It is built to sit alongside [tix](https://github.com/waygatetech/tix) (tickets
and plans) and [vet](https://github.com/waygatetech/vet), but works on its own
in any git repo.

## Install

```sh
go install github.com/waygatetech/why@latest   # Go 1.25+
```

Or grab a binary from [Releases](https://github.com/waygatetech/why/releases).

## Decision files

Decisions live in `<repo root>/decisions/<id>.md`. The id is
`<ticket>-<n>`, assigned automatically.

```markdown
---
id: why-590-1
ticket: why-590
concepts: [decision-format]
date: 2026-10-08
decided_by: human
supersedes: [why-579-2]
---

## Question
...

## Ruling
...

## Why
...
```

`decided_by` is one of `human`, `agent-proposed-human-approved` or `agent`.
Unknown frontmatter keys or sections are errors. Files are never overwritten;
to change a ruling, record a new decision that supersedes the old one.

## Commands

```sh
# Record one decision from flags
why record --ticket why-12 --concept auth \
  --question "Sessions or JWTs?" --ruling "Sessions" --why "One server, no revocation pain"

# Record a decision file from stdin (flags fill in missing fields)
why record --ticket why-12 < decision.md

# Record every decision in an approved tix plan (idempotent)
why record --from-plan plans/why-12.md

# Active decisions, optionally by concept; --format md for feeding to vet
why list --concept auth
why list --all --format md

# A decision plus its supersedes chain in both directions
why show why-12-1

# Fail if this branch forged or rewrote provenance
why check --base main
```

## Provenance

`why` only writes `human` or `agent-proposed-human-approved` when it runs from
the tix approve hook (`TIX_HOOK=approve`). Anywhere else, those values are
downgraded to `agent` with a warning. The human gesture is the Claude Code
permission prompt on `tix approve`, so configure:

```
ask:  Bash(tix approve:*)
deny: Bash(why record:*--decided-by human*),
      Bash(why record:*--decided-by agent-proposed-human-approved*)
```

When a human decision is written, `why` stores a receipt (the file's hash) in
`.git/why/approved`. `why check`, meant for the tix done hook, compares
`decisions/` against the merge base with `--base` and fails when:

- an existing decision's `decided_by` or ruling changed,
- a new decision claims human provenance without a receipt for its exact
  content, or
- an agent decision supersedes a human one.

Receipts live in `.git`, so they are local to the clone that recorded them;
run `why check` there, not in CI.

This guards against accidental upgrades, not a determined agent with shell
access — it could set `TIX_HOOK` or forge a receipt itself.

## License

[MIT](LICENSE)
