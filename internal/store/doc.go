// Package store provides a small, thread-safe in-memory key/value store for
// idempotency entries. It supports insertion, lookup, deletion, TTL-based
// expiration, and automatic cleanup using only the Go standard library.
package store
