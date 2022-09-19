package util

import (
	"errors"
	"io"
)

type sliceBuf struct {
	sBufArray *bufArray

	rIdx int
	wIdx int

	offset int
	dLen   int
}

func createSliceBufArray(src *bufArray, from, to int) *bufArray {
	offset, srcBA := findArray(src, from)
	sliceLen := to - from

	head := &bufArray{}

	l := len(srcBA.data) - offset
	if l <= sliceLen {
		head.data = srcBA.data[offset:]
		sliceLen -= l
	} else {
		head.data = srcBA.data[offset : offset+sliceLen]
		return head
	}

	head.next = &bufArray{}
	pBA := head.next
	for {
		srcBA = src.next
		l := len(srcBA.data)

		if l <= sliceLen {
			pBA.data = srcBA.data
			sliceLen -= l
		} else {
			pBA.data = srcBA.data[:sliceLen]
			sliceLen = 0
			break
		}

		if sliceLen < 1 {
			break
		}

		if src.next == nil {
			panic(errors.New("out of bound on create slice"))
		}

		pBA.next = &bufArray{}
		pBA = pBA.next
	}

	return head
}

func Slice(src BaseBuf, from, to int) BaseBuf {
	if from < 0 || to < 0 {
		panic(OutOfBufError("'from' and 'to' of slice must be a positive"))
	}

	size := src.Size()
	if from > size || to > size {
		panic(OutOfBufError("'from' or 'to' of slice out of Buffer"))
	}

	if from > to {
		temp := from
		from = to
		to = temp
	}

	return &sliceBuf{
		sBufArray: createSliceBufArray(src.Array(), from, to),
		rIdx:      0,
		wIdx:      0,
		offset:    from,
		dLen:      to - from,
	}
}

func (buf *sliceBuf) Size() int {
	return buf.dLen
}

func (buf *sliceBuf) Array() *bufArray {
	return buf.sBufArray
}

func (buf *sliceBuf) Copy() BaseBuf {
	cp := make([]byte, buf.Size())
	pBA := buf.sBufArray
	for cpOffset := 0; cpOffset < buf.Size(); {
		copy(cp, pBA.data)
		pBA = pBA.next
	}
	return NewBufWithArray(cp)
}

func (buf *sliceBuf) WriteIndex(idx int) error {
	return writeIndex(&buf.wIdx, idx, buf)
}

func (buf *sliceBuf) ReadIndex(idx int) error {
	return readIndex(&buf.rIdx, idx, buf)
}

func (buf *sliceBuf) GetWriteIndex() int {
	return buf.wIdx
}

func (buf *sliceBuf) GetReadIndex() int {
	return buf.rIdx
}

func (buf *sliceBuf) WriteableBytes() int {
	return buf.Size() - buf.wIdx
}

func (buf *sliceBuf) ReadableBytes() int {
	return buf.wIdx - buf.rIdx
}

func (buf *sliceBuf) DiscardReadBytes() {
	if buf.rIdx == 0 {
		return
	}

	transforWithBound(buf, 0, buf.Size(), buf, buf.rIdx, buf.wIdx)
}

func (buf *sliceBuf) GetByte(i int) (b byte, err error) {
	err = checkIndex(i, buf)
	if err != nil {
		return 0, err
	}

	offset, array := findArray(buf.sBufArray, i)
	return array.data[offset], nil
}

func (buf *sliceBuf) GetBytes(i int, dst []byte) error {
	return bufGetBytes(buf, i, dst)
}

func (buf *sliceBuf) SetByte(i int, b byte) error {
	err := checkIndex(i, buf)
	if err != nil {
		return err
	}

	offset, array := findArray(buf.Array(), i)
	array.data[offset] = b
	return nil
}

func (buf *sliceBuf) SetBytes(i int, src []byte) error {
	return bufSetBytes(buf, i, src)
}

func (buf *sliceBuf) WriteTo(w io.Writer) error {
	return buf.WriteToWithLen(buf.WriteableBytes(), w)
}

func (buf *sliceBuf) WriteToWithLen(len int, w io.Writer) error {
	return bufWriteTo(buf, len, w)
}

func (buf *sliceBuf) WriteByte(b byte) error {
	if buf.ReadableBytes() < 1 {
		return ErrInSufficientBytes
	}

	buf.SetByte(buf.wIdx, b)
	buf.wIdx += 1
	return nil
}

func (buf *sliceBuf) WriteBytes(b []byte) (n int, err error) {
	return bufWriteBytes(buf, b)
}

