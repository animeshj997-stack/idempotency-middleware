package middleware

import (
    "bytes"
    "encoding/hex"
    "hash/fnv"
    "io"
    "net/http"
    "time"

    "idempotency-middleware/internal/recorder"
    "idempotency-middleware/internal/store"
)

// Middleware wraps a handler and ensures POST requests with the same
// Idempotency-Key are replayed safely.
type Middleware struct {
    next      http.Handler
    keyHeader string
    store     *store.Store
}

// entryState represents one logical operation keyed by idempotency key.
type entryState struct {
    key         string
    fingerprint string
    statusCode  int
    headers     http.Header
    body        []byte
    done        chan struct{}
    completed   bool
}

func (e *entryState) markCompleted(statusCode int, headers http.Header, body []byte) {
    if e.completed {
        return
    }

    e.statusCode = statusCode
    if headers != nil {
        e.headers = headers.Clone()
    } else {
        e.headers = make(http.Header)
    }
    e.body = append([]byte(nil), body...)
    e.completed = true
    close(e.done)
}

func (e *entryState) snapshot() (int, http.Header, []byte, bool) {
    headers := make(http.Header)
    for k, values := range e.headers {
        headers[k] = append([]string(nil), values...)
    }

    body := append([]byte(nil), e.body...)
    return e.statusCode, headers, body, e.completed
}

// New creates a middleware that protects POST requests using an in-memory store.
func New(next http.Handler, keyHeader string) *Middleware {
    if next == nil {
        next = http.DefaultServeMux
    }
    if keyHeader == "" {
        keyHeader = "Idempotency-Key"
    }

    return &Middleware{
        next:      next,
        keyHeader: keyHeader,
        store:     store.New(store.WithTTL(5 * time.Minute), store.WithCleanupInterval(30*time.Second)),
    }
}

// Close stops the middleware's internal cleanup goroutine.
func (m *Middleware) Close() {
    if m == nil || m.store == nil {
        return
    }
    m.store.Close()
}

// ServeHTTP intercepts POST requests and enforces idempotency semantics.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r == nil || r.Method != http.MethodPost {
        m.next.ServeHTTP(w, r)
        return
    }

    key := r.Header.Get(m.keyHeader)
    if key == "" {
        m.next.ServeHTTP(w, r)
        return
    }

    fingerprint := fingerprintForRequest(r)

    storedEntry, created, err := m.store.GetOrSet(key, func() *store.Entry {
        entry := &entryState{key: key, fingerprint: fingerprint, done: make(chan struct{})}
        return &store.Entry{Key: key, Value: entry}
    })
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    entry, ok := storedEntry.Value.(*entryState)
    if !ok {
        http.Error(w, "idempotency entry is invalid", http.StatusInternalServerError)
        return
    }

    if !created {
        if entry.fingerprint != fingerprint {
            w.WriteHeader(http.StatusConflict)
            return
        }

        <-entry.done
        statusCode, headers, body, _ := entry.snapshot()
        replayResponse(w, statusCode, headers, body)
        return
    }

    defer func() {
        if recoverValue := recover(); recoverValue != nil {
            _ = m.store.Delete(key)
            entry.markCompleted(http.StatusInternalServerError, nil, nil)
            replayResponse(w, http.StatusInternalServerError, nil, nil)
        }
    }()

    rw := recorder.NewResponseWriter(nil)
    m.next.ServeHTTP(rw, r)

    statusCode := rw.StatusCode()
    if statusCode >= http.StatusInternalServerError {
        _ = m.store.Delete(key)
        entry.markCompleted(statusCode, rw.Header(), rw.Body())
        replayResponse(w, statusCode, rw.Header(), rw.Body())
        return
    }

    entry.markCompleted(statusCode, rw.Header(), rw.Body())

    statusCode, headers, body, _ := entry.snapshot()
    replayResponse(w, statusCode, headers, body)
}

func replayResponse(w http.ResponseWriter, statusCode int, headers http.Header, body []byte) {
    if statusCode == 0 {
        statusCode = http.StatusOK
    }

    for k, values := range headers {
        for _, v := range values {
            w.Header().Add(k, v)
        }
    }

    if len(body) > 0 {
        w.WriteHeader(statusCode)
        _, _ = w.Write(body)
        return
    }

    w.WriteHeader(statusCode)
}

func fingerprintForRequest(r *http.Request) string {
    if r == nil {
        return ""
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        body = nil
    }
    r.Body = io.NopCloser(bytes.NewReader(body))

    h := fnv.New64a()
    _, _ = h.Write([]byte(r.Method))
    _, _ = h.Write([]byte("\n"))
    _, _ = h.Write([]byte(r.URL.Path))
    _, _ = h.Write([]byte("\n"))
    _, _ = h.Write(body)
    return hex.EncodeToString(h.Sum(nil))
}

