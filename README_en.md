<!-- badges -->
[![License](https://img.shields.io/github/license/watanabe3tipapa/var-watcher.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Maintenance](https://img.shields.io/badge/Maintenance-Active-brightgreen.svg)](https://github.com/watanabe3tipapa/var-watcher)
[![Last commit](https://img.shields.io/github/last-commit/watanabe3tipapa/var-watcher/main.svg)](https://github.com/watanabe3tipapa/var-watcher/commits/main)
[![Live Demo](https://img.shields.io/badge/Live%20Demo-Netlify-00C7B7)](https://var-watcher.netlify.app)

[日本語](README.md) | [English](README_en.md)

# var-watcher

A macOS-focused monitoring tool (educational) that observes `/var` in real time. It unifies four monitoring engines (`fswatch`, `watchman`, `entr`, and `log stream`) behind a single TUI and a Web UI.

- TUI: `varwatch --tui` (implemented with tview)
- Web UI: `varwatch --web` (Vue 3 + WebSocket, embedded in the binary)
- Both: `varwatch --tui --web` (shared state and logs)

Homepage / documentation: https://watanabe3tipapa.github.io/var-watcher/

## Overview / Motivation

var-watcher was created to visualize filesystem footprints under macOS `/var` in real time. The project is aimed at observing changes (files created/modified/deleted, logs, caches, temporary files) that can serve as a footprint of external activity. The tool reports what changed on the filesystem; it does not attempt to infer reasoning or decisions of agents that caused the changes.

## Main features

- Unified control of multiple monitoring engines (fswatch / watchman / entr / log stream)
- Real-time logs streamed to both TUI and Web UI (via WebSocket)
- Log filtering in the TUI (keyword search with `/`)
- Event deduplication: identical events collapse into one line within a `dedup_ms` window (LRU)
- Persistent log storage + search: SQLite (pure Go, no CGO), searchable via `/api/logs` (time range / keyword / source)
- Rule-based alerts: fire when a pattern matches N times within a window; macOS notification (optional sound) + Web badge
- Log export: download matching logs as CSV / JSON (`/api/export`; TUI `e` key writes CSV)
- Stats dashboard: hourly / weekly / by-source aggregations and TOP changed paths (`/api/stats`)
- Directory tree heatmap: 24h changes aggregated by path depth with click-to-expand search (`/api/tree`)
- File diff view: snapshots of changed text files with line-level add/del highlighting (`/api/diffs`)
- Watch-target presets: `logs` / `caches` / `temp` switch target + enabled engines in one step (TUI `p` key / web)
- Performance monitoring: event rate, heap, and CPU as mini-charts with a memory warning at 512 MiB (`/api/perf`)
- i18n: Japanese / English — switch the Web UI instantly from the header (choice saved in localStorage; API error messages follow `config.lang`)
- Persistent configuration (stored at `~/.varwatch/config.json`, save with TUI `s` key)
- Versioning: `varwatch --version` (embedded via `git describe`, e.g. v0.2.1)
- macOS notifications via `osascript`
- Plugin support: drop scripts into `plugins/` and they run automatically
- Dependency detection with hints to install missing tools
- Single binary distribution: the Web UI is embedded using Go's embed.FS

## Screenshot

![Screenshot](assets/IMGSS.jpg)

## Requirements

- macOS (the project targets monitoring `/var` on macOS)
- Go toolchain (go.mod indicates Go 1.26)
- Optional monitoring engine tools (examples shown in Installation)

## Installation (verified steps)

The repository provides a Makefile with convenient targets. The README and Makefile include the following example steps:

1. Install optional monitoring engines (example via Homebrew):

   brew install fswatch watchman entr

2. Clone and build the project:

   git clone https://github.com/watanabe3tipapa/var-watcher.git
   cd var-watcher
   make build

3. (Optional) Put the built binary on your PATH:

   sudo mv varwatch /usr/local/bin/

The Makefile also contains targets for development and frontend build:

- make dev: runs `go run ./cmd/varwatch --tui`
- make frontend: builds the frontend (runs npm in `frontend/` and copies built assets to `internal/web/dist`)

## Usage (commands shown in repository)

Examples from the project README and Makefile:

  varwatch --tui                  # Terminal UI
  varwatch --web                  # Web UI at http://localhost:8080
  varwatch --tui --web            # Both simultaneously
  varwatch --config /path.json    # Use a custom config file
  varwatch --version              # Print version (e.g. v0.2.1)

Note: `/var` often requires elevated privileges on macOS. The original README notes running the TUI with `sudo` when monitoring `/var`. Running `--web` alone does not require root privileges.

### TUI shortcuts

| Key | Action |
|-----|--------|
| `Space` / `Enter` | Toggle the selected watcher ON/OFF |
| `/` | Enter log keyword filter |
| `f` / `Esc` | Clear the filter |
| `p` | Switch to the next preset |
| `e` | Export all logs to CSV (`~/.varwatch/export-<timestamp>.csv`) |
| `s` | Save the config to `--config` path (default `~/.varwatch/config.json`) |
| `n` | Test macOS notification |
| `q` / `Ctrl+C` | Quit |

## Configuration (`~/.varwatch/config.json`)

```json
{
  "lang": "ja",
  "target": "/var",
  "enabled": {"fswatch": true, "logstream": false},
  "args": {"fswatch": ["-xr"]},
  "notify": false,
  "max_log_lines": 2000,
  "dedup_ms": 500,
  "dedup_max": 4096,
  "db_path": "~/.varwatch/varwatch.db",
  "retention_days": 30,
  "presets": [
    {"id": "logs", "name": "Log files", "target": "/var/log", "enabled": {"fswatch": true, "logstream": true}},
    {"id": "caches", "name": "Caches", "target": "/var/folders", "enabled": {"fswatch": true, "watchman": true}},
    {"id": "temp", "name": "Temp files", "target": "/tmp", "enabled": {"fswatch": true}}
  ],
  "alerts": [
    {"id": "burst", "name": "Temp file burst", "pattern": "Created", "source": "fswatch", "min_events": 100, "window_sec": 60, "sound": true}
  ]
}
```

- `target` / `enabled` / `args`: monitored path, per-engine ON/OFF, per-engine arguments
- `lang`: UI language (`ja` / `en`, default `ja`); drives the Web UI's initial language and API error messages
- `notify`: enable/disable macOS notifications (`osascript`)
- `max_log_lines`: in-memory log buffer limit (default 2000)
- `dedup_ms` / `dedup_max`: deduplication window (ms, `0` disables) and LRU capacity
- `db_path` / `retention_days`: SQLite path and retention (expired rows pruned on startup)
- `alerts[]`: fire when `pattern` (case-sensitive) / `source` / `level` matches a line and `min_events` occur within `window_sec`. `sound: true` adds sound (notification itself is gated by the top-level `notify`, default off). fswatch event names are `Created` / `Updated` / `Removed` / `Renamed`.
- `presets[]`: presets bundling target + enabled engines; defaults to `logs` / `caches` / `temp` when unset.

## REST API

```bash
# Time-series / keyword search (since/until RFC3339, q substring)
curl 'http://localhost:8080/api/logs?since=2026-09-23T00:00:00+09:00&q=burst&limit=50'

# Alert history
curl 'http://localhost:8080/api/alerts'

# Export logs (CSV/JSON)
curl -o logs.csv 'http://localhost:8080/api/export?format=csv&source=fswatch&q=Created'
curl -o logs.json 'http://localhost:8080/api/export?format=json'

# Stats dashboard / directory tree / file diffs
curl 'http://localhost:8080/api/stats?hours=24'
curl 'http://localhost:8080/api/tree?hours=24&limit=20000'
curl 'http://localhost:8080/api/diffs?limit=50'

# Presets list & apply / performance samples / language
curl 'http://localhost:8080/api/presets'
curl -X POST 'http://localhost:8080/api/presets/logs/apply'
curl 'http://localhost:8080/api/perf'
curl 'http://localhost:8080/api/config'
```

## Documentation and additional references

- Project documentation / site: https://watanabe3tipapa.github.io/var-watcher/
- DEV notes in repository: DEV-MEMO.md
- Live demo (hosted): https://var-watcher.netlify.app

## Project structure (top-level files / directories present)

The repository includes, among others, the following entries (as present in the repository root):

- cmd/         (Go command sources)
- frontend/    (Vue frontend sources)
- internal/    (internal Go packages, includes embedded web assets)
- plugins/     (plugin scripts)
- demo/        (demo site)
- astro/       (static site tooling)
- assets/      (images and static assets)
- Makefile
- DEV-MEMO.md
- LICENSE

## Contributing

Contributions are welcome. The repository README includes these basic steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/name`)
3. Commit your changes
4. Push the branch
5. Open a Pull Request on GitHub

For more context and project-specific notes, consult DEV-MEMO.md and the documentation site.

## Development / Maintenance status

The repository indicates an active maintenance status (see badge in header). Refer to the commit history for recent activity.

## License

This project is licensed under the MIT License — see the LICENSE file in the repository for details.

## Contact

GitHub: https://github.com/watanabe3tipapa/var-watcher
Homepage / Docs: https://watanabe3tipapa.github.io/var-watcher/
