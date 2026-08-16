package main

import "net"

const bridgeRGBOwnerGEAddress = "127.0.0.1:6975"

// announceBeamNGRGBOwner clears historical Bridge ownership beacons left by
// older experimental builds. The stable architecture has exactly one runtime
// lightbar writer: BeamNG Device.setRGB().
func announceBeamNGRGBOwner() {
	addr, err := net.ResolveUDPAddr("udp", bridgeRGBOwnerGEAddress)
	if err != nil {
		return
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("DPH_RGB_OWNER_OFF"))
	_, _ = conn.Write([]byte("DPH_BT_OWNER_OFF"))
}
