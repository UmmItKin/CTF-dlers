<div align="center">

# CTF-dlers

Concurrent CLI downloader for **CTFd** challenges. It pulls every challenge over the CTFd API into a tidy `challenges/<category>/<name>/` tree with metadata, a README, and files, behind a live progress dashboard.

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white&style=for-the-badge)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
![Platform](https://img.shields.io/badge/platform-linux%20|%20macos%20|%20windows-lightgrey?style=for-the-badge)

</div>

## Install

```bash
git clone https://github.com/UmmItKin/CTF-dlers
cd CTF-dlers
just install        # or: ./install.sh [dir]   (defaults to /usr/local/bin)
```

Needs Go 1.25+. Build only: `just build` → `./CTF-dlers`.

## Usage

```bash
# Download every challenge (API token)
./CTF-dlers -url https://ctf.example.com -token ctfd_abc123

# No token? Use your browser session cookie instead
./CTF-dlers -url https://ctf.example.com -cookie "<session-cookie>"

# Check auth, or preview without downloading
./CTF-dlers -url https://ctf.example.com -token ctfd_abc123 -test
./CTF-dlers -url https://ctf.example.com -token ctfd_abc123 -dry-run
```

You authenticate one of two ways:

- **Token**: get one from **CTFd → Settings → Access Tokens**. Also read from `CTFD_TOKEN`.
- **Session cookie**: for instances without tokens, copy the `session` cookie from your
  logged-in browser (DevTools → Application → Cookies) and pass it with `-cookie` (or
  `CTFD_COOKIE`). Takes the bare value or a full `session=...` pair.

`-url` can also come from `CTFD_URL` or a config file. After a run you're prompted to
bundle everything into a `.tar.gz` to share with teammates.

Attachments come from CTFd's file list **and** from links in each challenge's description
(some CTFs host files on external storage like S3 or DigitalOcean Spaces, which are plain
public downloads). Any host works; only share-preview pages that can't be fetched directly
(Google Drive, Proton Drive, Dropbox, MEGA, …) are skipped. Description links over 250 MB
are skipped too, so a stray VM image or installer link can't run away.

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-url` | CTFd base URL (required) | n/a |
| `-token`, `-cookie` | Access token, or browser session cookie (one required) | n/a |
| `-output` | Output directory | `./challenges` |
| `-config` | YAML config file (see below) | n/a |
| `-workers` | Concurrent challenge workers | `5` |
| `-rate-limit` | Requests per second | `10` |
| `-retry`, `-retry-delay` | Retry attempts / delay between them | `3`, `1s` |
| `-hints`, `-solves` | Include hints / solves in metadata | `false` |
| `-skip-existing`, `-overwrite` | Skip already-downloaded / overwrite files | `true`, `false` |
| `-verbose`, `-dry-run`, `-test`, `-version` | Logging / preview / auth check / version | `false` |

### Config file

```yaml
base_url: "https://ctf.example.com"
token: "ctfd_abc123"      # or use cookie instead
# cookie: "<session>"
output_dir: "./challenges"
max_workers: 5
rate_limit: 10
retry_count: 3
retry_delay: "1s"
include_hints: false
include_solves: false
```

CLI flags override the config file, which overrides defaults. Run with `-config config.yml`.

## Output

```
challenges/
└── <category>/
    └── <challenge>/
        ├── challenge.yml   # metadata (id, value, tags, files with sha1, hints, solves)
        ├── README.md       # human-readable description
        └── <files>         # challenge attachments
```

## License

MIT. See [LICENSE](LICENSE).
