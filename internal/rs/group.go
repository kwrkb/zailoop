package rs

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// ErrUnordered は CSV の予算事業IDのグループが数値の昇順でないことを表す。
var ErrUnordered = errors.New("予算事業IDが昇順に並んでいません")

type groupReader struct {
	file    *csvFile
	pending []string
	pendID  string
	lastNum int64
	eof     bool
}

func (r *groupReader) rowError(id string, err error) error {
	line, _ := r.file.reader.FieldPos(0)
	return fmt.Errorf("%s:%d: 予算事業ID %s: %w", r.file.path, line, id, err)
}

// buffer starts a group. pendID remains set after draining so even the first
// group can have any valid int64 ID, without reserving a numeric sentinel.
func (r *groupReader) buffer(record []string, id string) error {
	num, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return r.rowError(id, err)
	}
	if r.pendID != "" && num <= r.lastNum {
		return r.rowError(id, ErrUnordered)
	}
	r.pending = slices.Clone(record)
	r.pendID, r.lastNum = id, num
	return nil
}

func (r *groupReader) peek() (id string, ok bool, err error) {
	if r.pending != nil {
		return r.pendID, true, nil
	}
	if r.eof {
		return "", false, nil
	}
	record, err := r.file.reader.Read()
	if err == io.EOF {
		r.eof = true
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("%s: CSV 読み取り: %w", r.file.path, err)
	}
	id = strings.TrimSpace(record[r.file.columns["予算事業ID"]])
	if err := r.buffer(record, id); err != nil {
		return "", false, err
	}
	return id, true, nil
}

func (r *groupReader) drain(visit func(*csvRow) error) error {
	id, ok, err := r.peek()
	if err != nil || !ok {
		return err
	}
	record := r.pending
	r.pending = nil
	for {
		row := csvRow{columns: r.file.columns, fields: record}
		err := visit(&row)
		if row.err != nil {
			err = row.err
		}
		if err != nil {
			return r.rowError(id, err)
		}
		record, err = r.file.reader.Read()
		if err == io.EOF {
			r.eof = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: CSV 読み取り: %w", r.file.path, err)
		}
		nextID := strings.TrimSpace(record[r.file.columns["予算事業ID"]])
		if nextID != id {
			return r.buffer(record, nextID)
		}
	}
}
