# Design Notes

## 1. Overall architecture

Chosen solution: a small middleware layer in front of a mutating POST handler, coordinated by an in-memory store.

Rejected alternative: a larger framework-style architecture with many abstractions and pluggable backends.

Why rejected: it would over-engineer the assignment and obscure the core semantics of idempotency, concurrency, and bounded memory.

## 2. Why in-memory storage was chosen

Chosen solution: an in-memory map keyed by Idempotency-Key, with each entry tracking the state of one logical operation.

Rejected alternative: a persistent store such as PostgreSQL, SQLite, or Redis from the start.

Why rejected: the brief explicitly calls for a single-node, in-memory implementation. A persistent store adds complexity without improving the core story for this scope.

## 3. Synchronization primitive

Chosen solution: a mutex-protected map plus a per-entry completion channel. The first request creates an entry and later requests wait on that channel until the first execution finishes.

Rejected alternative: busy-waiting with polling.

Why rejected: polling wastes CPU and makes the design less predictable. A channel-based wait is simpler and avoids unnecessary contention.

## 4. Request fingerprint design

Chosen solution: compute a fingerprint from the HTTP method, URL path, and request body. This allows the middleware to detect a true retry versus a conflicting reuse of the same key.

Rejected alternative: using the Idempotency-Key alone as identity.

Why rejected: the same key should not silently cover different requests. A key-only approach would make conflicts ambiguous and unsafe.

## 5. Response replay design

Chosen solution: capture the first execution response in a recorder wrapper and store its status code, headers, and body. Later requests replay that recorded response.

Rejected alternative: re-running the handler for every duplicate request.

Why rejected: that would break the purpose of idempotency and could create duplicate side effects.

## 6. Failure policy

Chosen solution: cache 2xx and 4xx responses, but not 5xx responses. When a 5xx occurs, the middleware removes the entry so future retries can run again.

Rejected alternative: caching all failures.

Why rejected: transient server failures should not look like stable, replayable outcomes.

## 7. TTL cleanup

Chosen solution: each entry stores a creation timestamp and a background cleanup goroutine removes expired entries on a fixed interval.

Rejected alternative: no cleanup at all.

Why rejected: that would cause unbounded growth and violate the memory-bounded requirement.

## 8. Redis, distributed systems, and scaling

Chosen solution: keep the implementation single-node and in-memory.

Rejected alternative: introducing Redis-backed coordination immediately.

Why rejected: distributed systems add new failure modes, locking concerns, and operational complexity that are not needed for the assignment. The important step is to nail the single-node semantics first.

If the system were extended to multiple nodes, the design would need a shared backing store, distributed locking or consensus, and a strategy for cross-node replay. In practice, that would mean replacing the local in-memory store with a strongly consistent distributed store and handling persistence, failover, replication, and recovery explicitly. That would be a different system design problem, and the trade-offs would be much more significant than this simple middleware layer.
