---
name: release
description: Cut a GoCast release — pick the next version from the conventional commits, preview the generated release notes, tag `dev`, and watch the tag-triggered workflows publish the notes and the images. Use when asked to release, cut a version, tag a release, or fix the notes on a release that already exists.
---

# Releasing GoCast

A release here is **one thing: pushing a tag.** Everything else is already wired to
that push — the container images for server, workers, runners, ingest and rtmp-proxy
build from it, the Matrix channel gets a message, and
[`.github/workflows/release.yml`](../../../.github/workflows/release.yml) creates the
GitHub release with notes rendered from the commit history.

So the work is in deciding *what* to tag and confirming the notes read well, not in
writing the release by hand.

## Cut a release

### 1. Check `dev` is worth releasing

```bash
git switch dev && git pull --ff-only
gh run list --branch dev --limit 5        # Go tests, eslint, golangci-lint, frontend
```

Tagging red `dev` ships broken images: the deploy workflows don't gate on the test
workflows. If a check is failing, say so and stop — don't tag around it.

### 2. Pick the version

```bash
.github/scripts/next-version.sh           # derived from the commits since the last tag
.github/scripts/next-version.sh minor     # or force a level
```

The derivation is mechanical (`!`/`BREAKING CHANGE` → major, any `feat` → minor, else
patch). It is a suggestion. GoCast has been shipping patch bumps for sizeable
changes, so **confirm the version with the user before tagging** rather than assuming
the script is right.

### 3. Preview the notes

```bash
.github/scripts/release-notes.sh HEAD v1.7.12 | less
```

The tag doesn't exist yet, so preview with `HEAD` as the range end and the *current*
last tag as the second argument; the published notes differ only in the compare link.
Read it and check:

- Does anything land in **📝 Other Changes**? That means a commit subject missed the
  conventional-commit shape. It's cosmetic, not worth rewriting history, but it is
  worth mentioning to the user.
- Anything user-visible in **📦 Dependencies** (a Tailwind or Node major, say) that
  deserves to be called out in the body rather than folded away?

### 4. Tag and push

```bash
git tag -a v1.7.13 -m "v1.7.13"
git push origin v1.7.13
```

Annotated tags only — `git describe` walks them to find the previous release, and the
notes range depends on it.

### 5. Watch it land

```bash
gh run list --limit 10                    # release + the five deploy workflows
gh release view v1.7.13
```

`Publish Release Notes` should finish in well under a minute. The image builds take
longer and are independent of it.

## Fixing notes after the fact

The notes are regenerated, not edited in place, so never hand-edit a release body —
the next re-run overwrites it. Re-run the workflow instead:

```bash
gh workflow run release.yml -f tag=v1.7.13
gh workflow run release.yml -f tag=v1.7.13 -f previous_tag=v1.7.11   # widen the range
```

It updates an existing release rather than failing on it, which is also what makes it
safe to re-run after a workflow-level failure.

## Where the shape of the notes comes from

[`.github/scripts/release-notes.sh`](../../../.github/scripts/release-notes.sh) maps
conventional-commit types to sections, links each entry to its PR (from the trailing
`(#1234)` that squash merges leave behind) or else to its commit, resolves author
handles through one GitHub compare API call, folds the Renovate/Dependabot flood into
a collapsed `<details>`, and lists first-time contributors.

If a category needs to change, change it there — the workflow is a thin wrapper and
the script runs identically on a laptop. Without `gh` authenticated it falls back to
git author names, so previewing offline still works.
