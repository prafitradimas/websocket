package websocket

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
)

const (
	opcodeBitMask     byte = 0xF  // 00001111
	finBitMask        byte = 0x80 // 1 << 7 // 10000000
	maskBitMask       byte = 0x80
	rsv1BitMask       byte = 0x40 // 1 << 6
	rsv2BitMask       byte = 0x20 // 1 << 5
	rsv3BitMask       byte = 0x10 // 1 << 4
	payloadLenBitMask byte = 0x7F
)

type FrameWriter interface {
	io.Writer
}

type frameWriter struct {
	writer    io.Writer
	buf       []byte
	writeSize int
	opcode    Opcode
	masked    bool
	err       error
}

func NewFrameWriter(opc Opcode, writer io.Writer, buf []byte, masked bool) FrameWriter {
	return &frameWriter{
		opcode:    opc,
		writer:    writer,
		buf:       buf,
		writeSize: cap(buf),
		masked:    masked,
	}
}

// Write implements FrameWriter.
func (frameWriter *frameWriter) Write(b []byte) (int, error) {
	n := len(b)
	first := true
	var offset, size, remaining int
	for frameWriter.err == nil && n > offset {
		if frameWriter.err != nil {
			return 0, frameWriter.err
		}

		size = frameWriter.writeSize - frameMaxHeaderSize
		remaining = n - offset
		if size > remaining {
			size = remaining
		}

		final := (offset + size) == n
		_, err := frameWriter.Flush(b[offset:offset+size], frameWriter.masked, final)
		if err != nil {
			frameWriter.err = err
			return 0, err
		}

		offset += size
		if first {
			first = false
			frameWriter.opcode = OpcodeContinueFrame
		}
	}

	return n, nil
}

func (frameWriter *frameWriter) Flush(payload []byte, masked, final bool) (n int, err error) {
	if frameWriter.err != nil {
		return 0, frameWriter.err
	}

	n = len(payload)
	if n > frameWriter.writeSize-frameMaxHeaderSize {
		return 0, bytes.ErrTooLarge
	}

	b0 := byte(frameWriter.opcode)
	if final {
		b0 |= finBitMask
	}
	frameWriter.buf[0] = b0

	b1 := byte(0)
	if masked {
		b1 |= maskBitMask
	}

	pos := 2

	if n <= 125 {
		b1 |= byte(n)
		frameWriter.buf[1] = b1
	} else if n > 125 && n <= 0xFFFF {
		b1 |= 126
		frameWriter.buf[1] = b1
		binary.BigEndian.PutUint16(frameWriter.buf[2:], uint16(n))
		pos += 2
	} else {
		b1 |= 127
		frameWriter.buf[1] = b1
		binary.BigEndian.PutUint64(frameWriter.buf[2:], uint64(n))
		pos += 8
	}

	if masked {
		mask, maskErr := generateMaskingKey()
		if maskErr != nil {
			return 0, maskErr
		}

		pos += copy(frameWriter.buf[pos:pos+frameMaskSize], mask[:frameMaskSize])

		for i := range payload {
			payload[i] ^= mask[i&3]
		}
	}

	copy(frameWriter.buf[pos:], payload)
	if _, err = frameWriter.writer.Write(frameWriter.buf[:pos+n]); err != nil {
		frameWriter.err = err
		return 0, err
	}

	return n, nil
}

func generateMaskingKey() (maskingKey []byte, err error) {
	maskingKey = make([]byte, 4)
	_, err = io.ReadFull(rand.Reader, maskingKey)
	return
}
