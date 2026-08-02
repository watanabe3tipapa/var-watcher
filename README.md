# var-watcher

macOS の `/var` ディレクトリを監視するツール。`fswatch` / `watchman` / `entr` / `log stream` の
4 つの監視エンジンを 1 つの **TUI** と **Web UI** から一元管理できます。

- TUI: `varwatch --tui`(tview)
- Web UI: `varwatch --web`(Vue3 + WebSocket、単一バイナリに同梱)
- 同時起動: `varwatch --tui --web`(状態とログは共有)

## なぜ作ったのか

このツールは **AI Agent の振る舞いを確認・追跡するため**に作られました。

### 背景

最近のコーディングエージェント(例: Claude Code、opencode などの AI Agent)は、
あなたの代わりにターミナルでコマンドを実行し、ファイルを作成・編集・削除し、
パッケージをインストールし、テストを走らせます。それらの作業の多くは
**目に見えない場所**で行われます。特に macOS の `/var` 配下は OS やアプリの
ログ・キャッシュ・一時ファイルが頻繁に書き換わる領域で、Agent が何かを
しているときも必ずと言っていいほど変更が発生します。

### このツールでできること

- **「いま何が起きているか」をリアルタイムに可視化**
  4 つの監視エンジン(fswatch / watchman / entr / log stream)で
  `/var` の変更を検知し、TUI や Web UI にログとして即時表示します。
- **どのエンジンが何を見ているかを一元管理**
  各エンジンを個別に ON/OFF でき、設定は `~/.varwatch/config.json` に保存されます。
- **振る舞いを「証跡」として残す**
  `s` キーで設定保存、プラグインで独自の記録スクリプトを追加し、
  Agent の活動を後から分析できるようにします。

### 使いどころの例

- AI Agent に長時間のタスクを任せている間、**画面を見て作業が進んでいるかを確認**
- 「Agent が勝手に予期しないファイルを触っていないか」を監視
- どの時間帯に・どのディレクトリで変更が起きたかの**傾向を記録・分析**
- 学習・教材用途として、Agent の動作をデモで見せる

> **補足**: このツールは「何が変わったか」を検知・記録する監視ツールです。
> Agent の判断内容までは見えません。あくまでファイルシステム上の「足あと」を
> 通して振る舞いを確認・追跡するための道具です。

## クイックスタート

```bash
brew install fswatch watchman entr
make build
sudo ./varwatch --tui        # 端末 UI
./varwatch --web             # http://localhost:8080
```

`/var` の大半は root 権限が必要なため、TUI は `sudo` での実行を推奨します。

## キーバインド(TUI)

| キー | 動作 |
|------|------|
| `Space` | 選択中 watcher の ON/OFF |
| `/` | ログのキーワードフィルタ |
| `f` | フィルタ解除 |
| `s` | 設定保存(`~/.varwatch/config.json`) |
| `n` | macOS 通知テスト |
| `q` / `Ctrl-C` | 終了 |

## Web API

```
GET  /api/watchers                  # watcher 一覧
POST /api/watchers/{name}/start     # 起動
POST /api/watchers/{name}/stop      # 停止
GET  /ws/logs                       # WebSocket でログ配信
```

## プラグイン

`plugins/*.sh` を置くと起動時に自動検出され、TUI / Web UI の一覧に追加されます。

## 公開するもの

- 教材・LP: GitHub Pages(Astro) — `astro/`、Actions で自動デプロイ
- Live Demo: Netlify(モックデータ) — `demo/`、`netlify/functions/log.js` がモックログを配信

## 開発

```bash
make dev       # go run ./cmd/varwatch --tui
make build     # リリースバイナリ
make test      # go test ./...
make vet       # go vet ./...
make frontend  # Vue ビルド + embed 更新
make astro     # 教材サイトビルド
make demo      # デモサイトビルド
```

詳細は [DEV-MEMO.md](./DEV-MEMO.md) を参照してください。


MIT
