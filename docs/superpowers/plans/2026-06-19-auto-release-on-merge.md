# Auto-cut releases on merge to main — Implementation Plan

> **Adaptation note (2026-06-21):** Tasks 1 (clamp script), 3 (`version.yml`),
> and 4 (`pr-title.yml`) were implemented as written. Task 2 was re-fit to the
> Tauri/Rust `release.yml` that replaced the Go-era workflow during the v2
> migration: the reusable `release.yml` now writes the computed version into
> `src-tauri/tauri.conf.json` at build time (jq, not committed) and `tauri-action`
> tags + builds it; a `publish` job flips the draft to published when called with
> `publish: true`. The Go-specific Task 2 steps (setup normalization,
> `create-release`/`checksums`, `persist-credentials`/SC2035) no longer apply.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On every merge to main, compute the next semver from Conventional Commits and produce a published, fully-built GitHub release — staying in the 0.x range until 1.0.0 is shipped deliberately.

**Architecture:** A new `version.yml` runs on push to main, computes the version with `mathieudutour/github-tag-action` (dry-run) + a 0.x clamp script, and calls the existing build pipeline via `workflow_call` in the same run (no pushed tag, no secret needed). `release.yml` is refactored to be callable and to flip its draft to published when invoked from the auto path. A `pr-title.yml` enforces Conventional Commits on PR titles (which become commit subjects under squash-merge).

**Tech Stack:** GitHub Actions, Bash, `mathieudutour/github-tag-action@v6.2`, `amannn/action-semantic-pull-request@v5.5.3`. Validation with `actionlint`, `zizmor`, `shellcheck`, `shfmt`.

**Pinned action SHAs (resolved 2026-06-19):**
- `actions/checkout` → `34e114876b0b11c390a56381ad16ebd13914f8d5` # v4.3.1
- `mathieudutour/github-tag-action` → `d28fa2ccfbd16e871a4bdf35e11b3ad1bd56c0c1` # v6.2
- `amannn/action-semantic-pull-request` → `0723387faaf9b38adef4775cd42cfd5155ed6017` # v5.5.3

---

## File Structure

- **Create** `build/ci/clamp_version.sh` — pure version-clamp logic (the only real algorithm; unit-tested).
- **Create** `build/ci/clamp_version_test.sh` — table-driven bash test for the clamp.
- **Modify** `.github/workflows/release.yml` — add `workflow_call` trigger, normalize `setup`, parameterize notes, add a `publish` job; minor touched-file hygiene (SC2035 fix, `persist-credentials: false`).
- **Create** `.github/workflows/version.yml` — on-merge versioner that calls `release.yml`.
- **Create** `.github/workflows/pr-title.yml` — Conventional-Commit PR-title lint.

---

## Lint baseline (grounding for the gates)

The existing `release.yml` already has pre-existing lint debt, captured before any change:
- `actionlint`: one finding — `SC2035` at the `checksums` job (`sha256sum *`).
- `zizmor`: 34 findings (mostly `artipacked` — checkouts without `persist-credentials: false`; plus `cache-poisoning` low-confidence on `setup-go` caching).

Gate for this work:
- `actionlint .github/workflows/` → **clean** (we fix the one pre-existing SC2035 since we are editing the file).
- `zizmor .github/workflows/version.yml .github/workflows/pr-title.yml` → **zero findings** (new files must be clean).
- `zizmor .github/workflows/release.yml` → **no regression** vs baseline; the `artipacked` count drops because we add `persist-credentials: false` to the checkouts we touch. Remaining `cache-poisoning`/unpinned-first-party-tag findings are pre-existing and accepted (out of scope to re-pin the whole file).

---

## Task 1: 0.x version clamp script

**Files:**
- Create: `build/ci/clamp_version.sh`
- Test: `build/ci/clamp_version_test.sh`

- [ ] **Step 1: Write the failing test**

Create `build/ci/clamp_version_test.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
clamp="$here/clamp_version.sh"

fail=0
check() {
  local desc="$1" prev="$2" computed="$3" want="$4" got
  got="$(bash "$clamp" "$prev" "$computed")"
  if [ "$got" = "$want" ]; then
    printf 'ok   - %s\n' "$desc"
  else
    printf 'FAIL - %s: clamp(%s, %s) = %s, want %s\n' "$desc" "$prev" "$computed" "$got" "$want"
    fail=1
  fi
}

check "breaking change stays in 0.x" 0.1.0 1.0.0 0.2.0
check "feat minor within 0.x passes through" 0.1.0 0.2.0 0.2.0
check "fix patch within 0.x passes through" 0.1.0 0.1.1 0.1.1
check "breaking at higher 0.x minor" 0.9.0 1.0.0 0.10.0
check "already 1.x passes through" 1.2.0 2.0.0 2.0.0
check "1.x minor passes through" 1.2.0 1.3.0 1.3.0

exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash build/ci/clamp_version_test.sh`
Expected: FAIL — the script does not exist yet, so every `check` errors / fails.

