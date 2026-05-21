// Package correlation defines the private HTTP metadata used when modeltap
// routes BFF-owned provider calls through its capture proxy.
package correlation

const (
	HeaderRunID   = "X-Modeltap-Run-Id"
	HeaderTraceID = "X-Modeltap-Trace-Id"
)
