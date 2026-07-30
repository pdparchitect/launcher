package main

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunRefreshLoopChecksImmediatelyAndRepeats(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var calls atomic.Int32

	runRefreshLoop(ctx, time.Millisecond, func(context.Context) {
		if calls.Add(1) == 3 {
			cancel()
		}
	})

	if calls.Load() != 3 {
		t.Fatalf("refresh calls = %d, want 3", calls.Load())
	}
}

func TestRunCatalogueRefreshLoopForcesOnlyStartupCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var calls []bool

	runCatalogueRefreshLoop(
		ctx,
		time.Millisecond,
		func(_ context.Context, force bool) {
			calls = append(calls, force)
			if len(calls) == 3 {
				cancel()
			}
		},
	)

	if !slices.Equal(calls, []bool{true, false, false}) {
		t.Fatalf("refresh force values = %v, want [true false false]", calls)
	}
}
