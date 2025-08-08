// Copyright ©2023 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*
 * Copyright (c) 2013-2014 Kurt Jung (Gmail: kurt.w.jung)
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package scribe

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/bits-and-blooms/bitset"
	"github.com/kofi-q/scribe-go/ttf"
)

// Version of FPDF from which this package is derived
const (
	libVersion = "0.1"
)

type blendModeType struct {
	strokeStr, fillStr, modeStr string
	objNum                      uint32
}

type gradientType struct {
	tp                int // 2: linear, 3: radial
	clr1Str, clr2Str  string
	x1, y1, x2, y2, r float32
	objNum            uint32
}

const (
	// OrientationPortrait represents the portrait orientation.
	OrientationPortrait = "portrait"

	// OrientationLandscape represents the landscape orientation.
	OrientationLandscape = "landscape"
)

const (
	// UnitPoint represents the size unit point
	UnitPoint = "pt"
	// UnitMillimeter represents the size unit millimeter
	UnitMillimeter = "mm"
	// UnitCentimeter represents the size unit centimeter
	UnitCentimeter = "cm"
	// UnitInch represents the size unit inch
	UnitInch = "inch"
)

type PageSize SizeType

var (
	PageSizeA1      = PageSize{1683.78, 2383.94}
	PageSizeA2      = PageSize{1190.55, 1683.78}
	PageSizeA3      = PageSize{841.89, 1190.55}
	PageSizeA4      = PageSize{595.28, 841.89}
	PageSizeA5      = PageSize{420.94, 595.28}
	PageSizeA6      = PageSize{297.64, 420.94}
	PageSizeA7      = PageSize{209.76, 297.64}
	PageSizeLegal   = PageSize{612, 1008}
	PageSizeLetter  = PageSize{612, 792}
	PageSizeTabloid = PageSize{792, 1224}
)

const (
	// BorderNone set no border
	BorderNone = ""
	// BorderFull sets a full border
	BorderFull = "1"
	// BorderLeft sets the border on the left side
	BorderLeft = "L"
	// BorderTop sets the border at the top
	BorderTop = "T"
	// BorderRight sets the border on the right side
	BorderRight = "R"
	// BorderBottom sets the border on the bottom
	BorderBottom = "B"
)

const (
	// LineBreakNone disables linebreak
	LineBreakNone = 0
	// LineBreakNormal enables normal linebreak
	LineBreakNormal = 1
	// LineBreakBelow enables linebreak below
	LineBreakBelow = 2
)

const (
	// AlignLeft left aligns the cell
	AlignLeft = "L"
	// AlignRight right aligns the cell
	AlignRight = "R"
	// AlignCenter centers the cell
	AlignCenter = "C"
	// AlignTop aligns the cell to the top
	AlignTop = "T"
	// AlignBottom aligns the cell to the bottom
	AlignBottom = "B"
	// AlignMiddle aligns the cell to the middle
	AlignMiddle = "M"
	// AlignBaseline aligns the cell to the baseline
	AlignBaseline = "B"
)

type colorMode int

const (
	colorModeRGB colorMode = iota
	colorModeSpot
)

type colorType struct {
	r, g, b    float32
	ir, ig, ib uint8
	mode       colorMode
	spotStr    string // name of current spot color
	gray       bool
	str        string
}

// SpotColorType specifies a named spot color value
type spotColorType struct {
	id, objID uint32
	val       cmykColorType
}

// cmykColorType specifies an ink-based CMYK color value
type cmykColorType struct {
	c, m, y, k byte // 0% to 100%
}

// SizeType fields Wd and Ht specify the horizontal and vertical extents of a
// document element such as a page.
type SizeType struct {
	Wd, Ht float32
}

// PointType fields X and Y specify the horizontal and vertical coordinates of
// a point, typically used in drawing.
type PointType struct {
	X, Y float32
}

// XY returns the X and Y components of the receiver point.
func (p PointType) XY() (float32, float32) {
	return p.X, p.Y
}