- [ ] **Step 3: Write the implementation**

Create `build/ci/clamp_version.sh`:

```bash
#!/usr/bin/env bash
# Clamp a computed semantic version to the 0.x range.
#
# github-tag-action sends 0.x + a breaking change to 1.0.0; we stay in 0.x
# until 1.0.0 is shipped deliberately. When the previous version is 0.x and the
# computed version is not, replace it with a minor bump of the previous version
# (0.<minor>.<patch> -> 0.<minor+1>.0). Otherwise pass the computed version
# through unchanged.
#
# Usage: clamp_version.sh <previous_version> <computed_version>
set -euo pipefail

prev="${1:?previous_version required}"
computed="${2:?computed_version required}"

prev_major="${prev%%.*}"
computed_major="${computed%%.*}"

if [ "$prev_major" = "0" ] && [ "$computed_major" != "0" ]; then
  rest="${prev#*.}"        # <minor>.<patch>
  prev_minor="${rest%%.*}" # <minor>
  printf '0.%s.0\n' "$((prev_minor + 1))"
else
  printf '%s\n' "$computed"
fi
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash build/ci/clamp_version_test.sh`
Expected: PASS — all six `ok` lines, exit 0.

- [ ] **Step 5: Lint the scripts**

Run: `shellcheck build/ci/clamp_version.sh build/ci/clamp_version_test.sh && shfmt -i 2 -d build/ci/clamp_version.sh build/ci/clamp_version_test.sh`
Expected: no output from either (clean). If `shfmt` prints a diff, apply it with `shfmt -i 2 -w build/ci/*.sh` and re-run.

- [ ] **Step 6: Commit**

```bash
git add build/ci/clamp_version.sh build/ci/clamp_version_test.sh
git commit -m "ci: add 0.x version clamp script with tests"
```

---

## Task 2: Make release.yml reusable

**Files:**
- Modify: `.github/workflows/release.yml`

This task changes the workflow's triggers, the `setup` job, the release-notes source, adds a `publish` job, and applies touched-file lint hygiene. No behavior change for the existing tag-push / manual-dispatch paths (they default to draft).

- [ ] **Step 1: Add the `workflow_call` trigger**

Replace the `on:` block (lines 3–11) with:

```yaml
on:
  push:
    tags: ["v*"]
  # Manual trigger: builds <version> and creates the v<version> tag + draft release.
  workflow_dispatch:
    inputs:
      version:
        description: "Version to release, e.g. 0.1.0"
        required: true
  # Called by version.yml on merge to main with a computed version.
  workflow_call:
    inputs:
      version:
        description: "Version to release, e.g. 0.1.0"
        required: true
        type: string
      notes:
        description: "Release body / notes (Markdown)"
        required: false
        type: string
        default: ""
      publish:
        description: "Flip the draft to a published release"
        required: false
        type: boolean
        default: false
```

- [ ] **Step 2: Normalize the `setup` job across all three triggers**

Replace the `setup` job (the `steps:` of the `v` step, lines 26–38) so it reads `inputs.version` (set for both `workflow_call` and `workflow_dispatch`) and falls back to the tag name, using env vars (no `${{ }}` interpolation inside the script — required for `zizmor`):

```yaml
  setup:
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.v.outputs.version }}
      tag: ${{ steps.v.outputs.tag }}
    steps:
      - id: v
        shell: bash
        env:
          IN_VERSION: ${{ inputs.version }}
          REF_NAME: ${{ github.ref_name }}
        run: |
          set -euo pipefail
          if [ -n "$IN_VERSION" ]; then
            raw="$IN_VERSION"
          else
            raw="$REF_NAME"
          fi
          version="${raw#v}"
          {
            echo "version=$version"
            echo "tag=v$version"
          } >> "$GITHUB_OUTPUT"
          echo "Releasing version $version (tag v$version)"
```

- [ ] **Step 3: Parameterize the release notes in `create-release`**

In the `create-release` job, replace the "Ensure draft release exists" step (lines 48–64) so notes come from the `notes` input when present (else the existing default), written via a file to avoid quoting issues. Also add `persist-credentials: false` to that job's checkout.

