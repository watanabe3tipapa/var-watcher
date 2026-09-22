# DEV-MEMO — var-watcher 実装メモ

macOS の `/var` を監視する 4 エンジン(fswatch / watchman / entr / log stream)統合ツール。
TUI(tview) と Web UI(Vue3) を同一 Core Engine で駆動し、単一バイナリで配布する。
公開物: GitHub Pages(教材・LP, Astro) / Netlify(Live Demo, Vue3 モック)。

## なぜ作ったのか(背景・目的)

このツールの目的は **AI Agent の振る舞いを確認・追跡すること**。

- AI コーディングエージェント(Claude Code / opencode 等)はターミナルでコマンド実行・
  ファイル作成/編集/削除・パッケージインストールなどを**目に見えない場所**で行う。
- macOS の `/var` は OS/アプリのログ・キャッシュ・一時ファイルが頻繁に書き換わる領域で、
  Agent の活動中ほぼ必ず変更が発生する → この「足あと」を監視すれば振る舞いを追跡できる。
- 4 つの監視エンジンを組み合わせ、TUI / Web UI でリアルタイム可視化し、
  設定保存・プラグインで証跡を残して後から分析できるようにする。
- 限界: ファイルシステム上の変更しか見えない。Agent の判断内容そのものは対象外。
- README の「なぜ作ったのか」節と内容は同一。更新時は両方を同期すること。

## 技術選定(確定)

| 項目 | 決定 |
|------|------|
| 言語 | Go 1.26 (`go 1.26` in go.mod) |
| TUI | github.com/rivo/tview |
| WebSocket | github.com/gorilla/websocket |
| HTTP | 標準 `net/http`(Go 1.22+ メソッドルーティング) |
| Web UI | Vue 3 + Vite → `dist/` を `embed.FS` で同梱 |
| 監視対象 | `/var` 丸ごと(既定)。config で変更可 |
| 通知 | macOS `osascript -e 'display notification …'` |
| プラグイン | `plugins/*.sh` を自動検出・実行 |

## 実行モード

```
varwatch --tui                # 端末 UI
varwatch --web [--addr :8080] # Web UI サーバ
varwatch --tui --web          # 同時起動(ログ・状態共有)
varwatch --config PATH        # 設定ファイル指定(既定 ~/.varwatch/config.json)
varwatch --plugins DIR        # プラグインディレクトリ(既定 ./plugins)
```

## ディレクトリ構成

```
cmd/varwatch/main.go          # フラグ解析・起動モード分岐
internal/engine/              # Engine: 状態・ログバス・子プロセス管理
  engine.go                   # Engine 本体 / Start / Stop / List / Status
  watcher.go                  # 汎用子プロセス Watcher (stdio 非同期読取)
  bus.go                      # ログバス(1→N ブロードキャスト chan)
  registry.go                 # 組み込み 4 エンジン定義
  detect.go                   # 未インストール・root 権限・Full Disk Access 検知
internal/config/              # ~/.varwatch/config.json 読書
internal/tui/                 # tview UI (tui.go / keys.go)
internal/web/                 # HTTP server (server.go / ws.go / embed.go)
internal/notify/              # osascript 通知 (notify.go)
internal/plugin/              # plugins/*.sh 検出・実行 (plugin.go)
frontend/                     # 本物 Web UI (Vue3+Vite) → dist/ を embed
astro/                        # GitHub Pages 教材・LP
demo/                         # Netlify Live Demo (Vue3 モック + Functions)
plugins/                      # サンプルプラグイン (例: sample.sh)
.github/workflows/build-pages.yml
Makefile / go.mod / README.md / DEV-MEMO.md   # PLAN.md は設計メモのため gitignore で公開除外
```

## Engine API

```go
type State int // stateStopped / stateRunning / stateError
type WatcherInfo struct {
    Name, Command, Args []string, Enabled bool,
    State State, Error string, Installed bool, Pid int
}
func New(cfg *config.Config, logDir ...) *Engine
func (e *Engine) Start(name string) error
func (e *Engine) Stop(name string) error
func (e *Engine) List() []WatcherInfo
func (e *Engine) Logs() <-chan LogLine     // ログバス購読
func (e *Engine) Notify(msg string)        // 通知
```

### ログバス(bus.go)
- `Subscribe() (ch <-chan LogLine, unsub func())` の Pub/Sub。バッファ枯渇時はドロップ(oldest を捨てる)。
- `LogLine { TS time.Time, Source, Level string, Message string }`。TS が空なら bus が現在時刻を付与。
- TUI / WebSocket はそれぞれ別購読者として購読。
- 重複排除: `EnableDedup(window, max)` を有効にすると `Level + Message` をキーにウィンドウ内の同一イベントを除去(dedup.go の LRU バッファ)。`window <= 0` で無効。

### Watcher(watcher.go)
- `exec.CommandContext(ctx, path, args...)`。起動失敗(未インストール等)は `ErrNotInstalled` / `ErrNotPermitted` で区別。
- stdout / stderr を `bufio.Scanner` で非同期読取し、bus へ送信。
- `Watchman` はデーモン型のため `-- trigger` ではなく `watchman subscribe` 相当をログ源として採用する方針(要検証)。実装では各エンジンの出力モデル違いを registry の `Kind` で吸収。
- 停止は `cancel()` + `Wait()`。終了時に `Enabled=false, State=Stopped`。

