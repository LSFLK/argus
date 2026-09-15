# Release process

Published versions are **immutable**. Do not delete, move, retarget, or reuse a tag, GitHub Release, Helm chart version, or container digest that has already been pushed. Go’s module proxy (`proxy.golang.org`) and checksum database (`sum.golang.org`) keep module versions even if the GitHub tag is later removed.

**`0.1.1` is the Helm chart, not the Go client.** They are not typos for each other:

| Artifact | Current version | Where |
| --- | --- | --- |
| Go client (`pkg/audit`) | `v0.1.0` | [GitHub Release](https://github.com/LSFLK/argus/releases/tag/pkg/audit/v0.1.0) (`go get …@v0.1.0`) |
| Helm chart | `0.1.1` | [GHCR `charts/argus`](https://github.com/LSFLK/argus/pkgs/container/charts%2Fargus) (`helm … --version 0.1.1`) |

Argus has **four independent version lines**. A number used in one line does not free it in another.

| Line | Identity | How it is published |
| --- | --- | --- |
| Root git tags | `github.com/LSFLK/argus` | `vX.Y.Z` git tags. Other services should not import this module. |
| Client module | `github.com/LSFLK/argus/pkg/audit` | Nested tags `pkg/audit/vX.Y.Z` (required for a module in a subdirectory). This is what `go get` consumes. The GitHub Release title may be `v0.1.0`; the git tag is still `pkg/audit/v0.1.0`. |
| Helm chart | `oci://ghcr.io/lsflk/charts/argus` | Chart `version` in `deployments/helm/argus/Chart.yaml`. OCI versions cannot be overwritten. |
| App image | `ghcr.io/lsflk/argus` | `:latest` (mutable) and `:<git sha>` only. There are no semver image tags. |

## Already published — do not reuse

These names are taken. Never create a new release, tag, or chart that recycles them.

### Git tags

| Tag | Points at | Status |
| --- | --- | --- |
| `v0.1.0` | Initial repo (`787b047`) | Root-module tag. Leave as-is. Do not move onto a later commit. |
| `v1.0.0` | JSONB work (`61371c4`) | Root-module tag. Do not move or delete. |
| `v1.0.1` | Client utilities (`2512797`) | Root-module tag. Do not move or delete. |
| `pkg/audit/v1.0.0` | `85d5ea5` | **Retracted** Go client. Cached by the module proxy and checksum DB. Do not move, delete, or `gh release delete --cleanup-tag`. |
| `pkg/audit/v1.0.1` | retract commit | Retract-announcement only (itself retracted). Not a usable 1.x client. |
| `pkg/audit/v0.1.0` | retract commit | **Current Go client.** Pin this. A later client patch would be `pkg/audit/v0.1.1` (unrelated to Helm chart `0.1.1`). |

Root `v0.1.0` and client `pkg/audit/v0.1.0` are different tags. Do not retarget the root tag to “match” the client.

### GitHub Releases

| Release | Tag | Status |
| --- | --- | --- |
| `[RETRACTED] pkg/audit v1.0.0` | `pkg/audit/v1.0.0` | Retracted. Notes point at `v0.1.0`. Do not delete. |
| `v0.1.0` | `pkg/audit/v0.1.0` | Current client release. |

Do not backfill GitHub Releases onto root tags `v0.1.0` / `v1.0.0` / `v1.0.1`.

### Helm

| Chart version | Registry | Status |
| --- | --- | --- |
| `0.1.1` | [`oci://ghcr.io/lsflk/charts/argus`](https://github.com/LSFLK/argus/pkgs/container/charts%2Fargus) | **Current Helm chart.** Published. Do not `helm push` this version again. Next chart is `0.1.2` or higher. |

`Chart.yaml` `appVersion` is metadata only and must not be “fixed” by republishing `0.1.1`.

### Container images

Images are `ghcr.io/lsflk/argus:<git sha>` and `:latest`. Do not invent `:1.0.0` / `:0.1.0` tags for old SHAs.

## Current policy

The **product and client** stay on **0.x** until the team agrees a stable 1.0. Breaking changes on 0.x bump the minor (`0.1.0` → `0.2.0`). Client `v1.0.0` stays published and is **retracted** in `pkg/audit/go.mod` so `go get @latest` selects `v0.1.0`.

`retract` does not unpublish a version. Pinning `@v1.0.0` still works. That is intentional.

## Cutting a client release (`pkg/audit`)

1. Land the change on `main`. Do not retarget an old tag.
2. Tag `pkg/audit/vX.Y.Z` on that commit. Never reuse a tag from the table above.
3. Push the new tag (`git push origin <tag>`). Do not `--force` tags.
4. `gh release create 'pkg/audit/vX.Y.Z' --title 'vX.Y.Z' --notes '...'`

Annotated tags, nested-module name (example of a later client patch — not the Helm chart):

```bash
git tag -a pkg/audit/v0.1.1 -m "pkg/audit v0.1.1"
git push origin pkg/audit/v0.1.1
```

Consumers:

```bash
go get github.com/LSFLK/argus/pkg/audit@v0.1.0
```

This repo uses `replace github.com/LSFLK/argus/pkg/audit => ./pkg/audit` so service builds do not wait on the proxy.

## Cutting a Helm chart release

1. Bump `deployments/helm/argus/Chart.yaml` `version` to a **new** number (never `0.1.1` again).
2. Merge to `main` or dispatch [build-dev-chart.yml](../.github/workflows/build-dev-chart.yml) with that version.
3. A path-only change under `deployments/helm/` on `main` publishes `0.0.0-dev.<run_number>`, not a stable chart. That is expected.

## What CI does (and does not)

No workflow runs on git tags or GitHub Releases today. Tagging `pkg/audit/v0.1.0` does not publish an image or chart by itself.

| Workflow | Trigger | Effect |
| --- | --- | --- |
| `build-image.yml` | Push/PR to `main` touching Go/Dockerfile paths | PR: build only. `main`: push `:sha` and retag `:latest`. |
| `build-dev-chart.yml` | Push to `main` touching `deployments/helm/**`, or manual dispatch | Packages a chart. Empty input → `0.0.0-dev.<run_number>`. Dispatching an **already published** chart version will fail (OCI immutable). |
| `helm-ci.yml` | PRs touching the chart | Lint/template only. |

There is no Go test workflow. Run `go test ./...` locally before tagging.

## Commands that are not allowed

```bash
# Do not — does not unpublish the Go module, and may delete the wrong tag.
gh release delete v1.0.0 --cleanup-tag
gh release delete 'pkg/audit/v1.0.0' --cleanup-tag

git tag -d v0.1.0
git push origin :refs/tags/v0.1.0
git tag -f v0.1.0
git tag -f pkg/audit/v0.1.0
```

To stop the toolchain from *selecting* a bad module version, add `retract` and ship a new tag. To warn humans, edit the GitHub Release notes. Do both; neither replaces the other.
