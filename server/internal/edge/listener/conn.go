package listener

import (
	"context"
	"errors"
	"io"

	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/edge/heartbeat"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Conn is one accepted WebSocket connection: ordered validation on the
// inbound path, bounded queue + supersede on the outbound path, heartbeat
// and the handshake window. All mutation of the validated state happens on
// the read goroutine; Send/SetPhase/Close are safe from other goroutines.
type Conn struct {
	l    *Listener
	ws   *websocket.Conn
	meta HelloMeta

	mu    sync.Mutex // guards st, queue, attached, serverSeq
	st    *ConnState
	queue *outboundQueue
	// wake nudges the write loop when the queue goes non-empty.
	wake chan struct{}
	// closed is latched once the WS close handshake has been initiated.
	closed    bool
	closeCode int
	closeWhy  string
	serverSeq uint64
	done      chan struct{} // closed when the conn is fully torn down

	lastValid    atomic.Int64 // unix-ms of last valid traffic (heartbeat watchdog)
	attachOK     atomic.Bool  // S2C_CHARACTER_ATTACH_OK (7) delivered
	baselineSeen atomic.Bool  // S2C_WORLD_BASELINE (300) delivered

	inbound chan Inbound // bounded pre-attach queue (Config.PreAttachInbound)
}

// handshake-specific flags set by the HELLO path.
func newConn(l *Listener, ws *websocket.Conn, meta HelloMeta) *Conn {
	c := &Conn{
		l:       l,
		ws:      ws,
		meta:    meta,
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
		inbound: make(chan Inbound, l.cfg.PreAttachInbound),
	}
	c.st = &ConnState{
		Phase:         PhasePreHello,
		protocolMajor: l.cfg.ProtocolMajor,
		now:           l.deps.Now,
	}
	c.queue = newOutboundQueue(l.cfg.OutboundFrames, l.cfg.OutboundBytes, l.deps.Now)
	c.lastValid.Store(l.deps.Now().UnixMilli())
	return c
}

// Send enqueues one outbound frame: `env` is the fully populated S2C
// envelope except server_seq, which the write loop assigns in wire order
// (protocol.md § Envelope). Non-replaceable frames that cannot fit close
// the connection with WS 4008 and return ErrSlowConsumer.
func (c *Conn) Send(env proto.Message, class DeliveryClass) error {
	e, ok := env.(*protocolv1.Envelope)
	if !ok {
		return errors.New("edge/listener: Send expects *protocolv1.Envelope")
	}
	// The registry's delivery class is authoritative for wire ids (a
	// mismatched caller hint cannot downgrade REPLACEABLE_STATE).
	if ent := lookup(e.MessageId); ent != nil {
		class = ent.class
	}
	payload := make([]byte, len(e.Payload))
	copy(payload, e.Payload)
	entry := &outEntry{
		msgID:         e.MessageId,
		correlationID: e.CorrelationId,
		payload:       payload,
		class:         class,
		estBytes:      len(payload) + envOverhead,
		supKey:        supKeyFor(e.MessageId, payload),
	}
	c.mu.Lock()
	err := c.queue.offer(entry)
	if err == nil {
		c.l.deps.Metrics.QueueDepth(c.queue.frames, c.queue.bytes)
		c.noteOutbound(e.MessageId)
	}
	c.mu.Unlock()
	if err != nil {
		c.closeWith(WSCloseSlowConsumer, "SLOW_CONSUMER")
		return err
	}
	c.nudge()
	return nil
}

// noteOutbound drives the phase transitions keyed on outbound S2C ids
// (protocol.md § Phase Legality): attach/baseline/death/transfer/pending.
// Called with c.mu held.
func (c *Conn) noteOutbound(msgID uint32) {
	switch msgID {
	case 7: // S2C_CHARACTER_ATTACH_OK — exits PLACEMENT_PENDING too
		c.attachOK.Store(true)
		c.st.Attached = true
		c.maybeEnterWorld()
		if c.st.Phase == PhasePlacementPending && !c.baselineSeen.Load() {
			c.st.Phase = PhaseCharacterSelect
		}
	case 300: // S2C_WORLD_BASELINE
		c.baselineSeen.Store(true)
		c.maybeEnterWorld()
	case 11: // S2C_CHARACTER_DETACH_OK — spec also enters CHARACTER_SELECT
		c.st.Phase = PhaseCharacterSelect
		c.st.Attached = false
		c.attachOK.Store(false)
		c.baselineSeen.Store(false)
	case 15: // S2C_PLACEMENT_PENDING
		c.st.Phase = PhasePlacementPending
	case 105: // S2C_TRANSFER_PREPARE
		c.st.Phase = PhaseTransfer
		c.baselineSeen.Store(false) // new baseline re-enters IN_WORLD
	case 206: // S2C_DEATH
		c.st.Phase = PhaseDead
	case 207: // S2C_RESPAWN
		c.st.Phase = PhaseInWorld
	}
}

// maybeEnterWorld promotes to IN_WORLD once both the attach ack and the
// world baseline have been delivered (protocol.md § Phase Legality:
// IN_WORLD = entered by 7 + baseline (300) received). Until then the
// connection stays in CHARACTER_SELECT — the attach window dispatches
// the CHARACTER_SELECT set {4, 6, 12}.
func (c *Conn) maybeEnterWorld() {
	if c.attachOK.Load() && c.baselineSeen.Load() {
		c.st.Phase = PhaseInWorld
	}
}

// SetPhase is the session adapter's override for phase transitions the
// listener cannot infer from outbound frames (e.g. session resume driving
// an immediate IN_WORLD). Adapters normally do not need it.
func (c *Conn) SetPhase(p Phase) {
	c.mu.Lock()
	c.st.Phase = p
	c.mu.Unlock()
}

// Close terminates the connection with the WS status code and reason.
func (c *Conn) Close(code int, reason string) { c.closeWith(code, reason) }

func (c *Conn) closeWith(code int, reason string) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.closeCode = code
	c.closeWhy = reason
	c.mu.Unlock()
	_ = c.ws.Close(websocket.StatusCode(code), reason)
}

