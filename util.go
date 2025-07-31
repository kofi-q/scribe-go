// Copyright ©2023 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*
 * Copyright (c) 2013 Kurt Jung (Gmail: kurt.w.jung)
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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func must(n int, err error) int {
	if err != nil {
		panic(err)
	}

	return n
}

func must64(n int64, err error) int64 {
	if err != nil {
		panic(err)
	}

	return n
}

func sprintf(fmtStr string, args ...interface{}) string {
	return fmt.Sprintf(fmtStr, args...)
}

// utf8toutf16 converts UTF-8 to UTF-16BE; from http://www.fpdf.org/
func utf8toutf16(s string, withBOM ...bool) string {
	bom := true
	if len(withBOM) > 0 {
		bom = withBOM[0]
	}
	res := make([]byte, 0, 8)
	if bom {
		res = append(res, 0xFE, 0xFF)
	}
	nb := len(s)
	i := 0
	for i < nb {
		c1 := byte(s[i])
		i++
		switch {
		case c1 >= 224:
			// [FIXME] We go out of bounds here for some Chinese text - not
			// sure why yet.
			if i+2 > nb {
				continue
			}

			// 3-byte character
			c2 := byte(s[i])
			i++
			c3 := byte(s[i])
			i++
			res = append(res, ((c1&0x0F)<<4)+((c2&0x3C)>>2),
				((c2&0x03)<<6)+(c3&0x3F))
		case c1 >= 192:
			// 2-byte character
			c2 := byte(s[i])
			i++
			res = append(res, ((c1 & 0x1C) >> 2),
				((c1&0x03)<<6)+(c2&0x3F))
		default:
			// Single-byte character
			res = append(res, 0, c1)
		}
	}
	return string(res)
}

// intIf returns a if cnd is true, otherwise b
func intIf(cnd bool, a, b int) int {
	if cnd {
		return a
	}
	return b
}

// strIf returns aStr if cnd is true, otherwise bStr
func strIf(cnd bool, aStr, bStr string) string {
	if cnd {
		return aStr
	}
	return bStr
}

// doNothing returns the passed string with no translation.
func doNothing(s string) string {
	return s
}

// Dump the internals of the specified values
// func dump(fileStr string, a ...interface{}) {
// 	fl, err := os.OpenFile(fileStr, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
// 	if err == nil {
// 		fmt.Fprintf(fl, "----------------\n")
// 		spew.Fdump(fl, a...)
// 		fl.Close()
// 	}
// }

func repClosure(m map[rune]byte) func(string) string {
	var buf bytes.Buffer
	return func(str string) string {
		var ch byte
		var ok bool
		buf.Truncate(0)
		for _, r := range str {
			if r < 0x80 {
				ch = byte(r)
			} else {
				ch, ok = m[r]
				if !ok {
					ch = byte('.')
				}
			}
			buf.WriteByte(ch)
		}
		return buf.String()
	}
}

// UnicodeTranslator returns a function that can be used to translate, where
// possible, utf-8 strings to a form that is compatible with the specified code
// page. The returned function accepts a string and returns a string.
//
// r is a reader that should read a buffer made up of content lines that
// pertain to the code page of interest. Each line is made up of three
// whitespace separated fields. The first begins with "!" and is followed by
// two hexadecimal digits that identify the glyph position in the code page of
// interest. The second field begins with "U+" and is followed by the unicode
// code point value. The third is the glyph name. A number of these code page
// map files are packaged with the gfpdf library in the font directory.
//
// An error occurs only if a line is read that does not conform to the expected
// format. In this case, the returned function is valid but does not perform
// any rune translation.
func UnicodeTranslator(r io.Reader) (f func(string) string, err error) {
	m := make(map[rune]byte)
	var uPos, cPos uint32
	var lineStr, nameStr string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		lineStr = sc.Text()
		lineStr = strings.TrimSpace(lineStr)
		if len(lineStr) > 0 {
			_, err = fmt.Sscanf(
				lineStr,
				"!%2X U+%4X %s",
				&cPos,
				&uPos,
				&nameStr,
			)
			if err == nil {
				if cPos >= 0x80 {
					m[rune(uPos)] = byte(cPos)
				}
			}
		}
	}
	if err == nil {
		f = repClosure(m)
	} else {
		f = doNothing
	}
	return
}

