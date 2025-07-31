// Copyright ©2023 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*
 * Copyright (c) 2014 Kurt Jung (Gmail: kurt.w.jung)
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
	"cmp"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var pathCmdSub *strings.Replacer

func init() {
	// Handle permitted constructions like "100L200,230"
	pathCmdSub = strings.NewReplacer(",", " ",
		"L", " L ", "l", " l ",
		"C", " C ", "c", " c ",
		"M", " M ", "m", " m ",
		"H", " H ", "h", " h ",
		"V", " V ", "v", " v ",
		"Q", " Q ", "q", " q ",
		"Z", " Z ", "z", " z ")
}

// SVGBasicSegmentType describes a single curve or position segment
type SVGBasicSegmentType struct {
	Cmd byte // See http://www.w3.org/TR/SVG/paths.html for path command structure
	Arg [6]float32
}

func absolutizePath(segs []SVGBasicSegmentType) {
	var x, y float32
	var segPtr *SVGBasicSegmentType
	adjust := func(pos int, adjX, adjY float32) {
		segPtr.Arg[pos] += adjX
		segPtr.Arg[pos+1] += adjY
	}
	for j, seg := range segs {
		segPtr = &segs[j]
		if j == 0 && seg.Cmd == 'm' {
			segPtr.Cmd = 'M'
		}
		switch segPtr.Cmd {
		case 'M':
			x = seg.Arg[0]
			y = seg.Arg[1]
		case 'm':
			adjust(0, x, y)
			segPtr.Cmd = 'M'
			x = segPtr.Arg[0]
			y = segPtr.Arg[1]
		case 'L':
			x = seg.Arg[0]
			y = seg.Arg[1]
		case 'l':
			adjust(0, x, y)
			segPtr.Cmd = 'L'
			x = segPtr.Arg[0]
			y = segPtr.Arg[1]
		case 'C':
			x = seg.Arg[4]
			y = seg.Arg[5]
		case 'c':
			adjust(0, x, y)
			adjust(2, x, y)
			adjust(4, x, y)
			segPtr.Cmd = 'C'
			x = segPtr.Arg[4]
			y = segPtr.Arg[5]
		case 'Q':
			x = seg.Arg[2]
			y = seg.Arg[3]
		case 'q':
			adjust(0, x, y)
			adjust(2, x, y)
			segPtr.Cmd = 'Q'
			x = segPtr.Arg[2]
			y = segPtr.Arg[3]
		case 'H':
			x = seg.Arg[0]
		case 'h':
			segPtr.Arg[0] += x
			segPtr.Cmd = 'H'
			x += seg.Arg[0]
		case 'V':
			y = seg.Arg[0]
		case 'v':
			segPtr.Arg[0] += y
			segPtr.Cmd = 'V'
			y += seg.Arg[0]
		case 'z':
			segPtr.Cmd = 'Z'
		}
	}
}

func MustPathParse(pathStr string, adjustToPt float32) []SVGBasicSegmentType {
	segs, err := PathParse(pathStr, adjustToPt, "")
	if err != nil {
		panic(err)
	}

	return segs
}

