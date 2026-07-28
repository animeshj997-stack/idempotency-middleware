package middleware

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
    "time"
)

func TestMiddlewareReplaysCompletedResponse(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("ok"))
    })

    middleware := New(next, "Idempotency-Key")

    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    middleware.ServeHTTP(rr1, req1)

    req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    middleware.ServeHTTP(rr2, req2)

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected handler to run once, got %d", atomic.LoadInt32(&calls))
    }

    if rr1.Code != http.StatusCreated || rr2.Code != http.StatusCreated {
        t.Fatalf("expected both responses to be %d, got %d and %d", http.StatusCreated, rr1.Code, rr2.Code)
    }

    if rr2.Body.String() != "ok" {
        t.Fatalf("expected replay body %q, got %q", "ok", rr2.Body.String())
    }
}

func TestMiddlewareIgnoresRequestsWithoutKey(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusNoContent)
    })

    middleware := New(next, "Idempotency-Key")

    req := httptest.NewRequest(http.MethodPost, "/items", nil)
    rr := httptest.NewRecorder()
    middleware.ServeHTTP(rr, req)

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected handler to run once for missing key, got %d", atomic.LoadInt32(&calls))
    }
}

func TestMiddlewareWaitsForInFlightRequest(t *testing.T) {
    var calls int32
    started := make(chan struct{})
    release := make(chan struct{})

    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        close(started)
        <-release
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("done"))
    })

    middleware := New(next, "Idempotency-Key")

    firstReq := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    firstReq.Header.Set("Idempotency-Key", "abc")
    firstRR := httptest.NewRecorder()

    done := make(chan struct{})
    go func() {
        middleware.ServeHTTP(firstRR, firstReq)
        close(done)
    }()

    <-started

    secondReq := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
    secondReq.Header.Set("Idempotency-Key", "abc")
    secondRR := httptest.NewRecorder()
    secondDone := make(chan struct{})
    go func() {
        middleware.ServeHTTP(secondRR, secondReq)
        close(secondDone)
    }()

    close(release)

    select {
    case <-done:
    case <-time.After(2 * time.Second):
        t.Fatal("timed out waiting for first request to complete")
    }

    select {
    case <-secondDone:
    case <-time.After(2 * time.Second):
        t.Fatal("timed out waiting for replay request to complete")
    }

    if atomic.LoadInt32(&calls) != 1 {
        t.Fatalf("expected handler to run once, got %d", atomic.LoadInt32(&calls))
    }

    if secondRR.Code != http.StatusCreated {
        t.Fatalf("expected replay status %d, got %d", http.StatusCreated, secondRR.Code)
    }
    if secondRR.Body.String() != "done" {
        t.Fatalf("expected replay body %q, got %q", "done", secondRR.Body.String())
    }
}

func TestMiddlewareRejectsDifferentFingerprint(t *testing.T) {
    calls := 0
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls++
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte("ok"))
    })

    middleware := New(next, "Idempotency-Key")

    req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("first"))
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    middleware.ServeHTTP(rr1, req1)

    req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("second"))
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    middleware.ServeHTTP(rr2, req2)

    if calls != 1 {
        t.Fatalf("expected handler to run once, got %d", calls)
    }

    if rr2.Code != http.StatusConflict {
        t.Fatalf("expected conflict status, got %d", rr2.Code)
    }
}