// UnicodeTranslatorFromFile returns a function that can be used to translate,
// where possible, utf-8 strings to a form that is compatible with the
// specified code page. See UnicodeTranslator for more details.
//
// fileStr identifies a font descriptor file that maps glyph positions to names.
//
// If an error occurs reading the file, the returned function is valid but does
// not perform any rune translation.
func UnicodeTranslatorFromFile(
	fileStr string,
) (f func(string) string, err error) {
	var fl *os.File
	fl, err = os.Open(fileStr)
	if err == nil {
		f, err = UnicodeTranslator(fl)
		fl.Close()
	} else {
		f = doNothing
	}
	return
}

// UnicodeTranslatorFromDescriptor returns a function that can be used to
// translate, where possible, utf-8 strings to a form that is compatible with
// the specified code page. See UnicodeTranslator for more details.
//
// cpStr identifies a code page. A descriptor file in the font directory, set
// with the fontDirStr argument in the call to New(), should have this name
// plus the extension ".map". If cpStr is empty, it will be replaced with
// "cp1252", the scribe-go code page default.
//
// If an error occurs reading the descriptor, the returned function is valid
// but does not perform any rune translation.
//
// The CellFormat_codepage example demonstrates this method.
func (f *Scribe) UnicodeTranslatorFromDescriptor(
	cpStr string,
) (rep func(string) string) {
	if f.err == nil {
		if len(cpStr) == 0 {
			cpStr = "cp1252"
		}
		emb, err := embFS.Open("font_embed/" + cpStr + ".map")
		if err == nil {
			defer emb.Close()
			rep, f.err = UnicodeTranslator(emb)
		} else {
			rep, f.err = UnicodeTranslatorFromFile(filepath.Join(f.fontpath, cpStr) + ".map")
		}
	} else {
		rep = doNothing
	}
	return
}

// Transform moves a point by given X, Y offset
func (p *PointType) Transform(x, y float32) PointType {
	return PointType{p.X + x, p.Y + y}
}

// Orientation returns the orientation of a given size:
// "P" for portrait, "L" for landscape
func (s *SizeType) Orientation() string {
	if s == nil || s.Ht == s.Wd {
		return ""
	}
	if s.Wd > s.Ht {
		return "L"
	}
	return "P"
}

// ScaleBy expands a size by a certain factor
func (s *SizeType) ScaleBy(factor float32) SizeType {
	return SizeType{s.Wd * factor, s.Ht * factor}
}

// ScaleToWidth adjusts the height of a size to match the given width
func (s *SizeType) ScaleToWidth(width float32) SizeType {
	height := s.Ht * width / s.Wd
	return SizeType{width, height}
}

// ScaleToHeight adjusts the width of a size to match the given height
func (s *SizeType) ScaleToHeight(height float32) SizeType {
	width := s.Wd * height / s.Ht
	return SizeType{width, height}
}

// The untypedKeyMap structure and its methods are copyrighted 2019 by Arteom Korotkiy (Gmail: arteomkorotkiy).
// Imitation of untyped Map Array
type untypedKeyMap struct {
	keySet   []interface{}
	valueSet []int
}

func isChinese(char rune) bool {
	// chinese unicode: 4e00-9fa5
	if char >= rune(0x4e00) && char <= rune(0x9fa5) {
		return true
	}
	return false
}

// Condition font family string to PDF name compliance. See section 5.3 (Names)
// in https://resources.infosecinstitute.com/pdf-file-format-basic-structure/
func fontFamilyEscape(familyStr string) (escStr string) {
	escStr = strings.Replace(familyStr, " ", "#20", -1)
	// Additional replacements can take place here
	return
}