func (c *Conn) nudge() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// run is the connection's lifecycle: handshake window, heartbeat monitor,
// inbound delivery, read loop, write loop. It returns when the WS is
// closed and torn down.
func (c *Conn) run(ctx context.Context) {
	defer close(c.done)
	defer c.l.forget(c)
	defer c.l.deps.Metrics.ConnClosed(c.closeCode, c.closeWhy)
	if d := c.l.deps.Disconnect; d != nil {
		defer d.Disconnected(c, c.closeCode, c.closeWhy)
	}

	// Delivery goroutine: pops the bounded inbound queue into the sink in
	// receive order. The queue depth is Config.PreAttachInbound; post-attach
	// delivery keeps the same bounded channel (order-preserving).
	deliverCtx, deliverCancel := context.WithCancel(ctx)
	defer deliverCancel()
	go c.deliverLoop(deliverCtx)

	// Handshake window: a HELLO not received within HelloWindow of the
	// upgrade closes with AUTH_REQUIRED (protocol.md § Handshake).
	helloTimer := time.AfterFunc(c.l.cfg.HelloWindow, func() {
		c.mu.Lock()
		pre := c.st.Phase == PhasePreHello
		c.mu.Unlock()
		if pre {
			c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_AUTH_REQUIRED, protocolv1.Retryability_RETRYABILITY_NEVER, wsClosePolicy, "AUTH_REQUIRED")
		}
	})
	defer helloTimer.Stop()

	// Heartbeat: S2C_HEARTBEAT every interval; conn lost after ConnTimeout
	// without valid traffic (protocol.md § Heartbeat).
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	mon := &heartbeat.Monitor{
		Interval: c.l.cfg.HeartbeatInterval,
		Timeout:  c.l.cfg.ConnTimeout,
		Now:      c.l.deps.Now,
		Send: func(serverMS uint64) error {
			return c.sendS2C(&protocolv1.S2CHeartbeat{ServerMs: serverMS}, 5, DeliveryControl)
		},
		LastValidMS: func() int64 { return c.lastValid.Load() },
		Lost: func() {
			c.closeWith(wsCloseNormal, "HEARTBEAT_TIMEOUT")
		},
	}
	go mon.Run(hbCtx)

	// Write loop: drains the outbound queue, assigns server_seq, encodes
	// envelope+payload into pooled buffers, writes WS binary frames.
	writeCtx, writeCancel := context.WithCancel(ctx)
	defer writeCancel()
	go c.writeLoop(writeCtx)

	c.readLoop(ctx)
}

// sendS2C marshals `msg` as the payload of an envelope with the given
// message id and class, then enqueues it. Used for listener-owned frames
// (heartbeat, errors, HELLO_OK).
func (c *Conn) sendS2C(msg proto.Message, msgID uint32, class DeliveryClass) error {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Send(&protocolv1.Envelope{MessageId: msgID, Payload: payload}, class)
}

