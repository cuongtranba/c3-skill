# Report — turning friction with C3 into a fixable issue

**Question:** did C3 itself just fail, and is that recorded anywhere?

A crash, a gate that refused valid input, a `help[]` hint that dead-ended, a reference this
CLI contradicts — you are the only one who sees it, and it is gone by the next turn. Report
turns that observation into a de-duplicated GitHub issue on `cuongtranba/c3-skill`, the queue
C3's maintenance loop drains.

The wrapper builds the issue; it never sends it. You run `gh`, and only after the consent
gate below clears.

## The three kinds

| Kind | Report it when | `--subject` |
|------|----------------|-------------|
| `fault` | c3x misbehaved — crash, a gate refusing valid input, malformed output | derived from the last failed command |
| `guidance` | a reference or `help[]` hint contradicts what the CLI actually does | **required** — the reference or op |
| `gap` | you needed an operation C3 does not have and had to stop | **required** — the operation you wanted |

`--subject` is required for the prose kinds because it, not your wording, is what holds the
fingerprint steady when the same defect is described twice.

## Never report these

The tracker is only useful while every issue in it is a real defect. Refuse to file for:

- **Your own input being wrong** — a bad id, malformed frontmatter, a schema violation. That
  is `check` working. A rejection is not a bug.
- **A frozen fact refusing a direct edit.** That is the contract, not a fault (SKILL.md
  §The shared contract). The refusal already names the legal path.
- **A synthesized membership row you expected to author.** Also the contract (SKILL.md
  §Membership).
- **Anything `check` or `eval` already reports.** Fix it through a change-unit (change.md).
- **A stale cache after a branch switch** — run `repair` first (audit.md §Layer 1). Report
  only if `repair` does not clear it.

Report the defect, not the symptom of using C3 correctly.

## Build the envelope

```bash
C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" report fault --summary "check panics on a sealed ref"
C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" report guidance --subject change.md --summary "says a frontmatter uses patch applies, apply rejects it"
C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" report gap --subject diff --summary "no way to diff two facts"
```

A `fault` attaches the last failed command and its cause from the activity trail — you do not
retype the error. The command works with no `.c3/` at all, which is deliberate: a wrecked
project is exactly when a report matters.

**What you write is what ships.** The wrapper strips filesystem paths (filenames survive,
directories are elided) and keeps entity ids, which are structural. It cannot strip domain
meaning out of your prose. Describe the *C3 defect*, never the user's business content.

## The consent gate

`consent` in the envelope is the project's policy, stored at `.c3/report.json`.

| Value | What you do |
|-------|-------------|
| `ask` (default) | Show the user the rendered `title` and `body`. **File nothing until they say yes.** |
| `auto` | File it without asking. |
| `off` | The wrapper refuses to build at all. Do not work around it. |

```bash
C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" report consent        # read the policy
C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" report consent auto   # set it
```

Never set `consent` on the user's behalf. Filing publishes to a public tracker and cannot be
cleanly withdrawn.

## Dedupe, then file

The `fingerprint` is in the body, so a full-text search finds every prior sighting. It
excludes the c3x version on purpose — a defect resurfacing after an upgrade belongs on the
issue that already exists.

```bash
gh issue list -R cuongtranba/c3-skill --state all --search "<fingerprint>" --json number,title,state
```

A hit → comment on it, naming the version you saw it on. No hit → search the title's key
words too; the fingerprint cannot catch a duplicate someone worded differently. Only then:

```bash
gh issue create -R cuongtranba/c3-skill --title "<title>" --body "<body>" --label <labels>
```

## Output

End in a verdict — never a bare issue link.

```
**C3 Self-Report**

| Field | Value |
|-------|-------|
| Kind | fault / guidance / gap |
| Subject | … |
| Fingerprint | … |
| Consent | ask / auto |
| Dedupe | new / duplicate of #N |
| Action | filed #N / commented on #N / awaiting your confirmation / not filed |

**Reported:** one line on what is broken.
**Not reported:** anything ruled out above, and why.
```

If the consent gate is `ask` and the user has not answered, **Action** is
`awaiting your confirmation` — the report is not done, and saying otherwise is a false claim.
