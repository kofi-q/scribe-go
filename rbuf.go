// Copyright ©2021 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package scribe

import (
	"encoding/binary"
	"io"
)

type rbuffer struct {
	src io.Reader
}

func (r *rbuffer) Read(p []byte) (int, error) {
	return r.src.Read(p)
}

func (r *rbuffer) ReadByte() (byte, error) {
	sink := [1]byte{}
	_, err := r.src.Read(sink[:1])
	return sink[0], err
}

func (r *rbuffer) u8() uint8 {
	b, err := r.ReadByte()
	if err != nil {
		// [TODO] Preserving previous behaviour for now - update to return err
		panic(err)
	}

	return b
}

func (r *rbuffer) u32() uint32 {
	buf := [4]byte{}
	if _, err := r.Read(buf[:]); err != nil {
		// [TODO] Preserving previous behaviour for now - update to return err
		panic(err)
	}

	return binary.BigEndian.Uint32(buf[:])
}

func (r *rbuffer) i32() int32 {
	return int32(r.u32())
}

func (r *rbuffer) Next(n int) []byte {
	buf := make([]byte, n)
	if _, err := r.Read(buf[:]); err != nil {
		// [TODO] Preserving previous behaviour for now - update to return err
		panic(err)
	}

	return buf[:]
}
