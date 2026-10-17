// Package exiftest builds minimal image files that carry an EXIF date, for tests.
package exiftest

import (
	"bytes"
	"encoding/binary"
	"time"
)

// TIFF returns a big-endian TIFF structure with DateTimeOriginal set to taken.
func TIFF(taken time.Time) []byte {
	var b bytes.Buffer
	be := binary.BigEndian

	b.WriteString("MM")
	binary.Write(&b, be, uint16(42))
	binary.Write(&b, be, uint32(8)) // IFD0

	// IFD0 at 8: one entry pointing to the Exif IFD at 26
	binary.Write(&b, be, uint16(1))
	binary.Write(&b, be, uint16(0x8769))
	binary.Write(&b, be, uint16(4)) // LONG
	binary.Write(&b, be, uint32(1))
	binary.Write(&b, be, uint32(26))
	binary.Write(&b, be, uint32(0)) // no next IFD

	// Exif IFD at 26: DateTimeOriginal, an ASCII string at 44
	binary.Write(&b, be, uint16(1))
	binary.Write(&b, be, uint16(0x9003))
	binary.Write(&b, be, uint16(2)) // ASCII
	binary.Write(&b, be, uint32(20))
	binary.Write(&b, be, uint32(44))
	binary.Write(&b, be, uint32(0))

	b.WriteString(taken.Format("2006:01:02 15:04:05"))
	b.WriteByte(0)
	return b.Bytes()
}

// JPEG returns a JPEG file with an APP1 Exif segment and no image data.
func JPEG(taken time.Time) []byte {
	tiff := TIFF(taken)

	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8})
	b.Write([]byte{0xFF, 0xE0, 0x00, 0x04, 0x00, 0x00}) // an unrelated APP0 segment first
	b.Write([]byte{0xFF, 0xE1})
	binary.Write(&b, binary.BigEndian, uint16(2+6+len(tiff)))
	b.WriteString("Exif\x00\x00")
	b.Write(tiff)
	b.Write([]byte{0xFF, 0xD9})
	return b.Bytes()
}

// HEIC returns an ISOBMFF file whose meta box describes one Exif item in mdat.
func HEIC(taken time.Time) []byte {
	be := binary.BigEndian
	payload := append([]byte{0, 0, 0, 6}, append([]byte("Exif\x00\x00"), TIFF(taken)...)...)

	ftyp := boxOf("ftyp", []byte("heic\x00\x00\x00\x00mif1heic"))

	infe := boxOf("infe", append([]byte{2, 0, 0, 0, 0, 1, 0, 0}, []byte("Exif\x00")...))
	iinf := boxOf("iinf", append([]byte{0, 0, 0, 0, 0, 1}, infe...))

	ilocBody := []byte{0, 0, 0, 0, 0x44, 0x00, 0, 1, 0, 1, 0, 0, 0, 1}
	ilocBody = append(ilocBody, make([]byte, 8)...) // extent offset and length, set below
	iloc := boxOf("iloc", ilocBody)

	meta := boxOf("meta", append(append([]byte{0, 0, 0, 0}, iinf...), iloc...))

	mdatOffset := len(ftyp) + len(meta) + 8
	extent := len(meta) - 8
	be.PutUint32(meta[extent:], uint32(mdatOffset))
	be.PutUint32(meta[extent+4:], uint32(len(payload)))

	var b bytes.Buffer
	b.Write(ftyp)
	b.Write(meta)
	b.Write(boxOf("mdat", payload))
	return b.Bytes()
}

func boxOf(boxType string, body []byte) []byte {
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], boxType)
	return append(b, body...)
}
