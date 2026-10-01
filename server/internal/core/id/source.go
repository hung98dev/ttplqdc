package id

import (
	"errors"
	"math"
	"strconv"
)

// Incarnation is one simulation owner's partition incarnation: a fresh
// crypto-random UUIDv4 minted at every owner start/restart together with a
// strictly increasing source event counter (save_rules.md § Database
// Outage). Retries and journal replay never mint a new incarnation for an
// already-finalized event.
type Incarnation struct {
	ID UUID
	// counter is the next source_event_id; 0 marks an exhausted incarnation.
	counter uint64
}

// NewIncarnation mints a partition incarnation whose source event counter
// starts at 1.
func NewIncarnation() Incarnation {
	return Incarnation{ID: NewV4(), counter: 1}
}

// ErrSourceEventCounterExhausted reports a source counter overflow; the
// partition stops before the counter could ever be reused (save_rules.md).
var ErrSourceEventCounterExhausted = errors.New("id: source event counter exhausted")

// SourceEventName returns the canonical ASCII source event name
// "sim:<map_id>:<channel_id>:<instance_id>:<partition_incarnation_id>:<source_event_id>:<tick>"
// (save_rules.md § Database Outage): the channel is the literal 0 for an
// instance source and the instance the literal 0 for a normal channel;
// counter and tick serialize as unsigned decimal without leading zeroes.
func SourceEventName(mapID string, channelID uint64, instanceID UUID, incarnationID UUID, counter, tick uint64) string {
	channel := strconv.FormatUint(channelID, 10)
	instance := "0"
	if !instanceID.IsNil() {
		channel = "0"
		instance = instanceID.String()
	}
	return "sim:" + mapID + ":" + channel + ":" + instance + ":" +
		incarnationID.String() + ":" + strconv.FormatUint(counter, 10) +
		":" + strconv.FormatUint(tick, 10)
}

// SourceOperationID derives the simulation/content-grant operation ID:
// UUIDv5 over ContentGrantNamespaceUUID and the canonical source event name
// (save_rules.md § Database Outage). Replay of one event reuses the retained
// incarnation/counter/tick and reproduces the identical operation, while a
// new incarnation with the same counter and tick yields a different one.
func SourceOperationID(mapID string, channelID uint64, instanceID UUID, incarnationID UUID, counter, tick uint64) UUID {
	return V5(ContentGrantNamespaceUUID,
		SourceEventName(mapID, channelID, instanceID, incarnationID, counter, tick))
}

// NextSourceEvent emits the incarnation's next finalized source event name
// and operation ID, then advances the counter. The counter is never reused:
// reaching the uint64 bound exhausts the incarnation and every later call
// fails with ErrSourceEventCounterExhausted (overflow stops the partition
// before reuse, save_rules.md).
func (i *Incarnation) NextSourceEvent(mapID string, channelID uint64, instanceID UUID, tick uint64) (string, UUID, error) {
	if i.counter == 0 {
		return "", UUID{}, ErrSourceEventCounterExhausted
	}
	counter := i.counter
	if counter == math.MaxUint64 {
		i.counter = 0
	} else {
		i.counter = counter + 1
	}
	name := SourceEventName(mapID, channelID, instanceID, i.ID, counter, tick)
	return name, V5(ContentGrantNamespaceUUID, name), nil
}