func (buf *sliceBuf) WriteString(s string) (n int, err error) {
	return unsafeWriteString(s, buf)
}

func (buf *sliceBuf) ReadFrom(r io.Reader) (n int, err error) {
	return buf.ReadFromWithLen(buf.WriteableBytes(), r)
}

func (buf *sliceBuf) ReadFromWithLen(len int, r io.Reader) (n int, err error) {
	return bufReadFrom(buf, len, r)
}

func (buf *sliceBuf) ReadByte() (rb byte, err error) {
	if buf.ReadableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	rb, err = buf.GetByte(buf.rIdx)
	buf.rIdx += 1
	return
}

func (buf *sliceBuf) ReadBytes(b []byte) (n int, err error) {
	return bufReadBytes(buf, b)
}

func (buf *sliceBuf) ReadString(len int) (rs string, err error) {
	return unsafeReadString(len, buf)
}

type duplicateBuf struct {
	src BaseBuf

	rIdx int
	wIdx int
}

func Duplicate(src BaseBuf) BaseBuf {
	switch t := src.(type) {
	case *duplicateBuf:
		return &duplicateBuf{
			src: t.src,
		}
	default:
		return &duplicateBuf{
			src: src,
		}
	}
}

func (buf *duplicateBuf) Size() int {
	return buf.src.Size()
}

func (buf *duplicateBuf) Array() *bufArray {
	return buf.src.Array()
}

func (buf *duplicateBuf) Copy() BaseBuf {
	return buf.src.Copy()
}

func (buf *duplicateBuf) WriteIndex(idx int) error {
	err := checkIndex(idx, buf)
	if err != nil {
		return err
	}

	buf.wIdx = idx
	return nil
}

func (buf *duplicateBuf) ReadIndex(idx int) error {
	err := checkIndex(idx, buf)
	if err != nil {
		return err
	}

	if idx > buf.wIdx {
		return OutOfBufError("idx must be less than or equal to writeIndex")
	}

	buf.rIdx = idx
	return nil
}

func (buf *duplicateBuf) GetWriteIndex() int {
	return buf.wIdx
}

func (buf *duplicateBuf) GetReadIndex() int {
	return buf.rIdx
}

func (buf *duplicateBuf) WriteableBytes() int {
	return buf.Size() - buf.wIdx
}

func (buf *duplicateBuf) ReadableBytes() int {
	return buf.wIdx - buf.rIdx
}

func (buf *duplicateBuf) DiscardReadBytes() {
	if buf.rIdx > 0 {
		if buf.rIdx != buf.wIdx {
			transforWithBound(buf, 0, buf.Size(), buf, buf.rIdx, buf.wIdx)
			buf.wIdx -= buf.rIdx
			buf.rIdx = 0
		} else {
			buf.wIdx = 0
			buf.rIdx = 0
		}
	}
}

func (buf *duplicateBuf) GetByte(i int) (b byte, err error) {
	return buf.src.GetByte(i)
}

func (buf *duplicateBuf) GetBytes(i int, dst []byte) error {
	return buf.src.GetBytes(i, dst)
}

func (buf *duplicateBuf) SetByte(i int, b byte) error {
	return buf.src.SetByte(i, b)
}

func (buf *duplicateBuf) SetBytes(i int, src []byte) error {
	return buf.src.SetBytes(i, src)
}

func (buf *duplicateBuf) WriteTo(w io.Writer) error {
	return buf.WriteToWithLen(buf.ReadableBytes(), w)
}

func (buf *duplicateBuf) WriteToWithLen(len int, w io.Writer) error {
	return bufWriteTo(buf, len, w)
}

func (buf *duplicateBuf) WriteByte(b byte) error {
	if buf.WriteableBytes() < 1 {
		return ErrInSufficientBytes
	}

	buf.SetByte(buf.wIdx, b)
	buf.wIdx += 1
	return nil
}

func (buf *duplicateBuf) WriteBytes(b []byte) (n int, err error) {
	return bufReadBytes(buf, b)
}

func (buf *duplicateBuf) WriteString(s string) (n int, err error) {
	return unsafeWriteString(s, buf)
}

func (buf *duplicateBuf) ReadFrom(r io.Reader) (n int, err error) {
	return buf.ReadFromWithLen(buf.WriteableBytes(), r)
}

func (buf *duplicateBuf) ReadFromWithLen(len int, r io.Reader) (n int, err error) {
	return bufReadFrom(buf, len, r)
}

