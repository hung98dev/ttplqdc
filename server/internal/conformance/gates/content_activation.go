package gates

import (
	"context"
	"os/exec"
	"path/filepath"
)

// contentActivation runs the IMP-004 activation re-verification suite — the
// gate's executable contract per the packet's ## Tests block — on the real
// source tree. config.md § Atomic Activation defines no separate evidence
// artifact, so there is nothing to wire into the manifest; the suite is the
// check. Failing test names surface through failLines before the tail.
func (r *Runner) contentActivation() (details []string, missing bool) {
	if _, err := exec.LookPath("go"); err != nil {
		return nil, true
	}
	out, code := r.runCmd(context.Background(), filepath.Join(r.Root, "server"),
		"go", "test", "./internal/config/")
	if code != 0 {
		return append(failLines(out), "go test ./internal/config/ tail: "+tail(out)), false
	}
	return nil, false
}
