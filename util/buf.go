package util

import (
	"errors"
	"io"
	"reflect"
	"unsafe"
)

const (
	defaultBufSize = 2048
)

type BufferError string

func (e BufferError) Error() string {
	return "websocket.buf:\n " + string(e)
}

type OutOfBufError string

func (e OutOfBufError) Error() string {
	return "websocket.buf:\n OutOfBufError: " + string(e)
}

type ReaderError string

func (e ReaderError) Error() string {
	return "websocket.buf:\n ReaderError: " + string(e)
}

const (
	ErrInSufficientBytes = BufferError("insufficient bytes available in the buffer")
	ErrReadNoEnough      = BufferError("length of data read from Buffer is not enough")
)

type bufArray struct {
	data []byte
	next *bufArray
}

func _NewBufArray(arrays ...[]byte) *bufArray {
	l := len(arrays)

	if len(arrays) == 1 {
		return &bufArray{
			data: arrays[0],
		}
	}

	head := &bufArray{}
	tail := head
	for i := 0; i < l; i++ {
		array := arrays[i]

		tail.data = array
		if i < l-1 {
			tail.next = &bufArray{}
			tail = tail.next
		}
	}

	return head
}

func (a *bufArray) Data() []byte {
	return a.data
}

func (a *bufArray) Next() *bufArray {
	return a.next
}

func findArray(head *bufArray, i int) (offset int, array *bufArray) {
	if i < 0 {
		panic(errors.New("index out of the bound"))
	}

	array = head
	length := 0

	for {
		l := len(array.data)
		if length+l > i {
			offset = i - length
			break
		} else if array.Next() == nil {
			panic(errors.New("index out of the bound"))
		}

		array = array.Next()
		length += l
	}

	return
}

type BaseBuf interface {
	Size() int

	Array() *bufArray

	Copy() BaseBuf

	WriteIndex(idx int) error

	ReadIndex(idx int) error

	GetWriteIndex() int

	GetReadIndex() int

	WriteableBytes() int

	ReadableBytes() int

	DiscardReadBytes()

	GetByte(i int) (b byte, err error)

	GetBytes(i int, dst []byte) error

	SetByte(i int, b byte) error

	SetBytes(i int, src []byte) error

	WriteTo(w io.Writer) error

	WriteToWithLen(len int, w io.Writer) error

	WriteByte(b byte) error

	WriteBytes(b []byte) (n int, err error)

	WriteString(s string) (n int, err error)

	ReadFrom(r io.Reader) (n int, err error)

	ReadFromWithLen(len int, r io.Reader) (n int, err error)

	ReadByte() (rb byte, err error)

	ReadBytes(b []byte) (n int, err error)

	ReadString(len int) (rs string, err error)
}

type ByteBuf struct {
	//reader index
	rIdx int
	//writer index
	wIdx int

	//buf
	array *bufArray
}

func NewBufWithSize(size int) BaseBuf {
	return &ByteBuf{
		rIdx:  0,
		wIdx:  0,
		array: _NewBufArray(make([]byte, size)),
	}
}

func NewBufWithArray(b []byte) BaseBuf {
	return &ByteBuf{
		rIdx:  0,
		wIdx:  0,
		array: _NewBufArray(b),
	}
}

func NewBuf() BaseBuf {
	return NewBufWithSize(defaultBufSize)
}

func unsafeWriteString(s string, dstBuf BaseBuf) (n int, err error) {
	if dstBuf.WriteableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	b := *(*[]byte)(unsafe.Pointer(&bh))

	n, err = dstBuf.WriteBytes(b)
	return
}

func unsafeReadString(len int, srcBuf BaseBuf) (str string, err error) {
	if srcBuf.ReadableBytes() < 1 {
		return "", ErrInSufficientBytes
	} else if srcBuf.ReadableBytes() < len {
		len = srcBuf.ReadableBytes()
	}

	rb := make([]byte, len)
	_, err = srcBuf.ReadBytes(rb)

	str = *(*string)(unsafe.Pointer(&rb))
	return
}

