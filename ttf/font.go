package ttf

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"

	"github.com/bits-and-blooms/bitset"
)

type f32 = float32
type f64 = float64

type i8 = int8
type i16 = int16
type i32 = int32
type i64 = int64

type u8 = uint8
type u16 = uint16
type u32 = uint32
type u64 = uint64

type tag u32

func (t tag) String() string {
	buf := [4]byte{}
	binary.BigEndian.PutUint32(buf[:], uint32(t))
	return string(buf[:])
}

type tableName tag

const (
	TableNameCmap tableName = 0x636d6170 // 'cmap'
	TableNameCvt  tableName = 0x63767420 // 'cvt '
	TableNameFpgm tableName = 0x6670676d // 'fpgm'
	TableNameGasp tableName = 0x67617370 // 'gasp'
	TableNameGlyf tableName = 0x676c7966 // 'glyf'
	TableNameHead tableName = 0x68656164 // 'head'
	TableNameHhea tableName = 0x68686561 // 'hhea'
	TableNameHmtx tableName = 0x686d7478 // 'hmtx'
	TableNameLoca tableName = 0x6c6f6361 // 'loca'
	TableNameMaxp tableName = 0x6d617870 // 'maxp'
	TableNameName tableName = 0x6e616d65 // 'name'
	TableNameOs2  tableName = 0x4f532f32 // 'OS/2'
	TableNamePost tableName = 0x706f7374 // 'post'
	TableNamePrep tableName = 0x70726570 // 'prep'
)

const (
	platformMicrosoft = 3
	platformUnicode   = 0

	codeMsUnicodeBmp = 1
	codeUnicodeExt   = 3

	cmapFormat4  = 4
	cmapFormat12 = 12
)

type flag u32

const (
	// Monospace font.
	FlagFixedWidth flag = 1 << 0

	// Stems have short strokes drawn at an angle.
	FlagSerif flag = 1 << 1

	// Contains symbols instead of letters and numbers.
	FlagSymbolic flag = 1 << 2

	// Font resembles cursive handwriting.
	FlagScript flag = 1 << 3

	// All font glyphs use Adobe standard encoding (non-symbolic).
	FlagAdobeStandard flag = 1 << 5

	// Slanted font.
	FlagItalic flag = 1 << 6

	// Contains no lowercase letters.
	FlagAllCap flag = 1 << 16

	// Lowercase letters retain the same size as other letters in the font
	// family, but are styled as capital letters.
	FlagSmallCap flag = 1 << 17

	// Bold characters are drawn with extra pixels, even at small text sizes.
	FlagForceBold flag = 1 << 18
)

type macStyle u8

const (
	MacStyleBold      macStyle = 1 << 0
	MacStyleItalic    macStyle = 1 << 1
	MacStyleUnderline macStyle = 1 << 2
	MacStyleOutline   macStyle = 1 << 3
	MacStyleShadow    macStyle = 1 << 4
	MacStyleCondensed macStyle = 1 << 5
	MacStyleExtended  macStyle = 1 << 6
)

type fword i16
type ufword u16

type Font struct {
	gids [256 * 256]u16

	widths []f32

	Bounds Bounds

	Ascent        f32
	CapHeight     f32
	Descent       f32
	Flags         flag
	ItalicAngle   f32
	Scale         f32
	StrikeoutPos  f32
	StrikeoutSize f32
	UnderlinePos  f32
	UnderlineSize f32

	GlyphCount  u16
	MetricCount u16
	WeightClass u16

	LocaFormat u8
}

func (f *Font) GlyphId(char rune) u16 {
	return f.gids[char]
}

func (f *Font) Scaled(val fword) f32 {
	return f.Scale * f32(val)
}

func (f *Font) Width(gid u16) f32 {
	return f.widths[gid]
}

type Bounds struct {
	Max [2]f32
	Min [2]f32
}

func Generate(
	in []byte,
	font *Font,
	chars *bitset.BitSet,
	out []byte,
) (subset []byte, gidRemap []u16, err error) {
	gen := Generator{
		chars:    chars.AsSlice(make([]uint, chars.Count())),
		font:     font,
		glyphIds: []uint{},
		reader:   NewReader(in),
		writer:   NewWriter(out),
	}

	return gen.generate()
}

type Generator struct {
	chars    []uint
	font     *Font
	gidRemap []u16
	glyphIds []uint
	reader   Reader
	writer   Writer
}

func (g *Generator) copy(tableIn *Table, tableOut *Table) {
	if tableIn.Ptr == 0 {
		return
	}

	tableOut.Ptr = g.writer.pos
	tableOut.Len = tableIn.Len
	g.writer.write(g.reader.readAt(tableIn.Ptr, u32(tableIn.Len)))
	g.writer.seekTo(tableOut.Ptr + tableOut.LenPadded())
}

