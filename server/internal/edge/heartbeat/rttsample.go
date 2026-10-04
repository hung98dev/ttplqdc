// Package heartbeat implements the application heartbeat of
// docs/05_network/protocol.md § Heartbeat: S2C_HEARTBEAT every 5 s,
// connection lost after 15 s without valid traffic, per-session RTT
// samples from C2S_HEARTBEAT echo_server_ms.
package heartbeat

import "time"

// Sample is one RTT observation per protocol.md § Heartbeat:
// RTT = server receive time of C2S_HEARTBEAT - echo_server_ms, taken only
// when echo_server_ms != 0.
type Sample struct {
	ServerRecv   time.Time
	EchoServerMS uint64
	RTT          time.Duration
}

// SampleFromEcho derives an RTT sample from a C2S_HEARTBEAT. ok=false when
// echo_server_ms == 0 (no sample yet per spec).
func SampleFromEcho(recv time.Time, echoServerMS uint64) (s Sample, ok bool) {
	if echoServerMS == 0 {
		return Sample{}, false
	}
	sent := time.UnixMilli(int64(echoServerMS))
	s = Sample{ServerRecv: recv, EchoServerMS: echoServerMS, RTT: recv.Sub(sent)}
	return s, true
}
