# Release process

Published versions are **immutable**. Do not delete, move, retarget, force-push, or reuse a git tag, GitHub Release, Helm chart version, or container digest that has already been pushed. Go’s module proxy (`proxy.golang.org`) and checksum database (`sum.golang.org`) keep module versions even if the GitHub tag is later removed.

Look up what already exists at [github.com/LSFLK/argus/releases](https://github.com/LSFLK/argus/releases) instead of copying numbers out of this file. Git tags (`git tag -l`), the [Helm package on GHCR](https://github.com/LSFLK/argus/pkgs/container/charts%2Fargus), and `ghcr.io/lsflk/argus` (`:latest` / `:<git sha>`) are the other sources of truth.

## Four independent version lines

A number used in one line does not occupy that number in another. Client `v0.1.0`, root tag `v0.1.0`, and Helm `0.1.1` can all exist without matching.

| Line | Identity | How it is published |
| --- | --- | --- |
| Root git tags | `github.com/LSFLK/argus` | `vX.Y.Z`. Other services should not import this module. |
| Client module | `github.com/LSFLK/argus/pkg/audit` | Nested tags `pkg/audit/vX.Y.Z`. This is what `go get` consumes. The GitHub Release title can be `vX.Y.Z`; the git tag is still `pkg/audit/vX.Y.Z`. |
| Helm chart | `oci://ghcr.io/lsflk/charts/argus` | `version` in `deployments/helm/argus/Chart.yaml`. OCI versions cannot be overwritten. |
| App image | `ghcr.io/lsflk/argus` | `:latest` (mutable) and `:<git sha>`. Do not invent semver image tags. |

Install docs should track **latest**, not a snapshot of today’s numbers. See [Releases](https://github.com/LSFLK/argus/releases) for current tags.

```bash
go get github.com/LSFLK/argus/pkg/audit@latest
helm upgrade --install argus oci://ghcr.io/lsflk/charts/argus
```

`@latest` skips versions listed in `retract` in `pkg/audit/go.mod`. Pinning a retracted version (for example `@v1.0.0`) still works; that is intentional.

## Policy

Stay on **0.x** until the team agrees a stable 1.0. On 0.x, breaking changes bump the minor. After a real 1.0, breaking changes bump the major.

To stop the toolchain from *selecting* a bad module version, add `retract` and ship a new tag. To warn humans, edit the GitHub Release notes. Do both; neither replaces the other. Do not `gh release delete --cleanup-tag` a published module version.

## Cutting a client release (`pkg/audit`)

1. Land the change on `main`. Run `go test ./...`.
2. Choose the next SemVer that does **not** already exist as `pkg/audit/vX.Y.Z`.
3. Tag and push (no `--force`):

```bash
git tag -a pkg/audit/vX.Y.Z -m "pkg/audit vX.Y.Z"
git push origin pkg/audit/vX.Y.Z
gh release create "pkg/audit/vX.Y.Z" --title "vX.Y.Z" --notes "..."
```

This repo may `replace github.com/LSFLK/argus/pkg/audit => ./pkg/audit` so service builds do not wait on the proxy. Tags do not trigger image or chart workflows.

## Cutting a Helm chart release

1. Bump `deployments/helm/argus/Chart.yaml` `version` to a number that is **not** already on [GHCR](https://github.com/LSFLK/argus/pkgs/container/charts%2Fargus). Never republish an existing chart version (including to “fix” `appVersion`).
2. Merge to `main`, or dispatch [build-dev-chart.yml](../.github/workflows/build-dev-chart.yml) with that unpublished `version`.
3. A path-only change under `deployments/helm/` on `main` publishes `0.0.0-dev.<run_number>`, not a stable chart. That is expected.

## What CI does

| Workflow | Trigger | Effect |
| --- | --- | --- |
| `build-image.yml` | Push/PR to `main` touching Go/Dockerfile paths | PR: build only. `main`: push `:sha` and retag `:latest`. |
| `build-dev-chart.yml` | Push to `main` touching `deployments/helm/**`, or manual dispatch | Empty input → `0.0.0-dev.<run_number>`. An already-published chart version will fail (OCI immutable). |
| `helm-ci.yml` | PRs touching the chart | Lint/template only. |

There is no Go test workflow. No workflow runs on git tags or GitHub Releases.

## Frozen history (do not rewrite)

These names are already out. Leave them; do not retarget or delete.

- Root tags `v0.1.0`, `v1.0.0`, `v1.0.1` — do not backfill GitHub Releases onto them. Root `v0.1.0` is not the Go client.
- `pkg/audit/v1.0.0` — published client, **retracted**. Cached by the module proxy. GitHub Release is marked retracted.
- `pkg/audit/v1.0.1` — retract-announcement tag only (itself retracted). Not a usable 1.x client.

```bash
# Do not.
gh release delete v1.0.0 --cleanup-tag
gh release delete 'pkg/audit/v1.0.0' --cleanup-tag
git tag -d v0.1.0
git push origin :refs/tags/v0.1.0
git tag -f v0.1.0
git tag -f pkg/audit/v0.1.0
```
