# Agent Guidelines (kwd)

Context and instructions for AI coding agents working on **kwd** (Kube Watch
Dog — a Kubernetes workload readiness watchdog). See
[agents.md](https://agents.md/) for the format.

## Project overview

- **What it is:** Go CLI that reads a YAML list of Kubernetes resources and
  checks each one's **readiness** in a single pass (`interval: 0`, `exit 0/1`).
  `deployment` and `statefulset` are implemented; `daemonset`, `service` and
  `pvc`, the daemon loop (`interval > 0`), the HTTP surface (`/healthz`,
  `/metrics`) and the remaining sinks are planned — see the README roadmap.
  Slack is the only sink today. **Read-only** — never mutates cluster state.
- **Entrypoint:** `cmd/kwd/main.go`. Packages: `internal/config` (YAML load and
  validation), `internal/cluster` (client-go REST config + clientset),
  `internal/check` (pure per-kind readiness checkers), `internal/engine`
  (drives the checker registry), `internal/report` (table + verdict),
  `internal/notify` (sinks and fan-out), `internal/identity` (the `client.id`
  cascade), `internal/exitcode`, `internal/cli` (Cobra tree). `internal/retry`
  exists but is not wired into the check path yet.
- **Config contract:** [SPECIFICATIONS.md](SPECIFICATIONS.md) (root) is the v0
  **target** contract; the README records what the shipped version actually
  does. Example profile: `configs/kwd.sample.yml`.
- **Kubernetes client:** **`client-go`** (no `kubectl` subprocess). Behavior is
  **configuration-first** — no domain-specific checks hardcoded in Go.

## Scope (product vs operator)

- **This repo (`kwd`):** Go CLI, config, checks, tests, CI,
  **`make release-check`**, binaries, `.deb`/`.rpm`, **`ghcr.io/hrodrig/kwd`**,
  cosign signatures and SBOMs — same split as **kzero** / **groot** / **pgwd**.
  A Homebrew cask and the BSD ports tree are not published yet (planned).
- **Not here:** cron/systemd scheduling, operator runbooks, reference hook
  scripts, in-cluster manifests → **[hrodrig/kwd-selfhosted]** (operator
  assets). Do not add `run/` or deployment trees to this repository.

## Setup and build

- Install deps: `go mod download`.
- Build: **`make build`** (reads **`VERSION`**, ldflags for
  version/commit/date/branch). Root **`GNUmakefile`** is canonical; on
  **FreeBSD** use **`gmake`** via the BSD **`Makefile`** stub.
- Install to `$GOBIN`: `make install` (`make install-man` for the man pages,
  `make install-kubectl-plugin` for the `kubectl-kwd` shim).
- Cross-compile: `make build-all` (output under `dist/`).
- Toolchain: pinned in **`go.mod`** (currently `go 1.27.2`); CI follows it via
  `go-version-file`, so bumping the directive moves the runner too.

## Test and quality commands

- **Unit tests:** `make test` (`go test -race ./...`).
- **Coverage:** `make cover`; gate **`make cover-check`** (total statement
  coverage **≥ 80%** over `./internal/...`).
- **Lint:** `make lint` — `check-pins`, `check-man-version`, `gofmt -s`,
  `go vet ./...`, gocyclo ≤ 14 (`make lint-fix` rewrites formatting).
- **Security:** `make security` (govulncheck + gocyclo + grype on the working
  tree) and `make docker-scan` (grype on the built image; requires Docker).
  `make tools` installs govulncheck, gocyclo and grype into `$GOBIN`; `grype`
  and `docker-scan` install grype on demand.
- **CVE pins:** `contrib/scripts/dependency-pins.txt` holds the minimum fixed
  versions; `make check-pins` fails when `go.mod` drops below one. Keep
  `.govulncheck-ignore.yaml` empty unless an advisory is a documented false
  positive — grype does not honour that file, so mirror anything you add in
  `.grype.yaml`.
- **Before release:** **`make release-check`** — `VERSION` semver, then lint
  (which includes the man `.TH` check), test, cover-check, security and
  `docker-scan`.

## Build tooling (GNUmakefile)

The target catalog converges on the `hrodrig/*` family (kzero is the closest
mold — Go CLI + client-go + kubectl plugin). Root `GNUmakefile` is canonical;
BSD uses `gmake` via the `Makefile` stub.

- **Build:** `help`, `build`, `build-all` (cross-compile to `dist/`), `install`,
  `install-man`, `install-kubectl-plugin`, `clean`.
- **Test:** `test`, `cover`, `cover-check` (total statement **≥ 80%**).
- **Quality:** `lint` (`check-pins`, `check-man-version`, `gofmt -s`, `go vet`,
  `gocyclo ≤ 14`), `lint-fix`, `check-pins`, `check-man-version`, `gocyclo`,
  `snapshot` (GoReleaser local snapshot; needs cosign and syft installed).
- **Security:** `tools`, `govulncheck`, `grype`, `security` (govulncheck +
  gocyclo + grype), `check-docker`, `docker-build`, `docker-scan`.
- **Release:** `release-check` (gate: VERSION semver + lint + test +
  cover-check + security + docker-scan), `release: release-check` (GoReleaser,
  `main` only).
- **Not implemented yet:** `fmt`, `fmt-check`, `test-kind`, `port-freebsd-sync`,
  `port-openbsd-sync`, `dist-freebsd`, `dist-openbsd` — add them together with
  the BSD ports tree and the kind e2e suite.

## Git flow

- Work on **topic branches** opened from `develop`; merge via **PR into
  `develop`**. **Never** commit or push directly to `develop` or `main` — the
  `protect-develop` / `protect-main` rulesets reject direct pushes
  (*Changes must be made through a pull request*).
- Both protected branches require the six real check contexts, on an up-to-date
  branch: `test`, `gofmt + go vet + gocyclo`, `Coverage gate (≥ 80%)`,
  `Go vulnerabilities`, `Grype (container image)` and `Analyze` (CodeQL
  **advanced** setup — keep the repository's default setup disabled).
- Release: **PR `develop` → `main`**, then annotated tag **`v<semver>`** on
  **`main`** only. After every merge into `main`, sync **`main` → `develop`** —
  also through a PR, since direct pushes are rejected.
- **Never** merge to `main`, create/push a release tag, or run `make release`
  / trigger GoReleaser **without explicit user approval** — even if
  `release-check` is green.
- **Commits:** show the proposed commit message and wait for user approval
  before `git commit`.
- **Language:** English only for code, comments, commit messages, docs, and
  UI strings.

## Version bump (on `develop`, before merge/tag)

| # | Artifact | Action |
|---|----------|--------|
| 1 | **`VERSION`** | New semver without `v` (e.g. `0.1.0`) |
| 2 | **`README.md`** | Static **Version** badge `version-<semver>`; update shipped tables if present |
| 3 | **`CHANGELOG.md`** | Move `[Unreleased]` into `## [X.Y.Z] - YYYY-MM-DD`; update compare links |
| 4 | **`contrib/man/man1/*.1`** | `.TH` date + `kwd v<VERSION>` on **both** pages; document new flags — `make check-man-version` enforces the version |
| 5 | **BSD ports** | Not applicable yet (no ports tree); add `make port-*-sync` with it |
| 6 | **Gate** | `make release-check` — run only after the user asks |
| 7 | **Ship** | Open PR `develop` → `main`, merge on GitHub, annotated tag, push tag — **only after the user explicitly approves**. Then sync `main` → `develop` (via PR). |

## Man page sync

Keep **`contrib/man/man1/kwd.1`** and **`contrib/man/man1/kubectl-kwd.1`**
aligned with the CLI. GoReleaser gzips them for the packages;
**`make check-man-version`** (run by `lint`, hence by `release-check`) fails
when either **`.TH`** version drifts from **`VERSION`**.

**Source of truth:** `internal/cli/`, `cmd/kwd/main.go`, SPECIFICATIONS.md.

## Repository structure

- `cmd/kwd/` — main package and black-box CLI tests.
- `internal/config/` — YAML load and validation.
- `internal/cluster/` — kubeconfig / in-cluster REST config and clientset.
- `internal/check/` — pure per-kind readiness checkers (`deployment`, `statefulset`).
- `internal/engine/` — runs the registry over the declared refs.
- `internal/report/` — table rendering and the verdict fold.
- `internal/notify/` — Slack sink and fan-out.
- `internal/identity/` — the `client.id` resolution cascade.
- `internal/exitcode/` — error → process exit code mapping.
- `internal/retry/` — bounded backoff helper, not wired into the check path yet.
- `internal/cli/` — Cobra commands and flags.
- `configs/` — sample configuration, embedded by `configs/sample_config.go`.
- `contrib/man/man1/` — man pages; `contrib/scripts/` — the dependency pin guard.
- `docs/` — README hero (`kwd-hero-oss.jpg`); VHS `demo.tape` → `demo.gif` still planned (see `docs/README.md`).
- Not present yet: `contrib/freebsd/`, `contrib/openbsd/`, `contrib/deb/`, the
  krew manifest, and `testing/` (kind e2e smoke).

## Other instructions

- **README:** Keep badges and the version badge in sync with **`VERSION`**.
- **CHANGELOG:** User-facing changes under `[Unreleased]`; finalize on release.
- **Supply chain:** Prefer resolving dependency work inside the clone
  (`go get`, `go mod tidy`, `go test ./...`). Do not disable checksum
  verification or use untrusted proxies unless the user explicitly accepts
  the risk.
