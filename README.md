# zailoop

予算事業レビューシートの公開 CSV から、1 つの予算事業の「要求 → 成立 → 執行 → 決算 → 評価 → 翌年度反映」を 1 画面で表示する Go CLI です。

公開中のサイト: https://zailoop.kwrkb.workers.dev

## インストール

```sh
go install github.com/kwrkb/zailoop/cmd/zailoop@latest
```

Go がない場合は [Releases](https://github.com/kwrkb/zailoop/releases) から OS に合ったバイナリを取ってください（`zailoop version` で版を確認できます）。

## 使い方

```sh
go build ./cmd/zailoop
./zailoop fetch --year 2024          # data/ に配布 ZIP を取得・展開（各ファイル 1 回だけ）
./zailoop show 884 --year 2024       # 予算事業ID 884 のライフサイクルを表示
./zailoop build --out site           # 全事業の静的サイトを生成（index.html / list.html / p/<ID>.html）
./zailoop fetch --year 2025
./zailoop show 884 --years 2024,2025 # 2 年度を結合し、推移とループ検証（評価・反映 → 翌年の成立）を表示
./zailoop build --years 2024,2025 --out site
```

`site/index.html` はブラウザで直接開けます（file:// 可）。一覧・「ループの断絶」・成果指標・「ループ検証」（複数年度のとき）の 4 ビューがあり、各事業の詳細ページ（JS なし）に飛べます。詳細ページにはシートの関連事業（親事業・子事業・統合・分割など）へのリンクと、金額がすべて 0 の事業ではシートの特記事項・増減理由の原文を出します。断絶判定の閾値は `build` の引数（`--gap-ratio`, `--min-exec-rate`, `--min-unused`, `--unused-ratio`, `--outcome-low`, `--outcome-high`, 複数年度の `--revision-ratio`）で変えられます。

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
本リポジトリと生成したサイトは非公式で、国や府省庁が作成したものではありません。兆候とループ検証は zailoop 独自の目印で、不正や無駄を判定するものではありません。人が調べるべき事業を絞り込むためのものです。

取得した CSV・ZIP はリポジトリに含めません（`data/` は `.gitignore`）。`testdata/` にはテスト用に数事業分（2024 年度 5 事業、2025 年度 7 事業）だけ抜粋しています。

## ライセンス

コードは [MIT](LICENSE)。`testdata/` の抜粋 CSV は配布元の条件（PDL1.0）に従います。

## ドキュメント

- `VISION.md` 目的・スコープ・設計判断
- `docs/data-survey.md` CSV の構造調査と列の対応表
