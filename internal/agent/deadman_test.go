package agent

import (
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/sim"
)

func TestDeadMansSwitchRevertsOnControlPlaneSilence(t *testing.T) {
	env := sim.New()
	env.Inject("run-1")
	if !env.Active() {
		t.Fatal("precondition: env should be active after inject")
	}

	start := time.Unix(1_700_000_000, 0).UTC()
	dms := New(start, 60*time.Second /*maxDuration*/, 10*time.Second /*hbTimeout*/, func(reason string) {
		env.Revert()
	})

	// While the control plane heartbeats, the switch holds.
	dms.Heartbeat(start.Add(2 * time.Second))
	if dms.Check(start.Add(5 * time.Second)) {
		t.Fatal("switch fired while heartbeats were fresh")
	}
	if env.Reverted() {
		t.Fatal("fault reverted while control plane was alive")
	}

	// Control plane goes silent. After hbTimeout, the switch reverts WITHOUT the
	// control plane — the fault self-heals.
	if !dms.Check(start.Add(20 * time.Second)) {
		t.Fatal("switch should have fired after heartbeat timeout")
	}
	if !env.Reverted() {
		t.Fatal("dead-man's switch did not revert the injection")
	}
	fired, reason := dms.Fired()
	if !fired || reason != ReasonControlPlaneUnreachable {
		t.Fatalf("expected control_plane_unreachable, got fired=%v reason=%q", fired, reason)
	}
}

func TestDeadMansSwitchHardDeadline(t *testing.T) {
	env := sim.New()
	env.Inject("run-2")
	start := time.Unix(1_700_000_000, 0).UTC()
	// Long heartbeat window so only the hard deadline can fire.
	dms := New(start, 60*time.Second, time.Hour, func(string) { env.Revert() })
	// Keep heart-beating so the hb timeout never trips.
	dms.Heartbeat(start.Add(60 * time.Second))
	// Hard deadline = start + 60s + 20% = 72s. At 80s it must fire.
	if !dms.Check(start.Add(80 * time.Second)) {
		t.Fatal("switch should have fired at the hard deadline")
	}
	_, reason := dms.Fired()
	if reason != ReasonHardDeadline {
		t.Fatalf("expected hard_deadline_exceeded, got %q", reason)
	}
}
