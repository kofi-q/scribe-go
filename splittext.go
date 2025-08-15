// Copyright ©2023 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package scribe

import (
	"fmt"
	"math"
	"unicode"
)

// TextSplit splits UTF-8 encoded text into several lines using the current
// font. Each line has its length limited to a maximum width given by width.
// This function can be used to determine the total height of wrapped text for
// vertical placement purposes.
func (f *Scribe) TextSplit(text string, width float32) (lines []string) {
	font := f.measurementFont
	if font.fontSizePt == 0 {
		f.err = fmt.Errorf("no measurement font set before calling TextSplit")
		return lines
	}

	widthMax := int(math.Ceil(float64((width) * 1000 / font.fontSizePt)))

	defaultGlyphWidth := f.font().GlyphWidthOnly(0xffff)
	lineCountEstimate := int(
		math.Ceil(
			float64(len(text)*int(defaultGlyphWidth)) / float64(widthMax),
		),
	)
	lines = make([]string, 0, lineCountEstimate)

	ixChar := 0
	ixBreak := -1
	ixBreakIsSpace := true
	ixLineStart := 0
	lenLine := 0

	var char rune
	for ixChar, char = range text {
		lenLine += int(f.fonts.Get(font.id).GlyphWidthOnly(char))

		isSpace := unicode.IsSpace(char)
		if isSpace || isChinese(char) {
			ixBreak = ixChar
			ixBreakIsSpace = isSpace
		}

		if char == '\n' || lenLine > widthMax {
			nextIxLineStart := ixBreak
			if ixBreakIsSpace {
				nextIxLineStart += 1
			}
			if ixBreak == -1 && char != '\n' {
				if ixChar == ixLineStart {
					continue
				}

				ixBreak = ixChar
				nextIxLineStart = ixBreak
			}

			lines = append(lines, text[ixLineStart:ixBreak])
			ixLineStart = nextIxLineStart

			lenLine = 0
			for _, char := range text[ixLineStart : ixChar+1] {
				lenLine += int(f.fonts.Get(font.id).GlyphWidthOnly(char))
			}

			ixBreak = -1
			ixBreakIsSpace = true
		}
	}

	if len(text[ixLineStart:]) > 0 {
		lines = append(lines, text[ixLineStart:])
	}

	return
}
