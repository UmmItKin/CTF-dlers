# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI (`CTF-dlers`, module path `ctfd-downloader`) that bulk-downloads challenges from a CTFd instance over its `/api/v1` REST API, writing an organized `output/<ctf-name>/[category]/[name]/` tree with `challenge.yml` + `README.md` + `view.html` + files per challenge. `-ctf-name` is required to download; auth is a token or a browser session cookie. Requires Go 1.25+. (rCTF and other platforms are planned.)

## Commands

Task runner is `just` (see `justfile`):

- `just build`: `go build -o CTF-dlers ./cmd`
- `just check`: `go fmt ./...` + `go vet ./...` (run before committing)
- `just build-all`: cross-compile to `dist/`
- `just download <url> <token> <name>` / `dry-run` / `test`: run against a live CTFd via `go run ./cmd` (`test` takes no name)
- Unit tests: `go test ./...`; single test: `go test ./pkg/client -run TestFileReq_TokenOnlyForSameHost`. (Note: `just test` is a *connection* test against a live server, not the unit suite.)

Verify changes with `go build ./... && go vet ./... && go test -race ./...`.

## Architecture

Flow: `cmd/main.go` (CLI/config/dashboard) → `pkg/client` (HTTP) → `pkg/services` (concurrency + filesystem) → `pkg/models` (wire + domain types).

**`pkg/client/ctfd.go`**: resty-based CTFd client. Auth is attached **per request** via `auth()`, not globally: a token (`Bearer`) or a session cookie (`Cookie: session=…`, from a logged-in browser). `apiReq()` always authenticates; `fileReq(url)` does so only for same-host or relative URLs, so a challenge's file list can't leak the token/cookie to a foreign host (external S3/Spaces links download with no auth). `checkResponseError` parses the server's error body first, then falls back to `checkHTTPStatusCode`. `FileSize` HEADs a URL for the description-attachment size cap.

**`pkg/services/download.go`**: two-level worker pool. `MaxWorkers` goroutines each process one challenge; within a challenge, `FileWorkers` (default 3) download its files. A single `rate.Limiter` gates every API and file request. `DownloadStats` is mutated only through the mutex-guarded helpers. Never write `ds.stats` fields directly. Key invariants:
- File stats are committed **once**, in `commitFileStats`, only after a challenge fully succeeds, because challenge-level retries re-run downloads and counting mid-flight would double-count.
- `DownloadAllChallenges` returns `ctx.Err()` when cancelled even if the drain loop ended cleanly (workers stopping closes the results channel normally, which would otherwise look like success).
- **Progress hook**: `SetProgressHook` keeps the service UI-agnostic. When set, the service suppresses its own logging (via `logf`) and fires `progress(done, total, result)` per challenge; `done==0` is a one-shot "total known" event. `cmd/main.go` uses this to drive the go-pretty progress bar and collect result rows.

**`pkg/services/filesystem.go`**: writes to a temp file plus atomic rename so a failed re-download never clobbers a prior good file; hashes via `io.MultiWriter` while writing (no re-read). Filenames are run through `utils.SanitizeName` and `..`/`.` are rejected to prevent path traversal. Also saves `view.html` (CTFd's rendered `view` field) so nothing in the challenge modal is lost, and the README notes challenges with a launchable instance (`hasLaunchableInstance`).

Attachments come from two sources: CTFd's `Files` **and** links parsed out of the challenge description by `utils.ExtractAttachmentURLs` (markdown or bare URLs — some CTFs host files on S3/DigitalOcean Spaces). It's a blocklist, not a whitelist: any host downloads except share viewers (`viewerHosts`: Google/Proton Drive, Dropbox, MEGA, …); bare URLs must look like a file. Description links over 250 MB are skipped via a HEAD check.

**`pkg/models`**: `ctfd.go` is the API wire format; `app.go` is the on-disk/domain format. `TagList.UnmarshalJSON` tolerates CTFd returning tags as either `["a","b"]` or `[{"value":"a"}]`. `ChallengeDetailed` embeds `Challenge`.

## Config precedence and gotchas

`cmd/main.go loadConfig`: defaults → config file (`mergeConfigs`) → CLI flags → env (`CTFD_URL`/`CTFD_TOKEN`/`CTFD_COOKIE`). Flags override the file **only when actually set**, detected via `flag.Visit` (not by comparing against default values, which would drop a flag set to its default). `retryDelay` must be parsed from the merged `config.RetryDelay` *after* `loadConfig`, or a config-file value is ignored. `-ctf-name` is required for a download (not for `-test`/`-version`); `main` joins it onto `OutputDir` as `output/<sanitized-ctf-name>/` after the `-test` early-return.

## Output UX

The dashboard (banner, live progress bar, colored `Challenges`/`Summary` tables, tarball prompt) lives entirely in `cmd/main.go` using `go-pretty`. The interactive tarball prompt reads stdin in a goroutine and `select`s on `ctx.Done()` so Ctrl-C stays responsive there. Non-interactive/log fallback in the service only runs when no progress hook is registered. The results table's category column comes from `DownloadResult.Category` (set in `processChallenge` from the already-fetched challenge list, so there's no extra API call); add new per-challenge columns the same way rather than re-querying.

`main` wires cancellation with `signal.NotifyContext`. On `errors.Is(err, context.Canceled)` it prints `Cancelled.` and exits `130` without rendering tables or the tarball prompt; a normal finish renders both and exits `1` only if any challenge failed. Because a cancelled download can leave the drain loop looking clean, the exit path relies on `DownloadAllChallenges` returning `ctx.Err()` (see Architecture) rather than inferring success.
