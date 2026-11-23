package rs

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Headers は d の各 CSV（*_RS_<Year>_*.csv）のヘッダ行を、表番号（"2-1" など）ごとに返す。
// zailoop が読まない表も含む。新しい年度のシートの形式が変わっていないかを確かめるために使う。
func (d Dir) Headers() (map[string][]string, error) {
	pattern := filepath.Join(d.Path, fmt.Sprintf("*_RS_%d_*.csv", d.Year))
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("CSV の検索 %q: %w", pattern, err)
	}
	out := make(map[string][]string, len(paths))
	for _, path := range paths {
		number, _, _ := strings.Cut(filepath.Base(path), "_")
		if _, dup := out[number]; dup {
			return nil, fmt.Errorf("CSV %q: 表 %s のファイルが複数あります", pattern, number)
		}
		header, err := csvHeader(path)
		if err != nil {
			return nil, err
		}
		out[number] = header
	}
	return out, nil
}

func csvHeader(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("CSV を開く: %w", err)
	}
	defer f.Close()
	input := bufio.NewReader(f)
	if prefix, _ := input.Peek(3); string(prefix) == "\xef\xbb\xbf" {
		_, _ = input.Discard(3)
	}
	header, err := csv.NewReader(input).Read()
	if err != nil {
		return nil, fmt.Errorf("%s: ヘッダ: %w", path, err)
	}
	return header, nil
}

// MissingColumns は zailoop が読む表（LoadSheet・Each と同じ 8 表）について、
// 必須の列のうち別名を考えてもヘッダにない列を、表番号ごとに返す。表のファイル自体が
// なければ、その表は ["（ファイルなし）"] とする。すべてそろっていれば空。
func (d Dir) MissingColumns() (map[string][]string, error) {
	headers, err := d.Headers()
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, t := range tables() {
		header, ok := headers[t.number]
		if !ok {
			out[t.number] = []string{"（ファイルなし）"}
			continue
		}
		columns, err := headerColumns(header)
		if err != nil {
			return nil, fmt.Errorf("表 %s: %w", t.number, err)
		}
		if missing := missingColumns(columns, t.columns); len(missing) > 0 {
			out[t.number] = missing
		}
	}
	return out, nil
}

// HeaderDiff は 1 つの表のヘッダの、年度間の差。
type HeaderDiff struct {
	Table    string
	Added    []string // cur にだけある列（cur の並び順）
	Removed  []string // prev にだけある列（prev の並び順）
	OnlyPrev bool     // 表のファイルが prev にしかない
	OnlyCur  bool     // 表のファイルが cur にしかない
}

// CompareHeaders は prev と cur のヘッダを表ごとに比べ、差のある表だけを表番号順に返す。
// 列の並び順だけの違いは差にしない。
func CompareHeaders(prev, cur Dir) ([]HeaderDiff, error) {
	ph, err := prev.Headers()
	if err != nil {
		return nil, err
	}
	ch, err := cur.Headers()
	if err != nil {
		return nil, err
	}
	numbers := make([]string, 0, len(ph)+len(ch))
	for n := range ph {
		numbers = append(numbers, n)
	}
	for n := range ch {
		if _, ok := ph[n]; !ok {
			numbers = append(numbers, n)
		}
	}
	slices.SortFunc(numbers, compareTableNumbers)
	var diffs []HeaderDiff
	for _, n := range numbers {
		p, inPrev := ph[n]
		c, inCur := ch[n]
		d := HeaderDiff{Table: n, OnlyPrev: !inCur, OnlyCur: !inPrev}
		if inPrev && inCur {
			d.Added = without(c, p)
			d.Removed = without(p, c)
		}
		if d.OnlyPrev || d.OnlyCur || len(d.Added) > 0 || len(d.Removed) > 0 {
			diffs = append(diffs, d)
		}
	}
	return diffs, nil
}

// without は a のうち b にない要素を a の順で返す。
func without(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}

// compareTableNumbers は "2-1" < "10-1" となるよう、表番号を数値の組として比べる。
func compareTableNumbers(a, b string) int {
	var a1, a2, b1, b2 int
	fmt.Sscanf(a, "%d-%d", &a1, &a2)
	fmt.Sscanf(b, "%d-%d", &b1, &b2)
	if a1 != b1 {
		return a1 - b1
	}
	if a2 != b2 {
		return a2 - b2
	}
	return strings.Compare(a, b)
}
