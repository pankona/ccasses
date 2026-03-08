# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

Claude Code のセッショントランスクリプション（JSONL）をパース・集計し、可視化するツール。
データソース: `~/.claude/projects/<project-name>/<session-id>.jsonl`

## ビルド・実行

```bash
# ビルド
go build ./cmd/ccasses

# 実行（JSON 生成）
./ccasses generate

# 実行（HTTP サーバー）
./ccasses serve --port 8080

# テスト
go test ./...

# Go fix（モダナイズ）
go fix ./...
```

## アーキテクチャ

```
JSONL → Go CLI(パーサー/集計) → JSON → フロントエンド(Chart.js)
```

- **`cmd/ccasses/main.go`** — エントリポイント。`generate`（JSON 出力）と `serve`（HTTP サーバー）の2サブコマンド
- **`internal/model/`** — データ構造体（`SessionSummary`, `SessionTimeline`, `RawEntry` 等）
- **`internal/parser/`** — JSONL パーサー。`ParseSession` でサマリー＋タイムライン生成、`ParseSubAgents` でサブエージェント情報抽出
- **`internal/server/`** — HTTP サーバー。API エンドポイント (`/api/sessions`, `/api/sessions/{id}/timeline`) + 静的ファイル配信
- **`cmd/ccasses/web/`** — フロントエンド HTML/JS。`embed.FS` でバイナリに埋め込み、バイナリ1つで完結

## 設計ドキュメント

詳細な設計・データ仕様は `design_memo.md` を参照。
