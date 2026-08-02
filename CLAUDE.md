# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI (`CTF-dlers`, module path `ctfd-downloader`) that bulk-downloads challenges from a CTFd instance over its `/api/v1` REST API, writing an organized `challenges/[category]/[name]/` tree with `challenge.yml` + `README.md` + files per challenge. Requires Go 1.25+.

## Commands

Task runner is `just` (see `justfile`):

- `just build`: `go build -o CTF-dlers ./cmd`
- `just check`: `go fmt ./...` + `go vet ./...` (run before committing)
- `just build-all`: cross-compile to `dist/`
- `just download <url> <token>` / `dry-run` / `test`: run against a live CTFd via `go run ./cmd`
- Unit tests: `go test ./...`; single test: `go test ./pkg/client -run TestFileReq_TokenOnlyForSameHost`. (Note: `just test` is a *connection* test against a live server, not the unit suite.)

Verify changes with `go build ./... && go vet ./... && go test -race ./...`.

## Architecture

Flow: `cmd/main.go` (CLI/config/dashboard) → `pkg/client` (HTTP) → `pkg/services` (concurrency + filesystem) → `pkg/models` (wire + domain types).

**`pkg/client/ctfd.go`**: resty-based CTFd client. Auth is attached **per request**, not globally: `apiReq()` always sends the token; `fileReq(url)` sends it only for same-host or relative download URLs, so a challenge's file list can't leak the token to a foreign host. `checkResponseError` parses the server's error body first, then falls back to `checkHTTPStatusCode`.

**`pkg/services/download.go`**: two-level worker pool. `MaxWorkers` goroutines each process one challenge; within a challenge, `FileWorkers` (default 3) download its files. A single `rate.Limiter` gates every API and file request. `DownloadStats` is mutated only through the mutex-guarded helpers. Never write `ds.stats` fields directly. Key invariants:
- File stats are committed **once**, in `commitFileStats`, only after a challenge fully succeeds, because challenge-level retries re-run downloads and counting mid-flight would double-count.
- `DownloadAllChallenges` returns `ctx.Err()` when cancelled even if the drain loop ended cleanly (workers stopping closes the results channel normally, which would otherwise look like success).
- **Progress hook**: `SetProgressHook` keeps the service UI-agnostic. When set, the service suppresses its own logging (via `logf`) and fires `progress(done, total, result)` per challenge; `done==0` is a one-shot "total known" event. `cmd/main.go` uses this to drive the go-pretty progress bar and collect result rows.

**`pkg/services/filesystem.go`**: writes to a temp file plus atomic rename so a failed re-download never clobbers a prior good file; hashes via `io.MultiWriter` while writing (no re-read). Filenames are run through `utils.SanitizeName` and `..`/`.` are rejected to prevent path traversal.

**`pkg/models`**: `ctfd.go` is the API wire format; `app.go` is the on-disk/domain format. `TagList.UnmarshalJSON` tolerates CTFd returning tags as either `["a","b"]` or `[{"value":"a"}]`. `ChallengeDetailed` embeds `Challenge`.

## Config precedence and gotchas

`cmd/main.go loadConfig`: defaults → config file (`mergeConfigs`) → CLI flags → env (`CTFD_URL`/`CTFD_TOKEN`). Flags override the file **only when actually set**, detected via `flag.Visit` (not by comparing against default values, which would drop a flag set to its default). `retryDelay` must be parsed from the merged `config.RetryDelay` *after* `loadConfig`, or a config-file value is ignored.

## Output UX

The dashboard (banner, live progress bar, colored `Challenges`/`Summary` tables, tarball prompt) lives entirely in `cmd/main.go` using `go-pretty`. The interactive tarball prompt reads stdin in a goroutine and `select`s on `ctx.Done()` so Ctrl-C stays responsive there. Non-interactive/log fallback in the service only runs when no progress hook is registered. The results table's category column comes from `DownloadResult.Category` (set in `processChallenge` from the already-fetched challenge list, so there's no extra API call); add new per-challenge columns the same way rather than re-querying.
