package middleware

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "sync"
    "sync/atomic"
    "testing"
    "time"
)

func TestMiddlewareNormalRequest(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("created"))
    })

    mw := New(next, "Idempotency-Key")
    req := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req.Header.Set("Idempotency-Key", "abc")
    rr := httptest.NewRecorder()

    mw.ServeHTTP(rr, req)

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected single execution, got %d", atomic.LoadInt32(&calls))
    }
    if rr.Code != http.StatusCreated {
        t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
    }
}

func TestMiddlewareDuplicateRequestReusesCachedResponse(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusAccepted)
        _, _ = w.Write([]byte("done"))
    })

    mw := New(next, "Idempotency-Key")

    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    mw.ServeHTTP(rr1, req1)

    req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    mw.ServeHTTP(rr2, req2)

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected handler to run once, got %d", atomic.LoadInt32(&calls))
    }
    if rr2.Code != http.StatusAccepted {
        t.Fatalf("expected replay status %d, got %d", http.StatusAccepted, rr2.Code)
    }
    if rr2.Body.String() != "done" {
        t.Fatalf("expected replay body %q, got %q", "done", rr2.Body.String())
    }
}

func TestMiddlewareConcurrentDuplicateRequests(t *testing.T) {
    var calls int32
    started := make(chan struct{})
    release := make(chan struct{})

    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        close(started)
        <-release
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("shared"))
    })

    mw := New(next, "Idempotency-Key")

    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()

    req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()

    done1 := make(chan struct{})
    done2 := make(chan struct{})

    go func() {
        mw.ServeHTTP(rr1, req1)
        close(done1)
    }()

    <-started

    go func() {
        mw.ServeHTTP(rr2, req2)
        close(done2)
    }()

    close(release)

    select {
    case <-done1:
    case <-time.After(2 * time.Second):
        t.Fatal("timed out waiting for first request")
    }
    select {
    case <-done2:
    case <-time.After(2 * time.Second):
        t.Fatal("timed out waiting for second request")
    }

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected handler to run once, got %d", atomic.LoadInt32(&calls))
    }
    if rr2.Body.String() != "shared" {
        t.Fatalf("expected replay body %q, got %q", "shared", rr2.Body.String())
    }
}

func TestMiddlewareDifferentBodyWithSameKey(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("ok"))
    })

    mw := New(next, "Idempotency-Key")

    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("first"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    mw.ServeHTTP(rr1, req1)

    req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("second"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    mw.ServeHTTP(rr2, req2)

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected first execution only, got %d", atomic.LoadInt32(&calls))
    }
    if rr2.Code != http.StatusConflict {
        t.Fatalf("expected conflict status %d, got %d", http.StatusConflict, rr2.Code)
    }
}

func TestMiddlewareDifferentPathWithSameKey(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("ok"))
    })

    mw := New(next, "Idempotency-Key")

    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    mw.ServeHTTP(rr1, req1)

    req2 := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString("payload"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    mw.ServeHTTP(rr2, req2)

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected no second execution, got %d", atomic.LoadInt32(&calls))
    }
    if rr2.Code != http.StatusConflict {
        t.Fatalf("expected conflict status %d, got %d", http.StatusConflict, rr2.Code)
    }
}

func TestMiddlewareMissingKeyBypassesIdempotency(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusNoContent)
    })

    mw := New(next, "Idempotency-Key")
    req := httptest.NewRequest(http.MethodPost, "/items", nil)
    rr := httptest.NewRecorder()

    mw.ServeHTTP(rr, req)

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected handler to run once for missing key, got %d", atomic.LoadInt32(&calls))
    }
}

func TestMiddlewareServerErrorsAreNotCached(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusInternalServerError)
        _, _ = w.Write([]byte("boom"))
    })

    mw := New(next, "Idempotency-Key")

    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    mw.ServeHTTP(rr1, req1)

    req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    mw.ServeHTTP(rr2, req2)

    if atomic.LoadInt32(&calls) != 2 {
        t.Fatalf("expected handler to run twice for 5xx, got %d", atomic.LoadInt32(&calls))
    }
    if rr2.Code != http.StatusInternalServerError {
        t.Fatalf("expected replay status %d, got %d", http.StatusInternalServerError, rr2.Code)
    }
}

func TestMiddlewareResponseReplay(t *testing.T) {
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Test", "value")
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("replayed"))
    })

    mw := New(next, "Idempotency-Key")
    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    mw.ServeHTTP(rr1, req1)

    req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    mw.ServeHTTP(rr2, req2)

    if rr2.Code != http.StatusCreated {
        t.Fatalf("expected replay status %d, got %d", http.StatusCreated, rr2.Code)
    }
    if rr2.Header().Get("X-Test") != "value" {
        t.Fatalf("expected replay header %q, got %q", "value", rr2.Header().Get("X-Test"))
    }
    if rr2.Body.String() != "replayed" {
        t.Fatalf("expected replay body %q, got %q", "replayed", rr2.Body.String())
    }
}

func TestMiddlewareRaceFreeExecution(t *testing.T) {
    var wg sync.WaitGroup
    var calls int32

    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })

    mw := New(next, "Idempotency-Key")

    const goroutines = 20
    wg.Add(goroutines)
    for i := 0; i < goroutines; i++ {
        go func() {
            defer wg.Done()
            req := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
            req.Header.Set("Idempotency-Key", "abc")
            rr := httptest.NewRecorder()
            mw.ServeHTTP(rr, req)
        }()
    }

    wg.Wait()

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected one execution under concurrent load, got %d", atomic.LoadInt32(&calls))
    }
}

func TestStoreTTLExpiration(t *testing.T) {
    s := New(nil, "Idempotency-Key")
    _ = s
    _ = time.Second
}
