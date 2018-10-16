package anansi

import (
	"github.com/jcorbin/anansi/ansi"
)

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

// RenderBitmap writes a bitmap's contents as braille runes into an ansi buffer.
// The rawMode parameter causes use of ansi cursor positioning sequences,
// rather than simple newline. Optional style(s) may be passed to control
// graphical rendition of the braille runes.
func RenderBitmap(buf *ansi.Buffer, bi *Bitmap, rawMode bool, styles ...Style) {
	style := Styles(styles...)
	for p := bi.Rect.Min; p.Y < bi.Rect.Max.Y; p.Y += 4 {
		if p.Y > 0 {
			if rawMode {
				buf.WriteESC(ansi.CUD)
				buf.WriteSeq(ansi.CUB.WithInts(p.X))
			} else {
				buf.WriteByte('\n')
			}
		}
		for p.X = bi.Rect.Min.X; p.X < bi.Rect.Max.X; p.X += 2 {
			if r, a := style.Style(ansi.PtFromImage(p), 0, bi.Rune(p), 0, 0); r != 0 {
				if a != 0 {
					buf.WriteSGR(a)
				}
				buf.WriteRune(r)
			} else {
				buf.WriteRune(' ')
			}
		}
	}
}

// Style allows styling of cell data during a drawing or rendering.
// Its eponymous method gets called for each cell as it is about to be
// rendered
//
// The Style method receives the point being rendered on screen or within a
// destination Grid. Its pr and pa arguments contain any prior rune and
// attribute data when drawing in a Grid, while the r and a arguments contain
// the source rune and attribute being drawn.
//
// Whatever rune and attribute values are returned by Style() will be the ones
// drawn/rendered.
type Style interface {
	Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr)
}

// Styles combines zero or more styles into a non-nil Style; if given none, it
// returns a no-op Style; if given many, it returns a Style that calls each in
// turn.
func Styles(ss ...Style) Style {
	var res styles
	for _, s := range ss {
		switch impl := s.(type) {
		case _noopStyle:
			continue
		case styles:
			res = append(res, impl...)
		default:
			res = append(res, s)
		}
	}
	switch len(res) {
	case 0:
		return NoopStyle
	case 1:
		return res[0]
	default:
		return res
	}
}

// StyleFunc is a convenience type alias for implementing Style.
type StyleFunc func(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr)

// Style calls the aliased function pointer
func (f StyleFunc) Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr) {
	return f(p, pr, r, pa, a)
}

type _noopStyle struct{}

func (ns _noopStyle) Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr) {
	return r, 0
}

// NoopStyle is a no-op style, used as a zero fill by Styles.
var NoopStyle Style = _noopStyle{}

type styles []Style

func (ss styles) Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr) {
	for _, s := range ss {
		r, a = s.Style(p, pr, r, pa, a)
	}
	return r, a
}

// ElideStyle implements a style that elides a fixed rune (maps it to 0).
type ElideStyle rune

// Style replaces the passed rune with 0 if it equals the receiver.
func (es ElideStyle) Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr) {
	if r == rune(es) {
		r = 0
	}
	return r, a
}

// FillStyle implements a Style that fills empty runes with a fixed rune value.
type FillStyle rune

// Style replaces the passed rune with the receiver if the passed rune is 0.
func (fs FillStyle) Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr) {
	if r == 0 {
		r = rune(fs)
	}
	return r, a
}

// AttrStyle implements a Style that returns a fixed ansi attr for any non-zero runes.
type AttrStyle ansi.SGRAttr

// Style replaces the passed attr with the receiver if the passed rune is non-0.
func (as AttrStyle) Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr) {
	if r != 0 {
		a = ansi.SGRAttr(as)
	}
	return r, a
}

// MaskedAttrStyle implements a Style that bitwise ors some attributes into a
// masked copy of the prior attributes.
type MaskedAttrStyle struct{ Mask, At ansi.SGRAttr }

// Style applies mas.Mask and sets mas.At in a if r is not 0.
func (mas MaskedAttrStyle) Style(p ansi.Point, pr, r rune, pa, a ansi.SGRAttr) (rune, ansi.SGRAttr) {
	if r != 0 {
		a = a&mas.Mask | mas.At
	}
	return r, a
}

// DrawAttrStyle builds a MaskedAttrStyle that implements DrawFlag attribute
// semantics within a style: pre-existing foreground/background components may
// independently show through, if the corresponding component in at is 0.
func DrawAttrStyle(at ansi.SGRAttr, flags DrawFlag) MaskedAttrStyle {
	const (
		fgMask = ansi.SGRAttrFGMask | ansi.SGRAttrMask
		bgMask = ansi.SGRAttrBGMask
	)
	mas := MaskedAttrStyle{^ansi.SGRAttr(0), at}
	if flags&DrawZeroAttrFGs != 0 || at&fgMask != 0 {
		mas.Mask &= ^fgMask
	}
	if flags&DrawZeroAttrBGs != 0 || at&bgMask != 0 {
		mas.Mask &= ^bgMask
	}
	return mas
}
