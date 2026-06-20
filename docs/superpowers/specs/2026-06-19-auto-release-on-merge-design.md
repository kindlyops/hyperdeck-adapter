# Auto-cut releases on merge to main

## Problem

Releases are cut manually today: a human pushes a `v*` tag (or runs the
`release.yml` `workflow_dispatch`), which builds the macOS DMG, Windows NSIS
installer, and Linux tarball/.deb, then creates a **draft** GitHub release. There
is no automatic versioning — the version is read from the tag name the human
chose.

The goal is to cut releases automatically on merge to main, following the
`posit-dev/vip` technique: read conventional commits since the last tag, compute
the next semantic version, and produce a published release — without manual
version bookkeeping.

## Approach

`vip` decouples version-cutting from publishing via a pushed tag, which requires
a deploy key because tags pushed with the default `GITHUB_TOKEN` do not trigger
other workflows. We avoid that secret by running the build **in the same
workflow run**: a new on-merge workflow computes the version and calls the
existing build workflow directly via `workflow_call`. Nothing is committed back
to main and no tag is pushed to trigger a second workflow, so there is no release
loop and no credential to provision.

Three workflows result:

1. **`version.yml`** (new) — on push to main, compute the next version and, when
   warranted, call `release.yml`.
2. **`release.yml`** (refactored) — gains a `workflow_call` trigger plus a final
   publish step; existing `workflow_dispatch` and `push: tags:[v*]` paths keep
   working.
3. **`pr-title.yml`** (new) — lint PR titles to Conventional Commits. Because the
   repo squash-merges, the PR title becomes the commit subject on main, so this
   is the enforcement point that makes the version computation reliable.

## Behavior

### Version computation (`version.yml`)

- Trigger: `push: branches: [main]`.
- Permissions: `contents: write`.
- Concurrency: `group: auto-release`, `cancel-in-progress: false` — rapid merges
  serialize rather than race on the same tag.