func (g *Generator) generate() (out []byte, gidRemap []u16, err error) {
	if err = g.reader.parseIndex(); err != nil {
		return
	}

	const requiredTableCount u16 = 9
	tableCount := requiredTableCount
	if g.reader.Tables.Cvt.Ptr != 0 {
		tableCount += 1
	}
	if g.reader.Tables.Fpgm.Ptr != 0 {
		tableCount += 1
	}
	if g.reader.Tables.Gasp.Ptr != 0 {
		tableCount += 1
	}
	if g.reader.Tables.Os2.Ptr != 0 {
		tableCount += 1
	}
	if g.reader.Tables.Prep.Ptr != 0 {
		tableCount += 1
	}

	lenIndex := 12 + u32(tableCount*16)
	g.writer.ensureCapRemaining(u32(lenIndex +
		// Tables with known length:
		g.reader.Tables.Cvt.LenPadded() +
		g.reader.Tables.Fpgm.LenPadded() +
		g.reader.Tables.Gasp.LenPadded() +
		g.reader.Tables.Head.LenPadded() +
		g.reader.Tables.Hhea.LenPadded() +
		g.reader.Tables.Maxp.LenPadded() +
		g.reader.Tables.Name.LenPadded() +
		g.reader.Tables.Os2.LenPadded() +
		g.reader.Tables.Prep.LenPadded(),
	))

	// Reserve index bytes for later writing:
	g.writer.seekTo(lenIndex)

	// Copy tables that don't need to be re-generated.
	g.copy(&g.reader.Tables.Cvt, &g.writer.Tables.Cvt)
	g.copy(&g.reader.Tables.Fpgm, &g.writer.Tables.Fpgm)
	g.copy(&g.reader.Tables.Gasp, &g.writer.Tables.Gasp)
	g.copy(&g.reader.Tables.Head, &g.writer.Tables.Head)
	g.copy(&g.reader.Tables.Hhea, &g.writer.Tables.Hhea)
	g.copy(&g.reader.Tables.Maxp, &g.writer.Tables.Maxp)
	g.copy(&g.reader.Tables.Name, &g.writer.Tables.Name)
	g.copy(&g.reader.Tables.Os2, &g.writer.Tables.Os2)
	g.copy(&g.reader.Tables.Prep, &g.writer.Tables.Prep)

	indexToLocFormat := g.genGlyfAndLoca()
	g.genCmap()
	g.genPost()
	metricCount := g.genHmtx()

	g.editHead(indexToLocFormat)
	g.editHhea(metricCount)
	g.editMaxp()

	g.genIndex(tableCount)

	out = g.writer.Bytes()
	gidRemap = g.gidRemap

	return
}

func (g *Generator) editHead(indexToLocFormat u16) {
	g.writer.seekTo(g.writer.Tables.Head.Ptr + 8)
	g.writer.u32(0)

	g.writer.seekTo(g.writer.Tables.Head.Ptr + 50)
	g.writer.u16(indexToLocFormat)
}

func (g *Generator) editHhea(metricCount u16) {
	g.writer.seekTo(g.writer.Tables.Hhea.Ptr + 34)
	g.writer.u16(metricCount)
}

