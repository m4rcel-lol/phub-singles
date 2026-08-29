package ratelimit

import (
	"testing"
	"time"
)

func TestAllowConsumesBurstThenRefills(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(1, 3, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("ip") {
			t.Fatalf("request %d within burst was rejected", i+1)
		}
	}
	if l.Allow("ip") {
		t.Fatal("burst was not enforced")
	}

	now = now.Add(time.Second) // one token refilled
	if !l.Allow("ip") {
		t.Fatal("bucket did not refill")
	}
	if l.Allow("ip") {
		t.Fatal("bucket refilled too much")
	}
}

func TestKeysAreIndependentAndExpire(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(1, 1, time.Minute)
	l.now = func() time.Time { return now }

	if !l.Allow("a") || !l.Allow("b") {
		t.Fatal("separate keys share a bucket")
	}
	if l.Size() != 2 {
		t.Fatalf("expected 2 buckets, got %d", l.Size())
	}

	now = now.Add(2 * time.Minute)
	if removed := l.Cleanup(); removed != 2 {
		t.Fatalf("expected 2 evictions, got %d", removed)
	}
}
