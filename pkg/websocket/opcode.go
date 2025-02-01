package websocket

type Opcode byte

// https://datatracker.ietf.org/doc/html/rfc6455#section-11.8
const (
	OpcodeContinueFrame Opcode = 0
	OpcodeTextFrame     Opcode = 1
	OpcodeBinaryFrame   Opcode = 2
	OpcodeCloseFrame    Opcode = 8
	OpcodePingFrame     Opcode = 9
	OpcodePongFrame     Opcode = 10
)

func (op Opcode) Valid() bool {
	return op == OpcodeTextFrame || op == OpcodeBinaryFrame || op == OpcodeCloseFrame || op == OpcodePingFrame || op == OpcodePongFrame || op == OpcodeContinueFrame
}

func (op Opcode) String() string {
	switch op {
	case OpcodeContinueFrame:
		return "CONTINUE"
	case OpcodeTextFrame:
		return "TEXT"
	case OpcodeBinaryFrame:
		return "BINARY"
	case OpcodeCloseFrame:
		return "CLOSE"
	case OpcodePingFrame:
		return "PING"
	case OpcodePongFrame:
		return "PONG"
	default:
		return "INVALID"
	}
}
