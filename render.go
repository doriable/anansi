package anansi

import "github.com/jcorbin/anansi/ansi"

// RenderGrid writes a grid's contents into an ansi buffer, relative to current
// cursor state and any prior screen contents. To force an absolute
// (non-differential) update, pass an empty prior grid.
// Returns the number of bytes written into the buffer and the final cursor
// state.
func RenderGrid(buf *ansi.Buffer, cur CursorState, g, prior Grid) (n int, _ CursorState) {
	if g.Stride != g.Rect.Dx() {
		panic("sub-grid update not implemented")
	}
	if g.Rect.Min != ansi.Pt(1, 1) {
		panic("sub-screen update not implemented")
	}

	if len(g.Attr) == 0 || len(g.Rune) == 0 {
		return n, cur
	}
	diffing := true
	if len(prior.Attr) == 0 || len(prior.Rune) == 0 || prior.Rect.Empty() || !prior.Rect.Eq(g.Rect) {
		diffing = false
		n += buf.WriteSeq(ansi.ED.With('2'))
	}

	for i, pt := 0, ansi.Pt(1, 1); i < len(g.Rune); /* next: */ {
		gr, ga := g.Rune[i], g.Attr[i]

		if diffing {
			if j, ok := prior.CellOffset(pt); !ok {
				diffing = false // out-of-bounds disengages diffing
			} else {
				pr, pa := prior.Rune[j], prior.Attr[j] // NOTE range ok since pt <= prior.Size
				if gr == 0 {
					gr, ga = ' ', 0
				}
				if pr == 0 {
					pr, pa = ' ', 0
				}
				if gr == pr && ga == pa {
					goto next // continue
				}
			}
		}

		if gr != 0 {
			mv := cur.To(pt)
			ad := cur.MergeSGR(ga)
			n += buf.WriteSeq(mv)
			n += buf.WriteSGR(ad)
			m, _ := buf.WriteRune(gr)
			n += m
			cur.ProcessRune(gr)
		}

	next:
		i++
		if pt.X++; pt.X >= g.Rect.Max.X {
			pt.X = g.Rect.Min.X
			pt.Y++
		}
	}
	return n, cur
}
