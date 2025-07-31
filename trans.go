// Copyright ©2023 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package scribe

import (
	"fmt"
	"math"
)

// Routines in this file are translated from the work of Moritz Wagner and
// Andreas Würmser.

// TransformMatrix is used for generalized transformations of text, drawings
// and images.
type TransformMatrix struct {
	A, B, C, D, E, F float32
}

// TransformBegin sets up a transformation context for subsequent text,
// drawings and images. The typical usage is to immediately follow a call to
// this method with a call to one or more of the transformation methods such as
// TransformScale(), TransformSkew(), etc. This is followed by text, drawing or
// image output and finally a call to TransformEnd(). All transformation
// contexts must be properly ended prior to outputting the document.
func (f *Scribe) TransformBegin() {
	f.transformNest++
	f.out("q")
}

// TransformScaleX scales the width of the following text, drawings and images.
// scaleWd is the percentage scaling factor. (x, y) is center of scaling.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformScaleX(scaleWd, x, y float32) {
	f.TransformScale(scaleWd, 100, x, y)
}

// TransformScaleY scales the height of the following text, drawings and
// images. scaleHt is the percentage scaling factor. (x, y) is center of
// scaling.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformScaleY(scaleHt, x, y float32) {
	f.TransformScale(100, scaleHt, x, y)
}

// TransformScaleXY uniformly scales the width and height of the following
// text, drawings and images. s is the percentage scaling factor for both width
// and height. (x, y) is center of scaling.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformScaleXY(s, x, y float32) {
	f.TransformScale(s, s, x, y)
}

// TransformScale generally scales the following text, drawings and images.
// scaleWd and scaleHt are the percentage scaling factors for width and height.
// (x, y) is center of scaling.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformScale(scaleWd, scaleHt, x, y float32) {
	if scaleWd == 0 || scaleHt == 0 {
		f.err = fmt.Errorf("scale factor cannot be zero")
		return
	}
	y = (f.h - y) * f.k
	x *= f.k
	scaleWd /= 100
	scaleHt /= 100
	f.Transform(TransformMatrix{scaleWd, 0, 0,
		scaleHt, x * (1 - scaleWd), y * (1 - scaleHt)})
}

// TransformMirrorHorizontal horizontally mirrors the following text, drawings
// and images. x is the axis of reflection.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformMirrorHorizontal(x float32) {
	f.TransformScale(-100, 100, x, f.y)
}

// TransformMirrorVertical vertically mirrors the following text, drawings and
// images. y is the axis of reflection.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformMirrorVertical(y float32) {
	f.TransformScale(100, -100, f.x, y)
}

// TransformMirrorPoint symmetrically mirrors the following text, drawings and
// images on the point specified by (x, y).
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformMirrorPoint(x, y float32) {
	f.TransformScale(-100, -100, x, y)
}

// TransformMirrorLine symmetrically mirrors the following text, drawings and
// images on the line defined by angle and the point (x, y). angles is
// specified in degrees and measured counter-clockwise from the 3 o'clock
// position.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformMirrorLine(angle, x, y float32) {
	f.TransformScale(-100, 100, x, y)
	f.TransformRotate(-2*(angle-90), x, y)
}

// TransformTranslateX moves the following text, drawings and images
// horizontally by the amount specified by tx.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformTranslateX(tx float32) {
	f.TransformTranslate(tx, 0)
}

// TransformTranslateY moves the following text, drawings and images vertically
// by the amount specified by ty.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformTranslateY(ty float32) {
	f.TransformTranslate(0, ty)
}

// TransformTranslate moves the following text, drawings and images
// horizontally and vertically by the amounts specified by tx and ty.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformTranslate(tx, ty float32) {
	f.Transform(TransformMatrix{1, 0, 0, 1, tx * f.k, -ty * f.k})
}

// TransformRotate rotates the following text, drawings and images around the
// center point (x, y). angle is specified in degrees and measured
// counter-clockwise from the 3 o'clock position.
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformRotate(angle, x, y float32) {
	y = (f.h - y) * f.k
	x *= f.k
	angle = angle * math.Pi / 180
	var tm TransformMatrix
	tm.A = float32(math.Cos(float64(angle)))
	tm.B = float32(math.Sin(float64(angle)))
	tm.C = -tm.B
	tm.D = tm.A
	tm.E = x + tm.B*y - tm.A*x
	tm.F = y - tm.A*y - tm.B*x
	f.Transform(tm)
}

// TransformSkewX horizontally skews the following text, drawings and images
// keeping the point (x, y) stationary. angleX ranges from -90 degrees (skew to
// the left) to 90 degrees (skew to the right).
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformSkewX(angleX, x, y float32) {
	f.TransformSkew(angleX, 0, x, y)
}

// TransformSkewY vertically skews the following text, drawings and images
// keeping the point (x, y) stationary. angleY ranges from -90 degrees (skew to
// the bottom) to 90 degrees (skew to the top).
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformSkewY(angleY, x, y float32) {
	f.TransformSkew(0, angleY, x, y)
}

// TransformSkew generally skews the following text, drawings and images
// keeping the point (x, y) stationary. angleX ranges from -90 degrees (skew to
// the left) to 90 degrees (skew to the right). angleY ranges from -90 degrees
// (skew to the bottom) to 90 degrees (skew to the top).
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformSkew(angleX, angleY, x, y float32) {
	if angleX <= -90 || angleX >= 90 || angleY <= -90 || angleY >= 90 {
		f.err = fmt.Errorf("skew values must be between -90° and 90°")
		return
	}
	x *= f.k
	y = (f.h - y) * f.k
	var tm TransformMatrix
	tm.A = 1
	tm.B = float32(math.Tan(float64(angleY) * math.Pi / 180))
	tm.C = float32(math.Tan(float64(angleX) * math.Pi / 180))
	tm.D = 1
	tm.E = -tm.C * y
	tm.F = -tm.B * x
	f.Transform(tm)
}

// Transform generally transforms the following text, drawings and images
// according to the specified matrix. It is typically easier to use the various
// methods such as TransformRotate() and TransformMirrorVertical() instead.
func (f *Scribe) Transform(tm TransformMatrix) {
	if f.transformNest > 0 {
		f.outf("%.5f %.5f %.5f %.5f %.5f %.5f cm",
			tm.A, tm.B, tm.C, tm.D, tm.E, tm.F)
	} else if f.err == nil {
		f.err = fmt.Errorf("transformation context is not active")
	}
}

// TransformEnd applies a transformation that was begun with a call to TransformBegin().
//
// The TransformBegin() example demonstrates this method.
func (f *Scribe) TransformEnd() {
	if f.transformNest > 0 {
		f.transformNest--
		f.out("Q")
	} else {
		f.err = fmt.Errorf("error attempting to end transformation operation out of sequence")
	}
}
