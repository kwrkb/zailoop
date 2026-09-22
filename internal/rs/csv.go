package rs

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// csvRow is valid only during a scan callback; the CSV reader reuses its slice.
type csvRow struct {
	columns map[string]int
	fields  []string
	err     error
}

func (r *csvRow) text(name string) string { return r.fields[r.columns[name]] }

func (r *csvRow) hasAny(names []string) bool {
	for _, name := range names {
		if strings.TrimSpace(r.text(name)) != "" {
			return true
		}
	}
	return false
}

func parseYen(value string) (Yen, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Yen{}, nil
	}
	integer := value
	if whole, fraction, ok := strings.Cut(integer, "."); ok {
		if fraction == "" || strings.Trim(fraction, "0") != "" {
			return Yen{}, fmt.Errorf("整数ではありません: %q", value)
		}
		integer = whole
	}
	integer = strings.ReplaceAll(integer, ",", "")
	n, err := strconv.ParseInt(integer, 10, 64)
	if err != nil {
		return Yen{}, err
	}
	return Yen{Value: n, Valid: true}, nil
}

func (r *csvRow) fail(name string, err error) {
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("列 %q の値 %q: %w", name, r.text(name), err)
	}
}

func (r *csvRow) yen(name string) Yen {
	value, err := parseYen(r.text(name))
	r.fail(name, err)
	return value
}

func (r *csvRow) year(name string) int {
	value, err := strconv.ParseInt(strings.TrimSpace(r.text(name)), 10, strconv.IntSize)
	r.fail(name, err)
	return int(value)
}

func (r *csvRow) ratio(name string) Ratio {
	value := strings.TrimSpace(r.text(name))
	if value == "" {
		return Ratio{}
	}
	n, err := strconv.ParseFloat(value, 64)
	if err == nil && (math.IsNaN(n) || math.IsInf(n, 0)) {
		err = fmt.Errorf("有限の数値ではありません")
	}
	r.fail(name, err)
	return Ratio{Value: n, Valid: err == nil}
}

// scan validates the schema before visiting only the requested project's rows.
// It reads to EOF, including records after the last matching ID.
func (d Dir) scan(number, id string, required []string, visit func(*csvRow) error) error {
	pattern := filepath.Join(d.Path, fmt.Sprintf("%s_RS_%d_*.csv", number, d.Year))
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("CSV の検索 %q: %w", pattern, err)
	}
	if len(paths) != 1 {
		return fmt.Errorf("CSV %q: 対象ファイルは1件必要です（%d件）", pattern, len(paths))
	}
	path := paths[0]
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("CSV を開く: %w", err)
	}
	defer f.Close()

	input := bufio.NewReader(f)
	if prefix, _ := input.Peek(3); string(prefix) == "\xef\xbb\xbf" {
		_, _ = input.Discard(3)
	}
	reader := csv.NewReader(input)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("%s: ヘッダ: %w", path, err)
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		if _, exists := columns[name]; exists {
			return fmt.Errorf("%s: ヘッダ %q が重複しています", path, name)
		}
		columns[name] = i
	}
	for _, name := range append([]string{"予算事業ID"}, required...) {
		if _, exists := columns[name]; !exists {
			return fmt.Errorf("%s: 必須ヘッダ %q がありません", path, name)
		}
	}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: CSV 読み取り: %w", path, err)
		}
		if strings.TrimSpace(record[columns["予算事業ID"]]) != id {
			continue
		}
		row := csvRow{columns: columns, fields: record}
		err = visit(&row)
		if row.err != nil {
			err = row.err
		}
		if err != nil {
			line, _ := reader.FieldPos(0)
			return fmt.Errorf("%s:%d: 予算事業ID %s: %w", path, line, id, err)
		}
	}
}
