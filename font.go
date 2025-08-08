package scribe

import "github.com/kofi-q/scribe-go/ttf"

func NewFontSet(capacity uint8) FontSet {
	return ttf.NewFontSet(capacity)
}