- `compute` job:
  - `actions/checkout` with `fetch-depth: 0` (full history for commit analysis).
  - `mathieudutour/github-tag-action@v6.2` with `dry_run: true`,
    `default_bump: false`, `release_branches: main`, `fetch_all_tags: true`.
    Outputs `previous_version`, `new_version`, `new_tag`, and a generated
    `changelog`.
  - `dry_run: true` computes without creating/pushing a tag (the build creates
    the tag when it makes the release). `default_bump: false` means a merge whose
    commits are only `docs:`/`chore:`/`style:`/etc. yields **no new version**
    (empty `new_tag`).
  - Bump rules (Conventional Commits, via `@semantic-release/commit-analyzer`):
    `fix:` → patch, `feat:` → minor, `BREAKING CHANGE`/`!` → major.
  - **Stay in `0.x` (clamp step):** the action has no `major_on_zero` option and
    natively sends `0.1.0` + a breaking change to `1.0.0`. A bash `clamp` step
    corrects this: when `new_tag` is non-empty, `previous_version` is `0.x`, and
    `new_version` is **not** `0.x`, it replaces the version with a minor bump of
    the previous version (`0.<minor>.0` → `0.<minor+1>.0`, e.g. `0.1.0` →
    `0.2.0`) and recomputes the tag. Otherwise it passes the action's values
    through unchanged. (The alternative — `custom_release_rules` enumerating every
    type as minor/patch — is rejected because it would force `docs:`/`chore:`
    merges to cut patch releases, violating the "no release on docs-only" rule.)
    Computed with shell arithmetic on the dotted version, so no `npx`/node
    dependency. When the maintainer is ready for `1.0.0`, the clamp is removed
    (or bypassed with the action's `custom_tag: 1.0.0`).
  - Job outputs (post-clamp): `new_tag`, `new_version`, `changelog`.
- `release` job: `needs: compute`, `if: needs.compute.outputs.new_tag != ''`,
  `uses: ./.github/workflows/release.yml` with inputs
  `version`, `tag`, `notes: <changelog>`, `publish: true`. Passes
  `secrets: inherit`.

### Build hand-off (`release.yml`)

- Triggers: add `workflow_call` (with the inputs below) to the existing
  `workflow_dispatch` and `push: tags: ["v*"]`.
- `workflow_call` inputs:
  - `version` (string, required)
  - `tag` (string, required)
  - `notes` (string, required) — release body.
  - `publish` (boolean, required) — flip the draft to published when true.
- `setup` job normalizes `version`/`tag` across all three triggers:
  - `workflow_call`/`workflow_dispatch`: read from `inputs`.
  - `push: tags`: derive from `GITHUB_REF_NAME`.
  - Also surfaces `notes` and `publish` as outputs (defaulting to the existing
    generic notes and `publish=false` for the dispatch/tag paths, which keeps
    today's draft-for-review behavior).
- `create-release` job: unchanged in spirit — ensures a **draft** release exists
  for `$TAG`, using `notes` from setup.
- Platform jobs (`macos`, `windows`, `linux`) and `checksums`: unchanged. They
  upload assets to the draft.
- `publish` job (new): `needs: [setup, checksums]`,
  `if: needs.setup.outputs.publish == 'true'`. Runs
  `gh release edit "$TAG" --draft=false`. Because it depends on `checksums`
  (which depends on all platform builds), the release goes public only after
  every asset is attached. If any platform build fails, `publish` is skipped and
  the release is left as a reviewable draft.

### PR-title linting (`pr-title.yml`)

- Trigger: `pull_request: types: [opened, edited, synchronize, reopened]`.
- Permissions: `pull-requests: read`.
- `amannn/action-semantic-pull-request` validates the PR title against
  Conventional Commits (the default type set: `feat`, `fix`, `docs`, `chore`,
  `style`, `refactor`, `perf`, `test`, `build`, `ci`, `revert`). A non-compliant
  title fails the check, blocking merge of a title the versioner cannot classify.

## Data flow

```
merge PR to main (squash; PR title = commit subject)
        |
        v
version.yml: compute (github-tag-action dry_run -> clamp to 0.x)
        |  new_tag == "" ?
        |-- yes --> end (no release; e.g. docs-only merge)
        |-- no  -->  release.yml [workflow_call]: version, tag, notes, publish=true
                          |
                          v
                 setup -> create-release (draft)
                          -> macos / windows / linux (upload assets)
                          -> checksums (upload checksums.txt)
                          -> publish (gh release edit --draft=false)
                          v
                 published release with assets + checksums
```

## Error handling

- **No releasable commits**: `new_tag` is empty, the `release` job is skipped,
  the run ends green. No empty release.
- **Platform build failure**: `checksums`/`publish` do not run; the release
  remains a draft for inspection. No partial release is published.
- **Two rapid merges**: serialized by `version.yml` concurrency; the second run
  computes against the tag the first created.
- **Non-conventional PR title**: blocked at PR time by `pr-title.yml`, so it
  never reaches main to confuse the versioner.

## Loop safety

No commit is pushed to main, and the release/tag is created with `GITHUB_TOKEN`,
whose tag push does not trigger `release.yml`'s `push: tags` path. The on-merge
workflow therefore cannot re-trigger itself or the build a second time. No
`chore(release):` commit-message guard is required.

## Out of scope

- Embedding a version string in the Go binary via ldflags (it reports none today;
  separate concern).
- Committing `CHANGELOG.md` to the repo (would need a push credential and risk a
  loop); notes live in the GitHub Release body.
- Code-signing/notarization (the existing no-op sign steps are unchanged;
  releases remain unsigned with the existing Gatekeeper/SmartScreen warning in
  the notes).

## Validation

- Lint all workflows with `actionlint` and `zizmor`; both must pass clean.
- First merge of a `feat:` or `fix:` PR produces a published release with the
  macOS/Windows/Linux assets and `checksums.txt`, and release notes generated
  from the commits.
- A `docs:`-only merge produces no release.
- A PR opened with a non-conventional title fails the `pr-title.yml` check.
- The clamp holds 0.x: given `previous_version` `0.1.0`, a `feat!:`/`BREAKING
  CHANGE` commit yields `0.2.0` (minor), not `1.0.0`; `feat:` yields `0.2.0`;
  `fix:` yields `0.1.1`. Verify the clamp's shell arithmetic against these
  before relying on it.