func PathParse(
	pathStr string,
	adjustToPt float32,
	fill string,
) (segs []SVGBasicSegmentType, err error) {
	if fill != "" && fill[0] == '#' {
		color, err := hex.DecodeString(fill[1:])
		if err != nil {
			return segs, err
		}
		if len(color) != 3 {
			return segs, fmt.Errorf("invalid path fill: %s", fill)
		}

		segs = append(segs, SVGBasicSegmentType{
			Cmd: 'F',
			Arg: [6]float32{
				float32(color[0]),
				float32(color[1]),
				float32(color[2]),
			},
		})
	} else if strings.Contains(fill, "fill:") {
		segs = append(segs, SVGBasicSegmentType{
			Cmd: 'F',
			Arg: [6]float32{0, 0, 0},
		})
	}

	var seg SVGBasicSegmentType
	var j, argJ, argCount, prevArgCount int
	setup := func(n int) {
		// It is not strictly necessary to clear arguments, but result may be clearer
		// to caller
		for j := 0; j < len(seg.Arg); j++ {
			seg.Arg[j] = 0.0
		}
		argJ = 0
		argCount = n
		prevArgCount = n
	}
	var str string
	var c byte
	pathStr = pathCmdSub.Replace(pathStr)
	strList := strings.Fields(pathStr)
	count := len(strList)
	for j = 0; j < count && err == nil; j++ {
		str = strList[j]
		if argCount == 0 { // Look for path command or argument continuation
			c = str[0]
			if c == '-' || (c >= '0' && c <= '9') { // More arguments
				if j > 0 {
					setup(prevArgCount)
					// Repeat previous action
					if seg.Cmd == 'M' {
						seg.Cmd = 'L'
					} else if seg.Cmd == 'm' {
						seg.Cmd = 'l'
					}
				} else {
					err = fmt.Errorf("expecting SVG path command at first position, got %s", str)
				}
			}
		}
		if err == nil {
			if argCount == 0 {
				seg.Cmd = str[0]
				switch seg.Cmd {
				case 'M', 'm': // Absolute/relative moveto: x, y
					setup(2)
				case 'C',
					'c': // Absolute/relative Bézier curve: cx0, cy0, cx1, cy1, x1, y1
					setup(6)
				case 'H', 'h': // Absolute/relative horizontal line to: x
					setup(1)
				case 'L', 'l': // Absolute/relative lineto: x, y
					setup(2)
				case 'Q',
					'q': // Absolute/relative quadratic curve: x0, y0, x1, y1
					setup(4)
				case 'V', 'v': // Absolute/relative vertical line to: y
					setup(1)
				case 'Z', 'z': // closepath instruction (takes no arguments)
					segs = append(segs, seg)
				default:
					err = fmt.Errorf(
						"expecting SVG path command at position %d, got %s",
						j,
						str,
					)
				}
			} else {
				arg, err := strconv.ParseFloat(str, 32)
				seg.Arg[argJ] = float32(arg)
				if err == nil {
					seg.Arg[argJ] *= adjustToPt
					argJ++
					argCount--
					if argCount == 0 {
						segs = append(segs, seg)
					}
				}
			}
		}
	}
	if err == nil {
		if argCount == 0 {
			absolutizePath(segs)
		} else {
			err = fmt.Errorf("expecting additional (%d) numeric arguments", argCount)
		}
	}
	return
}

// SVGBasicType aggregates the information needed to describe a multi-segment
// basic vector image
type SVGBasicType struct {
	Wd, Ht   float32
	Segments [][]SVGBasicSegmentType
}

// parseFloatWithUnit parses a float and its unit, e.g. "42pt".
//
// The result is converted into pt values wich is the default document unit.
// parseFloatWithUnit returns the factor to apply to positions or distances to
// convert their values in point units.
func parseFloatWithUnit(val string, extent float32) (float32, float32, error) {
	var adjustToPt float32
	var removeUnitChar int
	var err error

	switch {
	case strings.HasSuffix(val, "%"):
		removeUnitChar = 1
		adjustToPt = extent / 100
	case strings.HasSuffix(val, "pt"):
		removeUnitChar = 2
		adjustToPt = 1.0
	case strings.HasSuffix(val, "in"):
		removeUnitChar = 2
		adjustToPt = 72.0
	case strings.HasSuffix(val, "mm"):
		removeUnitChar = 2
		adjustToPt = 72.0 / 25.4
	case strings.HasSuffix(val, "cm"):
		removeUnitChar = 2
		adjustToPt = 72.0 / 2.54
	case strings.HasSuffix(val, "pc"):
		removeUnitChar = 2
		adjustToPt = 12.0
	default: // default is pixel
		removeUnitChar = 0
		adjustToPt = 72.0 / 96.0
	}

	floatValue, err := strconv.ParseFloat(val[:len(val)-removeUnitChar], 32)
	if err != nil {
		return 0.0, 0.0, err
	}
	return float32(floatValue) * adjustToPt, adjustToPt, nil
}

