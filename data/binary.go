package data

// Traffic
const (
	Inbound = iota
	Outbound
)

// Message types
const (
	NewConnect = iota
	ExitConnect
)

// BinaryData is client and server data transfer format.
type BinaryData struct {
	Traffic     int
	UUID        []byte
	Target      byte
	MessageType byte
	Payload     []byte
}

// NewBinaryData return a BinaryData struct parsed and formatted binary.
func NewBinaryData(b []byte, t int) (BinaryData, error) {
	p := BinaryData{}

	switch t {
	case Inbound: // ref https://github.com/iguagile/iguagile-engine/wiki/protocol#inbound
		p.Traffic = t
		p.Target = b[0:1][0]
		p.MessageType = b[1:2][0]
		p.Payload = b[2:]
	case Outbound: // ref: https://github.com/iguagile/iguagile-engine/wiki/protocol#outbound
		p.Traffic = t
		p.UUID = b[:16]
		p.MessageType = b[16:17][0]
		p.Payload = b[17:]
	}

	return p, nil
}