// sendError emits S2C_ERROR (id 3) with close_after per protocol.md
// § Envelope Validation.
func (c *Conn) sendError(code protocolv1.ErrorCode, retry protocolv1.Retryability, corr uint64, closeAfter bool) error {
	e := &protocolv1.S2CError{
		ErrorCode:    code,
		Retryability: retry,
		CloseAfter:   closeAfter,
	}
	payload, err := proto.Marshal(e)
	if err != nil {
		return err
	}
	return c.Send(&protocolv1.Envelope{MessageId: 3, CorrelationId: corr, Payload: payload}, DeliveryControl)
}

// applyReject acts on a *Reject from the validation table: optional S2C_ERROR
// frame, reject-budget accounting, optional close.
func (c *Conn) applyReject(env *protocolv1.Envelope, r *Reject) {
	if r.SilentDrop {
		return
	}
	if r.Close {
		// Closing rejections write the error frame directly before the
		// close handshake so it reaches the wire ahead of the close frame.
		if r.ErrCode != 0 {
			c.sendErrorNow(r.ErrCode, r.Retryability, env.ClientSeq)
		}
		c.closeWith(r.CloseStatus, r.CloseReason)
		return
	}
	if r.ErrCode != 0 {
		_ = c.sendError(r.ErrCode, r.Retryability, env.ClientSeq, false)
	}
	// Non-closing rejections count toward the reject budget; rate-limit
	// rejections count toward their own buckets, never the budget.
	if r.ErrCode != protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED {
		c.l.deps.Metrics.Reject(r.CloseReason)
		if over := c.st.recordReject(c.st.clock(), c.l.cfg.RejectWindow); over != nil {
			c.closeWith(over.CloseStatus, over.CloseReason)
		}
	}
}

// deliverLoop feeds the intent sink from the bounded inbound channel in
// order until the connection closes.
func (c *Conn) deliverLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case f := <-c.inbound:
			if c.l.deps.Intents == nil {
				continue
			}
			if err := c.l.deps.Intents.Enqueue(ctx, c, f); err != nil {
				c.l.logf("listener: intent enqueue failed: %v", err)
			}
		}
	}
}

// readLoop consumes inbound frames until close: bounded read → parse →
// Validate → dispatch / drop / reject per the ordered table.
func (c *Conn) readLoop(ctx context.Context) {
	for {
		// A closed conn also unwinds this loop via ws.Read's error.
		typ, r, err := c.ws.Reader(ctx)
		if err != nil {
			return // peer close, timeout, or our own Close
		}
		if c.isClosed() {
			return
		}

		// Row (a): one binary message = one envelope, at most ReadLimit
		// bytes. Non-binary frames and oversized frames close.
		if typ != websocket.MessageBinary {
			c.closeWith(wsCloseProtocol, "PROTOCOL_MALFORMED")
			return
		}
		lr := io.LimitReader(r, c.l.cfg.ReadLimit+1)
		buf, err := io.ReadAll(lr)
		if err != nil {
			return
		}
		if int64(len(buf)) > c.l.cfg.ReadLimit {
			c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_MESSAGE_TOO_LARGE, protocolv1.Retryability_RETRYABILITY_NEVER, wsCloseTooBig, "MESSAGE_TOO_LARGE")
			return
		}
		var env protocolv1.Envelope
		if err := proto.Unmarshal(buf, &env); err != nil {
			c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED, protocolv1.Retryability_RETRYABILITY_NEVER, wsCloseProtocol, "PROTOCOL_MALFORMED")
			return
		}

		c.mu.Lock()
		rej := Validate(&env, c.st)
		c.mu.Unlock()
		if rej != nil {
			// A silently dropped frame is still valid protocol traffic —
			// it proves liveness for the heartbeat watchdog.
			if rej.SilentDrop {
				c.lastValid.Store(c.st.clock().UnixMilli())
			}
			c.applyReject(&env, rej)
			if rej.Close {
				return
			}
			continue
		}
		c.lastValid.Store(c.st.clock().UnixMilli())
		if !c.dispatch(ctx, &env) {
			return
		}
	}
}

