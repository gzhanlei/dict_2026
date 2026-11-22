package service

import (
	"errors"
	"sync"
	"testing"
)

func TestSearchGuardGlobalLimit(t *testing.T) {
	g := NewSearchGuard(2, 1)

	if err := g.TryAcquire("1.1.1.1"); err != nil {
		t.Fatalf("first acquire should succeed: %v", err)
	}
	if err := g.TryAcquire("2.2.2.2"); err != nil {
		t.Fatalf("second acquire should succeed: %v", err)
	}
	// 全局额度已满，其他IP也被拒绝
	if err := g.TryAcquire("3.3.3.3"); !errors.Is(err, ErrSearchBusy) {
		t.Fatalf("third acquire should be rejected with ErrSearchBusy, got %v", err)
	}

	g.Release("1.1.1.1")
	if err := g.TryAcquire("3.3.3.3"); err != nil {
		t.Fatalf("acquire after release should succeed: %v", err)
	}
	g.Release("2.2.2.2")
	g.Release("3.3.3.3")
}

func TestSearchGuardPerIPLimit(t *testing.T) {
	g := NewSearchGuard(4, 1)

	if err := g.TryAcquire("1.1.1.1"); err != nil {
		t.Fatalf("first acquire should succeed: %v", err)
	}
	// 同IP第二个并发请求被拒，但全局额度尚未满
	if err := g.TryAcquire("1.1.1.1"); !errors.Is(err, ErrSearchBusy) {
		t.Fatalf("second acquire from same IP should be rejected, got %v", err)
	}
	// 其他IP不受影响
	if err := g.TryAcquire("2.2.2.2"); err != nil {
		t.Fatalf("acquire from another IP should succeed: %v", err)
	}

	g.Release("1.1.1.1")
	if err := g.TryAcquire("1.1.1.1"); err != nil {
		t.Fatalf("same IP acquire after release should succeed: %v", err)
	}
	g.Release("2.2.2.2")
	g.Release("1.1.1.1")
}

func TestSearchGuardPerIPCounterCleansUp(t *testing.T) {
	g := NewSearchGuard(4, 2)

	if err := g.TryAcquire("1.1.1.1"); err != nil {
		t.Fatalf("acquire #1 should succeed: %v", err)
	}
	if err := g.TryAcquire("1.1.1.1"); err != nil {
		t.Fatalf("acquire #2 should succeed (perIPLimit=2): %v", err)
	}
	if err := g.TryAcquire("1.1.1.1"); !errors.Is(err, ErrSearchBusy) {
		t.Fatalf("acquire #3 should be rejected, got %v", err)
	}

	// 释放全部额度后，perIP 记录应被清理，不泄漏内存
	g.Release("1.1.1.1")
	g.Release("1.1.1.1")
	g.mu.Lock()
	if len(g.perIP) != 0 {
		t.Fatalf("perIP map should be empty after all releases, got %v", g.perIP)
	}
	g.mu.Unlock()

	if err := g.TryAcquire("1.1.1.1"); err != nil {
		t.Fatalf("acquire after full release should succeed: %v", err)
	}
	g.Release("1.1.1.1")
}

func TestSearchGuardDefaults(t *testing.T) {
	g := NewSearchGuard(0, 0)
	if cap(g.global) != defaultMaxConcurrentSearches {
		t.Fatalf("expected default global limit %d, got %d", defaultMaxConcurrentSearches, cap(g.global))
	}
	if g.perIPLimit != defaultMaxConcurrentPerIP {
		t.Fatalf("expected default per-IP limit %d, got %d", defaultMaxConcurrentPerIP, g.perIPLimit)
	}
}

func TestSearchGuardConcurrentAcquireRelease(t *testing.T) {
	g := NewSearchGuard(4, 4)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ip := "10.0.0.1"
			if err := g.TryAcquire(ip); err == nil {
				g.Release(ip)
			}
		}()
	}
	wg.Wait()

	// 全部释放后应回到初始状态
	g.mu.Lock()
	left := len(g.perIP)
	g.mu.Unlock()
	if left != 0 {
		t.Fatalf("perIP map should be empty after concurrent acquire/release, got %d entries", left)
	}
	if len(g.global) != 0 {
		t.Fatalf("global slots should be empty after all releases, got %d", len(g.global))
	}
}
