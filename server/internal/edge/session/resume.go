package session

import (
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// consumeResumeLocked redeems a resume credential: single-use, TTL 10 min,
// newest plus predecessor; presenting the newest invalidates the
// predecessor (session.md § Resume Credential).
func (r *Registry) consumeResumeLocked(cred string, now time.Time) (*sess, error) {
	rc, ok := r.resumes[cred]
	if !ok {
		return nil, ErrInvalidCredential
	}
	s := r.resumeSess[cred]
	if now.After(rc.expiresAt) {
		r.dropResumeLocked(cred)
		return nil, ErrResumeExpired
	}
	rc.presented = true
	r.dropResumeLocked(cred)
	if s.resumeCurrent == rc {
		s.resumeCurrent = nil
		// Presenting the newest invalidates the predecessor.
		if s.resumePrev != nil {
			for c, e := range r.resumes {
				if e == s.resumePrev {
					r.dropResumeLocked(c)
					break
				}
			}
			s.resumePrev = nil
		}
	} else if s.resumePrev == rc {
		s.resumePrev = nil
	}
	return s, nil
}

// scheduleRotateLocked arms the 300 s rotation ticker (session.md §
// Resume Credential: S2C_RESUME_CREDENTIAL every 300 s while live).
func (r *Registry) scheduleRotateLocked(s *sess) {
	if s.rotateTimer != nil {
		s.rotateTimer.Stop()
	}
	s.rotateTimer = r.cfg.After(r.cfg.ResumeRotateEvery, func() { r.rotateResume(s) })
}

// rotateResume emits S2C_RESUME_CREDENTIAL every 300 s while the session
// is live, keyed to the bound conn, then re-arms.
func (r *Registry) rotateResume(s *sess) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[s.epoch]; !ok {
		return
	}
	r.scheduleRotateLocked(s)
	if s.conn == nil {
		return
	}
	now := r.cfg.Now()
	s.resumePrev = s.resumeCurrent
	_, credStr, err := r.issueResumeLocked(s)
	if err != nil {
		return
	}
	_ = r.send(s.conn, 16, &protocolv1.S2CResumeCredential{
		ResumeCredential:  credStr,
		ResumeExpiresAtMs: now.Add(r.cfg.ResumeTTL).UnixMilli(),
	})
}

// PruneExpired drops expired tickets/credentials — periodic janitor from
// cmd/server; correctness never depends on it.
func (r *Registry) PruneExpired(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c, t := range r.tickets {
		if now.After(t.expiresAt) {
			delete(r.tickets, c)
		}
	}
	for c, rc := range r.resumes {
		if now.After(rc.expiresAt) {
			delete(r.resumes, c)
			delete(r.resumeSess, c)
		}
	}
}