func (g *Generator) editMaxp() {
	g.writer.seekTo(g.writer.Tables.Maxp.Ptr + 4)
	g.writer.u16(u16(len(g.glyphIds)))
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6cmap.html
func (g *Generator) genCmap() {
	if g.glyphIds[0] != 0 {
		panic("expected 0 as first glyph in subset")
	}

	endCodes := [512]u16{}
	startCodes := [512]u16{u16(g.chars[0])}
	deltas := [512]u16{1 - startCodes[0]} // Delta arithmetic is modulo 0xffff

	seg := u16(0)
	charPrev := u16(g.chars[0])
	for _, c := range g.chars {
		char := u16(c)
		gidOld := g.font.GlyphId(rune(char))
		gidNew := g.gidRemap[gidOld]

		if char-charPrev > 1 {
			endCodes[seg] = u16(charPrev)
			seg += 1
			startCodes[seg] = char
			// Delta arithmetic is modulo 0xffff:
			deltas[seg] = u16(gidNew) - startCodes[seg]
		}

		charPrev = char
	}
	endCodes[seg] = u16(charPrev)

	seg += 1
	endCodes[seg] = 0xffff
	startCodes[seg] = 0xffff
	deltas[seg] = 1 // Maps char code 0xffff to GID 0

	const headerLen = 4     // version, numberSubtables
	const subtableLen = 8   // platformId, platformSpecificId, offset
	const segArrayCount = 4 // endCodes, startCodes, deltas, offsets
	const paddingBytes = 2

	segCount := seg + 1
	segCount2x := 2 * segCount
	mappingTableLen := 14 + segCount2x*segArrayCount + paddingBytes

	g.writer.Tables.Cmap.Ptr = g.writer.pos
	g.writer.Tables.Cmap.Len = u32(headerLen + subtableLen + mappingTableLen)
	tableLenPadded := g.writer.Tables.Cmap.LenPadded()

	g.writer.ensureCapRemaining(tableLenPadded)
	g.writer.u16(0) // version
	g.writer.u16(1) // numberSubtables
	g.writer.u16(platformMicrosoft)
	g.writer.u16(codeMsUnicodeBmp)
	g.writer.u32(headerLen + subtableLen) // offset
	g.writer.u16(cmapFormat4)
	g.writer.u16(mappingTableLen)
	g.writer.u16(0) // language
	g.writer.u16(segCount2x)

	searchRange := u16(
		2 * math.Pow(2, math.Floor(math.Log2(float64(segCount)))),
	)
	g.writer.u16(searchRange)
	g.writer.u16(u16(math.Log2(f64(searchRange) / 2)))
	g.writer.u16(u16(segCount2x - searchRange))

	g.writer.u16Array(endCodes[:segCount])
	g.writer.u16(0) // reservedPad
	g.writer.u16Array(startCodes[:segCount])
	g.writer.u16Array(deltas[:segCount])
	g.writer.skip(u32(segCount2x)) // idRangeOffset (leave all as 0)

	g.writer.seekTo(g.writer.Tables.Cmap.Ptr + tableLenPadded)
}

type glyfFlag u16

const (
	// If set, the arguments are words; If not set, they are bytes.
	GlyfFlagArg1And2AreWords glyfFlag = 1 << iota

	// If set, the arguments are xy values; If not set, they are points.
	GlyfFlagArgsAreXyValues

	// If set, round the xy values to grid; if not set do not round xy values to
	// grid (relevant only to bit 1 is set)
	GlyfFlagRoundXyToGrid

	// If set, there is a simple scale for the component.
	// If not set, scale is 1.0.
	GlyfFlagWeHaveAScale

	// (obsolete; set to zero)
	GlyfFlagObsolete

	// If set, at least one additional glyph follows this one.
	GlyfFlagMoreComponents

	// If set the x direction will use a different scale than the y direction.
	GlyfFlagWeHaveAnXAndYScale

	// If set there is a 2-by-2 transformation that will be used to scale the
	// component.
	GlyfFlagWeHaveATwoByTwo

	// If set, instructions for the component character follow the last
	// component.
	GlyfFlagWeHaveInstructions

	// Use metrics from this component for the compound glyph.
	GlyfFlagUseMyMetrics

	// If set, the components of this compound glyph overlap.
	GlyfFlagOverlapCompound
)

func (flag glyfFlag) Test(flags u16) bool {
	return glyfFlag(flags)&flag == flag
}

type GlyfEntry struct {
	len u32
	ptr u32

	gid u16
}

func (g *GlyfEntry) lenPadded() u32 {
	return (g.len + 1) &^ 1
}

func (g *Generator) genGlyfAndLoca() u16 {
	g.writer.Tables.Glyf.Ptr = g.writer.pos
	g.writer.Tables.Glyf.Len = 0

	glyphCountEstimate := len(g.chars) + 1
	glyfs := make([]GlyfEntry, 0, glyphCountEstimate)
	var seenGids bitset.BitSet

	// Glyph 0 is required:
	// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM07/appendixB.html
	seenGids.Set(0)

	for _, char := range g.chars {
		gid := g.font.GlyphId(rune(char))
		seenGids.Set(uint(gid))
	}

	gidStack := seenGids.AsSlice(make([]uint, glyphCountEstimate))

	for len(gidStack) > 0 {
		gid := gidStack[0]
		gidStack = gidStack[1:]

		offset, len := g.reader.glyfLocation(u16(gid), g.font.LocaFormat)
		ptr := g.reader.Tables.Glyf.Ptr + offset
		g.reader.seekTo(ptr)
		glyf := GlyfEntry{
			gid: u16(gid),
			len: len,
			ptr: ptr,
		}
		glyfs = append(glyfs, glyf)

		g.writer.Tables.Glyf.Len += glyf.lenPadded()

		g.reader.seekTo(ptr)
		if contourCount := g.reader.i16(); contourCount >= 0 {
			continue
		}

		g.reader.skip(2 * 4)

		for {
			flags := g.reader.u16()

			componentGid := g.reader.u16()
			if !seenGids.Test(uint(componentGid)) {
				seenGids.Set(uint(componentGid))
				gidStack = append(gidStack, uint(componentGid))
			}

			if !GlyfFlagMoreComponents.Test(flags) {
				break
			}

			g.reader.skip(2)
			if GlyfFlagArg1And2AreWords.Test(flags) {
				g.reader.skip(2)
			}

			if GlyfFlagWeHaveAScale.Test(flags) {
				g.reader.skip(2)
			} else if GlyfFlagWeHaveAnXAndYScale.Test(flags) {
				g.reader.skip(4)
			} else if GlyfFlagWeHaveATwoByTwo.Test(flags) {
				g.reader.skip(8)
			}
		}
	}

	glyphCountActual := seenGids.Count()
	if glyphCountActual != uint(len(glyfs)) {
		panic("mismatched glyph and glyph ID counts")
	}
	if int(glyphCountActual) > cap(gidStack) {
		gidStack = slices.Grow(
			gidStack[:cap(gidStack)],
			int(glyphCountActual)-cap(gidStack),
		)
	}

	g.glyphIds = seenGids.AsSlice(gidStack)
	g.gidRemap = make([]u16, g.glyphIds[len(g.glyphIds)-1]+1)
	for gidNew, gidOld := range g.glyphIds {
		g.gidRemap[gidOld] = u16(gidNew)
	}

	slices.SortFunc(glyfs, func(a, b GlyfEntry) int {
		return int(g.gidRemap[a.gid]) - int(g.gidRemap[b.gid])
	})

	tableLenPadded := g.writer.Tables.Glyf.LenPadded()
	g.writer.ensureCapRemaining(tableLenPadded)
	for _, glyf := range glyfs {
		g.reader.seekTo(glyf.ptr)

		ptrGlyf := g.writer.pos
		g.writer.write(g.reader.read(glyf.len))

		glyfReader := NewReader(g.writer.buf[ptrGlyf : ptrGlyf+glyf.len])
		if contourCount := glyfReader.i16(); contourCount >= 0 {
			g.writer.seekTo(ptrGlyf + glyf.lenPadded())
			continue
		}

		glyfReader.skip(2 * 4)

		for {
			flags := glyfReader.u16()

			componentGid := glyfReader.u16()
			binary.BigEndian.PutUint16(
				g.writer.buf[ptrGlyf+glyfReader.pos-2:],
				g.gidRemap[componentGid],
			)

			if !GlyfFlagMoreComponents.Test(flags) {
				break
			}

			glyfReader.skip(2)
			if GlyfFlagArg1And2AreWords.Test(flags) {
				glyfReader.skip(2)
			}

			if GlyfFlagWeHaveAScale.Test(flags) {
				glyfReader.skip(2)
			} else if GlyfFlagWeHaveAnXAndYScale.Test(flags) {
				glyfReader.skip(4)
			} else if GlyfFlagWeHaveATwoByTwo.Test(flags) {
				glyfReader.skip(8)
			}
		}

		g.writer.seekTo(ptrGlyf + glyf.lenPadded())
	}

	g.writer.seekTo(g.writer.Tables.Glyf.Ptr + tableLenPadded)

	return g.genLoca(glyfs)
}

func (g *Generator) genHmtx() (metricCount u16) {
	const stride = 4

	g.writer.Tables.Hmtx.Ptr = g.writer.pos
	g.writer.Tables.Hmtx.Len = u32(len(g.glyphIds)) * stride
	tableLenPadded := g.writer.Tables.Hmtx.LenPadded()
	g.writer.ensureCapRemaining(tableLenPadded)

	var gid uint
	var lastWidth u16
	var i int

	metricCountOrig := g.font.MetricCount

	for i, gid = range g.glyphIds {
		g.reader.seekTo(g.reader.Tables.Hmtx.Ptr + u32(gid)*stride)
		lastWidth = g.reader.u16()

		g.writer.u16(lastWidth)
		g.writer.u16(g.reader.u16()) // leftSideBearing

		if u16(gid) >= metricCountOrig {
			break
		}
	}

	metricCount = u16(i + 1)
	lastWidthGid := gid
	ptrBearingsOrig := g.reader.Tables.Hmtx.Ptr + u32(metricCountOrig)*stride
	for i, gid = range g.glyphIds[metricCount:] {
		g.reader.seekTo(ptrBearingsOrig + u32(gid-lastWidthGid)*2)

		g.writer.u16(lastWidth)
		g.writer.u16(g.reader.u16())
	}

	metricCount += u16(i + 1)

	g.writer.seekTo(g.writer.Tables.Hmtx.Ptr + tableLenPadded)

	return
}

func (g *Generator) genIndex(tableCount u16) {
	g.writer.seekTo(0)

	g.writer.u32(0x0001_0000) // TrueType identifier
	g.writer.u16(tableCount)

	entrySelector := math.Floor(math.Log2(float64(tableCount)))
	searchRange := u16(16 * math.Pow(2, entrySelector))
	rangeShift := tableCount*16 - searchRange

	g.writer.u16(searchRange)
	g.writer.u16(u16(entrySelector))
	g.writer.u16(rangeShift)

	// Required tables:
	g.genIndexEntry(TableNameCmap, &g.writer.Tables.Cmap)
	g.genIndexEntry(TableNameGlyf, &g.writer.Tables.Glyf)
	g.genIndexEntry(TableNameHead, &g.writer.Tables.Head)
	g.genIndexEntry(TableNameHhea, &g.writer.Tables.Hhea)
	g.genIndexEntry(TableNameHmtx, &g.writer.Tables.Hmtx)
	g.genIndexEntry(TableNameLoca, &g.writer.Tables.Loca)
	g.genIndexEntry(TableNameMaxp, &g.writer.Tables.Maxp)
	g.genIndexEntry(TableNameName, &g.writer.Tables.Name)
	g.genIndexEntry(TableNamePost, &g.writer.Tables.Post)

	// Optional tables:
	g.genIndexEntry(TableNameCvt, &g.writer.Tables.Cvt)
	g.genIndexEntry(TableNameFpgm, &g.writer.Tables.Fpgm)
	g.genIndexEntry(TableNameGasp, &g.writer.Tables.Gasp)
	g.genIndexEntry(TableNameOs2, &g.writer.Tables.Os2)
	g.genIndexEntry(TableNamePrep, &g.writer.Tables.Prep)
}

func (g *Generator) genIndexEntry(tag tableName, table *Table) {
	if table.Ptr == 0 {
		return
	}

	var checksum u32 = 0
	ptrChecksum := 0
	for nLongs := (table.Len + 3) / 4; nLongs > 0; nLongs -= 1 {
		checksum += binary.BigEndian.Uint32(
			g.writer.buf[ptrChecksum : ptrChecksum+4],
		)
		ptrChecksum += 4
	}

	g.writer.u32(u32(tag))
	g.writer.u32(checksum)
	g.writer.u32(table.Ptr)
	g.writer.u32(table.Len)
}

func (g *Generator) genLoca(glyfs []GlyfEntry) u16 {
	g.writer.Tables.Loca.Ptr = g.writer.pos

	locaFormat := u16(0)
	if g.writer.Tables.Glyf.Len > 0xffff*2 {
		locaFormat = 1
	}

	// Format 0 - u16 offsets:
	if locaFormat == 0 {
		g.writer.Tables.Loca.Len = u32(len(glyfs)+1) * 2
		tableLenPadded := g.writer.Tables.Loca.LenPadded()
		g.writer.ensureCapRemaining(tableLenPadded)

		var nextOffset u16 = 0
		for _, glyf := range glyfs {
			g.writer.u16(nextOffset)
			nextOffset += u16(glyf.lenPadded() >> 1)
		}
		g.writer.u16(nextOffset)

		g.writer.seekTo(g.writer.Tables.Loca.Ptr + tableLenPadded)

		return locaFormat
	}

	// Format 1 - u32 offsets:
	g.writer.Tables.Loca.Len = u32(len(glyfs)+1) * 4
	tableLenPadded := g.writer.Tables.Loca.LenPadded()
	g.writer.ensureCapRemaining(tableLenPadded)

	var nextOffset u32 = 0
	for _, glyf := range glyfs {
		g.writer.u32(nextOffset)
		nextOffset += glyf.lenPadded()
	}
	g.writer.u32(nextOffset)

	g.writer.seekTo(g.writer.Tables.Loca.Ptr + tableLenPadded)

	return locaFormat
}

func (g *Generator) genPost() {
	g.writer.Tables.Post.Ptr = g.writer.pos
	g.writer.Tables.Post.Len = 0 +
		4 + // format
		4 + // italicAngle
		2 + // underlinePosition
		2 + // underlineThickness
		4 + // isFixedPitch
		4 + // minMemType42
		4 + // maxMemType42
		4 + // minMemType1
		4 // maxMemType1
	tableLenPadded := g.writer.Tables.Post.LenPadded()
	g.writer.ensureCapRemaining(tableLenPadded)

	g.writer.u32(0x00030000) // Format 3.0

	g.reader.seekTo(g.reader.Tables.Post.Ptr + 4) // Skip format
	g.writer.write(g.reader.read(0 +
		4 + // italicAngle
		2 + // underlinePosition
		2 + // underlineThickness
		4, // isFixedPitch
	))

	g.writer.skip(16) // [min,max]MemType42 + [min,max]MemType1 (leave all as 0)

	g.writer.seekTo(g.writer.Tables.Post.Ptr + tableLenPadded)
}

func Parse(bytes []byte, font *Font) error {
	parser := Parser{
		font:   font,
		reader: NewReader(bytes),
	}

	return parser.parse()
}

type Parser struct {
	font   *Font
	reader Reader
}

func (p *Parser) fwordScaled() f32 {
	return f32(p.reader.fword()) * p.font.Scale
}

func (p *Parser) parse() error {
	var err error
	if err = p.reader.parseIndex(); err != nil {
		return err
	}

	if err = p.parseHead(); err != nil {
		return err
	}
	if p.font.Scale == 0 {
		panic("em scale not populated after parsing 'head' table")
	}

	if err = p.parseHhea(); err != nil {
		return err
	}

	if err = p.parseOs2(); err != nil {
		return err
	}

	p.parsePost()
	p.parseMaxP()

	if err = p.parseCmap(); err != nil {
		return err
	}

	if err = p.parseHmtx(); err != nil {
		return err
	}

	return nil
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6cmap.html
func (p *Parser) parseCmap() error {
	p.reader.seekTo(p.reader.Tables.Cmap.Ptr + 2) // Skip version

	subtableCount := p.reader.u16()

	var offset u32
	for range subtableCount {
		platform := p.reader.u16()
		code := p.reader.u16()

		if (platform == platformUnicode && code == codeUnicodeExt) ||
			(platform == platformMicrosoft && code == codeMsUnicodeBmp) {
			offset = p.reader.u32()
			break
		}

		p.reader.skip(4) // Skip offset
	}
	if offset == 0 {
		return fmt.Errorf("no supported unicode character map table found")
	}

	p.reader.seekTo(p.reader.Tables.Cmap.Ptr + offset)

	format := p.reader.u16()
	if format != cmapFormat4 {
		return fmt.Errorf(
			"expected cmap table format %d, found: %d",
			cmapFormat4,
			format,
		)
	}

	p.reader.skip(4) // length, language code

	segCount := p.reader.u16() >> 1

	p.reader.skip(6) // Search helper params

	endCodes := [512]u16{}
	startCodes := [512]u16{}
	deltas := [512]u16{}

	var i u16
	for i = range segCount {
		endCodes[i] = p.reader.u16()
	}

	p.reader.skip(2) // reservedPad

	for i = range segCount {
		startCodes[i] = p.reader.u16()
	}

	for i = range segCount {
		deltas[i] = p.reader.u16()
	}

	for i := range segCount {
		posRangeOffset := p.reader.pos
		rangeOffset := p.reader.u16()
		if rangeOffset == 0 {
			for char := startCodes[i]; ; char += 1 {
				gid := char + deltas[i]
				p.font.gids[rune(char)] = gid

				// Broken out of loop condition to handle 0xffff-0xffff range:
				if char == endCodes[i] {
					break
				}
			}
		} else {
			for char := startCodes[i]; ; char += 1 {
				posGlyphIndex := posRangeOffset + u32(
					rangeOffset,
				) + 2*(u32(char)-u32(startCodes[i]))

				gid := binary.BigEndian.Uint16(p.reader.buf[posGlyphIndex:])
				p.font.gids[rune(char)] = gid

				// Broken out of loop condition to handle 0xffff-0xffff range:
				if char == endCodes[i] {
					break
				}
			}
		}
	}

	return nil
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6head.html
func (p *Parser) parseHead() error {
	p.reader.seekTo(p.reader.Tables.Head.Ptr + 18)

	p.font.Scale = 1000.0 / f32(p.reader.u16())

	p.reader.skip(16) // created date + modified date

	p.font.Bounds = Bounds{
		Min: [2]f32{
			p.fwordScaled(),
			p.fwordScaled(),
		},
		Max: [2]f32{
			p.fwordScaled(),
			p.fwordScaled(),
		},
	}

	style := macStyle(p.reader.u16())
	if style&MacStyleItalic != 0 {
		p.font.Flags |= FlagItalic
	}

	p.reader.skip(4) // lowestRecPPEM, fontDirectionHint

	p.font.LocaFormat = u8(p.reader.u16())

	glyphDataFormat := p.reader.u16()
	if glyphDataFormat != 0 {
		return fmt.Errorf(
			`invalid glyphDataFormat in "head" table: %d`,
			glyphDataFormat,
		)
	}

	return nil
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6hhea.html
func (p *Parser) parseHhea() error {
	p.reader.seekTo(p.reader.Tables.Hhea.Ptr + 4)

	p.font.Ascent = p.fwordScaled()
	p.font.Descent = p.fwordScaled()

	p.reader.skip(24)

	if metricDataFormat := p.reader.u16(); metricDataFormat != 0 {
		return fmt.Errorf(
			`invalid metricDataFormat in "hhea" table: %d`,
			metricDataFormat,
		)
	}

	if p.font.MetricCount = p.reader.u16(); p.font.MetricCount == 0 {
		return fmt.Errorf("numOfLongHorMetrics == 0 in hhea table")
	}

	return nil
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6hmtx.html
func (p *Parser) parseHmtx() error {
	for gid := range p.font.GlyphCount {
		const stride = 4
		ptr := p.reader.Tables.Hmtx.Ptr + stride*u32(
			min(gid, p.font.MetricCount-1),
		)
		p.reader.seekTo(ptr)
		p.font.widths[gid] = p.fwordScaled()
	}

	return nil
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6.html
func (r *Reader) parseIndex() error {
	typ := r.u32()

	switch typ {
	case 0x7472_7565: // Four-char code: 'true'
		fallthrough
	case 0x0001_0000: // TrueType identifier
		break
	default:
		return fmt.Errorf("expected TrueType font, got type %x", typ)
	}

	tableCount := r.u16()

	r.skip(6) // searchRange, entrySelector, rangeShift (all u16)

	for range tableCount {
		name := tableName(r.tag())
		var table *Table

		switch name {
		case TableNameCmap:
			table = &r.Tables.Cmap
		case TableNameCvt:
			table = &r.Tables.Cvt
		case TableNameFpgm:
			table = &r.Tables.Fpgm
		case TableNameGasp:
			table = &r.Tables.Gasp
		case TableNameGlyf:
			table = &r.Tables.Glyf
		case TableNameHead:
			table = &r.Tables.Head
		case TableNameHhea:
			table = &r.Tables.Hhea
		case TableNameHmtx:
			table = &r.Tables.Hmtx
		case TableNameLoca:
			table = &r.Tables.Loca
		case TableNameMaxp:
			table = &r.Tables.Maxp
		case TableNameName:
			table = &r.Tables.Name
		case TableNameOs2:
			table = &r.Tables.Os2
		case TableNamePost:
			table = &r.Tables.Post
		case TableNamePrep:
			table = &r.Tables.Prep

		default:
			r.skip(12) // checksum + position + length (all u32)
			continue
		}

		r.skip(4) // checksum
		*table = Table{
			Ptr: r.u32(),
			Len: r.u32(),
		}
	}

	if r.Tables.Cmap.Ptr == 0 ||
		r.Tables.Glyf.Ptr == 0 ||
		r.Tables.Head.Ptr == 0 ||
		r.Tables.Hhea.Ptr == 0 ||
		r.Tables.Hmtx.Ptr == 0 ||
		r.Tables.Loca.Ptr == 0 ||
		r.Tables.Maxp.Ptr == 0 ||
		r.Tables.Name.Ptr == 0 ||
		r.Tables.Post.Ptr == 0 {
		return fmt.Errorf("missing one or more required TTF tables")
	}

	return nil
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6maxp.html
func (p *Parser) parseMaxP() {
	p.reader.seekTo(p.reader.Tables.Maxp.Ptr + 4) // Skip version.
	p.font.GlyphCount = p.reader.u16()
	p.font.widths = make([]f32, p.font.GlyphCount)
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6OS2.html
func (p *Parser) parseOs2() error {
	if p.reader.Tables.Os2.Ptr == 0 {
		return nil
	}

	p.reader.seekTo(p.reader.Tables.Os2.Ptr)

	version := p.reader.u16()

	p.reader.skip(2) // xAvgCharWidth

	p.font.WeightClass = p.reader.u16()

	p.reader.skip(2) // usWidthClass

	// 0 Reserved; must be 0
	// 1 Licensed (protected) font; should not be 1 if bits 2 or 3 are one.
	//   Fonts that have only this bit set must not be modified, embedded, or
	//   exchanged in any manner without first obtaining permission of the legal
	//   owner.
	// 2 Preview and print embedding; should not be 1 if bits 1 or 3 are one.
	//   Fonts that have only this bit set may be embedded in documents and
	//   temporarily loaded on the remote system. Documents containing such
	//   fonts must be opened “read-only;” no edits can be applied to the
	//   document.
	// 3 Editable embedding; should not be 1 if bits 1 or 2 are one. Fonts that
	//   have only this bit set may be embedded in documents and temporarily
	//   loaded on the remote system. Documents containing such fonts may be
	//   editable.
	// 4–7 Reserved; must be 0
	// 8 No subsetting. When this bit is set, the font may not be subsetted
	//   prior to embedding. Other embedding restrictions specified in bits 1–3
	//   and 9 also apply.
	// 9 Bitmap embedding only. When this bit is set, only bitmaps contained in
	//   the font may be embedded. No outline data may be embedded. Other
	//   embedding restrictions specified in bits 1–3 and 8 also apply.
	// 10–15 Reserved; must be 0
	fsType := p.reader.u16()

	const flagLicensed = 0b10
	const flagEmbedBitmapOnly = 0b100000000

	if fsType&(flagLicensed|flagEmbedBitmapOnly) != 0 {
		return fmt.Errorf("unable to parse protected or restricted font")
	}

	p.reader.skip(0 +
		2 + // ySubscriptXSize
		2 + // ySubscriptYSize
		2 + // ySubscriptXOffset
		2 + // ySubscriptYOffset
		2 + // ySuperscriptXSize
		2 + // ySuperscriptYSize
		2 + // ySuperscriptXOffset
		2, // ySuperscriptYOffset
	)

	p.font.StrikeoutSize = p.fwordScaled()
	p.font.StrikeoutPos = p.fwordScaled()

	p.reader.skip(0 +
		2 + // familyClass
		10 + // panose
		16 + // ulUnicodeRange
		4 + // achVendID
		2 + // fsSelection
		2 + // fsFirstCharIndex
		2, // fsLastCharIndex
	)

	typoAscender := p.fwordScaled()
	if p.font.Ascent == 0 {
		p.font.Ascent = typoAscender
	}
	p.font.CapHeight = p.font.Ascent

	typoDescender := p.fwordScaled()
	if p.font.Descent == 0 {
		p.font.Descent = typoDescender
	}

	if version <= 1 {
		return nil
	}

	p.reader.skip(16)
	p.font.CapHeight = p.fwordScaled()

	return nil
}

// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6post.html
func (p *Parser) parsePost() {
	p.reader.seekTo(p.reader.Tables.Post.Ptr + 4) // Skip format

	p.font.ItalicAngle = p.reader.fixed().float()
	p.font.UnderlinePos = p.fwordScaled()
	p.font.UnderlineSize = p.fwordScaled()

	if p.reader.u32() != 0 {
		p.font.Flags |= FlagFixedWidth
	}
	if p.font.ItalicAngle != 0 {
		p.font.Flags |= FlagItalic
	}
}

type Tables struct {
	Cmap Table
	Cvt  Table
	Fpgm Table
	Gasp Table
	Glyf Table
	Head Table
	Hhea Table
	Hmtx Table
	Loca Table
	Maxp Table
	Name Table
	Os2  Table
	Post Table
	Prep Table
}

type Table struct {
	Len u32
	Ptr u32
}

func (t *Table) LenPadded() u32 {
	return (t.Len + 3) &^ 3
}

func (t *Table) String() string {
	return fmt.Sprintf("0x%x: %d bytes", t.Ptr, t.Len)
}

type Reader struct {
	Tables Tables
	buf    []byte
	pos    u32
}

func NewReader(bytes []byte) Reader {
	return Reader{buf: bytes}
}

type fixed i32

func (f fixed) float() f32 {
	return f32(f64(f) / f64(1<<16))
}

func (r *Reader) fixed() fixed {
	return fixed(r.i32())
}

func (r *Reader) fword() fword {
	return fword(r.i16())
}

func (r *Reader) glyfLocation(gid u16, locaFormat u8) (offset u32, len u32) {
	if locaFormat == 0 {
		r.seekTo(r.Tables.Loca.Ptr + u32(gid)*2)
		offset = u32(r.u16()) * 2
		len = u32(r.u16())*2 - offset
	} else {
		r.seekTo(r.Tables.Loca.Ptr + u32(gid)*4)
		offset = r.u32()
		len = r.u32() - offset
	}

	return
}

func (r *Reader) i16() i16 {
	return i16(r.u16())
}

func (r *Reader) i32() i32 {
	return i32(r.u32())
}

func (r Reader) Len() u32 {
	return u32(len(r.buf))
}

func (r *Reader) read(count u32) (bytes []byte) {
	bytes = r.buf[r.pos:][0:count]
	r.pos += count
	return
}

func (r *Reader) readAt(pos u32, count u32) (bytes []byte) {
	bytes = r.buf[pos:][0:count]
	return
}

func (r *Reader) seekTo(pos u32) {
	r.pos = pos
}

func (r *Reader) skip(count u32) {
	r.pos += count
}

func (r *Reader) tag() tag {
	return tag(r.u32())
}

func (r *Reader) u16() u16 {
	return binary.BigEndian.Uint16(r.read(2))
}

func (r *Reader) u32() u32 {
	return binary.BigEndian.Uint32(r.read(4))
}

type Writer struct {
	Tables Tables
	buf    []byte
	len    u32
	pos    u32
}

func NewWriter(bytes []byte) Writer {
	return Writer{buf: bytes}
}

func (w *Writer) Bytes() []byte {
	return w.buf[:cap(w.buf)]
}

func (w *Writer) ensureCapRemaining(byteCount u32) {
	w.buf = slices.Grow(w.buf[:w.len], int(byteCount))
}

func (w Writer) Len() u32 {
	return u32(len(w.buf))
}

func (w *Writer) seekTo(pos u32) {
	w.pos = pos
	w.buf = w.buf[:w.pos]
	w.len = max(w.len, w.pos)
}

func (w *Writer) skip(count u32) {
	w.seekTo(w.pos + count)
}

func (w *Writer) u16(val u16) {
	binary.BigEndian.PutUint16(w.buf[:w.pos+2][w.pos:], val)
	w.skip(2)
}

func (w *Writer) u16Array(arr []u16) {
	for _, val := range arr {
		w.u16(val)
	}
}

func (w *Writer) u32(val u32) {
	binary.BigEndian.PutUint32(w.buf[:w.pos+4][w.pos:], val)
	w.skip(4)
}

func (w *Writer) write(src []byte) {
	copy(w.buf[:int(w.pos)+len(src)][w.pos:], src)
	w.skip(u32(len(src)))
}
