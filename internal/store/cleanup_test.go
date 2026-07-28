package store

import (
    "testing"
    "time"
)

func TestStoreCloseStopsCleanupLoop(t *testing.T) {
    s := New(WithTTL(20*time.Millisecond), WithCleanupInterval(10*time.Millisecond))
    if err := s.Set(&Entry{Key: "a", Value: "1"}); err != nil {
        t.Fatalf("Set returned error: %v", err)
    }

    s.Close()
    time.Sleep(30 * time.Millisecond)

    if _, ok := s.Get("a"); ok {
        t.Fatalf("expected entry to be removed after close, but it still exists")
    }
}
