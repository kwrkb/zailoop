package rs

import (
	"fmt"
	"io"
	"strings"
)

// IDs は 1-2 だけを読み、シートのある予算事業ID の集合を返す。
// 関連事業（1-5）のリンク先にページがあるかを、全事業を流す前に知るために使う。
func (d Dir) IDs() (map[string]bool, error) {
	file, err := d.openCSV("1-2", nil)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	ids := make(map[string]bool)
	for {
		record, err := file.reader.Read()
		if err == io.EOF {
			return ids, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: CSV 読み取り: %w", file.path, err)
		}
		ids[strings.TrimSpace(record[file.columns["予算事業ID"]])] = true
	}
}

// IDs は全年度の Dir.IDs の和集合を返す。
func (m Multi) IDs() (map[string]bool, error) {
	all := make(map[string]bool)
	for _, d := range m.Dirs {
		ids, err := d.IDs()
		if err != nil {
			return nil, err
		}
		for id := range ids {
			all[id] = true
		}
	}
	return all, nil
}
