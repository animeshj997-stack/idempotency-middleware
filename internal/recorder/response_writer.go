package recorder

import (
    "bytes"
    "net/http"
)

// ResponseWriter captures an HTTP response while still delegating to an
// underlying writer.
type ResponseWriter struct {
    underlying http.ResponseWriter
    header     http.Header
    statusCode int
    body       bytes.Buffer
    wroteHeader bool
}

// NewResponseWriter creates a recorder that wraps an existing writer.
func NewResponseWriter(underlying http.ResponseWriter) *ResponseWriter {
    return &ResponseWriter{
        underlying: underlying,
        header:     make(http.Header),
    }
}

// Header returns the header map that will be sent by the writer.
func (rw *ResponseWriter) Header() http.Header {
    return rw.header
}

// Write writes data to the captured body and the underlying writer.
func (rw *ResponseWriter) Write(p []byte) (int, error) {
    if !rw.wroteHeader {
        rw.statusCode = http.StatusOK
        rw.wroteHeader = true
    }

    n, err := rw.body.Write(p)
    if err != nil {
        return n, err
    }

    if rw.underlying != nil {
        return rw.underlying.Write(p)
    }

    return n, nil
}

// WriteHeader stores the status code and forwards it to the underlying writer.
func (rw *ResponseWriter) WriteHeader(statusCode int) {
    if rw.wroteHeader {
        return
    }
    rw.wroteHeader = true
    rw.statusCode = statusCode
    if rw.underlying != nil {
        for k, values := range rw.header {
            for _, v := range values {
                rw.underlying.Header().Add(k, v)
            }
        }
        rw.underlying.WriteHeader(statusCode)
    }
}

// StatusCode returns the captured status code.
func (rw *ResponseWriter) StatusCode() int {
    if rw.statusCode == 0 {
        return http.StatusOK
    }
    return rw.statusCode
}

// Body returns the captured body bytes.
func (rw *ResponseWriter) Body() []byte {
    return rw.body.Bytes()
}

// BodyString returns the captured body as a string.
func (rw *ResponseWriter) BodyString() string {
    return rw.body.String()
}

// Replay writes the captured response to a target writer.
func (rw *ResponseWriter) Replay(target http.ResponseWriter) {
    if target == nil {
        return
    }

    for k, values := range rw.header {
        for _, v := range values {
            target.Header().Add(k, v)
        }
    }

    target.WriteHeader(rw.StatusCode())
    _, _ = target.Write(rw.Body())
}
