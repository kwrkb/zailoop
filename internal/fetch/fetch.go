// Package fetch は行政事業レビュー見える化サイトが配布する年度別 CSV（ZIP）を
// 取得・展開する。既存の ZIP は再取得せず、配布元の更新を取り直すのは
// Options.Refresh を指定したときだけにする（年度の 15 ファイルを 1 回ずつ）。
// 取得したファイルの SHA-256 と取得日は rawDir の fetch-<year>.json に記録する。
package fetch

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultBaseURL は配布 URL の接頭辞。{year}/rs/{name}.zip が続く。
const DefaultBaseURL = "https://rssystem.go.jp/files"

// Files は年度ごとに配布される 15 ファイルの名前（拡張子なし）。
// {year} は年度に置換される。順序は公式のダウンロードページに合わせる。
var Files = []string{
	"1-1_RS_{year}_基本情報_組織情報",
	"1-2_RS_{year}_基本情報_事業概要等",
	"1-3_RS_{year}_基本情報_政策・施策、法令等",
	"1-4_RS_{year}_基本情報_補助率等",
	"1-5_RS_{year}_基本情報_関連事業",
	"2-1_RS_{year}_予算・執行_サマリ",
	"2-2_RS_{year}_予算・執行_予算種別・歳出予算項目",
	"3-1_RS_{year}_効果発現経路_目標・実績",
	"3-2_RS_{year}_効果発現経路_目標のつながり",
	"4-1_RS_{year}_点検・評価",
	"5-1_RS_{year}_支出先_支出情報",
	"5-2_RS_{year}_支出先_支出ブロックのつながり",
	"5-3_RS_{year}_支出先_費目・使途",
	"5-4_RS_{year}_支出先_国庫債務負担行為等による契約",
	"6-1_RS_{year}_その他備考",
}

// FileNames は年度を埋めたファイル名一覧を返す。
func FileNames(year int) []string {
	out := make([]string, len(Files))
	for i, f := range Files {
		out[i] = strings.ReplaceAll(f, "{year}", fmt.Sprint(year))
	}
	return out
}

// ZipURL は 1 ファイル分の配布 URL を返す。ファイル名は URL エンコードする。
func ZipURL(base string, year int, name string) string {
	return fmt.Sprintf("%s/%d/rs/%s.zip", strings.TrimRight(base, "/"), year, url.PathEscape(name))
}

// Options は Fetch の設定。
type Options struct {
	BaseURL string           // 空なら DefaultBaseURL
	Client  *http.Client     // 空なら 5 分タイムアウトの既定クライアント
	Delay   time.Duration    // 連続取得時の待ち時間（既定 1 秒）
	Log     io.Writer        // 進捗出力先（nil なら出力しない）
	Refresh bool             // 既存の ZIP も取り直し、内容が変わったものだけ差し替える
	Now     func() time.Time // 取得日の時計（nil なら time.Now）
}

// Result は 1 ファイルの取得結果。
type Result struct {
	Name       string
	ZipPath    string
	CSVPath    string
	Skipped    bool   // 既に ZIP があり取得を省略した
	Changed    bool   // Refresh で取り直した ZIP が既存の ZIP と違った
	CSVChanged bool   // Changed のうち、展開した CSV の中身も変わった（false なら ZIP の詰め直しだけ）
	Previous   string // Changed のとき、退避した旧 ZIP のパス
	ZipSHA     string // ZIP の SHA-256（16 進）
}

