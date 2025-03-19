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

func (frameReader *frameReader) Err() error {
	return frameReader.err
}

func NewFrameReader(reader *bufio.Reader) (fr FrameReader, err error) {
	var opcode Opcode
	frameBuff := bytes.NewBuffer(make([]byte, 0, 512))

	first, final := true, false
	for !final {
		header := make([]byte, frameMinHeaderSize)
		if _, err = io.ReadFull(reader, header); err != nil {
			return
		}

		b0 := header[0]
		final = b0&finBitMask != 0
		rsv1 := b0&rsv1BitMask != 0
		rsv2 := b0&rsv2BitMask != 0
		rsv3 := b0&rsv3BitMask != 0
		opc := Opcode(b0 & opcodeBitMask)

		if err = opc.Valid(); err != nil {
			return
		} else if first && !opc.IsContinue() {
			first = false
			opcode = opc
		} else if !first && !opc.IsContinue() {
			return
		}

		if rsv1 {
			panic("TODO: implement `permessage-deflate` extension")
		}

		if rsv2 {
			err = errors.New("RSV2 is set")
			return
		}
		if rsv3 {
			err = errors.New("RSV3 is set")
			return
		}

		b1 := header[1]
		masked := b1&maskBitMask == 1
		payloadLen := int64(b1 & payloadLenBitMask)

		switch payloadLen {
		case 126:
			if extPayloadLen, err := readN(reader, 2); err != nil {
				return nil, err
			} else {
				payloadLen = int64(binary.BigEndian.Uint16(extPayloadLen))
			}
		case 127:
			if extPayloadLen, err := readN(reader, frameMaxPayloadLength); err != nil {
				return nil, err
			} else {
				payloadLen = int64(binary.BigEndian.Uint64(extPayloadLen))
			}
		}

		var mask [frameMaskSize]byte
		if masked {
			var b byte
			for i := 0; i < frameMaskSize; i++ {
				if b, err = reader.ReadByte(); err != nil {
					return nil, err
				} else {
					mask[i] = b
				}
			}
		}

		r := io.LimitReader(reader, payloadLen)
		payload, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}

		for i := 0; masked && i < len(payload); i++ {
			payload[i] ^= mask[i%frameMaskSize]
		}

		_, err = frameBuff.Write(payload)
		if err != nil {
			return nil, err
		}
	}

	return &frameReader{
		opcode: opcode,
		buffer: frameBuff,
		err:    err,
	}, err
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
