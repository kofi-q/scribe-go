package scribe

import (
	"bytes"
	"strconv"
)

type xobject struct {
	name string

	buf  *bytes.Buffer
	size SizeType

	id uint32
}

type XobjectId struct {
	index uint32
}

func (f *Scribe) XobjectCreate(
	name string,
	size SizeType,
	bufSizeInit uint32,
	fn func(*Tpl),
) XobjectId {
	id := XobjectId{index: uint32(len(f.xobjects))}
	f.xobjects = append(f.xobjects, xobject{
		name: name,
		size: size,
	})
	f.xobjects[id.index].buf = bytes.NewBuffer(make([]byte, 0, bufSizeInit))

	f.xobjectsUsed = append(f.xobjectsUsed, false)

	statePrev := f.state
	f.state = 4

	x := f.x
	y := f.y
	w := f.w
	h := f.h

	f.x = 0
	f.y = 0
	f.w = size.Wd
	f.h = size.Ht

	f.xobjIndex = id.index
	fn(&Tpl{f})

	f.x = x
	f.y = y
	f.h = h
	f.w = w

	f.state = statePrev

	return id
}

func (f *Scribe) XobjectUse(x XobjectId, pos PointType) {
	f.XobjectUseScaled(x, pos, f.xobjects[x.index].size)
}

func (f *Scribe) XobjectUseScaled(
	id XobjectId,
	pos PointType,
	size SizeType,
) {
	xobj := f.xobjects[id.index]
	scaleX := size.Wd / xobj.size.Wd
	scaleY := size.Ht / xobj.size.Ht
	tx := pos.X * f.k
	ty := (f.curPageSize.Ht - pos.Y - size.Ht) * f.k

	f.put("q ")
	f.put(f.fmtF64(scaleX, -1))
	f.put(" 0 0 ")
	f.put(f.fmtF64(scaleY, -1))
	f.put(" ")
	f.put(f.fmtF64(tx, -1))
	f.put(" ")
	f.put(f.fmtF64(ty, -1))
	f.out(" cm")

	f.put("/TPL")
	f.put(xobj.name)
	f.out(" Do Q")

	f.xobjectsUsed[id.index] = true
}

func (f *Scribe) putXobjects() {
	filter := ""
	if f.compress {
		filter = "/Filter /FlateDecode "
	}

	for i := range f.xobjects {
		if !f.xobjectsUsed[i] {
			continue
		}

		f.newobj()
		f.xobjects[i].id = f.n
		f.put("<<")
		f.put(filter)
		f.out("/Type /XObject")
		f.out("/Subtype /Form")
		f.out("/Formtype 1")
		f.put("/BBox [0 0 ")
		f.put(f.fmtF64((f.xobjects[i].size.Wd), -1))
		f.put(" ")
		f.put(f.fmtF64((f.xobjects[i].size.Ht), -1))
		f.out("]")

		//  Write the template's byte stream
		buffer := f.xobjects[i].buf.Bytes()
		var mem *membuffer
		if f.compress {
			mem = xmem.compress(buffer)
			buffer = mem.bytes()
		}
		f.put("/Length ")
		f.put(strconv.Itoa(len(buffer)))
		f.out(">>")
		f.putstream(buffer)
		f.out("endobj")
		if mem != nil {
			mem.release()
		}
	}
}
