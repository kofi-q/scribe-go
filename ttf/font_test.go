package ttf

import (
	_ "embed"
	"testing"

	"github.com/bits-and-blooms/bitset"
)

var (
	//go:embed NotoSansSC-Regular.ttf
	notoSc []byte

	//go:embed Roboto-Italic.ttf
	robotoI []byte
)

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		var font Font
		Parse(notoSc, &font)
	}
}

func BenchmarkLookup(b *testing.B) {
	gids := [1024]u16{}
	var i int
	var font Font
	Parse(notoSc, &font)

	for b.Loop() {
		i = 0
		for _, c := range "是否應就與不動產改善工程建設有關的損害賠償問題對州憲法進行修訂，並禁止制定限製或損害業主因未進行不動產改善工程建設而造成損害賠償的權利的法律？ 靛經 濟損 the quick brown fox jumped over 01234 the lazy dog" {
			gids[i] = font.GlyphId(c)
			i += 1
		}
	}
}

func BenchmarkWidth(b *testing.B) {
	widths := [1024]f32{}
	var i int
	var font Font
	Parse(notoSc, &font)

	for b.Loop() {
		i = 0
		for _, c := range "是否應就與不動產改善工程建設有關的損害賠償問題對州憲法進行修訂，並禁止制定限製或損害業主因未進行不動產改善工程建設而造成損害賠償的權利的法律？ 靛經 濟損 the quick brown fox jumped over 01234 the lazy dog" {
			widths[i] = font.Width(font.GlyphId(c))
			i += 1
		}
	}
}

func BenchmarkGenerate(b *testing.B) {
	var font Font
	Parse(notoSc, &font)

	var gidSet bitset.BitSet
	for _, c := range "是否應就與不動產改善工程建設有關的損害賠償問題對州憲法進行修訂，並禁止制定限製或損害業主因未進行不動產改善工程建設而造成損害賠償的權利的法律？ 靛經 濟損 the quick brown fox jumped over 01234 the lazy dog" {
		gidSet.Set(uint(font.GlyphId(c)))
	}

	for b.Loop() {
		subset := []byte{}
		subset, _, _ = Generate(notoSc, &font, &gidSet, subset)
		_ = subset
	}
}

// func BenchmarkParseOld(b *testing.B) {
// 	for b.Loop() {
// 		scribe.ParseTtfFont(robotoI)
// 	}
// }
