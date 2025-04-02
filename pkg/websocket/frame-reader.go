package websocket

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const (
	frameMinHeaderSize    = 2
	frameMaskSize         = 4
	frameMaxPayloadLength = 8
	frameMaxHeaderSize    = frameMinHeaderSize + frameMaxPayloadLength + frameMaskSize
)

type FrameReader interface {
	Read([]byte) (int, error)
	Opcode() Opcode
	Len() int
	Data() []byte
	Err() error
}

type frameReader struct {
	opcode     Opcode
	buffer     *bytes.Buffer
	compressed bool
	err        error
}

func (frameReader *frameReader) Read(b []byte) (n int, err error) {
	return frameReader.buffer.Read(b)
}

func (frameReader *frameReader) Opcode() Opcode {
	return frameReader.opcode
}

func (frameReader *frameReader) Len() int {
	return frameReader.buffer.Len()
}

func (frameReader *frameReader) Data() []byte {
	if frameReader.err != nil {
		return frameReader.buffer.Bytes()
	}
	return []byte{}
}

func (frameReader *frameReader) Err() error {
	return frameReader.err
}

func NewFrameReader(reader io.Reader) FrameReader {
	frameBuff := bytes.NewBuffer(make([]byte, 512))
	fr := &frameReader{}
	fr.buffer = frameBuff

	first, final := true, false
	for fr.err == nil && !final {
		header := make([]byte, frameMinHeaderSize)
		if _, err := io.ReadFull(reader, header); err != nil {
			return fr
		}

		b0 := header[0]
		final = b0&finBitMask != 0
		rsv1 := b0&rsv1BitMask != 0
		rsv2 := b0&rsv2BitMask != 0
		rsv3 := b0&rsv3BitMask != 0
		opc := Opcode(b0 & opcodeBitMask)

		if err := opc.Valid(); err != nil {
			return fr
		}

		// If this is a control frame, it must be final.
		if first {
			if opc.IsControl() && !final {
				fr.err = errors.New("control frames must not be fragmented")
				return fr
			}
			fr.opcode = opc
			first = false
		} else {
			if !opc.IsContinue() {
				fr.err = errors.New("unexpected new frame when continuation expected")
				return fr
			}
		}

		if rsv1 {
			panic("TODO: implement `permessage-deflate` extension")
		}

		if rsv2 {
			fr.err = errors.New("RSV2 is set")
			return fr
		}
		if rsv3 {
			fr.err = errors.New("RSV3 is set")
			return fr
		}

		b1 := header[1]
		masked := b1&maskBitMask != 0
		payloadLen := int64(b1 & payloadLenBitMask)

		switch payloadLen {
		case 126:
			extPayloadLen := make([]byte, 2)
			if _, err := reader.Read(extPayloadLen); err != nil {
				fr.err = err
				return fr
			} else {
				payloadLen = int64(binary.BigEndian.Uint16(extPayloadLen))
			}
		case 127:
			extPayloadLen := make([]byte, frameMaxPayloadLength)
			if _, err := reader.Read(extPayloadLen); err != nil {
				fr.err = err
				return fr
			} else {
				payloadLen = int64(binary.BigEndian.Uint64(extPayloadLen))
			}
		}

		mask := make([]byte, frameMaskSize)
		if masked {
			_, err := reader.Read(mask)
			if err != nil {
				fr.err = err
				return fr
			}
		}

		payload := make([]byte, payloadLen)
		_, err := reader.Read(payload)
		if err != nil {
			fr.err = err
			return fr
		}

		for i := 0; masked && i < len(payload); i++ {
			payload[i] ^= mask[i&3]
		}

		_, err = frameBuff.Write(payload)
		if err != nil {
			fr.err = err
			return fr
		}
	}

	return fr
}

// https://github.com/golang/go/issues/17064
func readN(r *bufio.Reader, n int) ([]byte, error) {
	buf, err := r.Peek(n)
	if err != nil {
		return buf, err
	}
	r.Discard(n)
	return buf, err
}