func (c *Conn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// sendAndClose emits the error frame then closes with `status`.
func (c *Conn) sendAndClose(code protocolv1.ErrorCode, retry protocolv1.Retryability, status int, reason string) {
	c.sendErrorNow(code, retry, 0)
	c.closeWith(status, reason)
}

// sendErrorNow writes S2C_ERROR synchronously (bypassing the queue) — used
// only on closing paths so the frame is guaranteed ahead of the close
// handshake. server_seq is assigned under the conn mutex like the write
// loop's frames.
func (c *Conn) sendErrorNow(code protocolv1.ErrorCode, retry protocolv1.Retryability, corr uint64) {
	e := &protocolv1.S2CError{ErrorCode: code, Retryability: retry, CloseAfter: true}
	payload, err := proto.Marshal(e)
	if err != nil {
		return
	}
	c.mu.Lock()
	c.serverSeq++
	env := &protocolv1.Envelope{
		ProtocolMajor: c.l.cfg.ProtocolMajor,
		ProtocolMinor: c.l.cfg.ProtocolMinor,
		MessageId:     3,
		SessionEpoch:  c.st.SessionEpoch,
		ServerSeq:     c.serverSeq,
		CorrelationId: corr,
	}
	c.mu.Unlock()
	dst := EncodeEnvelope(make([]byte, 0, 256), env, payload)
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.ws.Write(wctx, websocket.MessageBinary, dst)
}

// dispatch routes a validated frame: HELLO → handshake, HEARTBEAT →
// RTT/watchdog, everything else → payload decode → intent sink.
func (c *Conn) dispatch(ctx context.Context, env *protocolv1.Envelope) bool {
	switch env.MessageId {
	case 1:
		return c.handleHello(ctx, env)
	case 4:
		return c.handleHeartbeat(env)
	}
	ent := lookup(env.MessageId)
	if ent == nil {
		return true // unreachable — Validate row (f) rejects unknowns
	}
	// Row (j): payload parse failure — PROTOCOL_MALFORMED without close.
	payload := ent.newPayload()
	if payload == nil || proto.Unmarshal(env.Payload, payload) != nil {
		c.applyReject(env, RejectPayloadParse())
		return true
	}
	// Row (k): per-message rate buckets.
	c.mu.Lock()
	ok := c.st.checkRateLimit(ent, c.st.clock())
	c.mu.Unlock()
	if !ok {
		c.applyReject(env, RejectRateLimited())
		return true
	}
	// Dispatch into the bounded inbound queue; delivery preserves order.
	f := Inbound{
		MessageID:     env.MessageId,
		SessionEpoch:  env.SessionEpoch,
		ClientSeq:     env.ClientSeq,
		CorrelationID: env.CorrelationId,
		Payload:       payload,
	}
	select {
	case c.inbound <- f:
	case <-ctx.Done():
		return false
	case <-c.done:
		return false
	}
	return true
}

// handleHeartbeat consumes C2S_HEARTBEAT: marks valid traffic and derives
// the RTT sample from echo_server_ms when non-zero (protocol.md §Heartbeat).
func (c *Conn) handleHeartbeat(env *protocolv1.Envelope) bool {
	var hb protocolv1.C2SHeartbeat
	if proto.Unmarshal(env.Payload, &hb) != nil {
		c.applyReject(env, RejectPayloadParse())
		return true
	}
	recv := c.st.clock()
	if s, ok := heartbeat.SampleFromEcho(recv, hb.EchoServerMs); ok && c.l.deps.RTT != nil {
		c.l.deps.RTT.Publish(c, s)
	}
	return true
}

// writeLoop drains the queue on wake, encoding into pooled buffers.
func (c *Conn) writeLoop(ctx context.Context) {
	dst := make([]byte, 0, 64<<10)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-tick.C:
		case <-c.wake:
		}
		for {
			c.mu.Lock()
			e := c.queue.pop()
			if e == nil {
				c.mu.Unlock()
				break
			}
			c.serverSeq++
			seq := c.serverSeq
			epoch := c.st.SessionEpoch
			c.mu.Unlock()

			env := &protocolv1.Envelope{
				ProtocolMajor: c.l.cfg.ProtocolMajor,
				ProtocolMinor: c.l.cfg.ProtocolMinor,
				MessageId:     e.msgID,
				ServerSeq:     seq,
				CorrelationId: e.correlationID,
				SessionEpoch:  epoch,
			}

			dst = dst[:0]
			dst = EncodeEnvelope(dst, env, e.payload)
			wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := c.ws.Write(wctx, websocket.MessageBinary, dst)
			cancel()
			if err != nil {
				return // conn dead
			}
		}
		// 75%-for-5s slow consumer rule.
		c.mu.Lock()
		slow := c.queue.checkSlowConsumer(c.st.clock())
		c.mu.Unlock()
		if slow {
			c.closeWith(WSCloseSlowConsumer, "SLOW_CONSUMER")
			return
		}
	}
}
