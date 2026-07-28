# Idempotency Middleware

A small Go middleware library that makes POST requests safe to retry by deduplicating repeated requests that share the same Idempotency-Key.

## Overview

The library is intentionally small and single-node. It uses only the Go standard library.

It protects mutating endpoints by:
- recognizing POST requests,
- requiring an Idempotency-Key for idempotency protection,
- computing a request fingerprint from method, path, and body,
- caching the first completed response for a given key,
- replaying that response to later retries,
- and allowing retries to run again after a 5xx failure.

## Architecture

The project is split into three focused packages:

- internal/store: in-memory, thread-safe store with TTL-based expiration and cleanup
- internal/recorder: response writer wrapper that captures status, headers, and body
- internal/middleware: middleware that coordinates request execution, replay, and conflict handling

## Middleware flow

1. A POST request arrives.
2. The middleware reads the Idempotency-Key header.
3. If the key is missing, the request is passed through unchanged.
4. If the key is new, the middleware creates an entry and executes the handler once.
5. If the same key is reused and the original execution is still running, the retry waits for the result.
6. If the same key is reused after completion, the cached response is replayed.
7. If the same key is reused with a different body or path, the middleware returns 409 Conflict.

## Request lifecycle

- First execution: handler runs once and the response is captured.
- In-flight retry: waiter blocks until the first execution completes.
- Completed retry: waiter receives the cached response immediately.
- Failed execution: 5xx responses are not cached, so the entry is removed and future retries can run again.

## Response lifecycle

The middleware captures the first execution response in a recorder wrapper and stores:
- status code,
- headers,
- body.

That recorded response is later replayed to duplicate requests.

## How to run

Run the example:

```bash
go run ./example
```

## How to test

Run the unit test suite:

```bash
go test ./...
```

Run the race detector if you have a working C toolchain:

```bash
go test -race ./...
```

## Assumptions

- In-memory, single-node only.
- No persistence or distributed coordination.
- Idempotency is keyed by the Idempotency-Key header plus request fingerprint.
- The middleware is intended for mutating POST handlers.

## Limitations

- The store is in-memory and loses state on process restart.
- Cleanup is TTL-based and periodic rather than exact real-time eviction.
- This implementation does not support multi-node coordination or cross-process deduplication.

## Future improvements

- add support for configurable failure policies,
- add explicit metrics and observability,
- support distributed storage for multi-instance deployments,
- make the public API more configurable and production-oriented.
