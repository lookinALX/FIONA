package scanner

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

// EXIF is read without a dependency: the standard library has no parser, and
// only one tag is needed. Supported containers: JPEG, TIFF and the RAW formats
// built on it (DNG, CR2, NEF, ARW, ORF, RW2), Fujifilm RAF, and HEIC/HEIF/AVIF.

const (
	tagExifIFD           = 0x8769
	tagDateTimeOriginal  = 0x9003
	tagDateTimeDigitized = 0x9004

	exifDateLayout = "2006:01:02 15:04:05"

	maxIFDEntries = 1024
	maxMetaBox    = 4 << 20
)

var errNoExif = errors.New("no exif")

// readTaken returns the date the photo was taken, in local time.
func readTaken(path string) (time.Time, bool) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return time.Time{}, false
	}

	tiff, err := findTIFF(f, info.Size())
	if err != nil {
		return time.Time{}, false
	}
	return dateFromTIFF(tiff)
}

// findTIFF locates the TIFF structure that holds the EXIF tags.
func findTIFF(f io.ReaderAt, size int64) (*io.SectionReader, error) {
	head := make([]byte, 16)
	n, _ := f.ReadAt(head, 0)
	head = head[:n]

	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8}):
		return jpegTIFF(f, 0, size)
	case isTIFFHeader(head):
		return io.NewSectionReader(f, 0, size), nil
	case bytes.HasPrefix(head, []byte("FUJIFILMCCD-RAW")):
		// the header points to an embedded JPEG that carries the EXIF
		var off [4]byte
		if _, err := f.ReadAt(off[:], 84); err != nil {
			return nil, errNoExif
		}
		return jpegTIFF(f, int64(binary.BigEndian.Uint32(off[:])), size)
	case len(head) >= 8 && string(head[4:8]) == "ftyp":
		return heifTIFF(f, size)
	default:
		return nil, errNoExif
	}
}

func isTIFFHeader(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	var order binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return false
	}
	switch order.Uint16(b[2:4]) {
	case 42, 0x4F52, 0x5352, 0x55: // TIFF, Olympus ORF (two variants), Panasonic RW2
		return true
	}
	return false
}

// jpegTIFF walks the JPEG markers from start up to the image data and returns
// the TIFF part of the APP1 Exif segment.
func jpegTIFF(f io.ReaderAt, start, size int64) (*io.SectionReader, error) {
	pos := start + 2
	var hdr [4]byte

	for pos+4 <= size {
		if _, err := f.ReadAt(hdr[:], pos); err != nil {
			return nil, errNoExif
		}
		if hdr[0] != 0xFF {
			return nil, errNoExif
		}
		marker := hdr[1]
		if marker == 0xFF { // fill byte
			pos++
			continue
		}
		if marker == 0xD9 || marker == 0xDA { // end of image, start of scan
			return nil, errNoExif
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) { // markers without length
			pos += 2
			continue
		}

		length := int64(binary.BigEndian.Uint16(hdr[2:4]))
		if length < 2 {
			return nil, errNoExif
		}
		if marker == 0xE1 && length >= 8 {
			sig := make([]byte, 6)
			if _, err := f.ReadAt(sig, pos+4); err == nil && string(sig) == "Exif\x00\x00" {
				return io.NewSectionReader(f, pos+10, length-8), nil
			}
		}
		pos += 2 + length
	}

	return nil, errNoExif
}

// heifTIFF finds the Exif item of an ISOBMFF file (HEIC, HEIF, AVIF) through
// the item info (iinf) and item location (iloc) boxes of the meta box.
func heifTIFF(f io.ReaderAt, size int64) (*io.SectionReader, error) {
	meta, err := findBox(f, 0, size, "meta")
	if err != nil || meta.size > maxMetaBox {
		return nil, errNoExif
	}
	data := make([]byte, meta.size)
	if _, err := f.ReadAt(data, meta.offset); err != nil {
		return nil, errNoExif
	}
	if len(data) < 4 {
		return nil, errNoExif
	}
	data = data[4:] // meta is a full box: version and flags

	var iinf, iloc []byte
	for b := data; len(b) >= 8; {
		boxType, body, rest, ok := nextBox(b)
		if !ok {
			break
		}
		switch boxType {
		case "iinf":
			iinf = body
		case "iloc":
			iloc = body
		}
		b = rest
	}

	id, ok := exifItemID(iinf)
	if !ok {
		return nil, errNoExif
	}
	offset, length, ok := itemExtent(iloc, id)
	if !ok || length < 4 || offset+length > uint64(size) {
		return nil, errNoExif
	}

	// the item starts with the offset of the TIFF header, usually past "Exif\0\0"
	var skip [4]byte
	if _, err := f.ReadAt(skip[:], int64(offset)); err != nil {
		return nil, errNoExif
	}
	tiffStart := uint64(4) + uint64(binary.BigEndian.Uint32(skip[:]))
	if tiffStart >= length {
		return nil, errNoExif
	}
	return io.NewSectionReader(f, int64(offset+tiffStart), int64(length-tiffStart)), nil
}