// ImageInfoType contains size, color and other information about an image.
// Changes to this structure should be reflected in its GobEncode and GobDecode
// methods.
type ImageInfoType struct {
	data  []byte  // Raw image data
	smask []byte  // Soft Mask, an 8bit per-pixel transparency mask
	n     uint32  // Image object number
	w     float32 // Width
	h     float32 // Height
	cs    string  // Color space
	pal   []byte  // Image color palette
	bpc   uint8   // Bits Per Component
	f     string  // Image filter
	dp    string  // DecodeParms
	trns  []int   // Transparency mask
	scale float32 // Document scale factor
	dpi   float32 // Dots-per-inch found from image file (png only)
	i     string  // SHA-1 checksum of the above values.
}

type idEncoder struct {
	w   io.Writer
	buf []byte
	err error
}

func newIDEncoder(w io.Writer) *idEncoder {
	return &idEncoder{
		w:   w,
		buf: make([]byte, 8),
	}
}

func (enc *idEncoder) i64(v int64) {
	if enc.err != nil {
		return
	}
	binary.LittleEndian.PutUint64(enc.buf, uint64(v))
	_, enc.err = enc.w.Write(enc.buf)
}

func (enc *idEncoder) i32(v int64) {
	if enc.err != nil {
		return
	}
	binary.LittleEndian.PutUint32(enc.buf, uint32(v))
	_, enc.err = enc.w.Write(enc.buf)
}

func (enc *idEncoder) u32(v uint32) {
	if enc.err != nil {
		return
	}
	binary.LittleEndian.PutUint32(enc.buf, v)
	_, enc.err = enc.w.Write(enc.buf)
}

func (enc *idEncoder) u16(v uint16) {
	if enc.err != nil {
		return
	}
	binary.LittleEndian.PutUint16(enc.buf, v)
	_, enc.err = enc.w.Write(enc.buf)
}

func (enc *idEncoder) f32(v float32) {
	if enc.err != nil {
		return
	}
	binary.LittleEndian.PutUint32(enc.buf, math.Float32bits((v)))
	_, enc.err = enc.w.Write(enc.buf)
}

func (enc *idEncoder) str(v string) {
	if enc.err != nil {
		return
	}
	_, enc.err = enc.w.Write([]byte(v))
}

func (enc *idEncoder) bytes(v []byte) {
	if enc.err != nil {
		return
	}
	_, enc.err = enc.w.Write(v)
}

func generateImageID(info *ImageInfoType) (string, error) {
	sha := sha1.New()
	enc := newIDEncoder(sha)
	enc.bytes(info.data)
	enc.bytes(info.smask)
	enc.u32(info.n)
	enc.f32(info.w)
	enc.f32(info.h)
	enc.str(info.cs)
	enc.bytes(info.pal)
	enc.bytes([]byte{info.bpc})
	enc.str(info.f)
	enc.str(info.dp)
	for _, v := range info.trns {
		enc.i64(int64(v))
	}
	enc.f32(info.scale)
	enc.f32(info.dpi)
	enc.str(info.i)

	return fmt.Sprintf("%x", sha.Sum(nil)), nil
}

// GobEncode encodes the receiving image to a byte slice.
func (info *ImageInfoType) GobEncode() (buf []byte, err error) {
	fields := []interface{}{
		info.data,
		info.smask,
		info.n,
		info.w,
		info.h,
		info.cs,
		info.pal,
		info.bpc,
		info.f,
		info.dp,
		info.trns,
		info.scale,
		info.dpi,
	}
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	for j := 0; j < len(fields) && err == nil; j++ {
		err = encoder.Encode(fields[j])
	}
	if err == nil {
		buf = w.Bytes()
	}
	return
}

// GobDecode decodes the specified byte buffer (generated by GobEncode) into
// the receiving image.
func (info *ImageInfoType) GobDecode(buf []byte) (err error) {
	fields := []interface{}{&info.data, &info.smask, &info.n, &info.w, &info.h,
		&info.cs, &info.pal, &info.bpc, &info.f, &info.dp, &info.trns, &info.scale, &info.dpi}
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	for j := 0; j < len(fields) && err == nil; j++ {
		err = decoder.Decode(fields[j])
	}

	info.i, err = generateImageID(info)
	return
}

