-- The in-game R2 sliders must survive persistence/normalization and reach
-- the common USB/Bluetooth telemetry settings packet independently.
local root = os.getenv("BEAMNG_MOD_ROOT") or "BeamNG-Mod"
package.path = root .. "/lua/?.lua;" .. root .. "/lua/?/init.lua;" .. package.path
local saved = {
  dphSettings = {schema=14, adaptiveTriggers={
    r2EffectsEnabled=true, r2EffectsPercent=25,
    r2WheelspinPulseEnabled=true, r2WheelspinPulsePercent=35,
  }}
}
jsonReadFile = function() return saved end
local store = require("dualsenseSuite/settingsStore")
local settings = require("dualsensePhysicsHaptics/userSettings")
local first = settings.read()
local baseline, baselineRevision = settings.packetSnapshot(first)
assert(baseline.r2EffectsStrength == 12, "default Jolt must transmit force 12/48")
assert(baseline.r2WheelspinPulseStrength == 17, "default wheelspin must transmit force 17/48")
local function change(jolt, wheelspin)
  saved = {dphSettings = {schema=14, adaptiveTriggers={
    r2EffectsEnabled=true, r2EffectsPercent=jolt,
    r2WheelspinPulseEnabled=true, r2WheelspinPulsePercent=wheelspin,
  }}}
  store.reload()
  local snapshot = settings.read()
  return settings.packetSnapshot(snapshot)
end
local low, revisionLow = change(1, 100)
assert(revisionLow > baselineRevision, "new settings must refresh the telemetry packet")
assert(low.r2EffectsStrength == 1 and low.r2WheelspinPulseStrength == 48, "Jolt=1 and Wheelspin=100 must be independent")
local high, revisionHigh = change(100, 1)
assert(revisionHigh > revisionLow, "second settings update must refresh the packet")
assert(high.r2EffectsStrength == 48 and high.r2WheelspinPulseStrength == 1, "Jolt=100 and Wheelspin=1 must be independent")
print("R2 sliders Lua settings packet contract: PASS")
