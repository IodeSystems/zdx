// Package testserver is a thin compatibility wrapper around internal/devserver
// that preserves the original New(dsn) signature used by e2e tests.
package testserver

import "github.com/iodesystems/zdx-go/internal/devserver"

// Handle is re-exported from devserver so existing callers see the same fields.
type Handle = devserver.Handle

// New spins up an ephemeral zdx-server against the given DSN. The returned
// cleanup func stops the server and closes the pool; call it from TestMain
// or t.Cleanup.
func New(dsn string) (*Handle, func(), error) {
	return devserver.Start(devserver.Options{DSN: dsn})
}
