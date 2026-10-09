# Agent Guidelines (kwd)

Context and instructions for AI coding agents working on **kwd** (Kube Watch
Dog — a Kubernetes cluster health checker; single-pass and daemon shapes). See
[agents.md](https://agents.md/) for the format.

## Project overview

- **What it is:** Go CLI that reads a YAML list of Kubernetes resources
  (`deployment`, `statefulset`, `service`), checks each one's readiness with
  retry/backoff. Two runtime shapes selected by `interval`: `0` = single-pass
  (`exit 0/1`), `> 0` = daemon loop (notifications + optional HTTP surface).
  Optionally pushes Slack notifications and Loki logs. **Read-only** — never
  mutates cluster state.
- **Entrypoint:** `cmd/kwd/main.go`. Core packages: `internal/config`,
  `internal/kube`, `internal/checker`, `internal/sinks`, `internal/cli`.
- **Config contract:** [SPECIFICATIONS.md](SPECIFICATIONS.md) (root). Example
  profile: `configs/kwd.sample.yml`.
- **Kubernetes client:** **`client-go`** (no `kubectl` subprocess). Behavior is
  **configuration-first** — no domain-specific checks hardcoded in Go.

## Scope (product vs operator)

- **This repo (`kwd`):** Go CLI, config, checks, tests, CI,
  **`make release-check`**, binaries, `.deb`/`.rpm`, **`ghcr.io/hrodrig/kwd`**,
  Homebrew — same split as **kzero** / **groot** / **pgwd**.
- **Not here:** cron/systemd scheduling, operator runbooks, reference hook
  scripts, in-cluster manifests → **[hrodrig/kwd-selfhosted]** (operator
  assets). Do not add `run/` or deployment trees to this repository.

## Setup and build

- Install deps: `go mod download`.
- Build: **`make build`** (reads **`VERSION`**, ldflags for
  version/commit/date/branch). Root **`GNUmakefile`** is canonical; on
  **FreeBSD** use **`gmake`** via the BSD **`Makefile`** stub.
- Install to `$GOBIN`: `make install`.
- Cross-compile: `make build-all` (output under `dist/`).

## Test and quality commands

- **Unit tests:** `make test` or `go test ./...`.
- **Coverage:** `make cover`; gate **`make cover-check`** (total statement
  coverage **≥ 80%**).
- **Lint:** `make lint` (`gofmt -s`, `go vet ./...`, gocyclo ≤ 14).
- **Security:** `make security` (govulncheck + gocyclo + grype) and
  `make docker-scan` (grype on the image).
- **Before release:** **`make release-check`** — validates **`VERSION`**
  semver + man **`.TH`**, then lint, test, cover-check, security.

## Build tooling (GNUmakefile)

Copy the `hrodrig/*` target catalog (kzero is the closest mold — Go CLI +
client-go + kubectl plugin). Root `GNUmakefile` is canonical; BSD uses `gmake`
via the `Makefile` stub.

- **Build:** `build`, `build-all` (cross-compile to `dist/`), `install`,
  `install-kubectl-plugin`, `clean`.
- **Test:** `test`, `test-kind` (e2e against a kind cluster). Coverage:
  `cover`, `cover-check` (total statement **≥ 80%**).
- **Quality:** `fmt`, `fmt-check`, `lint` (`gofmt -s`, `go vet`, `gocyclo ≤ 14`),
  `lint-fix`, `gocyclo`, `snapshot` (GoReleaser local snapshot).
- **Security:** `security` (govulncheck + gocyclo + grype), `docker-build`,
  `docker-scan` (grype on the built image).
- **Release:** `release-check` (gate: VERSION semver + man `.TH` + lint + test
  + cover + security), `release: release-check` (GoReleaser),
  `port-freebsd-sync`, `port-openbsd-sync`, `dist-freebsd`, `dist-openbsd`.

## Git flow

- Work on **topic branches** opened from `develop`; merge via **PR into
  `develop`**. **Never** commit or push directly to `develop` or `main`.
- Release: **PR `develop` → `main`**, then annotated tag **`v<semver>`** on
  **`main`** only. After every merge into `main`, sync **`main` → `develop`**.
- **Never** merge to `main`, create/push a release tag, or run `make release`
  / trigger GoReleaser **without explicit user approval** — even if
  `release-check` is green.
- **Commits:** Show the proposed commit message and wait for user approval
  before `git commit`.
- **Language:** English only for code, comments, commit messages, docs, and
  UI strings.

## Version bump (on `develop`, before merge/tag)

| # | Artifact | Action |
|---|----------|--------|
| 1 | **`VERSION`** | New semver without `v` (e.g. `0.1.0`) |
| 2 | **`README.md`** | Static **Version** badge `version-<semver>`; update shipped tables if present |
| 3 | **`CHANGELOG.md`** | Move `[Unreleased]` into `## [X.Y.Z] - YYYY-MM-DD`; update compare links |
| 4 | **`contrib/man/man1/kwd.1`** | `.TH` line: month/year + `kwd v<VERSION>`; new CLI flags |
| 5 | **BSD ports** | `make port-freebsd-sync` and/or `make port-openbsd-sync` |
| 6 | **Gate** | `make release-check` — run only after user asks |
| 7 | **Ship** | Open PR `develop` → `main`, merge on GitHub, annotated tag, push tag — **only after user explicitly approves**. Then sync `main` → `develop`. |

## Man page sync

Keep **`contrib/man/man1/kwd.1`** and **`contrib/man/man1/kubectl-kwd.1`**
aligned with the CLI. GoReleaser gzips them for packages; **`release-check`**
fails if either **`.TH`** version drifts from **`VERSION`**.

**Source of truth:** `internal/cli/`, `cmd/kwd/main.go`, SPECIFICATIONS.md.

## Repository structure

- `cmd/kwd/` — main package and black-box CLI tests.
- `internal/config/` — YAML load and validation.
- `internal/kube/` — Kubernetes client and readiness checks.
- `internal/checker/` — per-resource retry logic.
- `internal/sinks/` — notification sinks (Slack, webhook, Loki, SMTP) and fan-out.
- `internal/cli/` — Cobra commands and flags.
- `configs/` — sample configuration.
- `contrib/` — man pages, deb, krew manifest, FreeBSD/OpenBSD ports.
- `docs/demo.tape` — vhs CLI demo (renders `demo.gif`).
- `testing/` — optional integration smoke.

## Other instructions

- **README:** Keep badges and version badge in sync with **`VERSION`**.
- **CHANGELOG:** User-facing changes under `[Unreleased]`; finalize on release.
- **Supply chain:** Prefer resolving dependency work inside the clone
  (`go get`, `go mod tidy`, `go test ./...`). Do not disable checksum
  verification or use untrusted proxies unless the user explicitly accepts
  the risk.
