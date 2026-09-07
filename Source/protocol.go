package main

const (
	protocolVersion    = 43
	protocolMinVersion = 41
	protocolMaxVersion = 43
	protocolID         = "DPH"
)

func gameplayProtocolSupported(version int) bool {
	return version >= protocolMinVersion && version <= protocolMaxVersion
}