// PointConvert returns the value of pt, expressed in points (1/72 inch), as a
// value expressed in the unit of measure specified in New(). Since font
// management in Scribe uses points, this method can help with line height
// calculations and other methods that require user units.
func (f *Scribe) PointConvert(pt float32) (u float32) {
	return pt / f.k
}

// PointToUnitConvert is an alias for PointConvert.
func (f *Scribe) PointToUnitConvert(pt float32) (u float32) {
	return pt / f.k
}

// UnitToPointConvert returns the value of u, expressed in the unit of measure
// specified in New(), as a value expressed in points (1/72 inch). Since font
// management in Scribe uses points, this method can help with setting font sizes
// based on the sizes of other non-font page elements.
func (f *Scribe) UnitToPointConvert(u float32) (pt float32) {
	return u * f.k
}

// Extent returns the width and height of the image in the units of the Scribe
// object.
func (info *ImageInfoType) Extent() (wd, ht float32) {
	return info.Width(), info.Height()
}

// Width returns the width of the image in the units of the Scribe object.
func (info *ImageInfoType) Width() float32 {
	return info.w / (info.scale * info.dpi / 72)
}

// Height returns the height of the image in the units of the Scribe object.
func (info *ImageInfoType) Height() float32 {
	return info.h / (info.scale * info.dpi / 72)
}

// SetDpi sets the dots per inch for an image. PNG images MAY have their dpi
// set automatically, if the image specifies it. DPI information is not
// currently available automatically for JPG and GIF images, so if it's
// important to you, you can set it here. It defaults to 72 dpi.
func (info *ImageInfoType) SetDpi(dpi float32) {
	info.dpi = dpi
}

type fontFileType struct {
	length1, length2 int64
	n                uint32
	embedded         bool
	content          []byte
	fontType         string
}

type linkType struct {
	x, y, wd, ht float32
	link         int    // Auto-generated internal link ID or...
	linkStr      string // ...application-provided external link string
}

type intLinkType struct {
	page int
	y    float32
}

// outlineType is used for a sidebar outline of bookmarks
type outlineType struct {
	text                                   string
	level, parent, first, last, next, prev int
	y                                      float32
	p                                      int
}

// InitType is used with NewCustom() to customize an Scribe instance.
// OrientationStr, UnitStr, SizeStr and FontDirStr correspond to the arguments
// accepted by New(). If the Wd and Ht fields of Size are each greater than
// zero, Size will be used to set the default page size rather than SizeStr. Wd
// and Ht are specified in the units of measure indicated by UnitStr.
type InitType struct {
	OrientationStr string
	UnitStr        string
	Size           PageSize
	FontSet        *FontSet
}

// FontLoader is used to read fonts (JSON font specification and zlib compressed font binaries)
// from arbitrary locations (e.g. files, zip files, embedded font resources).
//
// Open provides an io.Reader for the specified font file (.json or .z). The file name
// never includes a path. Open returns an error if the specified file cannot be opened.
type FontLoader interface {
	Open(name string) (io.Reader, error)
}

// OutputIntentSubtype any of the pre defined types below or a value defined by ISO 32000 extension.
type OutputIntentSubtype string

const (
	OutputIntent_GTS_PDFX  OutputIntentSubtype = "GTS_PDFX"
	OutputIntent_GTS_PDFA1 OutputIntentSubtype = "GTS_PDFA1"
	OutputIntent_GTS_PDFE1 OutputIntentSubtype = "GTS_PDFE1"
)

// OutputIntentType defines an output intent with name and ICC color profile.
type OutputIntentType struct {
	SubtypeIdent              OutputIntentSubtype
	OutputConditionIdentifier string
	Info                      string
	ICCProfile                []byte
}

