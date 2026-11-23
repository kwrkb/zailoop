// Package fetch は行政事業レビュー見える化サイトが配布する年度別 CSV（ZIP）を
// 取得・展開する。既存の ZIP は再取得せず、配布元の更新を取り直すのは
// Options.Refresh を指定したときだけにする（年度の 15 ファイルを 1 回ずつ）。
// 取り直しは年度の 15 ファイルをすべて取得・展開できてから差し替え、途中で失敗したら
// 既存のファイルと記録を元に戻す。取得したファイルの SHA-256 と取得日は
// rawDir の fetch-<year>.json に記録する。
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
	// finish は展開済みの r の後始末をして results に足す。
	finish := func(r Result, downloaded bool) error {
		// 旧版が別の名前（ZIP エントリ名や文字化け名）で展開した同じ番号・年度の CSV が
		// 残っていると、読み取り側の「番号_RS_年度_*.csv は 1 件」検査に引っかかる。
		// 展開物は ZIP から再生成できる派生物なので、正規名以外を消す。
		removed, err := removeStaleCSVs(csvDir, r.Name, year, r.CSVPath)
		if err != nil {
			return fmt.Errorf("%s: %w", r.Name, err)
		}
		if opt.Log != nil {
			for _, p := range removed {
				fmt.Fprintf(opt.Log, "removed stale %s\n", filepath.Base(p))
			}
			fmt.Fprintf(opt.Log, "%s %s %s\n", state(r, downloaded), r.ZipSHA[:12], r.Name)
		}
		results = append(results, r)
		return nil
	}
	if opt.Refresh {
		files, err := refresh(ctx, year, rawDir, csvDir, m, opt)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			r := Result{
				Name: f.name, ZipPath: f.zipPath, CSVPath: f.csvPath, ZipSHA: f.zipSHA,
				Changed: f.state == refreshChanged, Previous: f.prev,
			}
			r.CSVChanged = r.Changed && f.csvSHA != f.prevCSV
			if err := finish(r, f.state == refreshNew); err != nil {
				return results, err
			}
		}
		return results, nil
	}
	for i, name := range FileNames(year) {
		r := Result{Name: name, ZipPath: filepath.Join(rawDir, name+".zip")}
		downloaded := false
		if _, serr := os.Stat(r.ZipPath); serr == nil && validateZip(r.ZipPath) == nil {
			r.Skipped = true
		} else {
			if serr == nil {
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
		if downloaded {
			e.FetchedAt = opt.Now().Format(time.RFC3339)
			e.CheckedAt = e.FetchedAt
			e.ZipSHA256 = "" // 下で計算し直す
		}
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
		csvPath, err := Unzip(r.ZipPath, csvDir)
		if err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		r.CSVPath = csvPath
		if e.CSVSHA256, err = fileSHA256(csvPath); err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		if err := finish(r, downloaded); err != nil {
			return results, err
		}
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

// refreshFile は refresh での 1 ファイル分の差し替え。
type refreshFile struct {
	name    string
	state   refreshState
	zipPath string // rawDir/<name>.zip
	part    string // 今回取得した ZIP（rawDir/<name>.zip.part）
	prev    string // refreshChanged のとき、旧 ZIP の退避先（rawDir/old/<name>.<12 桁>.zip）
	csvPath string // csvDir/<name>.csv
	staged  string // part から展開した CSV（作業ディレクトリ/<name>.csv）
	backup  string // 差し替え前の CSV の退避先（作業ディレクトリ/<name>.csv.old）
	oldSum  string // 既存 ZIP の SHA-256（refreshNew なら空）
	zipSHA  string // 今回取得した ZIP の SHA-256
	csvSHA  string // 今回展開した CSV の SHA-256
	prevCSV string // refreshChanged のとき、差し替え前の CSV の SHA-256

	// publish で済ませた手順。undo で戻す。
	zipArchived, zipPlaced, csvBackedUp, csvPlaced bool
}

// refresh は year 年度の 15 ファイルを次の 3 段で取り直す。どの段で失敗しても、
// 既存の ZIP・CSV と記録 m を refresh 前の状態にしてエラーを返す（年度の中で新旧の版が
// 混ざらないようにする。プロセスが強制終了されたときは戻せない）。
//
//  1. 取得: 15 ファイルをすべて rawDir/<name>.zip.part に取得・検証する。
//  2. 準備: 各 .part の CSV を csvDir 下の作業ディレクトリ（.refresh-*）に展開し、
//     ハッシュを求める。CSV の置き場所で容量が足りないときもここで失敗する。
//     ここまでは既存のファイルに触れない。
//  3. 差し替え: 名前の付け替えだけで行う。ZIP は内容（SHA-256）が既存と違うものだけ
//     差し替え、旧 ZIP は rawDir/old/<name>.<SHA-256 の先頭 12 桁>.zip に退避する。
//     CSV は 15 本とも展開し直したものに置き換える。途中で失敗したら、差し替えた分を
//     逆順に戻す。戻せなかったときは、作業ディレクトリを消さずにその場所をエラーに書く。
//
// m はすべて差し替えてから更新する。
func refresh(ctx context.Context, year int, rawDir, csvDir string, m *manifest, opt Options) ([]*refreshFile, error) {
	names := FileNames(year)
	files := make([]*refreshFile, 0, len(names))
	stage := ""
	cleanup := func() {
		for _, f := range files {
			os.Remove(f.part)
		}
		if stage != "" {
			os.RemoveAll(stage)
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
		f := &refreshFile{
			name:    name,
			zipPath: filepath.Join(rawDir, name+".zip"),
			part:    filepath.Join(rawDir, name+".zip.part"),
			csvPath: filepath.Join(csvDir, name+".csv"),
		}
		if err := download(ctx, opt.Client, ZipURL(opt.BaseURL, year, name), f.part); err != nil {
			cleanup()
			return nil, fmt.Errorf("%s: %w（既存のファイルは変更していない）", name, err)
		}
		files = append(files, f)
	}

	var err error
	if stage, err = os.MkdirTemp(csvDir, ".refresh-"); err != nil {
		cleanup()
		return nil, fmt.Errorf("作業ディレクトリを作れない: %w（既存のファイルは変更していない）", err)
	}
	for _, f := range files {
		if err := f.prepare(stage, m); err != nil {
			cleanup()
			return nil, fmt.Errorf("%s: %w（既存のファイルは変更していない）", f.name, err)
		}
	}

	for i, f := range files {
		if err := f.publish(); err != nil {
			var undoErrs []error
			for j := i; j >= 0; j-- {
				if uerr := files[j].undo(); uerr != nil {
					undoErrs = append(undoErrs, fmt.Errorf("%s: %w", files[j].name, uerr))
				}
			}
			if len(undoErrs) > 0 {
				return nil, fmt.Errorf("%s: %w。差し替えた分を元に戻せなかった（%w）。旧版は %s と %s に残っている",
					f.name, err, errors.Join(undoErrs...), stage, filepath.Join(rawDir, "old"))
			}
			cleanup()
			return nil, fmt.Errorf("%s: %w（差し替えた分は元に戻した）", f.name, err)
		}
	}
	cleanup()

	now := opt.Now().Format(time.RFC3339)
	for _, f := range files {
		e := m.entry(f.name)
		if f.state == refreshSame {
			if e.ZipSHA256 != f.oldSum {
				e.FetchedAt = "" // 記録のない ZIP。いつ取得したかは分からない
			}
			e.CheckedAt = now
		} else {
			e.FetchedAt, e.CheckedAt = now, now
		}
		e.ZipSHA256, e.CSVSHA256 = f.zipSHA, f.csvSHA
	}
	return files, nil
}

// prepare は取得した ZIP を既存の ZIP と比べ、CSV を stage に展開してハッシュを求める。
// 既存のファイルには触れない。
func (f *refreshFile) prepare(stage string, m *manifest) error {
	var err error
	if f.zipSHA, err = fileSHA256(f.part); err != nil {
		return err
	}
	f.oldSum, err = fileSHA256(f.zipPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		f.state, f.oldSum = refreshNew, ""
	case err != nil:
		return err
	case f.oldSum == f.zipSHA:
		f.state = refreshSame
	default:
		f.state = refreshChanged
		f.prev = filepath.Join(filepath.Dir(f.zipPath), "old", f.name+"."+f.oldSum[:12]+".zip")
		// 差し替え前の CSV のハッシュ。記録が手元の ZIP と合えば記録を、
		// 合わなければ展開済みの CSV から求める。
		if e := m.find(f.name); e != nil && e.ZipSHA256 == f.oldSum && e.CSVSHA256 != "" {
			f.prevCSV = e.CSVSHA256
		} else {
			f.prevCSV, _ = fileSHA256(f.csvPath)
		}
	}
	f.staged = filepath.Join(stage, f.name+".csv")
	f.backup = f.staged + ".old"
	if err := extractCSV(f.part, f.staged); err != nil {
		return err
	}
	f.csvSHA, err = fileSHA256(f.staged)
	return err
}

// publish は名前の付け替えだけで ZIP と CSV を差し替える。済ませた手順を f に記録する。
func (f *refreshFile) publish() error {
	if f.state == refreshChanged {
		if err := os.MkdirAll(filepath.Dir(f.prev), 0o755); err != nil {
			return err
		}
		if err := os.Rename(f.zipPath, f.prev); err != nil {
			return err
		}
		f.zipArchived = true
	}
	if f.state != refreshSame {
		if err := os.Rename(f.part, f.zipPath); err != nil {
			return err
		}
		f.zipPlaced = true
	}
	if err := os.Rename(f.csvPath, f.backup); err == nil {
		f.csvBackedUp = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(f.staged, f.csvPath); err != nil {
		return err
	}
	f.csvPlaced = true
	return nil
}

// undo は publish で済ませた手順を戻す。
func (f *refreshFile) undo() error {
	var errs []error
	switch {
	case f.csvBackedUp:
		errs = append(errs, os.Rename(f.backup, f.csvPath)) // 置いた CSV に上書きする
	case f.csvPlaced:
		errs = append(errs, os.Remove(f.csvPath)) // 差し替え前は CSV がなかった
	}
	switch {
	case f.zipArchived:
		errs = append(errs, os.Rename(f.prev, f.zipPath)) // 置いた ZIP に上書きする
	case f.zipPlaced:
		errs = append(errs, os.Remove(f.zipPath)) // 差し替え前は ZIP がなかった
	}
	return errors.Join(errs...)
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
	dst := filepath.Join(dstDir, strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))+".csv")
	tmp := dst + ".part"
	if err := extractCSV(zipPath, tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return dst, nil
}

// extractCSV は ZIP 内の唯一の CSV を dst に書き出す。失敗したら dst を消す。
func extractCSV(zipPath, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("%s: %w", zipPath, err)
	}
	defer zr.Close()
	f, err := csvEntry(&zr.Reader)
	if err != nil {
		return fmt.Errorf("%s: %w", zipPath, err)
	}
	if err := extract(f, dst); err != nil {
		os.Remove(dst)
		return fmt.Errorf("%s: %w", zipPath, err)
	}
	return nil
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