// Fetch は year 年度の全ファイルを rawDir に ZIP として保存し、csvDir に展開する。
// 既に ZIP が存在するファイルはダウンロードを省略し、展開だけ行う。
// opt.Refresh のときは既存の ZIP も含めて 15 ファイルを取り直す（refresh を参照）。
// どちらの場合も、ZIP と CSV の SHA-256 と取得日を rawDir の fetch-<year>.json に記録する。
func Fetch(ctx context.Context, year int, rawDir, csvDir string, opt Options) (results []Result, err error) {
	if opt.BaseURL == "" {
		opt.BaseURL = DefaultBaseURL
	}
	if opt.Client == nil {
		opt.Client = &http.Client{Timeout: 5 * time.Minute}
	}
	if opt.Delay == 0 {
		opt.Delay = time.Second
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(csvDir, 0o755); err != nil {
		return nil, err
	}
	m, err := loadManifest(rawDir, year)
	if err != nil {
		return nil, err
	}
	// 途中で失敗しても、それまでに確定したファイルの記録は残す。
	defer func() {
		if serr := m.save(rawDir); serr != nil && err == nil {
			err = serr
		}
	}()
	var fresh map[string]refreshState // Refresh で今回取得した（または内容を確認した）ファイル
	if opt.Refresh {
		if fresh, err = refresh(ctx, year, rawDir, m, opt); err != nil {
			return nil, err
		}
	}
	for i, name := range FileNames(year) {
		r := Result{Name: name, ZipPath: filepath.Join(rawDir, name+".zip")}
		downloaded := fresh[name] == refreshNew
		switch {
		case fresh != nil:
			// refresh で差し替え済み
		default:
			if _, serr := os.Stat(r.ZipPath); serr == nil && validateZip(r.ZipPath) == nil {
				r.Skipped = true
				break
			} else if serr == nil {
				// 既存 ZIP が不正（過去の取得失敗など）。消さずに退避して取り直す。
				bad := r.ZipPath + ".bad"
				if err := os.Rename(r.ZipPath, bad); err != nil {
					return results, fmt.Errorf("%s: 不正な ZIP を退避できない: %w", name, err)
				}
				if opt.Log != nil {
					fmt.Fprintf(opt.Log, "invalid zip moved to %s\n", bad)
				}
			}
			if i > 0 {
				select {
				case <-ctx.Done():
					return results, ctx.Err()
				case <-time.After(opt.Delay):
				}
			}
			if err := download(ctx, opt.Client, ZipURL(opt.BaseURL, year, name), r.ZipPath); err != nil {
				return results, fmt.Errorf("%s: %w", name, err)
			}
			downloaded = true
		}
		e := m.entry(name)
		if downloaded && fresh == nil {
			e.FetchedAt = opt.Now().Format(time.RFC3339)
			e.CheckedAt = e.FetchedAt
			e.ZipSHA256 = "" // 下で計算し直す
		}
		r.Changed = e.previous != ""
		r.Previous = e.previous
		sum, err := fileSHA256(r.ZipPath)
		if err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		if e.ZipSHA256 != sum {
			// 記録がない、または記録と違う ZIP（記録より前に取得した、手で置き換えた）。
			// その ZIP をいつ取得したかは分からないので、取得日は空にする。
			if e.ZipSHA256 != "" || r.Skipped {
				e.FetchedAt, e.CheckedAt = "", ""
			}
			e.ZipSHA256 = sum
		}
		r.ZipSHA = sum
		// 差し替え前の CSV のハッシュ。記録がなければ展開済みの CSV から求める。
		prevCSV := e.csvBefore
		if prevCSV == "" && r.Changed {
			prevCSV, _ = fileSHA256(filepath.Join(csvDir, name+".csv"))
		}
		csvPath, err := Unzip(r.ZipPath, csvDir)
		if err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		r.CSVPath = csvPath
		if e.CSVSHA256, err = fileSHA256(csvPath); err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		r.CSVChanged = r.Changed && e.CSVSHA256 != prevCSV
		// 旧版が別の名前（ZIP エントリ名や文字化け名）で展開した同じ番号・年度の CSV が
		// 残っていると、読み取り側の「番号_RS_年度_*.csv は 1 件」検査に引っかかる。
		// 展開物は ZIP から再生成できる派生物なので、正規名以外を消す。
		removed, err := removeStaleCSVs(csvDir, name, year, csvPath)
		if err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		if opt.Log != nil {
			for _, p := range removed {
				fmt.Fprintf(opt.Log, "removed stale %s\n", filepath.Base(p))
			}
			fmt.Fprintf(opt.Log, "%s %s %s\n", state(r, downloaded), sum[:12], name)
		}
		results = append(results, r)
	}
	return results, nil
}

// state はログに出す状態。fetched（新規取得）/ skipped（取得済みで省略）/
// unchanged（--refresh で同じ内容）/ updated（CSV の中身が変わった）/ repacked（ZIP だけ変わった）。
func state(r Result, downloaded bool) string {
	switch {
	case r.CSVChanged:
		return "updated"
	case r.Changed:
		return "repacked"
	case downloaded:
		return "fetched"
	case r.Skipped:
		return "skipped"
	}
	return "unchanged"
}

// refreshState は refresh での 1 ファイルの結果。
type refreshState int

const (
	refreshSame    refreshState = iota + 1 // 既存の ZIP と同じ内容
	refreshChanged                         // 既存の ZIP と違う内容で差し替えた
	refreshNew                             // 既存の ZIP がなかった
)

// refresh は year 年度の 15 ファイルをすべて <name>.zip.part に取得・検証してから、
// 内容（SHA-256）が既存の ZIP と違うものだけを差し替える。旧 ZIP は rawDir/old/ に
// <name>.<SHA-256 の先頭 12 桁>.zip として退避する。1 本でも取得に失敗したら
// .part をすべて消してエラーを返し、既存の ZIP と記録には触れない
// （年度の中で新旧の版が混ざらないようにする）。
func refresh(ctx context.Context, year int, rawDir string, m *manifest, opt Options) (map[string]refreshState, error) {
	names := FileNames(year)
	parts := make([]string, 0, len(names))
	cleanup := func() {
		for _, p := range parts {
			os.Remove(p)
		}
	}
	for i, name := range names {
		if i > 0 {
			select {
			case <-ctx.Done():
				cleanup()
				return nil, ctx.Err()
			case <-time.After(opt.Delay):
			}
		}
		part := filepath.Join(rawDir, name+".zip.part")
		if err := download(ctx, opt.Client, ZipURL(opt.BaseURL, year, name), part); err != nil {
			cleanup()
			return nil, fmt.Errorf("%s: %w（既存の ZIP は変更していない）", name, err)
		}
		parts = append(parts, part)
	}
	now := opt.Now().Format(time.RFC3339)
	fresh := map[string]refreshState{}
	for i, name := range names {
		zipPath := filepath.Join(rawDir, name+".zip")
		newSum, err := fileSHA256(parts[i])
		if err != nil {
			cleanup()
			return nil, err
		}
		e := m.entry(name)
		e.previous = ""
		e.csvBefore = e.CSVSHA256
		oldSum, err := fileSHA256(zipPath)
		if e.ZipSHA256 != oldSum {
			e.csvBefore = "" // 記録が手元の ZIP と合わない。CSV のハッシュは展開済みの CSV から求める
		}
		switch {
		case err == nil && oldSum == newSum:
			fresh[name] = refreshSame
			if e.ZipSHA256 != oldSum {
				e.FetchedAt = "" // 記録のない ZIP。いつ取得したかは分からない
			}
			e.CheckedAt = now
			os.Remove(parts[i])
		case err == nil:
			oldDir := filepath.Join(rawDir, "old")
			if err := os.MkdirAll(oldDir, 0o755); err != nil {
				cleanup()
				return nil, err
			}
			prev := filepath.Join(oldDir, name+"."+oldSum[:12]+".zip")
			if err := os.Rename(zipPath, prev); err != nil {
				cleanup()
				return nil, err
			}
			if err := os.Rename(parts[i], zipPath); err != nil {
				return nil, fmt.Errorf("%s: 旧 ZIP は %s に退避済み: %w", name, prev, err)
			}
			e.previous = prev
			e.FetchedAt, e.CheckedAt = now, now
			fresh[name] = refreshChanged
		case errors.Is(err, fs.ErrNotExist):
			fresh[name] = refreshNew
			if err := os.Rename(parts[i], zipPath); err != nil {
				cleanup()
				return nil, err
			}
			e.FetchedAt, e.CheckedAt = now, now
		default:
			cleanup()
			return nil, err
		}
		e.ZipSHA256 = newSum
	}
	return fresh, nil
}

// removeStaleCSVs は csvDir にある同じ番号・年度の CSV のうち keep 以外を削除して返す。
// 番号はファイル名の先頭（"2-1" など）、年度は "_RS_<year>_" で照合する。
func removeStaleCSVs(csvDir, name string, year int, keep string) ([]string, error) {
	number, _, ok := strings.Cut(name, "_")
	if !ok {
		return nil, nil
	}
	matches, err := filepath.Glob(filepath.Join(csvDir, fmt.Sprintf("%s_RS_%d_*.csv", number, year)))
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, m := range matches {
		if filepath.Clean(m) == filepath.Clean(keep) {
			continue
		}
		if err := os.Remove(m); err != nil {
			return removed, err
		}
		removed = append(removed, m)
	}
	return removed, nil
}

func download(ctx context.Context, c *http.Client, u, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "zailoop/0.1 (+https://github.com/kwrkb/zailoop)")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := validateZip(tmp); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("GET %s: 取得内容が ZIP ではない: %w", u, err)
	}
	return os.Rename(tmp, dst)
}