// Scriber defines the interface used for various methods. It is implemented by the
// main FPDF instance as well as templates.
type Scriber interface {
	AddLayer(name string, visible bool) (layerID int)
	AddLink() int
	AddPage()
	AddPageFormat(orientationStr string, size PageSize)
	AddSpotColor(nameStr string, c, m, y, k byte)
	AliasNbPages(aliasStr string)
	ArcTo(x, y, rx, ry, degRotate, degStart, degEnd float32)
	Arc(x, y, rx, ry, degRotate, degStart, degEnd float32, styleStr string)
	BeginLayer(id int)
	Beziergon(points []PointType, styleStr string)
	Bookmark(txtStr string, level int, y float32)
	CellFormat(
		w, h float32,
		txtStr, borderStr string,
		ln int,
		alignStr string,
		fill bool,
		link int,
		linkStr string,
	)
	Cellf(w, h float32, fmtStr string, args ...interface{})
	Cell(w, h float32, txtStr string)
	Circle(x, y, r float32, styleStr string)
	ClearError()
	ClipCircle(x, y, r float32, outline bool)
	ClipEllipse(x, y, rx, ry float32, outline bool)
	ClipEnd()
	ClipPolygon(points []PointType, outline bool)
	ClipRect(x, y, w, h float32, outline bool)
	ClipRoundedRect(x, y, w, h, r float32, outline bool)
	ClipText(x, y float32, txtStr string, outline bool)
	Close()
	ClosePath()
	CreateTemplateCustom(
		id string,
		corner PointType,
		size PageSize,
		fn func(*Tpl),
	) Template
	CreateTemplate(id string, fn func(*Tpl)) Template
	CurveBezierCubicTo(cx0, cy0, cx1, cy1, x, y float32)
	CurveBezierCubic(
		x0, y0, cx0, cy0, cx1, cy1, x1, y1 float32,
		styleStr string,
	)
	CurveCubic(x0, y0, cx0, cy0, x1, y1, cx1, cy1 float32, styleStr string)
	CurveTo(cx, cy, x, y float32)
	Curve(x0, y0, cx, cy, x1, y1 float32, styleStr string)
	DrawPath(styleStr string)
	Ellipse(x, y, rx, ry, degRotate float32, styleStr string)
	EndLayer()
	Err() bool
	Error() error
	GetAlpha() (alpha float32, blendModeStr string)
	GetAuthor() string
	GetAutoPageBreak() (auto bool, margin float32)
	GetCatalogSort() bool
	GetCellMargin() float32
	GetCompression() bool
	GetConversionRatio() float32
	GetCreationDate() time.Time
	GetCreator() string
	GetDisplayMode() (zoomStr, layoutStr string)
	GetDrawColor() (uint8, uint8, uint8)
	GetDrawSpotColor() (name string, c, m, y, k byte)
	GetFillColor() (uint8, uint8, uint8)
	GetFillSpotColor() (name string, c, m, y, k byte)
	GetFontLoader() FontLoader
	GetFontLocation() string
	GetFontSize() (ptSize, unitSize float32)
	GetFontStyle() FontStyle
	GetImageInfo(imageStr string) (info *ImageInfoType)
	GetJavascript() string
	GetKeywords() string
	GetLang() string
	GetLineCapStyle() string
	GetLineJoinStyle() string
	GetLineWidth() float32
	GetMargins() (left, top, right, bottom float32)
	GetModificationDate() time.Time
	GetPageSize() (width, height float32)
	GetProducer() string
	GetStringWidth(s string) float32
	GetSubject() string
	GetTextColor() (uint8, uint8, uint8)
	GetTextSpotColor() (name string, c, m, y, k byte)
	GetTitle() string
	GetUnderlineThickness() float32
	GetWordSpacing() float32
	GetX() float32
	GetXmpMetadata() []byte
	GetXY() (float32, float32)
	GetY() float32
	Image(
		imageNameStr string,
		x, y, w, h float32,
		flow bool,
		tp string,
		link int,
		linkStr string,
	)
	ImageOptions(
		imageNameStr string,
		x, y, w, h float32,
		flow bool,
		options ImageOptions,
		link int,
		linkStr string,
	)
	ImageTypeFromMime(mimeStr string) (tp string)
	LinearGradient(
		x, y, w, h float32,
		r1, g1, b1, r2, g2, b2 uint8,
		x1, y1, x2, y2 float32,
	)
	LineTo(x, y float32)
	Line(x1, y1, x2, y2 float32)
	LinkString(x, y, w, h float32, linkStr string)
	Link(x, y, w, h float32, link int)
	Ln(h float32)
	MoveTo(x, y float32)
	MultiCell(w, h float32, txtStr, borderStr, alignStr string, fill bool)
	Ok() bool
	OpenLayerPane()
	OutputAndClose(w io.WriteCloser) error
	OutputFileAndClose(fileStr string) error
	Output(w io.Writer) error
	PageCount() int
	PageNo() int
	PageSize(pageNum int) (wd, ht float32, unitStr string)
	PointConvert(pt float32) (u float32)
	PointToUnitConvert(pt float32) (u float32)
	Polygon(points []PointType, styleStr string)
	RadialGradient(
		x, y, w, h float32,
		r1, g1, b1, r2, g2, b2 uint8,
		x1, y1, x2, y2, r float32,
	)
	RawWriteBuf(r io.Reader)
	RawWriteStr(str string)
	Rect(x, y, w, h float32, styleStr string)
	RegisterAlias(alias, replacement string)
	RegisterImage(fileStr, tp string) (info *ImageInfoType)
	RegisterImageOptions(
		fileStr string,
		options ImageOptions,
	) (info *ImageInfoType)
	RegisterImageOptionsReader(
		imgName string,
		options ImageOptions,
		r io.Reader,
	) (info *ImageInfoType)
	RegisterImageReader(imgName, tp string, r io.Reader) (info *ImageInfoType)
	SetAcceptPageBreakFunc(fnc func() bool)
	SetAlpha(alpha float32, blendModeStr string)
	SetAuthor(authorStr string, isUTF8 bool)
	SetAutoPageBreak(auto bool, margin float32)
	SetCatalogSort(flag bool)
	SetCellMargin(margin float32)
	SetCompression(compress bool)
	SetCreationDate(tm time.Time)
	SetCreator(creatorStr string, isUTF8 bool)
	SetDashPattern(dashArray []float32, dashPhase float32)
	SetDisplayMode(zoomStr, layoutStr string)
	SetLang(lang string)
	SetDrawColor(r, g, b uint8)
	SetDrawSpotColor(nameStr string, tint byte)
	SetError(err error)
	SetErrorf(fmtStr string, args ...interface{})
	SetFillColor(r, g, b uint8)
	SetFillSpotColor(nameStr string, tint byte)
	SetFont(id FontId, style FontStyle, size float32)
	SetFontLoader(loader FontLoader)
	SetFontLocation(fontDirStr string)
	SetFontSize(size float32)
	SetFontStyle(style FontStyle)
	SetFontUnitSize(size float32)
	SetFooterFunc(fnc func())
	SetFooterFuncLpi(fnc func(lastPage bool))
	SetHeaderFunc(fnc func())
	SetHeaderFuncMode(fnc func(), homeMode bool)
	SetHomeXY()
	SetJavascript(script string)
	SetKeywords(keywordsStr string, isUTF8 bool)
	SetLeftMargin(margin float32)
	SetLineCapStyle(styleStr string)
	SetLineJoinStyle(styleStr string)
	SetLineWidth(width float32)
	SetLink(link int, y float32, page int)
	SetMargins(left, top, right float32)
	SetPageBoxRec(t string, pb PageBox)
	SetPageBox(t string, x, y, wd, ht float32)
	SetPage(pageNum int)
	SetProtection(actionFlag byte, userPassStr, ownerPassStr string)
	SetRightMargin(margin float32)
	SetSubject(subjectStr string, isUTF8 bool)
	SetTextColor(r, g, b uint8)
	SetTextSpotColor(nameStr string, tint byte)
	SetTitle(titleStr string, isUTF8 bool)
	SetTopMargin(margin float32)
	SetUnderlineThickness(thickness float32)
	SetXmpMetadata(xmpStream []byte)
	SetX(x float32)
	SetXY(x, y float32)
	SetY(y float32)
	SplitLines(txt []byte, w float32) [][]byte
	String() string
	SVGBasicWrite(sb *SVGBasicType, scale float32)
	Text(x, y float32, txtStr string)
	TransformBegin()
	TransformEnd()
	TransformMirrorHorizontal(x float32)
	TransformMirrorLine(angle, x, y float32)
	TransformMirrorPoint(x, y float32)
	TransformMirrorVertical(y float32)
	TransformRotate(angle, x, y float32)
	TransformScale(scaleWd, scaleHt, x, y float32)
	TransformScaleX(scaleWd, x, y float32)
	TransformScaleXY(s, x, y float32)
	TransformScaleY(scaleHt, x, y float32)
	TransformSkew(angleX, angleY, x, y float32)
	TransformSkewX(angleX, x, y float32)
	TransformSkewY(angleY, x, y float32)
	Transform(tm TransformMatrix)
	TransformTranslate(tx, ty float32)
	TransformTranslateX(tx float32)
	TransformTranslateY(ty float32)
	UnicodeTranslatorFromDescriptor(cpStr string) (rep func(string) string)
	UnitToPointConvert(u float32) (pt float32)
	UseTemplateScaled(t Template, corner PointType, size PageSize)
	UseTemplate(t Template)
	WriteAligned(width, lineHeight float32, textStr, alignStr string)
	Writef(h float32, fmtStr string, args ...interface{})
	Write(h float32, txtStr string)
	WriteLinkID(h float32, displayStr string, linkID int)
	WriteLinkString(h float32, displayStr, targetStr string)
}