func (buf *duplicateBuf) ReadByte() (rb byte, err error) {
	if buf.ReadableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	rb, err = buf.GetByte(buf.rIdx)
	buf.rIdx += 1
	return
}

func (buf *duplicateBuf) ReadBytes(b []byte) (n int, err error) {
	return bufReadBytes(buf, b)
}

func (buf *duplicateBuf) ReadString(len int) (rs string, err error) {
	return unsafeReadString(len, buf)
}

type bufComponent struct {
	sliced BaseBuf

	sIdx int
	eIdx int
}

func (bc *bufComponent) length() int {
	return bc.eIdx - bc.sIdx
}

func (bc *bufComponent) index(i int) int {
	return i - bc.sIdx
}

func (bc *bufComponent) reOffset(newOffset int) {
	move := newOffset - bc.eIdx
	bc.sIdx = newOffset
	bc.eIdx += move
}

const (
	defaultCompoentMaxSize = 8
)

type compositeBuf struct {
	child          []*bufComponent
	childArrays    *bufArray
	componentCount int
	maxSize        int

	rIdx int
	wIdx int

	lastComp *bufComponent
}

func CompositeBuffer(buffers ...BaseBuf) *compositeBuf {
	return _NewCompositeBuf(defaultCompoentMaxSize, buffers...)
}

func _NewCompositeBuf(maxSize int, buffers ...BaseBuf) *compositeBuf {
	var initCap int
	bufLen := len(buffers)

	if bufLen == 0 {
		return &compositeBuf{
			child:          make([]*bufComponent, defaultBufSize),
			lastComp:       &bufComponent{},
			maxSize:        initCap,
			componentCount: 0,
			wIdx:           0,
		}
	}

	if maxSize > bufLen {
		initCap = maxSize
	} else {
		initCap = bufLen
	}

	compoments := make([]*bufComponent, initCap)
	offset := 0
	count := 0

	for i, buf := range buffers {
		component := _NewBufComponent(offset, buf)
		offset += component.eIdx
		compoments[i] = component
		count++
	}

	p := &compositeBuf{
		child:          compoments,
		lastComp:       &bufComponent{},
		maxSize:        initCap,
		componentCount: count,
		wIdx:           compoments[count-1].eIdx,
	}
	p.buildBufArray()
	return p
}

func _NewBufComponent(offset int, src BaseBuf) *bufComponent {
	srcOffset := src.GetReadIndex()
	length := src.ReadableBytes()

	return &bufComponent{
		sliced: Slice(src, srcOffset, srcOffset+length),
		sIdx:   offset,
		eIdx:   offset + length,
	}
}

func (buf *compositeBuf) buildBufArray() {
	heap := &bufArray{}
	tail := heap
	l := buf.componentCount

	for i := 0; i < l; i++ {
		c := buf.child[i]
		tail.data = c.sliced.Array().data
		if i < l-1 {
			tail.next = &bufArray{}
			tail = tail.next
		}
	}
	buf.childArrays = heap
}

func (buf *compositeBuf) findComponent(idx int) (bc *bufComponent, err error) {

	if *buf.lastComp != (bufComponent{}) && buf.lastComp.sIdx <= idx && buf.lastComp.eIdx > idx {
		return buf.lastComp, nil
	}

	i, err := buf.binarySearchComponent(idx)

	if err == nil {
		bc = buf.child[i]
		buf.lastComp = bc
	}

	return
}

func (buf *compositeBuf) binarySearchComponent(idx int) (i int, err error) {
	err = checkIndex(idx, buf)
	if err != nil {
		return
	}

	start := 0
	end := buf.componentCount - 1

	for {
		mid := (end - start) >> 1
		bc := buf.child[mid]

		if idx < bc.sIdx {
			end = mid - 1
		} else if idx >= bc.eIdx {
			start = mid + 1
		} else {
			return mid, nil
		}

		if start > end {
			return -1, errors.New("cannot Reach here")
		}
	}
}

func (buf *compositeBuf) mergaChild(from, to int) {
	//TODO
}

func (buf *compositeBuf) Size() int {
	if buf.componentCount == 0 {
		return 0
	}
	return buf.child[buf.componentCount-1].eIdx
}

func (buf *compositeBuf) Array() *bufArray {
	return buf.childArrays
}

func (buf *compositeBuf) Copy() BaseBuf {
	data := make([]byte, buf.Size())
	cp := NewBufWithArray(data)
	transfor(cp, buf)
	return cp
}

