package websocket

type CloseStatus int

// https://datatracker.ietf.org/doc/html/rfc6455#section-7.4
//
// https://datatracker.ietf.org/doc/html/rfc6455#section-11.7
const (
	CloseNormalClosure                    CloseStatus = 1000
	CloseGoingAway                        CloseStatus = 1001
	CloseProtocolError                    CloseStatus = 1002
	CloseUnsupportedData                  CloseStatus = 1003
	CloseReservedButNotSpecifiedByRFC6455 CloseStatus = 1004
	CloseNoStatusReceived                 CloseStatus = 1005
	CloseAbnormalClosure                  CloseStatus = 1006
	CloseInvalidFramePayloadData          CloseStatus = 1007
	ClosePolicyViolation                  CloseStatus = 1008
	CloseMessageTooBig                    CloseStatus = 1009
	CloseMandatoryExtension               CloseStatus = 1010
	CloseInternalServerErr                CloseStatus = 1011
	CloseServiceRestart                   CloseStatus = 1012
	CloseTryAgainLater                    CloseStatus = 1013
	CloseTLSHandshake                     CloseStatus = 1015
)
