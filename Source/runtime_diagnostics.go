package main

import (
	"fmt"
	"sync/atomic"
)

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

func printRuntimeError(message string, err error) {
	fmt.Println(message)
	if runtimeDiagnosticsEnabled() && err != nil {
		fmt.Printf("DIAG error: %v\n", err)
	}
}
