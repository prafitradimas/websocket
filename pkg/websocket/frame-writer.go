package websocket

import (
	"bufio"
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
	opcode Opcode
	masked bool

	writer *bufio.Writer
	err    error
}

func NewFrameWriter(opc Opcode, writer *bufio.Writer, masked bool) FrameWriter {
	return &frameWriter{
		opcode: opc,
		masked: masked,
		writer: writer,
	}
}

// Write implements FrameWriter.
func (frameWriter *frameWriter) Write(b []byte) (int, error) {
	n := len(b)
	first := true
	var offset, size, remaining int
	for n > offset {
		if frameWriter.err != nil {
			return 0, frameWriter.err
		}

		size = frameWriter.writer.Available() - frameMaxHeaderSize
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
	if n > frameWriter.writer.Available()-frameMaxHeaderSize {
		return 0, bytes.ErrTooLarge
	}

	b0 := byte(frameWriter.opcode)
	if final {
		b0 |= finBitMask
	}
	frameWriter.writer.WriteByte(b0)

	b1 := byte(0)
	if masked {
		b1 |= maskBitMask
	}

	if n <= 125 {
		b1 |= byte(n)
		frameWriter.writer.WriteByte(b1)
	} else if n > 125 && n <= 0xFFFF {
		b1 |= 126
		frameWriter.writer.WriteByte(b1)
		err = binary.Write(frameWriter.writer, binary.BigEndian, uint16(n))
		if err != nil {
			frameWriter.err = err
			return 0, err
		}
	} else {
		b1 |= 127
		frameWriter.writer.WriteByte(b1)
		err = binary.Write(frameWriter.writer, binary.BigEndian, uint64(n))
		if err != nil {
			frameWriter.err = err
			return 0, err
		}
	}

	if masked {
		mask, maskErr := generateMaskingKey()
		if maskErr != nil {
			return 0, maskErr
		}

		frameWriter.writer.Write(mask)

		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	if _, err = frameWriter.writer.Write(payload); err != nil {
		frameWriter.err = err
		return 0, err
	}

	return n, frameWriter.writer.Flush()
}

func generateMaskingKey() (maskingKey []byte, err error) {
	maskingKey = make([]byte, 4)
	_, err = io.ReadFull(rand.Reader, maskingKey)
	return
}