func transfor(dst BaseBuf, src BaseBuf) int {
	return transforWithBound(dst, 0, dst.Size(), src, 0, src.Size())
}

func transforWithBound(dst BaseBuf, dFrom, dTo int, src BaseBuf, sFrom, sTo int) int {
	dstLen, srcLen := dTo-dFrom, sTo-sFrom
	if dstLen < 1 || srcLen < 1 {
		return 0
	}

	dstOffset, dstFrom := findArray(dst.Array(), dFrom)
	srcOffset, srcFrom := findArray(src.Array(), sFrom)
	transforLen := 0

	for {
		dEnd := len(dstFrom.data)
		sEnd := len(srcFrom.data)

		if dstLen < len(dstFrom.data) {
			dEnd = dstOffset + dstLen
		}
		if srcLen < len(dstFrom.data) {
			sEnd = srcOffset + srcLen
		}

		n := copy(dstFrom.data[dstOffset:dEnd], srcFrom.data[srcOffset:sEnd])
		dstOffset += n
		srcOffset += n
		dstLen -= n
		srcLen -= n

		transforLen += n

		//array transfor data done
		if dstLen == 0 || srcLen == 0 {
			break
		}

		if dstOffset == len(dstFrom.data) && dstLen > 0 {
			if dstFrom.next == nil {
				panic("out of bound on transfor bufarray data")
			}
			dstFrom = dstFrom.next
			dstOffset = 0
		}

		if srcOffset == len(srcFrom.data) && srcLen > 0 {
			if srcFrom.next == nil {
				panic("out of bound on transfor bufarray data")
			}
			srcFrom = srcFrom.next
			srcOffset = 0
		}
	}

	return transforLen
}

func bufGetBytes(srcBuf BaseBuf, i int, dst []byte) error {
	err := checkIndex(i, srcBuf)
	if err != nil {
		return err
	}

	dstLen := len(dst)
	offset, array := findArray(srcBuf.Array(), i)

	l := copy(dst, array.data[offset:])
	dstLen -= l

	if dstLen > 1 && array.next != nil {
		dOffset := l
		array = array.next
		for {
			l = copy(dst[dOffset:], array.data)
			dstLen -= l

			if dstLen < 1 {
				break
			}

			if array.next == nil {
				break
			}

			dOffset += l
			array = array.next
		}
	}

	return nil
}

func bufSetBytes(dstBuf BaseBuf, i int, src []byte) error {
	err := checkIndex(i, dstBuf)
	if err != nil {
		return err
	}

	dstLen := dstBuf.Size() - i
	offset, array := findArray(dstBuf.Array(), i)

	l := copy(array.data[offset:], src)
	dstLen -= l

	if dstLen > 1 && array.next != nil {
		sOffset := l
		array = array.next
		for {
			l = copy(array.data, src[sOffset:])
			dstLen -= l

			if dstLen < 1 {
				break
			}

			if array.next == nil {
				break
			}

			sOffset += l
			array = array.next
		}
	}

	return nil
}

func bufWriteBytes(dstBuf BaseBuf, src []byte) (n int, err error) {
	dstLen := dstBuf.WriteableBytes()
	srcLen := len(src)
	if dstLen < 1 {
		return 0, ErrInSufficientBytes
	}
	wIndex := dstBuf.GetWriteIndex()
	offset, array := findArray(dstBuf.Array(), wIndex)

	l := copy(array.data[offset:], src)
	dstLen -= l
	n += l

	if n < srcLen && dstLen > 1 && array.next != nil {
		sOffset := l
		array = array.next
		for n < srcLen {
			l = copy(array.data, src[sOffset:])
			dstLen -= l
			n += l

			if dstLen < 1 {
				break
			}

			if array.next == nil {
				err = ErrInSufficientBytes
				break
			}

			sOffset += l
			array = array.next
		}
	}

	dstBuf.WriteIndex(wIndex + n)
	return n, err
}

