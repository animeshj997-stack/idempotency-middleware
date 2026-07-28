package store

import (
    "testing"
    "time"
)

func TestStoreInsertAndLookup(t *testing.T) {
    s := New(WithTTL(100 * time.Millisecond))

    entry := &Entry{Key: "abc", Value: "value"}
    if err := s.Set(entry); err != nil {
        t.Fatalf("Set returned error: %v", err)
    }

    got, ok := s.Get("abc")
    if !ok {
        t.Fatalf("expected entry to be found")
    }
    if got.Value != "value" {
        t.Fatalf("expected value %q, got %q", "value", got.Value)
    }
}

func TestStoreDelete(t *testing.T) {
    s := New(WithTTL(100 * time.Millisecond))

    entry := &Entry{Key: "abc", Value: "value"}
    if err := s.Set(entry); err != nil {
        t.Fatalf("Set returned error: %v", err)
    }

    if err := s.Delete("abc"); err != nil {
        t.Fatalf("Delete returned error: %v", err)
    }

    if _, ok := s.Get("abc"); ok {
        t.Fatalf("expected entry to be deleted")
    }
}

func TestStoreTTLExpiration(t *testing.T) {
    s := New(WithTTL(20 * time.Millisecond), WithCleanupInterval(10*time.Millisecond))

    entry := &Entry{Key: "abc", Value: "value"}
    if err := s.Set(entry); err != nil {
        t.Fatalf("Set returned error: %v", err)
    }

    time.Sleep(80 * time.Millisecond)

    if _, ok := s.Get("abc"); ok {
        t.Fatalf("expected expired entry to be removed")
    }
}

func TestStoreCleanupRemovesExpiredEntries(t *testing.T) {
    s := New(WithTTL(20 * time.Millisecond), WithCleanupInterval(10*time.Millisecond))

    if err := s.Set(&Entry{Key: "a", Value: "1"}); err != nil {
        t.Fatalf("Set returned error: %v", err)
    }
    if err := s.Set(&Entry{Key: "b", Value: "2"}); err != nil {
        t.Fatalf("Set returned error: %v", err)
    }

    time.Sleep(80 * time.Millisecond)

    s.cleanupExpiredLocked()

    if len(s.entries) != 0 {
        t.Fatalf("expected cleanup to remove all expired entries, got %d", len(s.entries))
    }
}
