package main

import "strings"

// isDualSenseDefaultPlayback classifies default Windows playback outputs,
// not the WASAPI endpoint explicitly selected for controller haptics.
func isDualSenseDefaultPlayback(defaultID, defaultName, hapticEndpointID string) bool {
	id, haptic := strings.TrimSpace(defaultID), strings.TrimSpace(hapticEndpointID)
	if id != "" && haptic != "" && strings.EqualFold(id, haptic) { return true }
	name := strings.ToLower(strings.TrimSpace(defaultName))
	return strings.Contains(name, "dualsense") ||
		strings.Contains(name, "wireless controller") ||
		strings.Contains(name, "ps5 controller")
}
