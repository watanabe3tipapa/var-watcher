<!-- badges -->
[![License](https://img.shields.io/github/license/watanabe3tipapa/var-watcher.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Maintenance](https://img.shields.io/badge/Maintenance-Active-brightgreen.svg)](https://github.com/watanabe3tipapa/var-watcher)
[![Last commit](https://img.shields.io/github/last-commit/watanabe3tipapa/var-watcher/main.svg)](https://github.com/watanabe3tipapa/var-watcher/commits/main)
[![Live Demo](https://img.shields.io/badge/Live%20Demo-Netlify-00C7B7)](https://var-watcher.netlify.app)

[English](README.md) | [日本語](README_ja.md)

# var-watcher

A tool to monitor `/var` on macOS in real time, unifying four monitoring engines
(`fswatch` / `watchman` / `entr` / `log stream`) under a single **TUI** and **Web UI**.

- **TUI**: `varwatch --tui` (tview)
- **Web UI**: `varwatch --web` (Vue 3 + WebSocket, embedded in a single binary)
- **Both**: `varwatch --tui --web` (shared state and logs)

## Motivation

I built this tool to **observe and track the behavior of AI agents**.
Coding agents run commands, create/edit/delete files, and install packages in
places you cannot easily see. Since macOS `/var` is frequently rewritten by logs,
caches, and temporary files, changes there are a reliable "footprint" of agent
activity. var-watcher visualizes that footprint in real time so you can confirm
what is happening and record it for later analysis.

> Note: this tool detects *what changed*, not the agent's reasoning. It tracks
> the filesystem footprint, not the decision-making process.

## Features

- **Unified monitoring** — one ON/OFF switch per engine (fswatch / watchman / entr / log stream)
- **Real-time logs** — streaming to TUI and Web UI (WebSocket)
- **Log filtering** — keyword search in the TUI (`/` to filter, `f` to clear)
- **Persistent config** — `~/.varwatch/config.json`
- **macOS notifications** — `osascript` integration
- **Plugins** — drop `plugins/*.sh` and it runs automatically
- **Dependency detection** — shows `brew install` hints for missing commands
- **Single binary** — Web UI is embedded via `embed.FS`

## Screenshot

![Screenshot](assets/IMGSS.jpg)

## Installation

```bash
# Install the monitoring engines
brew install fswatch watchman entr

# Clone and build
git clone https://github.com/watanabe3tipapa/var-watcher.git
cd var-watcher
make build

# (optional) Put it on your PATH
sudo mv varwatch /usr/local/bin/
```

## Usage

```bash
varwatch --tui                  # Terminal UI
varwatch --web                  # Web UI at http://localhost:8080
varwatch --tui --web            # Both simultaneously
varwatch --config /path.json    # Custom config file
```

> `/var` mostly requires root privileges, so run the TUI with `sudo`.
> `--web` alone does not require root.

For detailed usage (TUI keybindings, REST API, config reference, plugins),
see the [Documentation](https://watanabe3tipapa.github.io/var-watcher/) or
[DEV-MEMO.md](DEV-MEMO.md).

Try the [Live Demo](https://var-watcher.netlify.app).

## Contributing

Contributions are welcome!

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a [Pull Request](https://github.com/watanabe3tipapa/var-watcher/pulls)

## License

MIT License — see the [LICENSE](LICENSE) file for details.

## Contact

GitHub: [https://github.com/watanabe3tipapa/var-watcher](https://github.com/watanabe3tipapa/var-watcher)