type box struct {
	offset int64 // of the body
	size   int64 // of the body
}

// findBox looks for a top-level box of the given type.
func findBox(f io.ReaderAt, start, end int64, want string) (box, error) {
	var hdr [16]byte
	for pos := start; pos+8 <= end; {
		if _, err := f.ReadAt(hdr[:8], pos); err != nil {
			return box{}, errNoExif
		}
		size := int64(binary.BigEndian.Uint32(hdr[:4]))
		boxType := string(hdr[4:8])
		headerLen := int64(8)

		switch size {
		case 0:
			size = end - pos
		case 1:
			if _, err := f.ReadAt(hdr[8:16], pos+8); err != nil {
				return box{}, errNoExif
			}
			size = int64(binary.BigEndian.Uint64(hdr[8:16]))
			headerLen = 16
		}
		if size < headerLen || pos+size > end {
			return box{}, errNoExif
		}

		if boxType == want {
			return box{offset: pos + headerLen, size: size - headerLen}, nil
		}
		pos += size
	}
	return box{}, errNoExif
}

// nextBox splits the first box off b.
func nextBox(b []byte) (boxType string, body, rest []byte, ok bool) {
	if len(b) < 8 {
		return "", nil, nil, false
	}
	size := uint64(binary.BigEndian.Uint32(b[:4]))
	headerLen := uint64(8)
	switch size {
	case 0:
		size = uint64(len(b))
	case 1:
		if len(b) < 16 {
			return "", nil, nil, false
		}
		size = binary.BigEndian.Uint64(b[8:16])
		headerLen = 16
	}
	if size < headerLen || size > uint64(len(b)) {
		return "", nil, nil, false
	}
	return string(b[4:8]), b[headerLen:size], b[size:], true
}

// exifItemID finds the item of type "Exif" in the body of an iinf box.
func exifItemID(iinf []byte) (uint32, bool) {
	if len(iinf) < 6 {
		return 0, false
	}
	entries := iinf[6:]
	if iinf[0] != 0 { // version 1+: 32-bit entry count
		if len(iinf) < 8 {
			return 0, false
		}
		entries = iinf[8:]
	}

	for b := entries; len(b) >= 8; {
		boxType, body, rest, ok := nextBox(b)
		if !ok {
			return 0, false
		}
		b = rest
		if boxType != "infe" || len(body) < 4 {
			continue
		}

		version := body[0]
		p := body[4:]
		var id uint32
		switch {
		case version == 2 && len(p) >= 8:
			id = uint32(binary.BigEndian.Uint16(p[:2]))
			p = p[2:]
		case version == 3 && len(p) >= 10:
			id = binary.BigEndian.Uint32(p[:4])
			p = p[4:]
		default:
			continue
		}
		// item_protection_index, then item_type
		if string(p[2:6]) == "Exif" {
			return id, true
		}
	}
	return 0, false
}

// itemExtent returns the file offset and length of the first extent of an item
// stored in the file itself (construction method 0).
func itemExtent(iloc []byte, want uint32) (offset, length uint64, ok bool) {
	r := byteReader{b: iloc}
	version := r.u8()
	r.skip(3)
	sizes := r.u8()
	offsetSize, lengthSize := int(sizes>>4), int(sizes&0x0F)
	sizes = r.u8()
	baseOffsetSize, indexSize := int(sizes>>4), int(sizes&0x0F)
	if version == 0 {
		indexSize = 0
	}

	var count uint32
	if version < 2 {
		count = uint32(r.u16())
	} else {
		count = r.u32()
	}

	for i := uint32(0); i < count && r.ok(); i++ {
		var id uint32
		if version < 2 {
			id = uint32(r.u16())
		} else {
			id = r.u32()
		}
		method := uint16(0)
		if version >= 1 {
			method = r.u16() & 0x0F
		}
		r.skip(2) // data_reference_index
		base := r.uint(baseOffsetSize)
		extents := r.u16()

		for e := uint16(0); e < extents && r.ok(); e++ {
			r.skip(indexSize)
			extOffset := r.uint(offsetSize)
			extLength := r.uint(lengthSize)
			if id == want && e == 0 {
				if method != 0 || !r.ok() {
					return 0, 0, false
				}
				return base + extOffset, extLength, true
			}
		}
	}
	return 0, 0, false
}

