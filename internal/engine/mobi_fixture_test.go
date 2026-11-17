package engine

import "encoding/binary"

// buildTestMOBI writes a minimal, uncompressed MOBI whose one text record
// holds html, so the reader's header parsing and text extraction can be
// checked without a real Kindle file.
func buildTestMOBI(html []byte) []byte {
	const hdrLen = 0xE8
	palmdoc := make([]byte, 16)
	binary.BigEndian.PutUint16(palmdoc[0:], 1) // no compression
	binary.BigEndian.PutUint32(palmdoc[4:], uint32(len(html)))
	binary.BigEndian.PutUint16(palmdoc[8:], 1) // one text record
	binary.BigEndian.PutUint16(palmdoc[12:], 65001)

	mobi := make([]byte, hdrLen)
	copy(mobi, "MOBI")
	binary.BigEndian.PutUint32(mobi[4:], hdrLen)
	binary.BigEndian.PutUint32(mobi[8:], 2)
	binary.BigEndian.PutUint32(mobi[12:], 65001)
	binary.BigEndian.PutUint32(mobi[16:], 6)
	name := []byte("Test Book")
	nameOff := 16 + hdrLen
	binary.BigEndian.PutUint32(mobi[0x54:], uint32(nameOff))
	binary.BigEndian.PutUint32(mobi[0x58:], uint32(len(name)))

	rec0 := append(append(palmdoc, mobi...), name...)
	records := [][]byte{rec0, html}

	out := make([]byte, 78+len(records)*8+2)
	copy(out[0:32], "Test Book")
	copy(out[60:68], "BOOKMOBI")
	binary.BigEndian.PutUint16(out[76:], uint16(len(records)))
	dataOff := 78 + len(records)*8 + 2
	var body []byte
	for i, r := range records {
		binary.BigEndian.PutUint32(out[78+i*8:], uint32(dataOff+len(body)))
		body = append(body, r...)
	}
	return append(out, body...)
}