func bufReadBytes(srcBuf BaseBuf, dst []byte) (n int, err error) {
	srcLen := srcBuf.ReadableBytes()
	dstLen := len(dst)
	if srcLen < 1 {
		return 0, ErrInSufficientBytes
	}
	rIndex := srcBuf.GetReadIndex()
	offset, array := findArray(srcBuf.Array(), rIndex)

	l := copy(dst, array.data[offset:])
	srcLen -= l
	n += l

	if n < dstLen && srcLen > 1 && array.next != nil {
		dOffset := l
		array = array.next
		for n < dstLen {
			l = copy(dst[dOffset:], array.data)
			srcLen -= l
			n += l

			if srcLen < 1 {
				break
			}

			if array.next == nil {
				err = ErrInSufficientBytes
				break
			}

			dOffset += l
			array = array.next
		}
	}

	srcBuf.ReadIndex(rIndex + n)
	return n, err
}

func bufWriteTo(buf BaseBuf, length int, w io.Writer) error {
	if length > buf.ReadableBytes() {
		length = buf.ReadableBytes()
	}

	readIndex := buf.GetReadIndex()
	offset, array := findArray(buf.Array(), readIndex)

	if length <= len(array.data)-offset {
		wb, err := w.Write(array.data[offset : offset+length])

		if wb < length && err == nil {
			err = io.ErrShortWrite
		}

		buf.ReadIndex(readIndex + wb)
		return err
	} else {
		wb, err := w.Write(array.data[offset:])
		readIndex += wb

		if wb < length && err == nil {
			err = io.ErrShortWrite
		}

		length -= wb
		for err != nil && length > 0 && array.Next() != nil {
			array = array.next
			srcLen := length

			if srcLen > len(array.data) {
				srcLen = len(array.data)
			}

			wb, err = w.Write(array.data[:srcLen])
			length -= wb
			readIndex += wb

			if wb < length && err == nil {
				err = io.ErrShortWrite
			}
		}

		if err != nil {
			buf.ReadIndex(readIndex)
			return err
		}
	}

	buf.ReadIndex(readIndex)
	return nil
}

func bufReadFrom(buf BaseBuf, length int, r io.Reader) (n int, err error) {
	if length > buf.WriteableBytes() {
		length = buf.WriteableBytes()
	}

	writeIndex := buf.GetWriteIndex()
	offset, array := findArray(buf.Array(), writeIndex)

	if length <= len(array.data)-offset {
		rb, err := r.Read(array.data[offset : offset+length])

		if rb < length && err == nil {
			err = ErrReadNoEnough
		}

		buf.WriteIndex(writeIndex + rb)
		return rb, err
	} else {
		rb, err := r.Read(array.data[offset:])
		writeIndex += rb
		n += rb

		if rb < length && err == nil {
			return n, ErrReadNoEnough
		}

		length -= rb
		for length > 0 && array.Next() != nil {
			array = array.next
			dstLen := length

			if dstLen > len(array.data) {
				dstLen = len(array.data)
			}

			rb, err = r.Read(array.data[:dstLen])
			writeIndex += rb
			n += rb

			if err != nil {
				return n, err
			} else if rb < length {
				return n, ErrReadNoEnough
			}
		}
	}

	return n, nil
}

func checkIndex(idx int, buf BaseBuf) error {
	if idx < 0 {
		return OutOfBufError("idx must be a positive")
	}

	if idx > buf.Size() {
		return OutOfBufError("idx out of Buffer")
	}

	return nil
}

func readIndex(old *int, new int, buf BaseBuf) error {
	if new < 0 {
		return OutOfBufError("idx must be a positive")
	}

	if new > buf.GetWriteIndex() {
		return OutOfBufError("idx must be less than or equal to writeIndex")
	}

	*old = new
	return nil
}

func writeIndex(old *int, new int, buf BaseBuf) error {
	if new < 0 {
		return OutOfBufError("idx must be a positive")
	}

	if new > buf.Size() {
		return OutOfBufError("idx must be less than or equal to bufsize")
	}

	*old = new
	return nil
}

func (b *ByteBuf) ReadIndex(idx int) error {
	return readIndex(&b.rIdx, idx, b)
}

func (b *ByteBuf) WriteIndex(idx int) error {
	return writeIndex(&b.wIdx, idx, b)
}

