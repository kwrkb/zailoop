package rs

import "fmt"

// Each は7種類の CSV を各1回走査し、予算事業IDの数値昇順にシートを渡す。
// 各 CSV は同じ ID の行が連続し、グループが数値昇順に並んでいる必要がある。
// fn がエラーを返すと直ちに終了する。渡したシートは呼び出し後も保持できる。
func (d Dir) Each(fn func(*Sheet) error) error {
	it, err := d.iter()
	if err != nil {
		return err
	}
	defer it.close()
	for {
		sheet, err := it.next()
		if err != nil || sheet == nil {
			return err
		}
		if err := fn(sheet); err != nil {
			return err
		}
	}
}

type sheetIter struct {
	year    int
	tables  []table
	readers []*groupReader
	idNum   int64 // 最後に返した Sheet の数値 ID。readers は次の ID を先読みし得る。
}

func (d Dir) iter() (*sheetIter, error) {
	it := &sheetIter{year: d.Year, tables: tables()}
	for _, t := range it.tables {
		file, err := d.openCSV(t.number, t.columns)
		if err != nil {
			it.close()
			return nil, err
		}
		it.readers = append(it.readers, &groupReader{file: file})
	}
	return it, nil
}

func (it *sheetIter) close() {
	for _, r := range it.readers {
		r.file.Close()
	}
}

// next は次のシートを組み立てる。EOF は (nil, nil) を返す。
func (it *sheetIter) next() (*Sheet, error) {
	var id string
	var minNum int64
	for _, r := range it.readers {
		nextID, ok, err := r.peek()
		if err != nil {
			return nil, err
		}
		if ok && (id == "" || r.lastNum < minNum) {
			id, minNum = nextID, r.lastNum
		}
	}
	if id == "" {
		return nil, nil
	}
	if it.readers[0].pending == nil || it.readers[0].pendID != id {
		return nil, fmt.Errorf("予算事業ID %s が 1-2 にありません", id)
	}
	sheet := new(Sheet)
	for i, r := range it.readers {
		if r.pending == nil || r.pendID != id {
			continue
		}
		b := it.tables[i].start(id, sheet)
		if err := r.drain(b.row); err != nil {
			return nil, err
		}
		if err := b.finish(); err != nil {
			return nil, err
		}
	}
	if sheet.FiscalYear != it.year {
		return nil, fmt.Errorf("事業年度 %d の CSV を年度 %d として読もうとしています", sheet.FiscalYear, it.year)
	}
	it.idNum = minNum
	return sheet, nil
}
