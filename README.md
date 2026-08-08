<div align="center">

# CTF-dlers

Concurrent CLI downloader for CTF challenges. It pulls every challenge into a tidy `output/<ctf-name>/<category>/<name>/` tree with metadata, a README, and files, behind a live progress dashboard.

Supports **CTFd** today, with more platforms (**rCTF** and others) coming soon.

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
# Download every challenge (API token) into output/scriptctf-2026/
./CTF-dlers -url https://ctf.example.com -token ctfd_abc123 -ctf-name scriptctf-2026

# No token? Point -from-browser at the CTF and it grabs your logged-in session cookie
./CTF-dlers -from-browser https://ctf.example.com -ctf-name scriptctf-2026

# ...or paste the cookie yourself
./CTF-dlers -url https://ctf.example.com -cookie "<session-cookie>" -ctf-name scriptctf-2026

# Check auth, or preview without downloading
./CTF-dlers -url https://ctf.example.com -token ctfd_abc123 -test
./CTF-dlers -url https://ctf.example.com -token ctfd_abc123 -ctf-name scriptctf-2026 -dry-run
```

`-ctf-name` is the competition name; challenges land under `output/<ctf-name>/`. It's required
to download (but not for `-test`).

You authenticate one of two ways:

- **Token**: get one from **CTFd → Settings → Access Tokens**. Also read from `CTFD_TOKEN`.
- **Session cookie**: for instances without tokens. Easiest is `-from-browser https://ctf...`,
  which takes the CTF URL and reads its `session` cookie straight from your logged-in
  Firefox/Floorp/LibreWolf profile (no `-url`/`-cookie` needed; Chrome isn't supported, its
  cookies are OS-encrypted). Or pass it yourself with `-cookie` (or `CTFD_COOKIE`), bare value
  or `session=...` pair.

`-url` can also come from `CTFD_URL` or a config file. After a run you're prompted to
bundle everything into a `.tar.gz` to share with teammates.

Attachments come from CTFd's file list **and** from links in each challenge's description
(some CTFs host files on external storage like S3 or DigitalOcean Spaces, which are plain
public downloads). Any host works; only share-preview pages that can't be fetched directly
(Google Drive, Proton Drive, Dropbox, MEGA, etc.) are skipped. Description links over 250 MB
are skipped too, so a stray VM image or installer link can't run away.

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-url` | CTFd base URL (or use `-from-browser`) | n/a |
| `-token`, `-cookie` | Access token or session cookie (one auth method required) | n/a |
| `-from-browser` | CTF URL to target with your browser's session cookie (sets URL + auth in one) | n/a |
| `-ctf-name` | Competition name; output goes under `output/<ctf-name>/` (required to download) | n/a |
| `-output` | Base output directory | `output` |
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
ctf_name: "scriptctf-2026"
output_dir: "output"
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
