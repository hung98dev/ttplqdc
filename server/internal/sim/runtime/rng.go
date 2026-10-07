package runtime

import (
	"thinhthan/internal/core/id"
	"thinhthan/internal/core/rng"
)

// Stream returns the deterministic stream for ctx, lazily creating and
// registering it so identical replay runs observe identical rolls. All
// streams of a partition share the partition's content revision; the caller
// supplies the identity fields (source/operation/event ids or seed) per
// docs/04_architecture/concurrency.md's independent-stream rule.
func (p *Partition) Stream(ctx rng.Context) (*rng.Stream, error) {
	if s, ok := p.streams[ctx]; ok {
		return s, nil
	}
	ctx.ContentRevision = p.cfg.ContentRevision
	s, err := rng.NewStream(ctx)
	if err != nil {
		return nil, err
	}
	p.streams[ctx] = s
	return s, nil
}

// SourceEvent returns the canonical sim source-event name and operation id
// for the partition incarnation counter — e.g. the identity a world
// consequence write carries.
func (p *Partition) SourceEvent() (name string, op id.UUID, err error) {
	n, opID, err := p.incarnation.NextSourceEvent(
		p.cfg.MapID, p.cfg.ChannelID, p.cfg.InstanceID, p.tickN.Load())
	if err != nil {
		return "", id.UUID{}, err
	}
	return n, opID, err
}
