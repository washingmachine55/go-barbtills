package tasks

import (
	"testing"
	"time"
)

func TestElapsedSumsSessionsAndExcludesPausedGaps(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	// Two finished sessions of 5s and 4s with a 3s paused gap between them.
	// The total must be 9s, not the 12s wall-clock span.
	paused := TaskRow{ClosedDuration: 9 * time.Second}
	if got := paused.Elapsed(now); got != 9*time.Second {
		t.Fatalf("paused task: want 9s, got %v", got)
	}
	if paused.IsRunning() {
		t.Fatal("task with no open session must not report as running")
	}

	// Same task resumed 6s ago: the live portion is added to the closed total.
	started := now.Add(-6 * time.Second)
	running := TaskRow{ClosedDuration: 9 * time.Second, OpenStartedAt: &started}
	if got := running.Elapsed(now); got != 15*time.Second {
		t.Fatalf("running task: want 15s, got %v", got)
	}
	if !running.IsRunning() {
		t.Fatal("task with an open session must report as running")
	}

	// A paused task's elapsed must not move as the clock advances; a running
	// one must.
	later := now.Add(30 * time.Second)
	if got := paused.Elapsed(later); got != 9*time.Second {
		t.Fatalf("paused elapsed moved with the clock: got %v", got)
	}
	if got := running.Elapsed(later); got != 45*time.Second {
		t.Fatalf("running elapsed did not advance: got %v", got)
	}
}

func TestElapsedIgnoresClockSkew(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Minute) // session start in the future
	r := TaskRow{ClosedDuration: time.Second, OpenStartedAt: &future}
	if got := r.Elapsed(now); got != time.Second {
		t.Fatalf("negative live portion must not subtract: got %v", got)
	}
}

func TestParseSeq(t *testing.T) {
	for _, in := range []string{"3", "#3", " 3 ", "s3"} {
		n, err := ParseSeq(in)
		if err != nil || n != 3 {
			t.Fatalf("ParseSeq(%q) = %d, %v; want 3, nil", in, n, err)
		}
	}
	for _, in := range []string{"", "abc", "0", "-1", "3x"} {
		if _, err := ParseSeq(in); err == nil {
			t.Fatalf("ParseSeq(%q) should have failed", in)
		}
	}
}

func TestLooksNumeric(t *testing.T) {
	for _, in := range []string{"5", "#5", "42"} {
		if !looksNumeric(in) {
			t.Fatalf("%q should look numeric", in)
		}
	}
	for _, in := range []string{"leetcode practice", "s5", "", "5a"} {
		if looksNumeric(in) {
			t.Fatalf("%q should not look numeric", in)
		}
	}
}

func TestArchivedTimeIsBankedOutOfTheRunningTotal(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	// Yesterday's 5s was rolled over; today has 4s recorded so far.
	r := TaskRow{
		SessionCount: 1, ArchivedSessionCount: 2,
		ClosedDuration: 4 * time.Second, ArchivedDuration: 5 * time.Second,
	}
	if got := r.Elapsed(now); got != 4*time.Second {
		t.Fatalf("current period should be 4s, got %v", got)
	}
	if got := r.Lifetime(now); got != 9*time.Second {
		t.Fatalf("lifetime should be 9s, got %v", got)
	}
	if !r.HasArchive() {
		t.Fatal("task with archived sessions should report an archive")
	}

	// Immediately after a rollover the running total is zero but the history
	// is intact.
	rolled := TaskRow{ArchivedSessionCount: 2, ArchivedDuration: 5 * time.Second}
	if got := rolled.Elapsed(now); got != 0 {
		t.Fatalf("a rolled-over task starts from zero, got %v", got)
	}
	if got := rolled.Lifetime(now); got != 5*time.Second {
		t.Fatalf("rollover must not lose history, got %v", got)
	}

	// A session left running across a rollover keeps counting into the new period.
	started := now.Add(-3 * time.Second)
	spanning := TaskRow{ArchivedDuration: 5 * time.Second, ArchivedSessionCount: 1, OpenStartedAt: &started}
	if got := spanning.Elapsed(now); got != 3*time.Second {
		t.Fatalf("running session should count into the new period, got %v", got)
	}
	if got := spanning.Lifetime(now); got != 8*time.Second {
		t.Fatalf("lifetime should include both, got %v", got)
	}

	// A task never rolled over reports no archive and identical totals.
	plain := TaskRow{ClosedDuration: 7 * time.Second}
	if plain.HasArchive() {
		t.Fatal("task with no archived sessions must not report an archive")
	}
	if plain.Elapsed(now) != plain.Lifetime(now) {
		t.Fatal("without an archive, current and lifetime totals must agree")
	}
}

func TestSessionRowArchivedFlag(t *testing.T) {
	at := time.Now()
	if !(SessionRow{ArchivedAt: &at}).IsArchived() {
		t.Fatal("session with archived_at set must report archived")
	}
	if (SessionRow{}).IsArchived() {
		t.Fatal("session without archived_at must not report archived")
	}
}
