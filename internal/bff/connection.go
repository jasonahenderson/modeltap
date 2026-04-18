package bff

// Connection represents a single harness-to-server protocol session.
//
// This file contains a minimal stub used by WU-046 so that the package
// compiles and the Dispatcher's Handler signature can reference *Connection.
// WU-048 fleshes out the state machine, heartbeat monitor, grace-period
// release, and session binding.
type Connection struct {
	// Fields are intentionally omitted at WU-046; WU-048 owns the layout.
}
