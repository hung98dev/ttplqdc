package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Timeouts of auth.md § Credential Types + session.md.
const (
	TicketTTL         = 60 * time.Second
	ResumeTTL         = 10 * time.Minute
	ResumeRotateEvery = 300 * time.Second
	AdmissionWindow   = 60 * time.Second
	ReconnectGrace    = 30 * time.Second
)

// Config wires the registry's tunables.
type Config struct {
	// Capacity is WORLD_CCU_CAP — accounts holding a slot.
	Capacity int
	// MinProtocolMinor / ServerContentRevision feed HELLO_OK fields.
	ProtocolMinor   uint32
	ContentRevision string
	// TicketTTL/ResumeTTL/Grace/AdmissionWindow/ResumeRotateEvery
	// override the spec defaults (tests).
	TicketTTL         time.Duration
	ResumeTTL         time.Duration
	Grace             time.Duration
	AdmissionWindow   time.Duration
	ResumeRotateEvery time.Duration
	// Now is the injectable clock.
	Now func() time.Time
	// After schedules delayed work (grace expiry, rotation); testable.
	After func(time.Duration, func()) Timer
}

// Timer is the cancellable handle After returns.
type Timer interface {
	Stop() bool
}

type realTimer struct{ *time.Timer }

func (t realTimer) Stop() bool { return t.Timer.Stop() }

func realAfter(d time.Duration, f func()) Timer {
	return realTimer{time.AfterFunc(d, f)}
}

var (
	// ErrOverloaded maps to SERVER_OVERLOADED at the HTTPS edge.
	ErrOverloaded = errors.New("session: login queue full")
	// ErrInvalidCredential maps to AUTH_INVALID on the wire.
	ErrInvalidCredential = errors.New("session: invalid credential")
	// ErrResumeExpired maps to RESUME_EXPIRED.
	ErrResumeExpired = errors.New("session: resume credential expired")
)

// Ticket is one issued gameplay credential.
type Ticket struct {
	Credential    string
	ExpiresAt     time.Time
	QueuePosition int32
	RetryAfterMs  int64
}

// ticket rec is stored against the opaque credential string.
type ticket struct {
	accountID       id.UUID
	clientBuild     uint32
	platform        protocolv1.ClientPlatform
	protocolMinor   uint32
	contentRevision string
	expiresAt       time.Time
}

// resumeCred is one outstanding resume credential (single-use).
type resumeCred struct {
	cred      string
	epoch     uint64
	expiresAt time.Time
	presented bool
}

// sess is one live session.
type sess struct {
	id        id.UUID
	accountID id.UUID
	epoch     uint64

	conn           *listener.Conn // bound on first inbound
	attaching      bool           // reserved → waiting HELLO→attach
	charID         *id.UUID       // attached character
	ownershipEpoch uint64         // minted per attach (messages.md id 7)

	contentRevision string
	platform        protocolv1.ClientPlatform
	pendingDeletion bool

	resumeCurrent *resumeCred
	resumePrev    *resumeCred

	graceTimer  Timer
	rotateTimer Timer
}

// credString renders raw bytes as one opaque credential string.
func credString(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

// fillRandom fills b from crypto/rand.
func fillRandom(b []byte) error {
	_, err := rand.Read(b)
	return err
}

// Registry owns sessions, the login queue and credential machines.
type Registry struct {
	cfg   Config
	store *account.Store
	q     *queue.Queue

	mu         sync.Mutex
	queue      *loginQueue
	tickets    map[string]*ticket
	resumes    map[string]*resumeCred // cred → cred entry (key lookup)
	resumeSess map[string]*sess
	sessions   map[uint64]*sess // epoch → session
	byAccount  map[id.UUID]*sess
	liveChar   map[id.UUID]*sess  // charID → session holding it
	ownership  map[id.UUID]uint64 // charID → last ownership epoch
	conns      map[*listener.Conn]*sess
	router     DurableRouter
}

// New builds the session registry. store backs ownership/login-signal
// reads; q receives character.activity records.
func New(cfg Config, store *account.Store, q *queue.Queue) *Registry {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.After == nil {
		cfg.After = realAfter
	}
	if cfg.TicketTTL <= 0 {
		cfg.TicketTTL = TicketTTL
	}
	if cfg.ResumeTTL <= 0 {
		cfg.ResumeTTL = ResumeTTL
	}
	if cfg.ResumeRotateEvery <= 0 {
		cfg.ResumeRotateEvery = ResumeRotateEvery
	}
	if cfg.Grace <= 0 {
		cfg.Grace = ReconnectGrace
	}
	if cfg.AdmissionWindow <= 0 {
		cfg.AdmissionWindow = AdmissionWindow
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = 1
	}
	return &Registry{
		cfg:        cfg,
		store:      store,
		q:          q,
		queue:      newLoginQueue(cfg.Capacity, cfg.AdmissionWindow),
		tickets:    make(map[string]*ticket),
		resumes:    make(map[string]*resumeCred),
		resumeSess: make(map[string]*sess),
		sessions:   make(map[uint64]*sess),
		byAccount:  make(map[id.UUID]*sess),
		liveChar:   make(map[id.UUID]*sess),
		ownership:  make(map[id.UUID]uint64),
		conns:      make(map[*listener.Conn]*sess),
	}
}

// Queue returns the durable queue for router submissions.
func (r *Registry) Queue() *queue.Queue { return r.q }

// mintEpoch mints a strictly-increasing per-account session epoch:
// unix-ms at HELLO, bumped past any previously issued epoch so the newer
// session always wins (session.md § Session Epoch).
func (r *Registry) mintEpochLocked(accountID id.UUID) uint64 {
	e := uint64(r.cfg.Now().UnixMilli())
	if old, ok := r.byAccount[accountID]; ok && e <= old.epoch {
		e = old.epoch + 1
	}
	return e
}

// mintOwnershipEpoch advances the per-character ownership epoch.
func (r *Registry) mintOwnershipEpochLocked(charID id.UUID) uint64 {
	r.ownership[charID]++
	return r.ownership[charID]
}

// send marshals msg and enqueues it on the bound conn.
func (r *Registry) send(c *listener.Conn, msgID uint32, msg proto.Message) error {
	if c == nil {
		return nil // session not yet bound to a conn
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Send(&protocolv1.Envelope{MessageId: msgID, Payload: payload}, listener.DeliveryControl)
}

// issueResumeLocked stores one resume credential for the session and
// returns its wire form (keeps newest + predecessor per session.md
// § Resume Credential).
func (r *Registry) issueResumeLocked(s *sess) (*resumeCred, string, error) {
	var raw [32]byte
	if err := fillRandom(raw[:]); err != nil {
		return nil, "", err
	}
	cred := credString(raw[:])
	rc := &resumeCred{epoch: s.epoch, expiresAt: r.cfg.Now().Add(r.cfg.ResumeTTL)}
	rc.cred = cred
	r.resumes[cred] = rc
	r.resumeSess[cred] = s
	s.resumeCurrent = rc
	return rc, cred, nil
}

// dropResumeLocked removes a presented/expired credential.
func (r *Registry) dropResumeLocked(cred string) {
	delete(r.resumes, cred)
	delete(r.resumeSess, cred)
}
