package main

import (
    "fmt"
    "log"
    "net/http"
    "net/http/httptest"
    "sync"

    middlewarepkg "idempotency-middleware/internal/middleware"
)

func main() {
    var mu sync.Mutex
    counter := 0

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        mu.Lock()
        counter++
        value := counter
        mu.Unlock()

        w.WriteHeader(http.StatusOK)
        _, _ = fmt.Fprintf(w, "counter=%d", value)
    })

    wrapped := middlewarepkg.New(handler, "Idempotency-Key")
    defer wrapped.Close()

    req1 := httptest.NewRequest(http.MethodPost, "/increment", nil)
    req1.Header.Set("Idempotency-Key", "abc")
    rr1 := httptest.NewRecorder()
    wrapped.ServeHTTP(rr1, req1)
    fmt.Println("Request 1")
    fmt.Println("Key=abc")
    fmt.Println("Counter=", counter)
    fmt.Println("Response=", rr1.Body.String())
    fmt.Println()

    req2 := httptest.NewRequest(http.MethodPost, "/increment", nil)
    req2.Header.Set("Idempotency-Key", "abc")
    rr2 := httptest.NewRecorder()
    wrapped.ServeHTTP(rr2, req2)
    fmt.Println("Retry")
    fmt.Println("Key=abc")
    fmt.Println("Counter=", counter)
    fmt.Println("Response=", rr2.Body.String())
    fmt.Println()

    req3 := httptest.NewRequest(http.MethodPost, "/increment", nil)
    req3.Header.Set("Idempotency-Key", "def")
    rr3 := httptest.NewRecorder()
    wrapped.ServeHTTP(rr3, req3)
    fmt.Println("New key")
    fmt.Println("Counter=", counter)
    fmt.Println("Response=", rr3.Body.String())
    fmt.Println()

    log.Println("Example completed")
}
