<!-- badges -->
[![License](https://img.shields.io/github/license/watanabe3tipapa/var-watcher.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Maintenance](https://img.shields.io/badge/Maintenance-Active-brightgreen.svg)](https://github.com/watanabe3tipapa/var-watcher)
[![Last commit](https://img.shields.io/github/last-commit/watanabe3tipapa/var-watcher/main.svg)](https://github.com/watanabe3tipapa/var-watcher/commits/main)
[![Live Demo](https://img.shields.io/badge/Live%20Demo-Netlify-00C7B7)](https://var-watcher.netlify.app)

[日本語](README.md) | [English](README_en.md)

# var-watcher

macOS の `/var` をリアルタイムに監視するツール。`fswatch` / `watchman` / `entr` / `log stream`
の 4 つの監視エンジンを 1 つの **TUI** と **Web UI** から一元管理できます。

- **TUI**: `varwatch --tui`(tview)
- **Web UI**: `varwatch --web`(Vue 3 + WebSocket、単一バイナリに同梱)
- **両方同時**: `varwatch --tui --web`(状態とログは共有)

## 動機

このツールは **AI Agent の振る舞いを確認・追跡するため**に作りました。
コーディングエージェントはコマンド実行やファイル作成・編集・削除を目に見えない場所で行います。
macOS の `/var` はログ・キャッシュ・一時ファイルが頻繁に書き換わる領域で、
そこへの変更は Agent の活動の確実な「足あと」です。var-watcher はその足あとを
リアルタイムに可視化し、後から分析するための証跡として記録できます。

> 補足: このツールが検知するのは「何が変わったか」であり、Agent の判断内容までは見えません。
> あくまでファイルシステム上の足あとを追跡するための道具です。

## 特徴

- **一元監視** — 各エンジン(fswatch / watchman / entr / log stream)を個別に ON/OFF
- **リアルタイムログ** — TUI と Web UI(WebSocket)へ即時配信
- **ログフィルタ** — TUI で `/` を押してキーワード検索、`f` で解除
- **重複排除** — 同一イベントを `dedup_ms`(既定 500ms)の窓内で LRU により 1 行に集約
- **ログ永続化 + 時系列検索** — SQLite(pure Go / CGO 不要)へ保存し、キーワード・日時範囲・source で検索
- **ルールベース アラート** — 「パターン + 時間窓内 N 件」で発火し、macOS 通知(サウンド可)+ Web バッジ
- **ログエクスポート** — 検索条件に一致するログを CSV / JSON でダウンロード
- **設定保存** — `~/.varwatch/config.json`
- **macOS 通知** — `osascript` 連携
- **プラグイン** — `plugins/*.sh` を置くだけで自動実行
- **依存検知** — 不足コマンドがあれば `brew install` のヒントを表示
- **単一バイナリ** — Web UI を `embed.FS` で同梱

## 設定例(`~/.varwatch/config.json`)

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
      "source": "fswatch",
      "min_events": 100,
      "window_sec": 60,
      "sound": true
    }
  ]
}
```

- `dedup_ms` / `dedup_max`: 重複排除ウィンドウ(ミリ秒、`0` で無効)と LRU の最大エントリ数
- `db_path` / `retention_days`: 永続化先と保持期間(超過分は起動時に削除)
- `alerts[]`: 発火条件。`pattern` に一致し、`window_sec` 秒以内に `min_events` 件あれば発火。
  `sound: true` でサウンド付き通知、`notify: true` で通知のみの設定も可。

## REST API

```bash
# 時系列・キーワード検索(since/until は RFC3339、q は部分一致)
curl 'http://localhost:8080/api/logs?since=2026-09-23T00:00:00+09:00&q=burst&limit=50'

# アラート発火履歴
curl 'http://localhost:8080/api/alerts'

# ログをエクスポート(CSV/JSON、検索条件を適用してダウンロード)
curl -o logs.csv 'http://localhost:8080/api/export?format=csv&source=fswatch&q=created'
curl -o logs.json 'http://localhost:8080/api/export?format=json&since=2026-09-23T00:00:00%2B09:00'
```

## スクリーンショット

![スクリーンショット](assets/IMGSS.jpg)

## インストール

```bash
# 監視エンジンをインストール
brew install fswatch watchman entr

# クローンしてビルド
git clone https://github.com/watanabe3tipapa/var-watcher.git
cd var-watcher
make build

# (任意)PATH に設定
sudo mv varwatch /usr/local/bin/
```

## 使い方

```bash
varwatch --tui                  # 端末 UI
varwatch --web                  # Web UI(http://localhost:8080)
varwatch --tui --web            # 両方同時
varwatch --config /path.json    # 設定ファイルを指定
```

> `/var` の大半は root 権限が必要なため、TUI は `sudo` で実行してください。
> `--web` のみの場合は root 不要で起動できます。

詳細な使い方(TUI キーバインド・REST API・設定リファレンス・プラグイン)は
[ドキュメント](https://watanabe3tipapa.github.io/var-watcher/)または
[DEV-MEMO.md](DEV-MEMO.md) を参照してください。

[Live Demo](https://var-watcher.netlify.app) もぜひお試しください。

## コントリビューション

コントリビューションは大歓迎です！

1. リポジトリをフォーク
2. 機能ブランチを作成 (`git checkout -b feature/amazing-feature`)
3. 変更をコミット (`git commit -m 'Add amazing feature'`)
4. ブランチにプッシュ (`git push origin feature/amazing-feature`)
5. [Pull Request](https://github.com/watanabe3tipapa/var-watcher/pulls) を作成

## ライセンス

MITライセンス — 詳細は[LICENSE](LICENSE)ファイルを参照してください。

## 連絡先

GitHub: [https://github.com/watanabe3tipapa/var-watcher](https://github.com/watanabe3tipapa/var-watcher)
Homepage / Docs: https://watanabe3tipapa.github.io/var-watcher/
