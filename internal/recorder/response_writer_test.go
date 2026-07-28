package recorder

import (
    "net/http"
    "testing"
)

type stubResponseWriter struct {
    header     http.Header
    statusCode int
    body       []byte
}

func newStubResponseWriter() *stubResponseWriter {
    return &stubResponseWriter{header: make(http.Header)}
}

func (s *stubResponseWriter) Header() http.Header {
    return s.header
}

func (s *stubResponseWriter) Write(p []byte) (int, error) {
    s.body = append(s.body, p...)
    return len(p), nil
}

func (s *stubResponseWriter) WriteHeader(statusCode int) {
    s.statusCode = statusCode
}

func TestRecorderCapturesStatusHeadersAndBody(t *testing.T) {
    underlying := newStubResponseWriter()
    recorder := NewResponseWriter(underlying)

    recorder.Header().Set("X-Test", "true")
    recorder.WriteHeader(http.StatusCreated)

    if _, err := recorder.Write([]byte("hello")); err != nil {
        t.Fatalf("Write returned error: %v", err)
    }

    if recorder.StatusCode() != http.StatusCreated {
        t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.StatusCode())
    }

    if got := recorder.BodyString(); got != "hello" {
        t.Fatalf("expected body %q, got %q", "hello", got)
    }

    if got := recorder.Header().Get("X-Test"); got != "true" {
        t.Fatalf("expected header value %q, got %q", "true", got)
    }

    if underlying.statusCode != http.StatusCreated {
        t.Fatalf("expected underlying writer to receive status %d, got %d", http.StatusCreated, underlying.statusCode)
    }

    if underlying.header.Get("X-Test") != "true" {
        t.Fatalf("expected underlying writer to receive header %q", "true")
    }
}

func TestRecorderReplay(t *testing.T) {
    underlying := newStubResponseWriter()
    recorder := NewResponseWriter(underlying)

    recorder.Header().Set("X-Replay", "yes")
    recorder.WriteHeader(http.StatusAccepted)
    if _, err := recorder.Write([]byte("replay")); err != nil {
        t.Fatalf("Write returned error: %v", err)
    }

    replayTarget := newStubResponseWriter()
    recorder.Replay(replayTarget)

    if replayTarget.statusCode != http.StatusAccepted {
        t.Fatalf("expected replay status %d, got %d", http.StatusAccepted, replayTarget.statusCode)
    }

    if got := string(replayTarget.body); got != "replay" {
        t.Fatalf("expected replay body %q, got %q", "replay", got)
    }

    if replayTarget.header.Get("X-Replay") != "yes" {
        t.Fatalf("expected replay header %q", "yes")
    }
}