func (buf *compositeBuf) WriteIndex(idx int) error {
	err := checkIndex(idx, buf)
	if err != nil {
		return err
	}

	buf.wIdx = idx
	return nil
}

func (buf *compositeBuf) ReadIndex(idx int) error {
	err := checkIndex(idx, buf)
	if err != nil {
		return err
	}

	buf.rIdx = idx
	return nil
}

func (buf *compositeBuf) GetWriteIndex() int {
	return buf.wIdx
}

func (buf *compositeBuf) GetReadIndex() int {
	return buf.rIdx
}

func (buf *compositeBuf) WriteableBytes() int {
	return buf.wIdx - buf.Size()
}

func (buf *compositeBuf) ReadableBytes() int {
	return buf.wIdx - buf.rIdx
}

func (buf *compositeBuf) DiscardReadBytes() {
	if buf.rIdx == 0 {
		return
	}

	rOffset := buf.GetReadIndex()
	wOffset := buf.GetWriteIndex()
	first := 0

	if rOffset == wOffset && wOffset == buf.Size() {
		buf.lastComp = nil
		buf.childArrays = nil
		for i := range buf.child {
			buf.child[i] = nil
		}
		buf.rIdx = 0
		buf.wIdx = 0
		buf.componentCount = 0
	} else {
		var c *bufComponent

		for cnt := buf.componentCount; first < cnt; first++ {
			c = buf.child[first]
			if rOffset < c.eIdx {
				break
			}
			buf.child[first] = nil
		}

		if buf.lastComp != nil && buf.lastComp.eIdx <= rOffset {
			buf.lastComp = nil
		}

		c.sIdx = 0
		c.eIdx -= rOffset

		copy(buf.child, buf.child[first:])
		buf.rIdx = 0
		buf.wIdx -= rOffset
		buf.buildBufArray()
	}
}

func (buf *compositeBuf) GetByte(i int) (b byte, err error) {
	c, err := buf.findComponent(i)
	if err != nil {
		return 0, err
	}

	return c.sliced.GetByte(c.index(i))
}

func (buf *compositeBuf) GetBytes(i int, dst []byte) error {
	return bufGetBytes(buf, i, dst)
}

func (buf *compositeBuf) SetByte(i int, b byte) error {
	c, err := buf.findComponent(i)
	if err != nil {
		return err
	}

	return c.sliced.SetByte(c.index(i), b)
}

func (buf *compositeBuf) SetBytes(i int, src []byte) error {
	return bufSetBytes(buf, i, src)
}

func (buf *compositeBuf) WriteTo(w io.Writer) error {
	return buf.WriteToWithLen(buf.ReadableBytes(), w)
}

func (buf *compositeBuf) WriteToWithLen(len int, w io.Writer) error {
	return bufWriteTo(buf, len, w)
}

func (buf *compositeBuf) WriteByte(b byte) error {
	if buf.WriteableBytes() < 1 {
		return ErrInSufficientBytes
	}

	buf.SetByte(buf.wIdx, b)
	buf.wIdx += 1
	return nil
}

func (buf *compositeBuf) WriteBytes(b []byte) (n int, err error) {
	return bufWriteBytes(buf, b)
}

func (buf *compositeBuf) WriteString(s string) (n int, err error) {
	return unsafeWriteString(s, buf)
}

func (buf *compositeBuf) ReadFrom(r io.Reader) (n int, err error) {
	return buf.ReadFromWithLen(buf.WriteableBytes(), r)
}

func (buf *compositeBuf) ReadFromWithLen(len int, r io.Reader) (n int, err error) {
	return bufReadFrom(buf, len, r)
}

func (buf *compositeBuf) ReadByte() (rb byte, err error) {
	if buf.ReadableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	rb, err = buf.GetByte(buf.rIdx)
	buf.rIdx += 1
	return
}

func (buf *compositeBuf) ReadBytes(b []byte) (n int, err error) {
	return bufReadBytes(buf, b)
}

func (buf *compositeBuf) ReadString(len int) (rs string, err error) {
	return unsafeReadString(len, buf)
}

type bufferWriter struct {
	buf BaseBuf
}

func NewBufferWriter(buf BaseBuf) *bufferWriter {
	return &bufferWriter{
		buf: buf,
	}
}

func (bw *bufferWriter) Write(p []byte) (n int, err error) {
	return bw.buf.WriteBytes(p)
}

func (bw *bufferWriter) WriteString(s string) (n int, err error) {
	return bw.buf.WriteString(s)
}
