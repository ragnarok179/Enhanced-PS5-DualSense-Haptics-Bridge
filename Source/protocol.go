package main

const (
	protocolVersion    = 42
	protocolMinVersion = 40
	protocolMaxVersion = 42
	protocolID         = "DPH"
)

func gameplayProtocolSupported(version int) bool {
	return version >= protocolMinVersion && version <= protocolMaxVersion
}
