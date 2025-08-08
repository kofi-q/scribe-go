package scribe

import (
	"fmt"
	"math"
	"unicode"
	"unicode/utf8"
)

func (f *Scribe) Scratch(width float32) (sc ScratchPad, err error) {
	if f.fontSizePt == 0 {
		err = fmt.Errorf("no measurement font set before calling TextSplit")
		return
	}

	sc = ScratchPad{
		f:     f,
		width: width,
		widthCharMax: uint16(
			math.Ceil(float64((width) * 1000 / sc.font.fontSizePt)),
		),
		x: 0,
		y: 0,
	}
	sc.SetFont(f.currentFont, f.fontStyle, f.fontSize)

	return
}

type ScratchPad struct {
	font fontSpec

	width            float32
	widthLongestLine float32
	f                *Scribe
	x                float32
	y                float32

	widthBreakChar     uint16
	widthCharsLine     uint16
	widthCharMax       uint16
	widthCharsPreBreak uint16
}

func (sc *ScratchPad) StyleAdd(style FontStyle) {
	sc.font.fontStyle |= style
}

func (sc *ScratchPad) StyleRemove(style FontStyle) {
	sc.font.fontStyle &= ^style
}

func (sc *ScratchPad) Ln(height float32) {
	sc.widthCharsLine = 0
	sc.widthCharsPreBreak = 0
	sc.widthBreakChar = 0
	sc.x = 0
	sc.y += height
}

func (sc *ScratchPad) Reset(width float32) {
	sc.width = width
	sc.widthBreakChar = 0
	sc.widthCharsLine = 0
	sc.widthCharsPreBreak = 0
	sc.widthLongestLine = 0
	sc.x = 0
	sc.y = 0

	sc.SetFont(sc.f.currentFont, sc.f.fontStyle, sc.f.fontSize)
}

func (sc *ScratchPad) SetFont(id FontId, style FontStyle, size float32) {
	sc.font = sc.f.getFont(id, style, size)
	sc.widthCharMax = uint16(
		math.Ceil(float64(sc.width * 1000 / sc.font.fontSizePt)),
	)
}

func (sc *ScratchPad) Text(lnHeight float32, text string) (lines []string) {
	var char rune
	var ixChar int
	var ixBreak int
	var ixAfterBreak int
	var ixLine int
	for ixChar, char = range text {
		widthChar := uint16(sc.f.fonts.Get(sc.font.id).GlyphWidthOnly(char))
		sc.widthCharsLine += widthChar

		isSpace := unicode.IsSpace(char)
		if isSpace || isChinese(char) {
			sc.widthCharsPreBreak = sc.widthCharsLine
			ixBreak = ixChar
			ixAfterBreak = ixBreak

			if isSpace {
				sc.widthBreakChar = widthChar
				sc.widthCharsPreBreak -= widthChar
				runeLen := utf8.RuneLen(char)
				if runeLen > -1 {
					ixAfterBreak += runeLen
				}
			}
		}

		if char == '\n' || sc.widthCharsLine > sc.widthCharMax {
			// Scratch pad width is too small to fit a single character.
			// Skip line break until after this char, to avoid infinite line
			// breaks.
			if sc.widthCharsPreBreak == 0 && sc.x == 0 && char != '\n' {
				continue
			}

			sc.y += lnHeight

			lines = append(lines, text[ixLine:ixBreak])
			ixLine = ixAfterBreak

			sc.widthCharsLine -= (sc.widthCharsPreBreak + sc.widthBreakChar)

			sc.widthLongestLine = float32(math.Max(
				float64(sc.widthLongestLine),
				float64(sc.widthCharsPreBreak)*float64(sc.font.fontSizePt/1000),
			))
			sc.widthCharsPreBreak = 0
			sc.widthBreakChar = 0
			ixBreak = 0
		}
	}

	if len(text[ixLine:]) > 0 {
		lines = append(lines, text[ixLine:])
	}

	sc.x = float32(sc.widthCharsLine) * sc.font.fontSizePt / 1000
	sc.widthLongestLine = float32(math.Max(
		float64(sc.x),
		float64(sc.widthLongestLine),
	))

	return
}

func (sc *ScratchPad) WidthLongestLine() float32 {
	return sc.widthLongestLine
}

func (sc *ScratchPad) X() float32 {
	return sc.x
}

func (sc *ScratchPad) Xy() (float32, float32) {
	return sc.x, sc.y
}

func (sc *ScratchPad) Y() float32 {
	return sc.y
}
