package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunCatalogueRefreshLoopChecksImmediatelyAndRepeats(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var calls atomic.Int32

	runCatalogueRefreshLoop(ctx, time.Millisecond, func(context.Context) {
		if calls.Add(1) == 3 {
			cancel()
		}
	})

	if calls.Load() != 3 {
		t.Fatalf("refresh calls = %d, want 3", calls.Load())
	}
}
