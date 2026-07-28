package middleware

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
)

func TestMiddlewareDoesNotCacheServerErrors(t *testing.T) {
    var calls int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&calls, 1)
        w.WriteHeader(http.StatusInternalServerError)
        _, _ = w.Write([]byte("boom"))
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

    if atomic.LoadInt32(&calls) != 2 {
        t.Fatalf("expected handler to run twice for server errors, got %d", atomic.LoadInt32(&calls))
    }

    if rr1.Code != http.StatusInternalServerError || rr2.Code != http.StatusInternalServerError {
        t.Fatalf("expected both responses to be %d, got %d and %d", http.StatusInternalServerError, rr1.Code, rr2.Code)
    }
}
