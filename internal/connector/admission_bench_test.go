package connector

import (
	"context"
	"testing"
	"time"
)

func BenchmarkAdmissionAcquireRelease(b *testing.B) {
	a := newAdmissionController(32, time.Second)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		release, admitted := a.Acquire(ctx, "benchmark-connection", 8)
		if !admitted {
			b.Fatal("unexpected admission rejection")
		}
		release()
	}
}

func BenchmarkAdmissionParallelAcquireRelease(b *testing.B) {
	a := newAdmissionController(64, time.Second)
	ctx := context.Background()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			release, admitted := a.Acquire(ctx, "benchmark-connection", 64)
			if !admitted {
				b.Fatal("unexpected admission rejection")
			}
			release()
		}
	})
}
