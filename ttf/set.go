package ttf

import (
	"fmt"
	"log"
	"slices"
	"strings"
)

type FontInfo struct {
	font Font
	key  Key
}

func (i *FontInfo) Font() *Font {
	return &i.font
}

func (i *FontInfo) GlyphWidth(char rune) (gid uint16, width float32) {
	gid = i.font.GlyphId(char)
	width = i.font.Width(gid)

	return
}

func (i *FontInfo) GlyphWidthOnly(char rune) float32 {
	_, width := i.GlyphWidth(char)
	return width
}

func (i *FontInfo) String() string {
	return i.key.String()
}

func (i FontInfo) Key() Key {
	return i.key
}

type Id uint8

type FontSet struct {
	fonts []FontInfo
}

func NewFontSet(capacity uint8) FontSet {
	return FontSet{
		fonts: make([]FontInfo, 0, capacity),
	}
}

func (f *FontSet) Get(id Id) *FontInfo {
	return &f.fonts[id]
}

func (f *FontSet) Key(id Id) Key {
	return f.fonts[id].key
}

func (f *FontSet) AddTtf(family string, style Style, bytes []byte) (Id, error) {
	id := len(f.fonts)
	f.fonts = append(f.fonts, FontInfo{
		key: Key{Family: strings.ToLower(family), Style: style},
	})

	err := Parse(bytes, &f.fonts[id].font)
	if err != nil {
		return Id(id), fmt.Errorf("unable to parse font file: %w", err)
	}

	return Id(id), nil
}

func (f *FontSet) Grow(amt uint8) {
	f.fonts = slices.Grow(f.fonts, int(amt))
}

func (f *FontSet) Len() int {
	return len(f.fonts)
}

func (f *FontSet) MustAddTtf(family string, style Style, bytes []byte) Id {
	id, err := f.AddTtf(family, style, bytes)
	if err != nil {
		log.Panicf(
			"unable to add font family(%s), style(%s): %v",
			family,
			style,
			err,
		)
	}

	return id
}

type Key struct {
	Family string
	Style  Style
}

func (k Key) String() string {
	return strings.ToLower(k.Family) + k.Style.String()
}

type Style uint8

const (
	StyleNone Style = 0
)

const (
	StyleB Style = 1 << iota
	StyleI
	StyleS
	StyleU
)

func (s Style) Strike() bool {
	return s&StyleS != 0
}

func (s Style) String() string {
	style := s & ^(StyleS | StyleU)

	switch style {
	case StyleB:
		return "b"
	case StyleI:
		return "i"
	case StyleB | StyleI:
		return "bi"
	}

	return ""
}

func (s Style) Underline() bool {
	return s&StyleU != 0
}
