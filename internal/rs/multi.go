package rs

import (
	"cmp"
	"fmt"
	"slices"
)

// Multi は複数年度の Dir をまとめ、同じ予算事業ID のシートを年度順に供給する。
type Multi struct{ Dirs []Dir }

// Each は全 Dir を並走させ、予算事業ID の昇順（数値）に、その ID を持つ年度の Sheet を
// 事業年度の昇順で並べた sheets を fn に渡す。ある年度に無い ID はその年度分が欠ける。
// 同じ Year の Dir が複数あるとエラーを返す。Dirs が空なら fn を呼ばない。
// fn がエラーを返すと直ちに終了する。渡した sheets は呼び出し後も保持できる。
func (m Multi) Each(fn func(sheets []*Sheet) error) error {
	dirs := slices.Clone(m.Dirs)
	slices.SortFunc(dirs, func(a, b Dir) int { return cmp.Compare(a.Year, b.Year) })
	for i := 1; i < len(dirs); i++ {
		if dirs[i-1].Year == dirs[i].Year {
			return fmt.Errorf("事業年度 %d の Dir が重複しています", dirs[i].Year)
		}
	}

	iters := make([]*sheetIter, len(dirs))
	heads := make([]*Sheet, len(dirs))
	for i, dir := range dirs {
		it, err := dir.iter()
		if err != nil {
			return err
		}
		defer it.close()
		iters[i] = it
		heads[i], err = it.next()
		if err != nil {
			return err
		}
	}
	for {
		first := -1
		for i, sheet := range heads {
			if sheet != nil && (first == -1 || iters[i].idNum < iters[first].idNum) {
				first = i
			}
		}
		if first == -1 {
			return nil
		}
		minNum := iters[first].idNum
		var sheets []*Sheet
		for i, sheet := range heads {
			if sheet != nil && iters[i].idNum == minNum {
				sheets = append(sheets, sheet)
			}
		}
		if err := fn(sheets); err != nil {
			return err
		}
		for i, sheet := range heads {
			if sheet != nil && iters[i].idNum == minNum {
				var err error
				heads[i], err = iters[i].next()
				if err != nil {
					return err
				}
			}
		}
	}
}
