package world

import "thinhthan/internal/edge/router"

// Register installs the durable ids on the router and binds 208 on the
// ADR-0082 non-durable table.
func (s *Service) Register(rt *router.Registry) error {
	for _, b := range []struct {
		msgID uint32
		h     router.Handler
	}{
		{msgIDInteract, s.interact},
		{msgIDPortal, s.portal},
		{msgIDChannel, s.channel},
	} {
		if err := rt.Register(b.msgID, b.h); err != nil {
			return err
		}
	}
	return rt.RegisterNonDurable(msgIDRespawn, s.respawn)
}
