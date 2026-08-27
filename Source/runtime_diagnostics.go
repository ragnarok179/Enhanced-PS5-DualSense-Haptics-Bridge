package main

import "sync/atomic"

var runtimeDiagnostics atomic.Bool
var touchpadBindingDiagnostics atomic.Bool

func setRuntimeDiagnosticsEnabled(enabled bool) {
	runtimeDiagnostics.Store(enabled)
}

func runtimeDiagnosticsEnabled() bool {
	return runtimeDiagnostics.Load()
}

func setTouchpadBindingDiagnosticsEnabled(enabled bool) {
	touchpadBindingDiagnostics.Store(enabled)
}

func touchpadBindingDiagnosticsEnabled() bool {
	return touchpadBindingDiagnostics.Load()
}
