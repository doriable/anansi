package anansi

import "github.com/jcorbin/anansi/ansi"

// DrawFlag is an optional flag to a drawing routine.
type DrawFlag uint

const (
	// DrawZeroRunes disables rune transparency when drawing; zero runes from
	// the source override any prior rune in the destination.
	DrawZeroRunes DrawFlag = 1 << iota

	// DrawZeroAttrFGs disables foreground transparency when drawing; zero
	// foreground attributes from the source override those in the destination.
	DrawZeroAttrFGs

	// DrawZeroAttrBGs disables background transparency when drawing; zero
	// background attributes from the source override those in the destination.
	DrawZeroAttrBGs
)

// DrawGrid copies the source grid's cells into the destination grid.
//
// The copy is done transparently by default: zero values in the source aren't
// copied, allowing prior destination values to remain. To control this, pass
// any combination of the DrawZeroRunes, DrawZeroAttrFGs, or DrawZeroAttrBGs
// flags.
//
// Use sub-grids to copy to/from specific regions; see Grid.SubRect.
func DrawGrid(dst, src Grid, flags DrawFlag) {
	stride := src.Rect.Dx()
	if dstride := dst.Rect.Dx(); stride > dstride {
		stride = dstride
	}

	if flags&DrawZeroRunes != 0 {
		for sp, dp, di, si := copySetup(dst, src); sp.Y < src.Rect.Max.Y && dp.Y < dst.Rect.Max.Y; {
			copy(dst.Rune[di:di+stride], src.Rune[si:si+stride])
			sp.Y++
			dp.Y++
			si += src.Stride
			di += dst.Stride
		}
	} else {
		for sp, dp, di, si := copySetup(dst, src); sp.Y < src.Rect.Max.Y && dp.Y < dst.Rect.Max.Y; {
			ii, jj := si, di
			sp.X = src.Rect.Min.X
			dp.X = dst.Rect.Min.X
			for sp.X < src.Rect.Max.X && dp.X < dst.Rect.Max.X {
				if r := src.Rune[ii]; r != 0 {
					dst.Rune[jj] = r
				}
				ii++
				jj++
				sp.X++
			}
			si += src.Stride
			di += dst.Stride
			dp.Y++
		}
	}

	dzf := flags&DrawZeroAttrFGs != 0
	dzb := flags&DrawZeroAttrBGs != 0
	if dzf && dzb {
		for sp, dp, di, si := copySetup(dst, src); sp.Y < src.Rect.Max.Y && dp.Y < dst.Rect.Max.Y; {
			copy(dst.Attr[di:di+stride], src.Attr[si:si+stride])
			sp.Y++
			dp.Y++
			si += src.Stride
			di += dst.Stride
		}
	} else {
		const (
			fgMask = ansi.SGRAttrFGMask | ansi.SGRAttrMask
			bgMask = ansi.SGRAttrBGMask
		)
		for sp, dp, di, si := copySetup(dst, src); sp.Y < src.Rect.Max.Y && dp.Y < dst.Rect.Max.Y; {
			ii, jj := si, di
			sp.X = src.Rect.Min.X
			dp.X = dst.Rect.Min.X
			for sp.X < src.Rect.Max.X && dp.X < dst.Rect.Max.X {
				da := dst.Attr[jj]
				at := src.Attr[ii]
				if dzf || at&fgMask != 0 {
					da &= ^fgMask
				}
				if dzb || at&bgMask != 0 {
					da &= ^bgMask
				}
				dst.Attr[jj] = da | at
				ii++
				jj++
				sp.X++
			}
			si += src.Stride
			di += dst.Stride
			dp.Y++
		}
	}
}

func copySetup(dst, src Grid) (dp, sp ansi.Point, di, si int) {
	dp, sp = dst.Rect.Min, src.Rect.Min
	di, _ = dst.CellOffset(dp)
	si, _ = src.CellOffset(sp)
	return dp, sp, di, si
}

// DrawBitmap draw's a bitmap's braille runes into the destination grid.
//
// Optional rendering styles may be passed to control the graphical rendition
// and transparency of the braille runes. The styles are passed any prior grid
// attributes for each target cell.
//
// One particularly useful style to use is ElideStyle(0x2800), which will map
// any empty braille runes to the zero rune, causing only non-empty braille
// runes to be drawn.
//
// Use sub-grids to target specific regions; see Grid.SubRect.
func DrawBitmap(dst Grid, src *Bitmap, styles ...Style) {
	style := Styles(styles...)
	for gp, bp := dst.Rect.Min, src.Rect.Min; bp.Y < src.Rect.Max.Y; bp.Y += 4 {
		gp.X, bp.X = dst.Rect.Min.X, src.Rect.Min.X
		for gp.X < dst.Rect.Max.X && bp.X < src.Rect.Max.X {
			if i, ok := dst.CellOffset(gp); ok {
				r, a := src.Rune(bp), dst.Attr[i]
				if r, a = style.Style(bp, r, a); r != 0 {
					dst.Rune[i], dst.Attr[i] = r, a
				}
			}
			gp.X++
			bp.X += 2
		}
		gp.Y++
	}
}
