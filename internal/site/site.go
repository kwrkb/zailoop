// Package site は全事業のライフサイクルを静的サイトとして書き出す。
//
//	out/index.html      一覧・横断ビュー（JSON 埋め込み + assets/app.js）
//	out/list.html       JS なしの全事業リンク一覧
//	out/p/<ID>.html     事業詳細（JS なし）
//	out/assets/         style.css, app.js
//
// フロント（assets/app.js）は web/ の TypeScript から生成する。再生成は次で行う（go build からは呼ばれない）:
//
//go:generate npm --prefix ../../web run build
package site

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/rs"
)

// Source は全事業を 1 件ずつ供給する。rs.Dir が満たす。
type Source interface {
	Each(fn func(*rs.Sheet) error) error
}

// MultiSource は同じ事業の複数年度シートをまとめて供給する。rs.Multi が満たす。
type MultiSource interface {
	Each(fn func([]*rs.Sheet) error) error
}

// single は Source を MultiSource に持ち上げる。
type single struct{ Source }

func (s single) Each(fn func([]*rs.Sheet) error) error {
	return s.Source.Each(func(sh *rs.Sheet) error { return fn([]*rs.Sheet{sh}) })
}

// idLister は全事業を流す前にページのある予算事業ID を返す。rs.Dir と rs.Multi が満たす。
// 満たさない Source では関連事業をリンクにせず名前だけ出す。
type idLister interface {
	IDs() (map[string]bool, error)
}

func (s single) IDs() (map[string]bool, error) {
	if l, ok := s.Source.(idLister); ok {
		return l.IDs()
	}
	return nil, nil
}

// Options は Build の設定。
type Options struct {
	Year       int                  // 事業年度（meta 用。0 なら最初の Sheet から取る）
	Thresholds lifecycle.Thresholds // 断絶判定の閾値（ゼロ値は既定）
	TopBlocks  int                  // 詳細ページに出す支出先ブロック数（0 なら 10）
	Log        io.Writer            // 進捗出力先
	Now        func() time.Time     // 生成時刻（nil なら time.Now）
}

// Stats は Build の結果。
type Stats struct {
	Projects int
	Bytes    int64
	Elapsed  time.Duration
}

var idPattern = regexp.MustCompile(`^[0-9]+$`)

// Build は src の全事業を out 配下に書き出す（単年度）。
func Build(src Source, out string, opt Options) (Stats, error) {
	return BuildMulti(single{src}, out, opt)
}

// BuildMulti は複数年度のシートを束ねて書き出す。最新シートを基準にし、推移とループ検証を加える。
func BuildMulti(src MultiSource, out string, opt Options) (Stats, error) {
	start := time.Now()
	if opt.TopBlocks == 0 {
		opt.TopBlocks = 10
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	generated := opt.Now().Format("2006-01-02 15:04 MST")
	for _, d := range []string{filepath.Join(out, "p"), filepath.Join(out, "assets")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Stats{}, err
		}
	}
	var known map[string]bool
	if l, ok := src.(idLister); ok {
		var err error
		if known, err = l.IDs(); err != nil {
			return Stats{}, err
		}
	}
	var st Stats
	var ib *indexBuilder
	sheetYear, actualYear := opt.Year, 0
	err := src.Each(func(sheets []*rs.Sheet) error {
		tl := lifecycle.Track(sheets, opt.Thresholds)
		if tl == nil {
			return nil
		}
		if !idPattern.MatchString(tl.ID) {
			return fmt.Errorf("予算事業ID %q はファイル名に使えません", tl.ID)
		}
		lc := lifecycle.Build(sheets[len(sheets)-1], lifecycle.Options{TopBlocks: opt.TopBlocks})
		for _, sh := range sheets {
			if sh.FiscalYear > lc.SheetYear {
				lc = lifecycle.Build(sh, lifecycle.Options{TopBlocks: opt.TopBlocks})
			}
		}
		tl.Latest = lc
		if ib == nil {
			if sheetYear == 0 {
				sheetYear = lc.SheetYear
			}
			actualYear = sheetYear - 1
			ib = newIndexBuilder(sheetYear)
		}
		sm := lifecycle.SummarizeTimeline(tl, opt.Thresholds)
		n, err := writeFile(filepath.Join(out, "p", tl.ID+".html"), func(w io.Writer) error {
			return writeDetail(w, tl, sm, opt.Thresholds, known, generated)
		})
		if err != nil {
			return err
		}
		st.Bytes += n
		st.Projects++
		ib.add(sm)
		if opt.Log != nil && st.Projects%1000 == 0 {
			fmt.Fprintf(opt.Log, "%d 件\n", st.Projects)
		}
		return nil
	})
	if err != nil {
		return st, err
	}
	if ib == nil {
		return st, fmt.Errorf("事業が 1 件もありません")
	}
	p := ib.payload(sheetYear, actualYear, opt.Thresholds.WithDefaults(), generated)
	for name, fn := range map[string]func(io.Writer) error{
		"index.html": func(w io.Writer) error { return writeIndex(w, p) },
		"list.html":  func(w io.Writer) error { return writeList(w, p) },
		"terms.html": writeTerms,
	} {
		n, err := writeFile(filepath.Join(out, name), fn)
		if err != nil {
			return st, err
		}
		st.Bytes += n
	}
	n, err := copyAssets(filepath.Join(out, "assets"))
	if err != nil {
		return st, err
	}
	st.Bytes += n
	st.Elapsed = time.Since(start)
	return st, nil
}

// writeFile は一時ファイルに書いて成功時だけ置き換える。
func writeFile(path string, fn func(io.Writer) error) (int64, error) {
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	bw := bufio.NewWriterSize(f, 64<<10)
	cw := &countWriter{w: bw}
	if err := fn(cw); err != nil {
		f.Close()
		os.Remove(tmp)
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return 0, err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	return cw.n, nil
}

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func copyAssets(dir string) (int64, error) {
	var total int64
	entries, err := fs.ReadDir(assetFS, "assets")
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		b, err := assetFS.ReadFile("assets/" + e.Name())
		if err != nil {
			return total, err
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644); err != nil {
			return total, err
		}
		total += int64(len(b))
	}
	return total, nil
}
