package data

import (
	"errors"
)

const lengthUUID = 16
const lengthMessageType = 1
const lengthSubType = 1

// Message types
const (
	systemMessage = iota
	dataMessage
)

// ReceivedData is Client side transfer data struct.

type BinaryData struct {
	UUID        []byte
	MessageType byte
	SubType     byte
	Payload     []byte
}

func NewBinaryData(b []byte) (BinaryData, error) {
	p := BinaryData{}

	p.UUID = b[:lengthUUID]

	p.MessageType = b[lengthUUID : lengthUUID+lengthMessageType][0]

	switch p.MessageType {
	case systemMessage:
		sub := b[lengthUUID+lengthMessageType : lengthUUID+lengthMessageType+lengthSubType]
		p.SubType = sub[0]
		return p, nil

	case dataMessage:
		p.Payload = b[lengthUUID+lengthMessageType+lengthSubType:]
		return p, nil

	default:
		// TODO SET VARIABLE
		return p, errors.New("unknown MessageType %v")
	}
}
