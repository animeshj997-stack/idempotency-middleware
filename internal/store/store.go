package store

import (
    "errors"
    "sync"
    "time"
)

// Entry is a single value stored by idempotency key.
type Entry struct {
    Key       string
    Value     any
    CreatedAt time.Time
}

// Config controls the in-memory store behavior.
type Config struct {
    TTL             time.Duration
    CleanupInterval time.Duration
}

// Option configures the store.
type Option func(*Config)

// WithTTL sets the maximum lifetime for entries.
func WithTTL(ttl time.Duration) Option {
    return func(cfg *Config) {
        cfg.TTL = ttl
    }
}

// WithCleanupInterval sets how often expired entries are removed.
func WithCleanupInterval(interval time.Duration) Option {
    return func(cfg *Config) {
        cfg.CleanupInterval = interval
    }
}

// Store is a thread-safe in-memory map keyed by idempotency key.
//
// It uses a RWMutex so reads can proceed concurrently while writes are still
// protected. The cleanup loop also takes the write lock before removing stale
// entries, so the map remains race-free.
type Store struct {
    mu      sync.RWMutex
    entries map[string]*Entry
    ttl     time.Duration
    cleanup time.Duration
    stopCh  chan struct{}
    once    sync.Once
}

// New creates a store with the provided options.
func New(options ...Option) *Store {
    cfg := Config{TTL: time.Minute, CleanupInterval: 10 * time.Second}
    for _, option := range options {
        option(&cfg)
    }

    s := &Store{
        entries: make(map[string]*Entry),
        ttl:     cfg.TTL,
        cleanup: cfg.CleanupInterval,
        stopCh:  make(chan struct{}),
    }

    if cfg.CleanupInterval > 0 {
        go s.runCleanupLoop()
    }

    return s
}

// Set stores an entry by key.
func (s *Store) Set(entry *Entry) error {
    if entry == nil {
        return errors.New("store: entry is nil")
    }
    if entry.Key == "" {
        return errors.New("store: key is empty")
    }

    s.mu.Lock()
    defer s.mu.Unlock()
    entry.CreatedAt = time.Now()
    s.entries[entry.Key] = entry
    return nil
}

// Get returns an entry by key if it exists and has not expired.
func (s *Store) Get(key string) (*Entry, bool) {
    if key == "" {
        return nil, false
    }

    s.mu.RLock()
    entry, ok := s.entries[key]
    s.mu.RUnlock()
    if !ok {
        return nil, false
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    entry, ok = s.entries[key]
    if !ok {
        return nil, false
    }

    if s.isExpiredLocked(entry) {
        delete(s.entries, key)
        return nil, false
    }

    return entry, true
}

// GetOrSet returns an existing entry for the key if present, otherwise it creates
// one atomically under the store lock.
func (s *Store) GetOrSet(key string, create func() *Entry) (*Entry, bool, error) {
    if key == "" {
        return nil, false, errors.New("store: key is empty")
    }
    if create == nil {
        return nil, false, errors.New("store: create is nil")
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    if entry, ok := s.entries[key]; ok {
        if s.isExpiredLocked(entry) {
            delete(s.entries, key)
        } else {
            return entry, false, nil
        }
    }

    entry := create()
    if entry == nil {
        return nil, false, errors.New("store: entry is nil")
    }
    if entry.Key == "" {
        entry.Key = key
    }

    entry.CreatedAt = time.Now()
    s.entries[key] = entry
    return entry, true, nil
}

// Delete removes an entry by key.
func (s *Store) Delete(key string) error {
    if key == "" {
        return errors.New("store: key is empty")
    }

    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.entries, key)
    return nil
}

// Close stops the cleanup loop.
func (s *Store) Close() {
    s.once.Do(func() {
        close(s.stopCh)
    })
}

func (s *Store) runCleanupLoop() {
    ticker := time.NewTicker(s.cleanup)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            s.mu.Lock()
            s.cleanupExpiredLocked()
            s.mu.Unlock()
        case <-s.stopCh:
            return
        }
    }
}

func (s *Store) cleanupExpiredLocked() {
    if s.ttl <= 0 {
        return
    }

    for key, entry := range s.entries {
        if s.isExpiredLocked(entry) {
            delete(s.entries, key)
        }
    }
}

func (s *Store) isExpiredLocked(entry *Entry) bool {
    if s.ttl <= 0 {
        return false
    }
    return time.Since(entry.CreatedAt) > s.ttl
}