### registry.go(組み込み定義)
| Name | コマンド | 引数(例) |
|------|----------|----------|
| fswatch | fswatch | `-xr /var`(パス列挙, イベントフラグ付き) |
| watchman | watchman | `subscribe` ベース(未対応時は watch 一覧表示) |
| entr | entr | `sh -c 'printf…'`(stdin に find の結果を流す) |
| logstream | log | `stream --predicate 'eventMessage CONTAINS "/var"' --style syslog` |

- 監視対象パスは config の `Target`(`/var`)から組み立て。
- 依存コマンドが無い場合は `detect.go` が brew インストールコマンドを提示。

## TUI(tview)
- `tview.NewList` で 4 エンジン + プラグインを列挙。`Space` で ON/OFF トグル、`Enter` で詳細。
- `tview.NewTextView` ログビュー(自動スクロール・`DynamicColors(true)`)。
- `/` でフィルタ入力(バッファ内行を絞り込み)。`f` でフィルタ解除。
- フッターにキーヒント。ログは `maxLogLines`(既定 2000)で切り捨て。
- イベント駆動: goroutine で bus を購読 → `app.QueueUpdateDraw`。

## Web(net/http + WebSocket)
```
GET  /api/watchers                  → []WatcherInfo
POST /api/watchers/{name}/start     → 204
POST /api/watchers/{name}/stop      → 204
GET  /api/logs?since&until&source&q&limit → {enabled, logs:[LogLine]}
GET  /ws/logs                         → WebSocket で LogLine(JSON) を配信
GET  /                                → embed.FS の Vue dist/
```
- Vue フロントは `vite build` → `frontend/dist` → `//go:embed dist` で同梱。
- WebSocket は購読者として bus を購読。クライアント切断で unsubscribe。

## 設定(config)
```json
{
  "target": "/var",
  "enabled": {"fswatch": true, "logstream": false},
  "args": {"fswatch": ["-xr"]},
  "notify": false,
  "max_log_lines": 2000,
  "dedup_ms": 500,
  "dedup_max": 4096,
  "db_path": "~/.varwatch/varwatch.db",
  "retention_days": 30,
  "alerts": [
    {
      "id": "burst",
      "name": "一時ファイル急増アラート",
      "pattern": "created",
      "source": "entr",
      "min_events": 100,
      "window_sec": 60,
      "sound": true
    }
  ]
}
```
- 保存先: `~/.varwatch/config.json`(既定)。`--config` で変更。
- `dedup_ms`: 同一イベントの重複排除ウィンドウ(ミリ秒)。`0` で無効化。
- `dedup_max`: 重複排除バッファの最大エントリ数(LRU で追い出し)。
- `db_path`: ログ永続化先(SQLite)。未指定なら `~/.varwatch/varwatch.db`。
- `retention_days`: 保持期間(超過分は起動時に `Prune` で削除)。

## 永続化(store)
- `internal/store` は `modernc.org/sqlite`(pure Go / CGO 不要)を使用。
- スキーマ: `logs(id, ts, source, level, message)`。`ts` は Unix ミリ秒。
- 書き込み: main の `persistLogs` が bus を購読 → dedup 済みの行を INSERT。
- 検索: `GET /api/logs?since=&until=&source=&q=&limit=`。
  - `since`/`until` は RFC3339。`q` は message の部分一致(LIKE)。
  - 結果は新しい順(ts DESC, id DESC)。既定 200 / 上限 5000 件。
- Web UI に「Log Search」パネル(キーワード / 日時範囲 / source / limit)。
- store を開けない場合は `log persistence disabled` と警告し動作継続(非必須)。

## 通知(notify)
- `osascript -e 'display notification "msg" with title "var-watcher"'` を非同期実行。
- config `notify: true` のとき各エンジンの変更検知時に発火。

## プラグイン(plugin)
- `plugins/*.sh` を起動時スキャン。ファイル名 = プラグイン名。
- 各プラグインを子プロセスとして起動し、stdout をログバスへ(`[plugin:name]` プレフィックス)。
- TUI/Web の一覧にも追加され、ON/OFF 可能。

## astro/ (GitHub Pages)
- Astro(最新安定版)+ Tailwind。`astro.config.mjs` の `site` に Pages URL。
- ページ: index / install / usage / faq / changelog。
- Actions で `npm ci && npm run build` → `dist/` → `gh-pages` へ `peaceiris/actions-gh-pages` で deploy。

## demo/ (Netlify)
- Vue3 + Vite。`netlify/functions/log.mjs` がモックログ配信(2 秒ごと fetch)。
- `netlify.toml`: `command = "npm run build"`, `publish = "dist"`, `functions = "netlify/functions"`。
- フッターに GitHub Pages 教材へのリンク。

## Makefile ターゲット

```make
make dev       # go run ./cmd/varwatch --tui
make build     # リリースバイナリ (-ldflags="-s -w")
make test      # go test ./...
make vet       # go vet ./...
make frontend  # cd frontend && npm ci && npm run build (embed 更新)
make astro     # cd astro && npm ci && npm run build
make demo      # cd demo && npm ci && npm run build
make fmt       # gofmt -w .
```

## 検証コマンド

```
go build -o varwatch ./cmd/varwatch
./varwatch --tui
./varwatch --web
curl localhost:8080/api/watchers
```
- engine のユニットテストは偽コマンド(`sh -c 'echo test'`)で stdio 読取・停止を検証。
- `fswatch` 等が未導入でも TUI/Web が起動し、brew 導入ガイドを表示できること。

## 注意点
- `/var` の大半は root 権限が必要 → `sudo varwatch` 前提の警告を表示。Full Disk Access が無いと一部ファイルが読めない旨も表示。
- log stream は高頻度出力 → 既定 OFF 推奨。
- ログはメモリ保持のため `max_log_lines` で切り捨て。
