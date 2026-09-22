# zailoop

予算事業レビューシートの公開 CSV から、1 つの予算事業の「要求 → 成立 → 執行 → 決算 → 評価 → 翌年度反映」を 1 画面で表示する Go CLI です。

```sh
go build ./cmd/zailoop
./zailoop fetch --year 2024          # data/ に配布 ZIP を取得・展開（各ファイル 1 回だけ）
./zailoop show 884 --year 2024       # 予算事業ID 884 のライフサイクルを表示
./zailoop build --out site           # 全事業の静的サイトを生成（index.html / list.html / p/<ID>.html）
./zailoop fetch --year 2025
./zailoop show 884 --years 2024,2025 # 2 年度を結合し、推移とループ検証（評価・反映 → 翌年の成立）を表示
./zailoop build --years 2024,2025 --out site
```

`site/index.html` はブラウザで直接開けます（file:// 可）。一覧・「ループの断絶」・成果指標・「ループ検証」（複数年度のとき）の 4 ビューがあり、各事業の詳細ページ（JS なし）に飛べます。断絶判定の閾値は `build` の引数（`--gap-ratio`, `--min-exec-rate`, `--min-unused`, `--unused-ratio`, `--outcome-low`, `--outcome-high`）で変えられます。

## フロント（web/）の開発

一覧ページのフロントは TypeScript です。Go のビルドには Node は不要で、成果物 `internal/site/assets/app.js` をコミットしています。変更したときは次で再生成してください。

```sh
cd web && npm install && npm run verify   # 型検査・テスト・バンドル
```

## 公開（Cloudflare Workers）

生成した `site/` を Workers の静的アセットとしてそのまま配信します（Worker スクリプトはありません）。`wrangler.jsonc` が `site/` を指しているので、ビルド後に deploy するだけです。`_redirects` は `/` を `index.html` に 200 でリライトするためのもので、`html_handling: none`（相対リンクの `.html` をリダイレクトさせない）では `/` が自動では解決されません。

```sh
./zailoop build --years 2024,2025 --out site
printf '/ /index.html 200\n' > site/_redirects   # build は既存ファイルを消さないので初回だけ
npx wrangler login                                # 初回だけ
npx wrangler deploy                               # https://zailoop.kwrkb.workers.dev
```

## データ出典

行政事業レビュー見える化サイト（https://rssystem.go.jp/ ）のデータを zailoop（kwrkb）が加工して作成しています。
配布データは[公共データ利用規約 第1.0版（PDL1.0）](https://www.digital.go.jp/resources/open_data/public_data_license_v1.0)に基づいて利用しています。
本リポジトリと生成したサイトは非公式で、国や府省庁が作成したものではありません。兆候とループ検証は zailoop 独自の判定です。

取得した CSV・ZIP はリポジトリに含めません（`data/` は `.gitignore`）。`testdata/` にはテスト用に 4 事業分だけ抜粋しています。

## ドキュメント

- `VISION.md` 目的・スコープ・設計判断
- `PLAN.md` フェーズと現在地
- `docs/data-survey.md` CSV の構造調査と列の対応表