```yaml
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4.3.1
        with:
          persist-credentials: false
      - name: Ensure draft release exists
        shell: bash
        env:
          IN_NOTES: ${{ inputs.notes }}
        run: |
          set -euo pipefail
          default_notes="See the assets below to download and install.

          Installers are unsigned until code-signing credentials are configured:
          macOS Gatekeeper and Windows SmartScreen may warn on first launch."
          notes="${IN_NOTES:-$default_notes}"
          if gh release view "$TAG" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
            echo "Release $TAG already exists; reusing it."
          else
            printf '%s\n' "$notes" > "$RUNNER_TEMP/notes.md"
            gh release create "$TAG" \
              --repo "$GITHUB_REPOSITORY" \
              --draft \
              --target "$GITHUB_SHA" \
              --title "HyperDeck Adapter $TAG" \
              --notes-file "$RUNNER_TEMP/notes.md"
          fi
```

- [ ] **Step 4: Add `persist-credentials: false` to the build-job checkouts**

In each of the `macos`, `windows`, and `linux` jobs, change the checkout step from `- uses: actions/checkout@v4` to:

```yaml
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
```

(These jobs authenticate to GitHub via the `GH_TOKEN` env, not the checkout credentials, so dropping persisted credentials is safe and clears `zizmor`'s `artipacked` findings.)

- [ ] **Step 5: Fix the pre-existing SC2035 in `checksums`**

In the `checksums` job's run block, change `sha256sum *` to `sha256sum -- *`:

```bash
          ( cd dist && sha256sum -- * > ../checksums.txt )
```

- [ ] **Step 6: Add the `publish` job**

Append a new job at the end of the file (after `checksums`). It flips the draft to published only when called with `publish: true`; the tag-push and manual-dispatch paths leave it a draft.

```yaml
  publish:
    needs: [setup, checksums]
    if: ${{ inputs.publish }}
    runs-on: ubuntu-latest
    env:
      GH_TOKEN: ${{ github.token }}
      TAG: ${{ needs.setup.outputs.tag }}
    steps:
      - name: Publish the release
        shell: bash
        run: |
          set -euo pipefail
          gh release edit "$TAG" --repo "$GITHUB_REPOSITORY" --draft=false
```

- [ ] **Step 7: Lint the workflow**

Run: `actionlint .github/workflows/release.yml`
Expected: clean (no output). The pre-existing SC2035 is now fixed.

Run: `zizmor .github/workflows/release.yml 2>&1 | tail -3`
Expected: the summary line shows **fewer** findings than the baseline 34 (the four `persist-credentials: false` additions remove `artipacked` findings). Remaining findings are pre-existing `cache-poisoning` (setup-go cache) and unpinned-first-party-tag, which are out of scope.

- [ ] **Step 8: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: make release workflow reusable via workflow_call"
```

---

## Task 3: On-merge versioner (version.yml)

**Files:**
- Create: `.github/workflows/version.yml`

Depends on Task 1 (`build/ci/clamp_version.sh`) and Task 2 (`release.yml` `workflow_call`).

- [ ] **Step 1: Create the workflow**

Create `.github/workflows/version.yml`:

```yaml
name: Auto Release

on:
  push:
    branches: [main]

permissions:
  contents: write

concurrency:
  group: auto-release
  cancel-in-progress: false

jobs:
  compute:
    runs-on: ubuntu-latest
    outputs:
      new_tag: ${{ steps.out.outputs.new_tag }}
      new_version: ${{ steps.out.outputs.new_version }}
      changelog: ${{ steps.tag.outputs.changelog }}
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4.3.1
        with:
          fetch-depth: 0
          persist-credentials: false
      - id: tag
        uses: mathieudutour/github-tag-action@d28fa2ccfbd16e871a4bdf35e11b3ad1bd56c0c1 # v6.2
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          dry_run: true
          default_bump: false
          release_branches: main
          fetch_all_tags: true
      - id: out
        shell: bash
        env:
          PREV_VERSION: ${{ steps.tag.outputs.previous_version }}
          NEW_VERSION: ${{ steps.tag.outputs.new_version }}
          NEW_TAG: ${{ steps.tag.outputs.new_tag }}
        run: |
          set -euo pipefail
          if [ -z "$NEW_TAG" ]; then
            echo "No releasable commits since the last tag; skipping release."
            {
              echo "new_tag="
              echo "new_version="
            } >> "$GITHUB_OUTPUT"
            exit 0
          fi
          version="$(bash build/ci/clamp_version.sh "$PREV_VERSION" "$NEW_VERSION")"
          {
            echo "new_version=$version"
            echo "new_tag=v$version"
          } >> "$GITHUB_OUTPUT"
          echo "Release $version (previous $PREV_VERSION; action proposed $NEW_VERSION)"

  release:
    needs: compute
    if: needs.compute.outputs.new_tag != ''
    permissions:
      contents: write
    uses: ./.github/workflows/release.yml
    with:
      version: ${{ needs.compute.outputs.new_version }}
      notes: ${{ needs.compute.outputs.changelog }}
      publish: true
    secrets: inherit
```

- [ ] **Step 2: Lint the workflow**

Run: `actionlint .github/workflows/version.yml`
Expected: clean. (actionlint also validates that the `release` job's `with:` inputs match `release.yml`'s `workflow_call` inputs — confirming Task 2 wired them correctly.)

Run: `zizmor .github/workflows/version.yml 2>&1 | tail -3`
Expected: zero findings. Note: `secrets: inherit` is intentional — the called `release.yml` needs the optional Apple/Windows signing secrets for future signed releases. If `zizmor` raises `secrets-inherit` (medium), it is an accepted, documented choice; record it in the commit body. All other findings must be zero.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/version.yml
git commit -m "ci: auto-cut releases on merge to main

Computes the next semver from conventional commits (clamped to 0.x) and
calls release.yml in the same run. secrets: inherit is intentional so the
reusable build keeps access to the optional signing secrets."
```

---

## Task 4: PR-title lint (pr-title.yml)

**Files:**
- Create: `.github/workflows/pr-title.yml`

- [ ] **Step 1: Create the workflow**

Create `.github/workflows/pr-title.yml`:

```yaml
name: PR Title

on:
  pull_request:
    types: [opened, edited, synchronize, reopened]

permissions:
  pull-requests: read

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@0723387faaf9b38adef4775cd42cfd5155ed6017 # v5.5.3
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

The action validates the PR title against the default Conventional-Commit type set (`feat`, `fix`, `docs`, `chore`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `revert`). Because the repo squash-merges, the PR title becomes the commit subject on main, so this keeps `version.yml`'s commit analysis reliable.

- [ ] **Step 2: Lint the workflow**

Run: `actionlint .github/workflows/pr-title.yml`
Expected: clean.

Run: `zizmor .github/workflows/pr-title.yml 2>&1 | tail -3`
Expected: zero findings.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/pr-title.yml
git commit -m "ci: lint PR titles as conventional commits"
```

---

## Task 5: Final validation

**Files:** none (validation only; commit only if a formatter rewrites something).

- [ ] **Step 1: Lint every workflow**

Run: `actionlint .github/workflows/`
Expected: clean (no output).

- [ ] **Step 2: zizmor the new workflows**

Run: `zizmor .github/workflows/version.yml .github/workflows/pr-title.yml 2>&1 | tail -3`
Expected: `0 findings` (aside from the documented, accepted `secrets-inherit` on `version.yml` if present).

- [ ] **Step 3: zizmor release.yml — confirm no regression**

Run: `zizmor .github/workflows/release.yml 2>&1 | tail -1`
Expected: total findings **≤ 34** (the baseline). Confirm the `artipacked` findings dropped; remaining ones are pre-existing `cache-poisoning`/unpinned-first-party-tag.

- [ ] **Step 4: Re-run the clamp test and shell linters**

Run: `bash build/ci/clamp_version_test.sh && shellcheck build/ci/*.sh && shfmt -i 2 -d build/ci/*.sh`
Expected: test PASS, no shellcheck output, no shfmt diff.

- [ ] **Step 5: Commit (only if Step 4 rewrote files)**

```bash
git add -A
git commit -m "style: shfmt clamp scripts"
```

### Manual verification (after merge to main — cannot run in CI dry)

These confirm end-to-end behavior once the workflows are on main:
1. Merge a PR titled `feat: …` → a published release `v0.2.0` appears with macOS/Windows/Linux assets and `checksums.txt`, notes generated from commits.
2. Merge a PR titled `docs: …` only → no release is cut (the `release` job is skipped).
3. Merge a PR titled `feat!: …` (breaking) → the release stays in 0.x (e.g. `v0.3.0`), not `v1.0.0`.
4. Open a PR titled `add stuff` (non-conventional) → the `PR Title` check fails.

---

## Self-Review Notes

- **Spec coverage:** version computation + dry-run + clamp → Task 1 & 3; `default_bump: false` no-release-on-docs → Task 3; reusable `release.yml` + `workflow_call` inputs → Task 2; auto-publish without race (draft → upload → `publish` job) → Task 2; manual/tag paths stay draft (`if: ${{ inputs.publish }}`) → Task 2; PR-title linting → Task 4; loop safety (no commit/tag push) → inherent in Task 3 design; actionlint/zizmor validation → Tasks 2–5.
- **Spec deviation (intentional):** the spec listed a `tag` `workflow_call` input; the plan derives the tag from `version` in `setup` instead (single source of truth, less surface). The release still produces `v<version>`.
- **Type/name consistency:** `version.yml` outputs `new_version`/`new_tag`/`changelog`; the `release` job passes `version`/`notes`/`publish`, matching `release.yml`'s `workflow_call` inputs exactly. `setup` outputs `version`/`tag` consumed by `create-release`/`publish`.
- **Placeholder scan:** all SHAs resolved to concrete values; all code blocks complete; no TODO/TBD.
