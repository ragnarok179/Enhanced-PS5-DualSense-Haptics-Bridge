package main

const (
	protocolVersion    = 43
	protocolMinVersion = 40
	protocolMaxVersion = 43
	protocolID         = "DPH"
)

func gameplayProtocolSupported(version int) bool {
	return version >= protocolMinVersion && version <= protocolMaxVersion
}