// SVGBasicParse parses a simple scalable vector graphics (SVG) buffer into a
// descriptor. Only a small subset of the SVG standard, in particular the path
// information generated by jSignature, is supported. The returned path data
// includes only the commands 'M' (absolute moveto: x, y), 'L' (absolute
// lineto: x, y), 'C' (absolute cubic Bézier curve: cx0, cy0, cx1, cy1,
// x1,y1), 'Q' (absolute quadratic Bézier curve: x0, y0, x1, y1) and 'Z'
// (closepath). The document is returned with "pt" unit.
func SVGBasicParse(buf []byte) (sig SVGBasicType, err error) {
	type pathType struct {
		D     string `xml:"d,attr"`
		Fill  string `xml:"fill,attr"`
		Style string `xml:"style,attr"`
	}
	type rectType struct {
		Width  string  `xml:"width,attr"`
		Height string  `xml:"height,attr"`
		X      float32 `xml:"x,attr"`
		Y      float32 `xml:"y,attr"`
		Fill   string  `xml:"fill,attr"`
		Style  string  `xml:"style,attr"`
	}
	type srcType struct {
		Wd      string     `xml:"width,attr"`
		Ht      string     `xml:"height,attr"`
		Paths   []pathType `xml:"path"`
		Rects   []rectType `xml:"rect"`
		ViewBox string     `xml:"viewBox,attr"`
	}
	var src srcType
	var wd float32
	var ht float32
	var adjustToPt float32
	var viewBox svgViewBox

	start := bytes.Index(buf, []byte("<svg"))
	err = xml.Unmarshal(buf[start:], &src)
	if err != nil {
		return
	}

	viewBox, err = svgParseViewbox(src.ViewBox)
	if err != nil {
		return
	}

	if viewBox.width != 0 {
		wd, ht, adjustToPt = viewBox.width, viewBox.height, viewBox.unitScale
	} else {
		if src.Wd == "" || src.Ht == "" {
			return sig, fmt.Errorf("invalid SVG - missing/incomplete size info")
		}

		wd, adjustToPt, err = parseFloatWithUnit(src.Wd, viewBox.width)
		if err != nil {
			return
		}

		ht, _, err = parseFloatWithUnit(src.Ht, viewBox.height)
		if err != nil {
			return
		}
	}

	if wd == 0 || ht == 0 {
		return sig, fmt.Errorf(
			"unacceptable values for basic SVG extent: %.2f x %.2f", wd, ht,
		)
	}

	sig.Wd, sig.Ht = wd, ht
	var segs []SVGBasicSegmentType
	for _, rect := range src.Rects {
		widthRect, _, err := parseFloatWithUnit(rect.Width, viewBox.width)
		if err != nil {
			return sig, err
		}
		heightRect, _, err := parseFloatWithUnit(rect.Height, viewBox.height)
		if err != nil {
			return sig, err
		}

		segs = nil
		if rect.Fill != "" && rect.Fill[0] == '#' {
			color, err := hex.DecodeString(rect.Fill[1:])
			if err != nil {
				return sig, err
			}
			if len(color) != 3 {
				return sig, fmt.Errorf("invalid rect fill: %s", rect.Fill)
			}

			fmt.Println("rect color:", color)
			segs = append(segs, SVGBasicSegmentType{
				Cmd: 'F',
				Arg: [6]float32{
					float32(color[0]),
					float32(color[1]),
					float32(color[2]),
				},
			})
		}
		segs = append(segs, SVGBasicSegmentType{
			Cmd: 'M',
			Arg: [6]float32{rect.X * adjustToPt, rect.Y * adjustToPt},
		})
		segs = append(segs, SVGBasicSegmentType{
			Cmd: 'L',
			Arg: [6]float32{
				(rect.X * adjustToPt) + widthRect,
				rect.Y * adjustToPt,
			},
		})
		segs = append(segs, SVGBasicSegmentType{
			Cmd: 'L',
			Arg: [6]float32{
				(rect.X * adjustToPt) + widthRect,
				(rect.Y * adjustToPt) + heightRect,
			},
		})
		segs = append(segs, SVGBasicSegmentType{
			Cmd: 'L',
			Arg: [6]float32{
				rect.X * adjustToPt,
				(rect.Y * adjustToPt) + heightRect,
			},
		})
		segs = append(segs, SVGBasicSegmentType{
			Cmd: 'Z',
		})
		sig.Segments = append(sig.Segments, segs)
	}
	for _, path := range src.Paths {
		segs, err = PathParse(
			path.D,
			adjustToPt,
			cmp.Or(path.Fill, path.Style),
		)
		if err == nil {
			sig.Segments = append(sig.Segments, segs)
		}
	}

	return
}

type svgViewBox struct {
	height    float32
	unitScale float32
	width     float32
	x         float32
	y         float32
}

func svgParseViewbox(src string) (box svgViewBox, err error) {
	if src == "" {
		return
	}

	vb := strings.Split(src, " ")
	if len(vb) != 4 {
		return
	}

	box.width, box.unitScale, err = parseFloatWithUnit(vb[0], 0)
	if err != nil {
		return
	}
	box.height, _, err = parseFloatWithUnit(vb[1], 0)
	if err != nil {
		return
	}
	box.width, _, err = parseFloatWithUnit(vb[2], 0)
	if err != nil {
		return
	}
	box.height, _, err = parseFloatWithUnit(vb[3], 0)
	if err != nil {
		return
	}

	return
}

// SVGBasicFileParse parses a simple scalable vector graphics (SVG) file into a
// basic descriptor. The SVGBasicWrite() example demonstrates this method.
func SVGBasicFileParse(svgFileStr string) (sig SVGBasicType, err error) {
	var buf []byte
	buf, err = os.ReadFile(svgFileStr)
	if err == nil {
		sig, err = SVGBasicParse(buf)
	}
	return
}