func (b *ByteBuf) GetWriteIndex() int {
	return b.wIdx
}

func (b *ByteBuf) GetReadIndex() int {
	return b.rIdx
}

func (b *ByteBuf) Size() int {
	return len(b.array.data)
}

func (b *ByteBuf) Array() *bufArray {
	return b.array
}

func (b *ByteBuf) Copy() BaseBuf {
	cp := make([]byte, b.Size())
	copy(cp, b.array.data)
	return NewBufWithArray(cp)
}

func (b *ByteBuf) WriteableBytes() int {
	return b.Size() - b.wIdx
}

func (b *ByteBuf) ReadableBytes() int {
	return b.wIdx - b.rIdx
}

func (buf *ByteBuf) GetByte(i int) (b byte, err error) {
	err = checkIndex(i, buf)
	if err != nil {
		return 0, err
	}

	b = buf.array.data[i]
	return
}

func (b *ByteBuf) GetBytes(i int, dst []byte) error {
	err := checkIndex(i, b)
	if err != nil {
		return err
	}

	copy(dst, b.array.data[i:])
	return nil
}

func (buf *ByteBuf) SetByte(i int, b byte) error {
	err := checkIndex(i, buf)
	if err != nil {
		return err
	}

	buf.array.data[i] = b
	return nil
}

func (b *ByteBuf) SetBytes(i int, src []byte) error {
	err := checkIndex(i, b)
	if err != nil {
		return err
	}

	copy(b.array.data[i:], src)
	return nil
}

func (b *ByteBuf) DiscardReadBytes() {
	if b.rIdx == 0 {
		return
	}

	copy(b.array.data, b.array.data[b.rIdx:b.wIdx])
}

func (b *ByteBuf) WriteTo(w io.Writer) error {
	return b.WriteToWithLen(b.ReadableBytes(), w)
}

func (b *ByteBuf) WriteToWithLen(len int, w io.Writer) (err error) {
	if len > b.ReadableBytes() {
		len = b.ReadableBytes()
		err = ErrInSufficientBytes
	}

	wb, err := w.Write(b.array.data[b.rIdx : b.rIdx+len])
	if wb < len && err == nil {
		err = io.ErrShortWrite
	}

	b.rIdx += wb
	return
}

func (buf *ByteBuf) WriteByte(b byte) error {
	if buf.WriteableBytes() < 1 {
		return ErrInSufficientBytes
	}

	buf.array.data[buf.wIdx] = b
	buf.wIdx += 1
	return nil
}

func (buf *ByteBuf) WriteBytes(b []byte) (n int, err error) {
	if buf.WriteableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	wb := copy(buf.array.data[buf.wIdx:], b)
	if wb < len(b) {
		err = ErrInSufficientBytes
	}

	buf.wIdx += wb

	return wb, err
}

func (buf *ByteBuf) WriteString(s string) (n int, err error) {
	return unsafeWriteString(s, buf)
}

func (b *ByteBuf) ReadFrom(r io.Reader) (n int, err error) {
	return b.ReadFromWithLen(b.WriteableBytes(), r)
}

func (b *ByteBuf) ReadFromWithLen(len int, r io.Reader) (n int, err error) {
	if b.WriteableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	rb, err := r.Read(b.array.data[b.wIdx : b.wIdx+len])

	if rb < 0 {
		panic(ReaderError("reader returned nagative count from Read"))
	}

	b.wIdx += rb

	return rb, err
}

func (b *ByteBuf) ReadByte() (rb byte, err error) {
	if b.ReadableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	rb = b.array.data[b.rIdx]
	b.rIdx += 1
	return
}

func (buf *ByteBuf) ReadBytes(b []byte) (n int, err error) {
	if buf.ReadableBytes() < 1 {
		return 0, ErrInSufficientBytes
	}

	n = copy(b, buf.array.data[buf.rIdx:buf.wIdx])

	if n < len(b) {
		err = ErrReadNoEnough
	}

	buf.rIdx += n
	return
}

func (b *ByteBuf) ReadString(len int) (string, error) {
	return unsafeReadString(len, b)
}
