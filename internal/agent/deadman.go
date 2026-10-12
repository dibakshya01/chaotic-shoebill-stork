// Package agent holds the agent-side safety mechanism: a dead-man's switch that
// reverts an injection if the control plane stops heart-beating or a hard deadline
// passes — WITHOUT the control plane. This is why the platform cannot cause an incident
// by being down: the fault self-heals.
package agent

import (
	"context"
	"sync"
	"time"
)

// Revert functions are called with the reason the switch fired.
type RevertFunc func(reason string)

// Reasons the switch reverts.
const (
	ReasonControlPlaneUnreachable = "control_plane_unreachable"
	ReasonHardDeadline            = "hard_deadline_exceeded"
	ReasonContextCancelled        = "context_cancelled"
)

// DeadMansSwitch reverts an injection when heartbeats stop or the hard deadline passes.
// It is deterministic: Check(now) is pure given the recorded heartbeat, so tests can
// drive it without sleeping. Run(ctx) wraps Check on a ticker for production use.
type DeadMansSwitch struct {
	mu           sync.Mutex
	lastAck      time.Time
	hbTimeout    time.Duration
	hardDeadline time.Time
	revert       RevertFunc
	fired        bool
	reason       string
}

// New arms a switch. startedAt is when injection began; maxDuration is the experiment's
// declared duration; hbTimeout is how long the control plane may be silent before the
// fault is reverted. The hard deadline is startedAt + maxDuration + 20%.
func New(startedAt time.Time, maxDuration, hbTimeout time.Duration, revert RevertFunc) *DeadMansSwitch {
	return &DeadMansSwitch{
		lastAck:      startedAt,
		hbTimeout:    hbTimeout,
		hardDeadline: startedAt.Add(maxDuration + maxDuration/5),
		revert:       revert,
	}
}

// Heartbeat records a liveness signal from the control plane.
func (d *DeadMansSwitch) Heartbeat(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.fired {
		d.lastAck = now
	}
}

// Fired reports whether the switch has reverted, and why.
func (d *DeadMansSwitch) Fired() (bool, string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fired, d.reason
}

// Disarm stops the switch WITHOUT reverting — the control plane calls it after it has
// cleanly reverted the injection itself, so neither Check nor the Run loop's
// context-cancel path fires a second (spurious) revert.
func (d *DeadMansSwitch) Disarm() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.fired {
		d.fired = true
		d.reason = "disarmed"
	}
}

// Fire invokes the revert callback once (idempotent) with the given reason. The control
// plane calls it to force a revert when its own revert failed — a synchronous safety net
// that does not depend on the background Run loop.
func (d *DeadMansSwitch) Fire(reason string) {
	d.mu.Lock()
	if d.fired {
		d.mu.Unlock()
		return
	}
	d.fired = true
	d.reason = reason
	revert := d.revert
	d.mu.Unlock()
	if revert != nil {
		revert(reason)
	}
}

// Check evaluates the switch at time now, reverting if the heartbeat is stale or the
// hard deadline passed. It is idempotent (fires at most once). Returns true once fired.
func (d *DeadMansSwitch) Check(now time.Time) bool {
	d.mu.Lock()
	if d.fired {
		d.mu.Unlock()
		return true
	}
	var reason string
	switch {
	case now.Sub(d.lastAck) > d.hbTimeout:
		reason = ReasonControlPlaneUnreachable
	case !now.Before(d.hardDeadline):
		reason = ReasonHardDeadline
	}
	if reason == "" {
		d.mu.Unlock()
		return false
	}
	d.fired = true
	d.reason = reason
	revert := d.revert
	d.mu.Unlock()
	if revert != nil {
		revert(reason)
	}
	return true
}

// Run drives the switch on a ticker until it fires or ctx is done. On ctx cancellation
// it reverts with ReasonContextCancelled. Production callers use this; tests use Check.
func (d *DeadMansSwitch) Run(ctx context.Context, tick time.Duration) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			if !d.fired {
				d.fired = true
				d.reason = ReasonContextCancelled
				revert := d.revert
				d.mu.Unlock()
				if revert != nil {
					revert(ReasonContextCancelled)
				}
				return
			}
			d.mu.Unlock()
			return
		case now := <-t.C:
			if d.Check(now) {
				return
			}
		}
	}
}
