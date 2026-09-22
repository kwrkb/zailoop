// Package fetch は行政事業レビュー見える化サイトが配布する年度別 CSV（ZIP）を
// 取得・展開する。アクセスは 1 ファイルにつき 1 回に留め、既存ファイルは再取得しない。
package fetch

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
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
	BaseURL string        // 空なら DefaultBaseURL
	Client  *http.Client  // 空なら 5 分タイムアウトの既定クライアント
	Delay   time.Duration // 連続取得時の待ち時間（既定 1 秒）
	Log     io.Writer     // 進捗出力先（nil なら出力しない）
}

// Result は 1 ファイルの取得結果。
type Result struct {
	Name    string
	ZipPath string
	CSVPath string
	Skipped bool // 既に ZIP があり取得を省略した
}

// Fetch は year 年度の全ファイルを rawDir に ZIP として保存し、csvDir に展開する。
// 既に ZIP が存在するファイルはダウンロードを省略し、展開だけ行う。
func Fetch(ctx context.Context, year int, rawDir, csvDir string, opt Options) ([]Result, error) {
	if opt.BaseURL == "" {
		opt.BaseURL = DefaultBaseURL
	}
	if opt.Client == nil {
		opt.Client = &http.Client{Timeout: 5 * time.Minute}
	}
	if opt.Delay == 0 {
		opt.Delay = time.Second
	}
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(csvDir, 0o755); err != nil {
		return nil, err
	}
	var results []Result
	for i, name := range FileNames(year) {
		r := Result{Name: name, ZipPath: filepath.Join(rawDir, name+".zip")}
		if _, err := os.Stat(r.ZipPath); err == nil && validateZip(r.ZipPath) == nil {
			r.Skipped = true
		} else {
			if err == nil {
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
		}
		csvPath, err := Unzip(r.ZipPath, csvDir)
		if err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		r.CSVPath = csvPath
		if opt.Log != nil {
			state := "fetched"
			if r.Skipped {
				state = "skipped"
			}
			fmt.Fprintf(opt.Log, "%s %s\n", state, name)
		}
		results = append(results, r)
	}
	return results, nil
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
// パス走査を防ぐためエントリ名はベース名だけ使う。
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
	dst := filepath.Join(dstDir, filepath.Base(f.Name))
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