// validateZip は ZIP として開け、CSV をちょうど 1 本含み、そのエントリを
// 最後まで読めること（CRC 一致）を確認する。セントラルディレクトリだけの検査では
// 本文が壊れたキャッシュを検出できないため、内容を読み切る。
func validateZip(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	f, err := csvEntry(&zr.Reader)
	if err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("%s: %w", f.Name, err)
	}
	return nil
}

// csvEntry は ZIP 内の唯一の CSV エントリを返す。
func csvEntry(zr *zip.Reader) (*zip.File, error) {
	var found *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("ZIP に CSV が複数含まれる")
		}
		found = f
	}
	if found == nil {
		return nil, fmt.Errorf("ZIP に CSV が含まれない")
	}
	return found, nil
}

// Unzip は ZIP 内の CSV を dstDir に展開し、展開した CSV のパスを返す。
// 一時ファイルに展開して成功したときだけ置き換えるので、失敗しても既存 CSV は残る。
// 展開後の名前は ZIP のベース名 + ".csv" にする。配布 ZIP のエントリ名は年度によって
// Shift_JIS（UTF-8 フラグなし）で入っていることがあり、そのまま使うと文字化けするため。
func Unzip(zipPath, dstDir string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("%s: %w", zipPath, err)
	}
	defer zr.Close()
	f, err := csvEntry(&zr.Reader)
	if err != nil {
		return "", fmt.Errorf("%s: %w", zipPath, err)
	}
	dst := filepath.Join(dstDir, strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))+".csv")
	tmp := dst + ".part"
	if err := extract(f, tmp); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("%s: %w", zipPath, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return dst, nil
}

func extract(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, rc); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
