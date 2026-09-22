package rs

import "fmt"

// Each は6種類の CSV を各1回走査し、予算事業IDの数値昇順にシートを渡す。
// 各 CSV は同じ ID の行が連続し、グループが数値昇順に並んでいる必要がある。
// fn がエラーを返すと直ちに終了する。渡したシートは呼び出し後も保持できる。
func (d Dir) Each(fn func(*Sheet) error) error {
	tables := tables()
	readers := make([]*groupReader, len(tables))
	for i, t := range tables {
		file, err := d.openCSV(t.number, t.columns)
		if err != nil {
			return err
		}
		defer file.Close()
		readers[i] = &groupReader{file: file}
	}
	for {
		var id string
		var minNum int64
		for _, r := range readers {
			nextID, ok, err := r.peek()
			if err != nil {
				return err
			}
			if ok && (id == "" || r.lastNum < minNum) {
				id, minNum = nextID, r.lastNum
			}
		}
		if id == "" {
			return nil
		}
		if readers[0].pending == nil || readers[0].pendID != id {
			return fmt.Errorf("予算事業ID %s が 1-2 にありません", id)
		}
		sheet := new(Sheet)
		for i, r := range readers {
			if r.pending == nil || r.pendID != id {
				continue
			}
			b := tables[i].start(id, sheet)
			if err := r.drain(b.row); err != nil {
				return err
			}
			if err := b.finish(); err != nil {
				return err
			}
		}
		if err := fn(sheet); err != nil {
			return err
		}
	}
}
