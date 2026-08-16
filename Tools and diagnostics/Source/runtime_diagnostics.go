package main

import "sync/atomic"

var runtimeDiagnostics atomic.Bool

func setRuntimeDiagnosticsEnabled(enabled bool) {
	runtimeDiagnostics.Store(enabled)
}

func runtimeDiagnosticsEnabled() bool {
	return runtimeDiagnostics.Load()
}
