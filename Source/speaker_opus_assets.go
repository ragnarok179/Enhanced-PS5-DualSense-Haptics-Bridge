package main

// Opus silence frame used when the Bluetooth controller-speaker stream is idle.
// The public Bridge ships no pre-encoded collision cues: real collisions are
// decoded from the user's BeamNG FSB5 banks and encoded to Opus at runtime.
var speakerOpusSilence = [200]byte{244, 255, 254}
