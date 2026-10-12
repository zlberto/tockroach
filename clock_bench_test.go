package tockroach_test

import (
	"github.com/zlberto/tockroach"
	"testing"
)

func BenchmarkNow(b *testing.B) {
	if _, err := tockroach.Now(); err != nil {
		b.Skip("system clock unavailable")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tockroach.Now(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNowParallel(b *testing.B) {
	if _, err := tockroach.Now(); err != nil {
		b.Skip("system clock unavailable")
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := tockroach.Now(); err != nil {
				b.Error(err)
				return
			}
		}
	})
}