// dateFromTIFF reads DateTimeOriginal (or DateTimeDigitized) from the Exif IFD.
func dateFromTIFF(r *io.SectionReader) (time.Time, bool) {
	var hdr [8]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil || !isTIFFHeader(hdr[:]) {
		return time.Time{}, false
	}
	order := binary.ByteOrder(binary.LittleEndian)
	if hdr[0] == 'M' {
		order = binary.BigEndian
	}

	ifd0 := readIFD(r, order, int64(order.Uint32(hdr[4:8])))
	exifPtr, ok := ifd0[tagExifIFD]
	if !ok {
		return time.Time{}, false
	}
	exif := readIFD(r, order, int64(order.Uint32(exifPtr.value[:])))

	for _, tag := range []uint16{tagDateTimeOriginal, tagDateTimeDigitized} {
		entry, ok := exif[tag]
		if !ok || entry.typ != 2 || entry.count < 19 || entry.count > 64 {
			continue
		}
		raw := make([]byte, entry.count)
		if _, err := r.ReadAt(raw, int64(order.Uint32(entry.value[:]))); err != nil {
			continue
		}
		if t, ok := parseExifDate(string(raw)); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

type ifdEntry struct {
	typ   uint16
	count uint32
	value [4]byte // the value itself or the offset to it
}

func readIFD(r *io.SectionReader, order binary.ByteOrder, offset int64) map[uint16]ifdEntry {
	entries := make(map[uint16]ifdEntry)

	var n [2]byte
	if offset <= 0 || offset >= r.Size() {
		return entries
	}
	if _, err := r.ReadAt(n[:], offset); err != nil {
		return entries
	}
	count := int(order.Uint16(n[:]))
	if count > maxIFDEntries {
		return entries
	}

	buf := make([]byte, 12*count)
	if _, err := r.ReadAt(buf, offset+2); err != nil {
		return entries
	}
	for i := 0; i < count; i++ {
		e := buf[12*i : 12*i+12]
		var entry ifdEntry
		entry.typ = order.Uint16(e[2:4])
		entry.count = order.Uint32(e[4:8])
		copy(entry.value[:], e[8:12])
		entries[order.Uint16(e[0:2])] = entry
	}
	return entries
}

func parseExifDate(s string) (time.Time, bool) {
	s = strings.TrimRight(s, "\x00 ")
	if len(s) < len(exifDateLayout) {
		return time.Time{}, false
	}
	s = s[:len(exifDateLayout)]
	// some cameras write dashes in the date
	if s[4] == '-' && s[7] == '-' {
		s = s[:4] + ":" + s[5:7] + ":" + s[8:]
	}
	t, err := time.ParseInLocation(exifDateLayout, s, time.Local)
	if err != nil || t.Year() < 1900 {
		return time.Time{}, false
	}
	return t, true
}

// byteReader reads big-endian numbers and remembers if it ran out of data.
type byteReader struct {
	b   []byte
	bad bool
}

func (r *byteReader) ok() bool { return !r.bad }

func (r *byteReader) take(n int) []byte {
	if r.bad || n > len(r.b) {
		r.bad = true
		return make([]byte, n)
	}
	p := r.b[:n]
	r.b = r.b[n:]
	return p
}

func (r *byteReader) skip(n int)  { r.take(n) }
func (r *byteReader) u8() byte    { return r.take(1)[0] }
func (r *byteReader) u16() uint16 { return binary.BigEndian.Uint16(r.take(2)) }
func (r *byteReader) u32() uint32 { return binary.BigEndian.Uint32(r.take(4)) }
func (r *byteReader) u64() uint64 { return binary.BigEndian.Uint64(r.take(8)) }

// uint reads a number of 0, 4 or 8 bytes, as iloc fields are sized.
func (r *byteReader) uint(size int) uint64 {
	switch size {
	case 0:
		return 0
	case 4:
		return uint64(r.u32())
	case 8:
		return r.u64()
	default:
		r.bad = true
		return 0
	}
}