// PageBox defines the coordinates and extent of the various page box types
type PageBox struct {
	SizeType
	PointType
}

// Scribe is the principal structure for creating a single PDF document
type Scribe struct {
	measurementFont fontSpec // current measurement font info

	color struct {
		// Composite values of colors
		draw, fill, text colorType
	}

	protect protectType // document protection structure

	fmt struct {
		col bytes.Buffer // buffer used to build color strings.
		buf []byte       // buffer used to format numbers.
	}

	buffer fmtBuffer    // buffer holding in-memory PDF
	layer  layerRecType // manages optional layers in document

	attachments     []Attachment    // slice of content to embed globally
	blendList       []blendModeType // slice[idx] of alpha transparency modes, 1-based
	dashArray       []float32       // dash array
	diffs           []string        // array of encoding differences
	fontObjIds      []uint32
	fonts           *FontSet
	gradientList    []gradientType       // slice[idx] of gradient records
	links           []intLinkType        // array of internal links
	offsets         []uint32             // array of object offsets
	outlines        []outlineType        // array of outlines
	outputIntents   []OutputIntentType   // OutputIntents
	pageAttachments [][]annotationAttach // 1-based array of annotation for file attachments (per page)
	pageLinks       [][]linkType         // pageLinks[page][link], both 1-based
	pages           []*bytes.Buffer      // slice[page] of page content; 1-based
	usedRunes       []bitset.BitSet      // Runes added to the document with this font.
	xmp             []byte               // XMP metadata

	defOrientation  string // default orientation
	curOrientation  string // current orientation
	unitStr         string // unit of measure for all rendered objects except fonts
	fontpath        string // path containing fonts
	zoomMode        string // zoom display mode
	layoutMode      string // layout display mode
	producer        string // producer
	title           string // title
	subject         string // subject
	author          string // author
	lang            string // lang
	keywords        string // keywords
	creator         string // creator
	aliasNbPagesStr string // alias for total number of pages
	fontDirStr      string // location of font definition files
	blendMode       string // current blend mode

	creationDate time.Time  // override for document CreationDate value
	modDate      time.Time  // override for document ModDate value
	fontLoader   FontLoader // used to load font files from arbitrary locations

	err error // Set if error occurs during life cycle of instance

	capStyle      int // line cap style: butt 0, round 1, square 2
	clipNest      int // Number of active clipping contexts
	joinStyle     int // line segment join style: miter 0, round 1, bevel 2
	page          int // current page number
	transformNest int // Number of active transformation contexts

	javascript *string // JavaScript code to include in the PDF

	templates       map[string]Template          // templates used in this document
	templateObjects map[string]uint32            // template object IDs within this document
	importedObjs    map[string][]byte            // imported template objects (gofpdi)
	importedObjPos  map[string]map[uint32]string // imported template objects hashes and their positions (gofpdi)
	importedTplObjs map[string]string            // imported template names and IDs (hashed) (gofpdi)
	importedTplIDs  map[string]uint32            // imported template ids hash to object id int (gofpdi)
	defPageBoxes    map[string]PageBox           // default page size
	pageSizes       map[int]PageSize             // used for pages with non default sizes or orientations
	pageBoxes       map[int]map[string]PageBox   // used to define the crop, trim, bleed and art boxes
	images          map[string]*ImageInfoType    // array of used images
	aliasMap        map[string]string            // map of alias->replacement
	blendMap        map[string]int               // map into blendList
	spotColorMap    map[string]spotColorType     // Map of named ink-based colors

	acceptPageBreak func() bool // returns true to accept page break
	footerFnc       func()      // function provided by app and called to write footer
	footerFncLpi    func(bool)  // function provided by app and called to write footer with last page flag
	headerFnc       func()      // function provided by app and called to write header

	k                      float32 // scale factor (number of points in user unit)
	wPt, hPt               float32 // dimensions of current page in points
	w, h                   float32 // dimensions of current page in user unit
	lMargin                float32 // left margin
	tMargin                float32 // top margin
	rMargin                float32 // right margin
	bMargin                float32 // page break margin
	cMargin                float32 // cell margin
	x, y                   float32 // current position in user unit
	lasth                  float32 // height of last printed cell
	lineWidth              float32 // line width in user unit
	fontSizePt             float32 // current font size in points
	fontSize               float32 // current font size in user unit
	ws                     float32 // word spacing
	pageBreakTrigger       float32 // threshold used to trigger page breaks
	dashPhase              float32 // dash phase
	alpha                  float32 // current transpacency
	userUnderlineThickness float32 // A custom user underline thickness multiplier.

	n                  uint32 // current object number
	nJs                uint32 // JavaScript object number
	nXMP               uint32 // XMP object number
	outlineRoot        uint32 // root of outlines
	outputIntentStartN uint32 // Start object number for

	pdfVersion  pdfVersion // PDF version number
	currentFont FontId     // current font info

	curPageSize PageSize  // current page size
	defPageSize PageSize  // default page size
	fontStyle   FontStyle // current font style
	state       uint8     // current document state

	isRTL          bool // is is right to left mode enabled
	compress       bool // compression flag
	autoPageBreak  bool // automatic page breaking
	inHeader       bool // flag set when processing header
	headerHomeMode bool // set position to home after headerFnc is called
	inFooter       bool // flag set when processing footer
	catalogSort    bool // sort resource catalogs in document
	colorFlag      bool // indicates whether fill and text colors are different
}

type FontSet = ttf.FontSet

func (f *Scribe) font() *ttf.FontInfo {
	return f.fonts.Get(f.currentFont)
}

func (f *Scribe) currentFontKey() ttf.Key {
	return f.fonts.Key(f.currentFont)
}

const (
	pdfVers1_3 = pdfVersion(uint16(1)<<8 | uint16(3))
	pdfVers1_4 = pdfVersion(uint16(1)<<8 | uint16(4))
	pdfVers1_5 = pdfVersion(uint16(1)<<8 | uint16(5))
)

type pdfVersion uint16

func pdfVersionFrom(maj, min uint) pdfVersion {
	if min > 255 {
		panic(fmt.Errorf("scribe: invalid PDF version %d.%d", maj, min))
	}
	return pdfVersion(uint16(maj)<<8 | uint16(min))
}

func (v pdfVersion) String() string {
	maj := int64(byte(v >> 8))
	min := int64(byte(v))
	return strconv.FormatInt(maj, 10) + "." + strconv.FormatInt(min, 10)
}

type encType struct {
	uv   int
	name string
}

type encListType [256]encType

type fontBoxType struct {
	Xmin, Ymin, Xmax, Ymax int
}
