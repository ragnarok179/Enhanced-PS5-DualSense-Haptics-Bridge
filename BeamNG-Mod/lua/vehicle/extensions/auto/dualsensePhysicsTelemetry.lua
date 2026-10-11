-- Enhanced PS5 DualSense Haptics
-- BeamNG vehicle telemetry and effect-state publisher.

local M = {}
local CONFIG = require("dualsensePhysicsHaptics/config")

local COMPATIBILITY = assert(CONFIG.compatibility, "Missing DualSense compatibility configuration")

local socket = require("socket")
local TRIGGER = require("dualsensePhysicsHaptics/triggerModel")

local min, max, abs, exp, floor, sqrt, sin, pi = math.min, math.max, math.abs, math.exp, math.floor, math.sqrt, math.sin, math.pi
local clockhp = os.clockhp or os.clock
local udp
local udpBluetooth

local state = {
  initialized = false,
  initRetry = 0,
  controlTimer = 0,
  wheelTimer = 0,
  telemetryTimer = 0,
  settingsHeartbeatTimer = 0,
  lastSettingsRevisionSent = -1,
  lightTelemetryTimer = 0,
  seq = 0,
  wheelCount = 0,
  wheelSide = {},
  wheelAxle = {},
  axleY = {},
  gfxImpactArmed = {},
  gfxPendingCue = {},
  gfxPendingPeakRatio = {},
  gfxPendingPeakForce = {},
  gfxPendingLoadRise = {},
  gfxPendingClock = {},
  gfxPendingActive = false,
  gfxPendingStartClock = -999,
  gfxLastEpisodeClock = -999,
  gfxLastEpisodeSide = 0,
  gfxLastEpisodeAxle = -1,
  gfxLastEpisodeCue = 0,
  gfxLastPrimaryClock = -999,
  gfxLastPrimaryAxle = -1,
  gfxLastPrimarySide = 0,
  gfxLastPrimaryCue = 0,
  gfxBumpsSuppressed = 0,
  loadBaseline = {},
  lastLoad = {},
  lastDepth = {},
  engines = {},
  maxTorque = 100,
  gearbox = nil,

  wheel = {
    brakeCapacity = 0,
    desiredBrake = 0,
    appliedBrake = 0,
    absWheelCount = 0,
    absSeverity = 0,
    absControlHz = 0,
    drivenSlip = 0,
    drivenCount = 0,
    lockSeverity = 0,
    lockedWheelCount = 0,
    candidateL = 0,
    candidateR = 0,
    peakImpulseL = 0,
    peakImpulseR = 0,
    peakForceL = 0,
    peakForceR = 0,
    groundedWheelCount = 0,
    groundedLeftCount = 0, groundedRightCount = 0,
    groundedDrivenCount = 0,
    groundedBrakeCount = 0,
    roughnessL = 0,
    roughnessR = 0,
    surfaceMaterialL = -1,
    surfaceMaterialR = -1,
    roadExcitationL = 0, roadExcitationR = 0,
    roadRollingExcitationL = 0, roadRollingExcitationR = 0,
    roadSlipL = 0, roadSlipR = 0,
  },

  l2Level = 0,
  r2Level = 0,
  absSamples = 0,
  absRelease = 0,
  lockSamples = 0,
  lockRelease = 0,
  lockPulsePhase = 0,
  tcsSamples = 0,
  tcsRelease = 0,
  wheelspinSamples = 0,
  wheelspinRelease = 0,
  limiterSamples = 0,
  limiterRelease = 0,

  lastGear = nil,
  lastClutch = 1,
  lastShifting = false,
  shiftPending = false,
  shiftAge = 0,
  shiftOriginGear = 0,
  shiftNeutralAge = 99,
  lastTorqueNorm = 0,
  shiftEvent = 0,
  shiftFineActive = false,
  shiftFineAge = 0,

  bodyEvent = 0,
  bodySide = 0,
  bodyStrength = 0,
  bodyLeftStrength = 0,
  bodyRightStrength = 0,
  bodyDurationMS = 0,
  bodyKind = "none",
  bodyProfile = "none",
  bodySourceReason = "none",
  bodySourceConfidence = 0,
  bodySourceLeftScore = 0,
  bodySourceRightScore = 0,
  bodySourceLeftContact = 0,
  bodySourceRightContact = 0,
  bodySourceLeftPeak = 0,
  bodySourceRightPeak = 0,
  bodySourceLeftAxle = -1,
  bodySourceRightAxle = -1,
  bodyEventOccupiedThisPacket = false,
  pendingBodyEvents = {},
  currentSpeedMS = 0,
  gravityZ = -9.81,
  lastGZ = -9.81,
  lastGX = 0,
  lastGY = 0,
  collisionCooldown = 0,
  damageTotal = 0,
  damageDelta = 0,
  lastDamage = 0,
  damageBaselineReady = false,
  collisionSource = "none",
  airborne = false,
  airTime = 0,
  lastLandingAirTime = 0,
  landingCooldown = 0,
  nativeRumbleTimer = 0,
  nativeRumblePrevGX = 0,
  nativeRumblePrevGY = 0,
  nativeRumblePrevGZ = -9.81,
  nativeShiftPulseAge = 999,
  nativeRumbleBaseForce = 0,
  bumpSourceReason = "none",
  bumpSourceLeftScore = 0,
  bumpSourceRightScore = 0,
  bumpSourceLeftContact = 0,
  bumpSourceRightContact = 0,
  bumpSourceLeftPeak = 0,
  bumpSourceRightPeak = 0,
  bumpSourceConfidence = 0,
  bumpSourceLeftWheel = -1,
  bumpSourceRightWheel = -1,

  l2Effect = {},
  r2Effect = {},
  lastL2Signature = {},
  lastR2Signature = {},
  lastSignatureBodyEvent = -1,
  lastSignatureShiftEvent = -1,
  errorCooldown = 0,

}

local function num(v, fallback)
  return type(v) == "number" and v or (fallback or 0)
end
local function flag(v) return v == true or v == 1 end
local function clamp(v, lo, hi) return min(max(num(v, lo), lo), hi) end
local function clamp01(v) return clamp(v, 0, 1) end
local function clampInt(v, lo, hi) return floor(clamp(v, lo, hi) + 0.5) end
local function lerp(a, b, t) return a + (b - a) * clamp01(t) end
local function lower(v) return string.lower(tostring(v or "")) end

-- Protocol-40 migration mirror for historical Bridge builds. V1.41 keeps the same
-- canonical semantic payload; V1.41 advertises migration generation 43 so old Bridges
-- format did not change. Current Bridges ignore this marked mirror.
local function legacyOfficialStep(force)
  force = clamp01(force)
  if force <= 0 then return 0 end
  return clampInt(force * 8, 1, 8)
end

local function legacyFineForce(force)
  force = clamp01(force)
  if force <= 0 then return 0 end
  return clampInt(force * 48, 1, 48)
end

local function legacyTriggerFields(effect)
  effect = type(effect) == "table" and effect or {}
  local kind = tostring(effect.kind or "off")
  if kind == "resistance" then
    return 1, clampInt(clamp01(effect.startPosition) * 9, 0, 9),
      legacyOfficialStep(effect.startForce), legacyOfficialStep(effect.endForce), 0, 0
  elseif kind == "vibration" then
    return 2, clampInt(clamp01(effect.startPosition) * 9, 0, 9),
      0, 0, legacyOfficialStep(effect.amplitude), clampInt(effect.frequencyHz or 0, 0, 255)
  elseif kind == "fine" then
    local force = legacyFineForce(effect.startForce)
    return 3, clampInt(clamp01(effect.startPosition) * 255, 0, 255), force, force, 0, 0
  end
  return 0, 0, 0, 0, 0, 0
end

local function buildLegacyV40Packet(packet)
  if type(packet) ~= "table" then return nil end
  local legacyVersion = tonumber(COMPATIBILITY.legacyProtocolVersion) or 40
  local l2Mode, l2Zone, l2Start, l2End, l2Amp, l2Hz = legacyTriggerFields(packet.l2Effect)
  local r2Mode, r2Zone, r2Start, r2End, r2Amp, r2Hz = legacyTriggerFields(packet.r2Effect)
  local legacy = {
    protocolId = COMPATIBILITY.protocolId,
    modVersion = COMPATIBILITY.modVersion,
    protocolMin = COMPATIBILITY.protocolMin,
    protocolMax = COMPATIBILITY.protocolMax,
    legacyCompat = true,
    v = legacyVersion,
    active = packet.active == true,
    seq = packet.seq,
    shiftLEDsInUse = packet.shiftLEDsInUse == true,
    l2Mode = l2Mode, l2StartZone = l2Zone, l2StartStrength = l2Start, l2EndStrength = l2End, l2Amplitude = l2Amp, l2Hz = l2Hz,
    r2Mode = r2Mode, r2StartZone = r2Zone, r2StartStrength = r2Start, r2EndStrength = r2End, r2Amplitude = r2Amp, r2Hz = r2Hz,
    bodyEvent = packet.bodyEvent,
    bodyStrength = packet.bodyStrength, bodyLeftStrength = packet.bodyLeftStrength, bodyRightStrength = packet.bodyRightStrength,
    bodyDurationMS = packet.bodyDurationMS, bodyKind = packet.bodyKind, bodySide = packet.bodySide, bodyProfile = packet.bodyProfile,
    bodySourceReason = packet.bodySourceReason, bodySourceConfidence = packet.bodySourceConfidence,
    bodySourceLeftScore = packet.bodySourceLeftScore, bodySourceRightScore = packet.bodySourceRightScore,
    bodySourceLeftContact = packet.bodySourceLeftContact, bodySourceRightContact = packet.bodySourceRightContact,
    bodySourceLeftPeak = packet.bodySourceLeftPeak, bodySourceRightPeak = packet.bodySourceRightPeak,
    bodySourceLeftEnergy = packet.bodySourceLeftEnergy, bodySourceRightEnergy = packet.bodySourceRightEnergy,
    bodySourceLeftWheel = packet.bodySourceLeftWheel, bodySourceRightWheel = packet.bodySourceRightWheel,
    bodySourceLeftAxle = packet.bodySourceLeftAxle, bodySourceRightAxle = packet.bodySourceRightAxle,
    shiftEvent = packet.shiftEvent, shiftStrength = packet.shiftStrength, shiftDurationMS = packet.shiftDurationMS,
    raw = packet.raw,
  }
  return legacy
end

local function legacyDualPublishEnabled()
  return COMPATIBILITY.dualPublishLegacy == true and
    tonumber(COMPATIBILITY.legacyProtocolVersion) ~= tonumber(COMPATIBILITY.protocolVersion)
end

local USER_SETTINGS = require("dualsensePhysicsHaptics/userSettings")

local function closeUDP()
  if udp then pcall(function() udp:close() end) end
  if udpBluetooth then pcall(function() udpBluetooth:close() end) end
  udp = nil
  udpBluetooth = nil
end


local function nodeSide(nodeId)
  local node = v and v.data and v.data.nodes and v.data.nodes[nodeId]
  local p = node and node.pos
  local x = p and (p.x or p[1])
  if type(x) == "number" then return x > 0 and -1 or (x < 0 and 1 or 0) end
  return 0
end

local function sideOfWheel(w, index)
  local name = lower(w and w.name)
  if name:find("left", 1, true) or name:match("[_%-]l%d*$") then return -1 end
  if name:find("right", 1, true) or name:match("[_%-]r%d*$") then return 1 end
  if w and w.node1 then
    local side = nodeSide(w.node1)
    if side ~= 0 then return side end
  end
  return (index % 2 == 0) and -1 or 1
end

local function buildWheelAxles()
  state.wheelAxle, state.axleY = {}, {}
  local order, wheelY = {}, {}
  for i = 0, state.wheelCount - 1 do
    local w = wheels and ((wheels.wheelRotators and wheels.wheelRotators[i]) or
      (wheels.wheels and wheels.wheels[i])) or nil
    local node = w and w.node1 and v and v.data and v.data.nodes and v.data.nodes[w.node1] or nil
    local pos = node and node.pos or nil
    local y = pos and (pos.y or pos[2])
    wheelY[i] = type(y) == "number" and y or (state.wheelCount - i)
    order[#order + 1] = i
  end
  table.sort(order, function(a, b) return wheelY[a] > wheelY[b] end)
  local axle, lastY, sumY, countY = -1, nil, {}, {}
  for _, i in ipairs(order) do
    local y = wheelY[i]
    if lastY == nil or abs(y - lastY) > 0.42 then
      axle = axle + 1
      lastY = y
    end
    state.wheelAxle[i] = axle
    sumY[axle] = num(sumY[axle], 0) + y
    countY[axle] = num(countY[axle], 0) + 1
  end
  for a = 0, axle do
    state.axleY[a] = num(sumY[a], 0) / max(num(countY[a], 1), 1)
  end
end

local function contactSurface(w)
  local m1 = floor(num(w and w.contactMaterialID1, -1))
  local m2 = floor(num(w and w.contactMaterialID2, -1))
  if m1 == 4 then m1, m2 = m2, m1 end
  if m1 < 0 then return -1 end
  return m1
end

local surfaceRoughnessByMaterial = {
  [10] = 0.030, -- asphalt: very light, subordinate to transient events
  [11] = 0.060, -- wet asphalt
  [12] = 0.045, -- slippery surface
  [13] = 0.58, -- rock
  [14] = 0.34, -- dusty dirt
  [15] = 0.30, -- dirt
  [16] = 0.22, -- sand
  [17] = 0.30, -- sandy road
  [18] = 0.28, -- mud
  [19] = 0.48, -- gravel
  [20] = 0.25, -- grass
  [21] = 0.040, -- ice
  [22] = 0.24, -- snow
  [29] = 1.00, -- rumble strip
  [30] = 0.78, -- cobblestone
}

local surfaceProfileByMaterial = {
  [10] = "asphalt",
  [11] = "asphalt_wet",
  [12] = "slippery",
  [13] = "rock",
  [14] = "dusty_dirt",
  [15] = "dirt",
  [16] = "sand",
  [17] = "sandy_road",
  [18] = "mud",
  [19] = "gravel",
  [20] = "grass",
  [21] = "ice",
  [22] = "snow",
  [29] = "rumble_strip",
  [30] = "cobblestone",
}

local function surfaceRoughness(materialId)
  return surfaceRoughnessByMaterial[materialId] or 0
end

local function surfaceProfile(materialId)
  return surfaceProfileByMaterial[materialId] or "unknown_surface"
end

local function mapABSHapticHz(controlHz, tc)
  if controlHz <= 0 then return tc.absHzFallback or 20 end
  local nativeLo, nativeHi = tc.absNativeHzMin or 20, tc.absNativeHzMax or 140
  local tactileLo, tactileHi = tc.absHzMin or 14, tc.absHzMax or 30
  local bounded = clamp(controlHz, nativeLo, nativeHi)
  local n = math.log(bounded / nativeLo) / max(math.log(nativeHi / nativeLo), 0.001)
  return floor(lerp(tactileLo, tactileHi, n) + 0.5)
end

local getWheel

local function updateNativeRumble(dt, speedMS)
  local cfg = CONFIG.nativeRumble or {}
  if cfg.enabled == false then
    state.nativeRumbleBaseForce = 0
    return
  end

  -- Only BeamNG's generic jerk/slip bed is consumed by the Bridge. Surface,
  -- shift and ABS feedback are synthesized by the Common Feel Engine, so no
  -- second native-surface model is maintained here.
  state.nativeRumbleTimer = state.nativeRumbleTimer + dt
  state.nativeShiftPulseAge = state.nativeShiftPulseAge + dt
  local period = 1 / max(cfg.updateHz or 30, 10)
  if state.nativeRumbleTimer < period then return end
  local sampleDt = min(max(state.nativeRumbleTimer, period), 0.10)
  state.nativeRumbleTimer = max(state.nativeRumbleTimer - period, 0)

  local gx, gy, gz = num(sensors.gx2, 0), num(sensors.gy2, 0), num(sensors.gz2, -9.81)
  local jx = abs(gx - state.nativeRumblePrevGX) / max(sampleDt, 0.001)
  local jy = abs(gy - state.nativeRumblePrevGY) / max(sampleDt, 0.001)
  local jz = abs(gz - state.nativeRumblePrevGZ) / max(sampleDt, 0.001)
  state.nativeRumblePrevGX, state.nativeRumblePrevGY, state.nativeRumblePrevGZ = gx, gy, gz
  if jx < 50 then jx = 0 end
  if jy < 20 then jy = 0 end
  if jz < 15 then jz = 0 end
  local jerkForce = sqrt(jx * jx + jy * jy + jz * jz) / 325

  local highest, second = 0, 0
  for i = 0, state.wheelCount - 1 do
    local w = getWheel(i)
    local e = max(num(w and w.slipEnergy, 0), 0)
    if e > highest then second, highest = highest, e
    elseif e > second then second = e end
  end
  local averageSlipEnergy = (highest + second) / (state.wheelCount > 1 and 2 or 1)
  local wheelSlipForce = 0
  if averageSlipEnergy >= 15000 then wheelSlipForce = min(averageSlipEnergy / 600000, 0.2) end

  local speedFast = max(num(cfg.modelSpeedFastMS, 30), 1)
  local normalizedSpeed = clamp(speedMS / speedFast, 0, 1)
  local stoppedMultiplier = clamp((normalizedSpeed - 0.2) / 0.8, 0, 1)
  local coefficient = max(num(cfg.modelForceCoefficient, 1.0), 0)
  local nativeForce = 0
  if cfg.preserveNativeModel ~= false then
    nativeForce = min((jerkForce + wheelSlipForce) * coefficient, 1)
    if nativeForce > 0 then nativeForce = max(nativeForce, 0.008) end
    nativeForce = nativeForce * stoppedMultiplier
  end
  state.nativeRumbleBaseForce = nativeForce
end

getWheel = function(index)
  if wheels and wheels.wheels then return wheels.wheels[index] end
  if wheels and wheels.wheelRotators then return wheels.wheelRotators[index] end
  return nil
end

local function dependenciesReady()
  return wheels and powertrain and electrics and electrics.values and input and input.state and sensors and playerInfo
end


local function resetWheelCache()
  state.wheel = {
    brakeCapacity = 0,
    desiredBrake = 0,
    appliedBrake = 0,
    absWheelCount = 0,
    absSeverity = 0,
    absControlHz = 0,
    drivenSlip = 0,
    drivenCount = 0,
    lockSeverity = 0,
    lockedWheelCount = 0,
    candidateL = 0,
    candidateR = 0,
    peakImpulseL = 0,
    peakImpulseR = 0,
    peakForceL = 0,
    peakForceR = 0,
    groundedWheelCount = 0,
    groundedLeftCount = 0, groundedRightCount = 0,
    groundedDrivenCount = 0,
    groundedBrakeCount = 0,
    roughnessL = 0,
    roughnessR = 0,
    surfaceMaterialL = -1,
    surfaceMaterialR = -1,
    roadExcitationL = 0, roadExcitationR = 0,
    roadRollingExcitationL = 0, roadRollingExcitationR = 0,
    roadSlipL = 0, roadSlipR = 0,
  }
end

local function initialize()
  if not dependenciesReady() then return false end
  closeUDP()
  udp = socket.udp()
  udp:settimeout(0)
  udp:setpeername(CONFIG.host or "127.0.0.1", CONFIG.port or 6972)

  local bt = CONFIG.bluetoothSpatialHaptics or {}
  if bt.enabled ~= false then
    udpBluetooth = socket.udp()
    udpBluetooth:settimeout(0)
    udpBluetooth:setpeername(bt.host or "127.0.0.1", bt.port or 6974)
  end

  state.wheelCount = floor(clamp(wheels.wheelCount or wheels.wheelRotatorCount or 0, 0, CONFIG.maxWheels or 16))
  state.wheelSide, state.wheelAxle, state.axleY = {}, {}, {}
  state.gfxImpactArmed, state.gfxPendingCue = {}, {}
  state.gfxPendingPeakRatio, state.gfxPendingPeakForce = {}, {}
  state.gfxPendingLoadRise, state.gfxPendingClock = {}, {}
  state.gfxPendingActive, state.gfxPendingStartClock = false, -999
  state.gfxLastEpisodeClock, state.gfxLastEpisodeSide, state.gfxLastEpisodeAxle, state.gfxLastEpisodeCue = -999, 0, -1, 0
  state.gfxLastPrimaryClock, state.gfxLastPrimaryAxle, state.gfxLastPrimarySide, state.gfxLastPrimaryCue = -999, -1, 0, 0
  state.loadBaseline, state.lastLoad, state.lastDepth = {}, {}, {}
  for i = 0, state.wheelCount - 1 do
    local w = getWheel(i)
    state.wheelSide[i] = sideOfWheel(w, i)
    local load = max(num(w and w.downForce, w and w.downForceRaw or 0), 0)
    state.loadBaseline[i], state.lastLoad[i] = load, load
    state.lastDepth[i] = max(num(w and w.contactDepth, 0), 0)
    state.gfxImpactArmed[i] = true
    state.gfxPendingCue[i], state.gfxPendingPeakRatio[i], state.gfxPendingPeakForce[i] = 0, 0, 0
    state.gfxPendingLoadRise[i], state.gfxPendingClock[i] = 0, -999
  end
  buildWheelAxles()
  resetWheelCache()
  state.engines = powertrain.getDevicesByCategory("engine") or {}
  state.maxTorque = 0
  for _, engine in ipairs(state.engines) do
    if engine and type(engine.getTorqueData) == "function" then
      local ok, data = pcall(engine.getTorqueData, engine)
      if ok and data then state.maxTorque = state.maxTorque + max(num(data.maxTorque, 0), 0) end
    end
  end
  state.maxTorque = max(state.maxTorque, 80)
  state.gearbox = powertrain.getDevice("gearbox")
  state.lightTelemetryTimer = 0

  state.l2Level, state.r2Level = 0, 0
  state.absSamples, state.absRelease = 0, 0
  state.lockSamples, state.lockRelease, state.lockPulsePhase = 0, 0, 0
  state.tcsSamples, state.tcsRelease = 0, 0
  state.wheelspinSamples, state.wheelspinRelease = 0, 0
  state.limiterSamples, state.limiterRelease = 0, 0
  state.lastGear = num(electrics.values.gearIndex, state.gearbox and state.gearbox.gearIndex or 0)
  state.lastClutch = clamp01(num(electrics.values.clutchRatio,
    state.gearbox and state.gearbox.parent and state.gearbox.parent.clutchRatio or 1))
  state.lastShifting = flag(electrics.values.isShifting) or flag(state.gearbox and state.gearbox.isShifting)
  state.shiftPending, state.shiftAge = false, 0
  state.shiftOriginGear, state.shiftNeutralAge, state.lastTorqueNorm = 0, 99, 0
  state.shiftEvent, state.bodyEvent = 0, 0
  state.shiftFineActive, state.shiftFineAge = false, 0
  state.bodySide, state.bodyStrength = 0, 0
  state.bodyLeftStrength, state.bodyRightStrength = 0, 0
  state.bodyDurationMS, state.bodyKind = 0, "none"
  state.bodyProfile = "none"
  state.bodySourceReason, state.bodySourceConfidence = "none", 0
  state.bodySourceLeftScore, state.bodySourceRightScore = 0, 0
  state.bodySourceLeftContact, state.bodySourceRightContact = 0, 0
  state.bodySourceLeftPeak, state.bodySourceRightPeak = 0, 0
  state.bodySourceLeftEnergy, state.bodySourceRightEnergy = 0, 0
  state.bodySourceLeftWheel, state.bodySourceRightWheel = -1, -1
  state.bodySourceLeftAxle, state.bodySourceRightAxle = -1, -1
  state.bodyEventOccupiedThisPacket = false
  state.pendingBodyEvents = {}
  state.nativeRumbleTimer = 0
  state.nativeRumblePrevGX, state.nativeRumblePrevGY, state.nativeRumblePrevGZ = 0, 0, num(sensors.gz2, -9.81)
  state.nativeShiftPulseAge = 999
  state.nativeRumbleBaseForce = 0
  state.bumpSourceReason = "none"
  state.bumpSourceLeftScore, state.bumpSourceRightScore = 0, 0
  state.bumpSourceLeftContact, state.bumpSourceRightContact = 0, 0
  state.bumpSourceLeftPeak, state.bumpSourceRightPeak = 0, 0
  state.bumpSourceConfidence = 0
  state.bumpSourceLeftWheel, state.bumpSourceRightWheel = -1, -1
  state.controlTimer, state.wheelTimer, state.telemetryTimer = 0, 0, 0
  state.settingsHeartbeatTimer = 0
  state.lastSettingsRevisionSent = -1
  state.seq = 0
  state.lastL2Signature, state.lastR2Signature = {}, {}
  state.lastSignatureBodyEvent, state.lastSignatureShiftEvent = -1, -1
  state.initialized = true
  return true
end

local function applyBodyEventPayload(ev)
  if not ev then return false end
  state.bodyEvent = state.bodyEvent + 1
  state.bodyLeftStrength = clamp01(num(ev.leftStrength, 0))
  state.bodyRightStrength = clamp01(num(ev.rightStrength, 0))
  state.bodyStrength = max(state.bodyLeftStrength, state.bodyRightStrength)
  state.bodySide = num(ev.side, 0)
  state.bodyDurationMS = floor(clamp(num(ev.durationMS, 18), 18, 220) + 0.5)
  state.bodyKind = ev.kind or "wheel"
  state.bodyProfile = ev.profile or state.bodyKind
  state.bodySourceReason = ev.sourceReason or "none"
  state.bodySourceConfidence = clamp01(num(ev.sourceConfidence, 0))
  state.bodySourceLeftScore = clamp01(num(ev.sourceLeftScore, 0))
  state.bodySourceRightScore = clamp01(num(ev.sourceRightScore, 0))
  state.bodySourceLeftContact = clamp01(num(ev.sourceLeftContact, 0))
  state.bodySourceRightContact = clamp01(num(ev.sourceRightContact, 0))
  state.bodySourceLeftPeak = clamp01(num(ev.sourceLeftPeak, 0))
  state.bodySourceRightPeak = clamp01(num(ev.sourceRightPeak, 0))
  state.bodySourceLeftEnergy = clamp01(num(ev.sourceLeftEnergy, ev.sourceLeftContact or 0))
  state.bodySourceRightEnergy = clamp01(num(ev.sourceRightEnergy, ev.sourceRightContact or 0))
  state.bodySourceLeftWheel = floor(num(ev.sourceLeftWheel, -1))
  state.bodySourceRightWheel = floor(num(ev.sourceRightWheel, -1))
  state.bodySourceLeftAxle = floor(num(ev.sourceLeftAxle, -1))
  state.bodySourceRightAxle = floor(num(ev.sourceRightAxle, -1))
  state.bodyEventOccupiedThisPacket = true
  return true
end

local function queueBodyEventPayload(ev)
  if not ev then return false end
  state.pendingBodyEvents = state.pendingBodyEvents or {}
  local maxQueued = 8
  if #state.pendingBodyEvents >= maxQueued then
    local weakestIndex, weakestStrength = 1, 999
    for i, queued in ipairs(state.pendingBodyEvents) do
      local queuedStrength = max(num(queued.leftStrength, 0), num(queued.rightStrength, 0))
      if queuedStrength < weakestStrength then
        weakestIndex, weakestStrength = i, queuedStrength
      end
    end
    local newStrength = max(num(ev.leftStrength, 0), num(ev.rightStrength, 0))
    if newStrength > weakestStrength then
      state.pendingBodyEvents[weakestIndex] = ev
    end
    return true
  end
  table.insert(state.pendingBodyEvents, ev)
  return true
end

local function flushPendingBodyEvent()
  if state.bodyEventOccupiedThisPacket then return false end
  local queue = state.pendingBodyEvents
  if not queue or #queue == 0 then return false end
  local ev = table.remove(queue, 1)
  return applyBodyEventPayload(ev)
end

local function queuePeakForceBump(side, strength, durationMS, profile)
  local bc = CONFIG.body or {}
  strength = clamp01(strength)
  if not bc.enabled or strength < num(bc.minimumOutput, 0.025) then return false end

  local speedBlend = clamp01(max(num(state.currentSpeedMS, 0), 0) /
    max(num(bc.bumpFullStrengthSpeedMS, 4.0), 0.1))
  local strengthMin = clamp(num(bc.bumpLowSpeedStrengthMin, 0.48), 0.20, 1.0)
  local durationMin = clamp(num(bc.bumpLowSpeedDurationMin, 0.72), 0.35, 1.0)
  local strengthScale = strengthMin + (1 - strengthMin) * sqrt(speedBlend)
  local durationScale = durationMin + (1 - durationMin) * speedBlend
  strength = strength * strengthScale
  durationMS = durationMS * durationScale

  local leftStrength, rightStrength
  if side < 0 then
    leftStrength, rightStrength = strength, 0
  elseif side > 0 then
    leftStrength, rightStrength = 0, strength
  else
    local c = strength * 0.70710678118
    leftStrength, rightStrength = c, c
  end

  return queueBodyEventPayload({
    leftStrength = leftStrength,
    rightStrength = rightStrength,
    side = side,
    durationMS = durationMS,
    kind = "wheel",
    profile = profile or "suspension_bump",
    material = -1,
    sourceReason = state.bumpSourceReason,
    sourceConfidence = state.bumpSourceConfidence,
    sourceLeftScore = state.bumpSourceLeftScore,
    sourceRightScore = state.bumpSourceRightScore,
    sourceLeftContact = state.bumpSourceLeftContact,
    sourceRightContact = state.bumpSourceRightContact,
    sourceLeftPeak = state.bumpSourceLeftPeak,
    sourceRightPeak = state.bumpSourceRightPeak,
    sourceLeftEnergy = state.bumpSourceLeftContact,
    sourceRightEnergy = state.bumpSourceRightContact,
    sourceLeftWheel = state.bumpSourceLeftWheel,
    sourceRightWheel = state.bumpSourceRightWheel,
    sourceLeftAxle = state.bumpSourceLeftWheel >= 0 and num(state.wheelAxle[state.bumpSourceLeftWheel], -1) or -1,
    sourceRightAxle = state.bumpSourceRightWheel >= 0 and num(state.wheelAxle[state.bumpSourceRightWheel], -1) or -1
  })
end

local function clearGfxPeakPending()
  state.gfxPendingActive = false
  state.gfxPendingStartClock = -999
  for i = 0, state.wheelCount - 1 do
    state.gfxPendingCue[i] = 0
    state.gfxPendingPeakRatio[i] = 0
    state.gfxPendingPeakForce[i] = 0
    state.gfxPendingLoadRise[i] = 0
    state.gfxPendingClock[i] = -999
  end
end

local function recordGfxPeakCandidate(i, cue, peakRatio, peakForce, loadRise, now)
  if cue <= num(state.gfxPendingCue[i], 0) then return end
  state.gfxPendingCue[i] = cue
  state.gfxPendingPeakRatio[i] = peakRatio
  state.gfxPendingPeakForce[i] = peakForce
  state.gfxPendingLoadRise[i] = loadRise
  state.gfxPendingClock[i] = now
  if not state.gfxPendingActive then
    state.gfxPendingActive = true
    state.gfxPendingStartClock = now
  end
end

local function resolveGfxPeakBump(now)
  local bc = CONFIG.body or {}
  if bc.gfxPeakBumpEnabled == false or not state.gfxPendingActive then return end
  local wait = clamp(num(bc.gfxPeakPairWindowSeconds, 0.026), 0, 0.080)
  if now - num(state.gfxPendingStartClock, now) < wait then return end

  local primaryWheel, primaryCue = -1, 0
  for i = 0, state.wheelCount - 1 do
    local cue = clamp01(num(state.gfxPendingCue[i], 0))
    if cue > primaryCue then primaryWheel, primaryCue = i, cue end
  end
  if primaryWheel < 0 or primaryCue <= 0 then clearGfxPeakPending(); return end

  local primarySide = num(state.wheelSide[primaryWheel], 0)
  local primaryAxle = floor(num(state.wheelAxle[primaryWheel], -1))
  local primaryClock = num(state.gfxPendingClock[primaryWheel], now)
  local primaryPeakRatio = num(state.gfxPendingPeakRatio[primaryWheel], 0)

  local mateWheel, mateCue, matePeakRatio, mateClock = -1, 0, 0, -999
  for i = 0, state.wheelCount - 1 do
    if i ~= primaryWheel and num(state.wheelSide[i], 0) == -primarySide and
        floor(num(state.wheelAxle[i], -2)) == primaryAxle then
      local cue = clamp01(num(state.gfxPendingCue[i], 0))
      if cue > mateCue then
        mateWheel, mateCue = i, cue
        matePeakRatio = num(state.gfxPendingPeakRatio[i], 0)
        mateClock = num(state.gfxPendingClock[i], -999)
      end
    end
  end

  local side = primarySide
  local reason = primarySide < 0 and "gfx_peakforce_left" or "gfx_peakforce_right"
  local centerMin = clamp(num(bc.gfxPeakCenterMin, 0.22), 0.05, 0.9)
  local centerBalance = clamp(num(bc.gfxPeakCenterBalanceRatio, 0.78), 0.55, 0.98)
  if mateWheel >= 0 and primaryCue >= centerMin and mateCue >= centerMin and
      mateCue / max(primaryCue, 0.001) >= centerBalance and
      abs(primaryClock - mateClock) <= max(wait, 0.018) then
    side = 0
    reason = "gfx_same_axle_true_center"
  end

  local profile = "suspension_bump"
  local suppress = false
  local episodeAge = now - num(state.gfxLastEpisodeClock, -999)
  local lastAxle = floor(num(state.gfxLastEpisodeAxle, -2))
  local lastSide = num(state.gfxLastEpisodeSide, 0)
  local lastCue = clamp01(num(state.gfxLastEpisodeCue, 0))

  if primaryAxle == lastAxle and episodeAge >= 0 and
      episodeAge <= num(bc.gfxPeakSameAxleReboundSeconds, 0.180) then
    local override = primaryCue >= num(bc.gfxPeakReboundOverrideCue, 0.84) and
      primaryCue >= lastCue * num(bc.gfxPeakReboundOverrideRatio, 1.30)
    if not override then
      suppress = true
      reason = side ~= 0 and lastSide ~= 0 and side == -lastSide and
        "gfx_opposite_rebound_suppressed" or "gfx_same_axle_rebound_suppressed"
    end
  end

  if not suppress and primaryAxle >= 0 and state.gfxLastPrimaryAxle >= 0 and
      primaryAxle ~= state.gfxLastPrimaryAxle then
    local speed = max(num(state.currentSpeedMS, 0), 0.1)
    local yNow = num(state.axleY[primaryAxle], 0)
    local yPrev = num(state.axleY[state.gfxLastPrimaryAxle], yNow)
    local axleDistance = abs(yNow - yPrev)
    local expected = axleDistance > 0.35 and axleDistance / speed or 999
    local age = now - num(state.gfxLastPrimaryClock, -999)
    if expected < 999 and age >= expected * num(bc.gfxPeakSecondAxleTimingMin, 0.45) and
        age <= expected * num(bc.gfxPeakSecondAxleTimingMax, 1.85) and
        age <= num(bc.gfxPeakSecondAxleMaxSeconds, 0.70) then
      profile = "suspension_secondary"
      reason = "gfx_second_axle"
    end
  end

  local leftCue, rightCue, leftWheel, rightWheel = 0, 0, -1, -1
  local leftPeakRatio, rightPeakRatio = 0, 0
  if primarySide < 0 then
    leftCue, leftWheel, leftPeakRatio = primaryCue, primaryWheel, primaryPeakRatio
    if mateWheel >= 0 then rightCue, rightWheel, rightPeakRatio = mateCue, mateWheel, matePeakRatio end
  else
    rightCue, rightWheel, rightPeakRatio = primaryCue, primaryWheel, primaryPeakRatio
    if mateWheel >= 0 then leftCue, leftWheel, leftPeakRatio = mateCue, mateWheel, matePeakRatio end
  end

  state.bumpSourceReason = reason
  state.bumpSourceConfidence = primaryCue
  state.bumpSourceLeftScore, state.bumpSourceRightScore = leftCue, rightCue
  state.bumpSourceLeftContact, state.bumpSourceRightContact = leftCue, rightCue
  state.bumpSourceLeftPeak, state.bumpSourceRightPeak = leftPeakRatio, rightPeakRatio
  state.bumpSourceLeftWheel, state.bumpSourceRightWheel = leftWheel, rightWheel

  if suppress then
  else
    local trigger = clamp(num(bc.gfxPeakTrigger, 0.10), 0.02, 0.8)
    local severity = clamp01((primaryCue - trigger) / max(1 - trigger, 0.001))
    local shaped = severity ^ clamp(num(bc.gfxPeakSeverityExponent, 0.55), 0.30, 1.2)
    local strength = lerp(num(bc.gfxPeakStrengthMin, 0.38), num(bc.gfxPeakStrengthMax, 1.00), shaped)
    local duration = lerp(num(bc.gfxPeakDurationMinMS, 50), num(bc.gfxPeakDurationMaxMS, 118), shaped)
    if profile == "suspension_secondary" then
      strength = strength * clamp(num(bc.gfxPeakSecondAxleStrengthScale, 0.78), 0.45, 1.0)
      duration = duration * clamp(num(bc.gfxPeakSecondAxleDurationScale, 0.82), 0.55, 1.0)
    end
    queuePeakForceBump(side, strength, duration, profile)
    state.gfxLastEpisodeClock, state.gfxLastEpisodeSide = now, side
    state.gfxLastEpisodeAxle, state.gfxLastEpisodeCue = primaryAxle, primaryCue
    if profile == "suspension_bump" then
      state.gfxLastPrimaryClock, state.gfxLastPrimaryAxle = now, primaryAxle
      state.gfxLastPrimarySide, state.gfxLastPrimaryCue = side, primaryCue
    end
  end
  clearGfxPeakPending()
end

local function setBodyEventStereo(leftStrength, rightStrength, durationMS, kind, profile, material)
  local bc = CONFIG.body
  leftStrength, rightStrength = clamp01(leftStrength), clamp01(rightStrength)
  if bc.swapSides then leftStrength, rightStrength = rightStrength, leftStrength end
  local strength = max(leftStrength, rightStrength)
  if not bc.enabled or strength < (bc.minimumOutput or 0.030) then return false end
  local dominance = bc.outputSideDominance or 1.12
  local side = 0
  if leftStrength > rightStrength * dominance then
    side = -1
  elseif rightStrength > leftStrength * dominance then
    side = 1
  end
  return queueBodyEventPayload({
    leftStrength = leftStrength, rightStrength = rightStrength, side = side,
    durationMS = durationMS, kind = kind or "wheel",
    profile = profile or kind or "wheel", material = floor(num(material, -1))
  })
end

local function emitCollisionStereo(leftWeight, rightWeight, severity)
  local bc = CONFIG.body
  local strength = lerp(bc.collisionStrengthMin or 0.22, bc.collisionStrengthMax or 0.62, severity)
  local duration = lerp(bc.collisionDurationMinMS or 78, bc.collisionDurationMaxMS or 145, severity)
  return setBodyEventStereo(strength * clamp01(leftWeight), strength * clamp01(rightWeight),
    duration, "collision", "collision", -1)
end

local function emitLanding(severity)
  local bc = CONFIG.body
  local strength = lerp(bc.landingStrengthMin or 0.11, bc.landingStrengthMax or 0.48, severity)
  local duration = lerp(bc.landingDurationMinMS or 52, bc.landingDurationMaxMS or 112, severity)
  return setBodyEventStereo(strength, strength, duration, "landing", "landing", -1)
end

local function updateShift(dt, clutch, gear, shifting, speedMS, throttle, torqueNorm)
  local tc = CONFIG.triggers
  local gearboxType = lower(state.gearbox and state.gearbox.type)
  local unsupported = gearboxType:find("cvt", 1, true) or gearboxType:find("electric", 1, true)
  if not tc.shiftEnabled or unsupported then
    state.shiftOriginGear, state.shiftNeutralAge = 0, 99
    state.shiftFineActive, state.shiftFineAge = false, 0
    state.lastGear, state.lastClutch, state.lastShifting, state.lastTorqueNorm = gear, clutch, shifting, torqueNorm
    return
  end

  state.shiftNeutralAge = state.shiftNeutralAge + dt
  local fromGear, toGear = state.lastGear or 0, gear
  local validSpeed = speedMS >= (tc.shiftMinSpeedMS or 2.0)
  local changed = false
  local enteringNeutral = fromGear ~= 0 and toGear == 0 and validSpeed

  if validSpeed and fromGear ~= 0 and toGear ~= 0 and fromGear ~= toGear and fromGear * toGear > 0 then
    changed = true
    state.shiftOriginGear = 0
  elseif enteringNeutral then
    state.shiftOriginGear = fromGear
    state.shiftNeutralAge = 0
  elseif fromGear == 0 and toGear ~= 0 and state.shiftOriginGear ~= 0 then
    if validSpeed and state.shiftNeutralAge <= (tc.shiftNeutralBridgeSeconds or 0.70) and
        state.shiftOriginGear ~= toGear and state.shiftOriginGear * toGear > 0 then
      changed = true
    end
    state.shiftOriginGear = 0
  elseif toGear ~= 0 and state.shiftNeutralAge > (tc.shiftNeutralBridgeSeconds or 0.70) then
    state.shiftOriginGear = 0
  end

  if changed then
    state.shiftEvent = state.shiftEvent + 1
    state.nativeShiftPulseAge = 0
    local nr = CONFIG.nativeRumble or {}
    state.nativeRumbleTimer = max(state.nativeRumbleTimer, 1 / max(nr.updateHz or 30, 10))
  end

  if tc.shiftFineReleaseEnabled ~= false and validSpeed then
    local openThreshold = clamp01(num(tc.shiftFineClutchOpen, 0.78))
    local closedThreshold = max(openThreshold, clamp01(num(tc.shiftFineClutchClosed, 0.92)))
    local clutchOpening = state.lastClutch >= closedThreshold and clutch < openThreshold
    local sequenceActive = shifting or enteringNeutral or changed or
      (state.shiftOriginGear ~= 0 and clutch < closedThreshold)
    if sequenceActive or clutchOpening then
      state.shiftFineActive = true
      state.shiftFineAge = 0
    elseif state.shiftFineActive then
      state.shiftFineAge = state.shiftFineAge + dt
      if (not shifting and toGear ~= 0 and clutch >= closedThreshold and state.shiftFineAge >= 0.025) or
          state.shiftFineAge >= (tc.shiftFineMaxSeconds or 0.90) then
        state.shiftFineActive, state.shiftFineAge = false, 0
      end
    end
  else
    state.shiftFineActive, state.shiftFineAge = false, 0
  end

  state.shiftPending = false
  state.shiftAge = 0
  state.lastGear, state.lastClutch, state.lastShifting, state.lastTorqueNorm = gear, clutch, shifting, torqueNorm
end

local function sampleWheels(dt, speedMS, brake, nativeABS, nativeTCS)
  local now = clockhp()
  local tc, bc = CONFIG.triggers, CONFIG.body
  resolveGfxPeakBump(now)
  local cache = {
    brakeCapacity = 0, desiredBrake = 0, appliedBrake = 0,
    absWheelCount = 0, absSeverity = 0, absControlHz = 0, absControlCount = 0,
    drivenSlip = 0, drivenCount = 0,
    lockSeverity = 0, lockedWheelCount = 0,
    candidateL = 0, candidateR = 0,
    peakImpulseL = 0, peakImpulseR = 0,
    peakForceL = 0, peakForceR = 0,
    groundedWheelCount = 0,
    groundedLeftCount = 0, groundedRightCount = 0,
    groundedDrivenCount = 0,
    groundedBrakeCount = 0,
    roughnessL = 0, roughnessR = 0,
    surfaceMaterialL = -1, surfaceMaterialR = -1,
    roadExcitationL = 0, roadExcitationR = 0,
    roadRollingExcitationL = 0, roadRollingExcitationR = 0,
    roadSlipL = 0, roadSlipR = 0,
  }
  local baselineAlpha = 1 - exp(-dt / max(bc.loadBaselineTime or 0.60, 0.01))
  local bodyAllowed = bc.enabled and speedMS >= (bc.minSpeedMS or 2.5)
  local lockNeeded = brake >= (tc.absMinBrakeInput or 0.08) and
    speedMS >= (tc.lockMinSpeedMS or 0.5) and not nativeABS

  for i = 0, state.wheelCount - 1 do
    local w = getWheel(i)
    if w then
      local side = state.wheelSide[i] or 0
      local load = max(num(w.downForce, w.downForceRaw or 0), 0)
      local oldBaseline = max(num(state.loadBaseline[i], load), 1)
      local previousLoad = num(state.lastLoad[i], load)
      local depth = max(num(w.contactDepth, 0), 0)
      local material = contactSurface(w)
      local grounded = material >= 0
      local peakForce = max(num(w.peakForce, 0), 0)
      local peakRatio = peakForce / max(oldBaseline, 250)
      local peakImpulse = 0
      if bc.peakForceEnabled ~= false and grounded then
        peakImpulse = clamp01((peakRatio - (bc.peakForceRatioThreshold or 1.15)) /
          max((bc.peakForceRatioFull or 3.80) - (bc.peakForceRatioThreshold or 1.15), 0.10))
        if side < 0 then
          cache.peakImpulseL = max(cache.peakImpulseL, peakImpulse)
          cache.peakForceL = max(cache.peakForceL, peakForce)
        elseif side > 0 then
          cache.peakImpulseR = max(cache.peakImpulseR, peakImpulse)
          cache.peakForceR = max(cache.peakForceR, peakForce)
        end
      end
      if grounded then
        cache.groundedWheelCount = cache.groundedWheelCount + 1
        if side < 0 then cache.groundedLeftCount = cache.groundedLeftCount + 1
        elseif side > 0 then cache.groundedRightCount = cache.groundedRightCount + 1 end
      end
      local roughness = surfaceRoughness(material)
      if roughness > 0 and load > 0 then
        if side < 0 and roughness > cache.roughnessL then
          cache.roughnessL, cache.surfaceMaterialL = roughness, material
        elseif side > 0 and roughness > cache.roughnessR then
          cache.roughnessR, cache.surfaceMaterialR = roughness, material
        end
      end
      local previousDepth = num(state.lastDepth[i], depth)
      local loadRiseRate = (load - previousLoad) / max(dt, 0.001) / oldBaseline
      local depthVelocity = (depth - previousDepth) / max(dt, 0.001)

      if bc.gfxPeakBumpEnabled ~= false and bodyAllowed and grounded and side ~= 0 then
        local trigger = clamp(num(bc.gfxPeakTrigger, 0.10), 0.02, 0.8)
        local rearm = clamp(num(bc.gfxPeakRearm, 0.035), 0.005, trigger - 0.005)
        local loadStart = num(bc.gfxPeakLoadRiseStart, 2.5)
        local loadFull = max(num(bc.gfxPeakLoadRiseFull, 14.0), loadStart + 0.1)
        local loadCue = clamp01((loadRiseRate - loadStart) / (loadFull - loadStart))
        local cue = max(peakImpulse, loadCue * clamp(num(bc.gfxPeakLoadAssistWeight, 0.24), 0, 0.5))
        local armed = state.gfxImpactArmed[i] ~= false
        if peakImpulse <= rearm and loadCue <= num(bc.gfxPeakLoadRearm, 0.10) then armed = true end
        local directPeakMin = max(num(bc.gfxPeakDirectPeakMin, 0.075), trigger * 0.60)
        if armed and cue >= trigger and peakImpulse >= directPeakMin then
          recordGfxPeakCandidate(i, cue, peakRatio, peakForce, loadRiseRate, now)
          armed = false
        end
        state.gfxImpactArmed[i] = armed
      end

      state.loadBaseline[i] = oldBaseline + (load - oldBaseline) * baselineAlpha
      state.lastLoad[i], state.lastDepth[i] = load, depth

      if grounded and load > 0 then
        local loadTexture = clamp01((abs(loadRiseRate) - 0.010) / 0.55)
        local depthTexture = clamp01(abs(depthVelocity) / 0.24)
        local slipAbs = abs(num(w.lastSlip, 0))
        local slipTexture = clamp01((slipAbs - 0.35) / 18.0)
        local rollingTexture = max(loadTexture * 0.88, depthTexture)
        -- Keep the legacy mixed excitation on the wire for older Bridges, but
        -- also send a dedicated rolling-only channel. New Bridges assign the
        -- old slip-induced excitation increment to the Slip slider, making the
        -- two user controls genuinely independent without changing stock output.
        local roadTexture = max(rollingTexture, slipTexture * 0.24)
        if side < 0 then
          cache.roadExcitationL = max(cache.roadExcitationL, roadTexture)
          cache.roadRollingExcitationL = max(cache.roadRollingExcitationL, rollingTexture)
          cache.roadSlipL = max(cache.roadSlipL, slipTexture)
        elseif side > 0 then
          cache.roadExcitationR = max(cache.roadExcitationR, roadTexture)
          cache.roadRollingExcitationR = max(cache.roadRollingExcitationR, rollingTexture)
          cache.roadSlipR = max(cache.roadSlipR, slipTexture)
        end
      end

      if bodyAllowed then
        local loadImpulse = clamp01((loadRiseRate - (bc.loadRiseRateThreshold or 5.0)) /
          max((bc.loadRiseRateFull or 18.0) - (bc.loadRiseRateThreshold or 5.0), 0.1))
        local depthImpulse = clamp01((depthVelocity - (bc.depthVelocityThreshold or 0.16)) /
          max((bc.depthVelocityFull or 0.58) - (bc.depthVelocityThreshold or 0.16), 0.01))
        local candidate = max(loadImpulse * 0.78, depthImpulse,
          peakImpulse * (bc.peakForceWeight or 0.86))
        if side < 0 then cache.candidateL = max(cache.candidateL, candidate)
        elseif side > 0 then cache.candidateR = max(cache.candidateR, candidate) end
      end

      local brakeTorque = max(num(w.brakeTorque, 0), 0)
      local desiredBrake = max(num(w.desiredBrakingTorque, 0), 0)
      local appliedBrake = max(num(w.brakingTorque, w.coreData and w.coreData.brakeTorqueApplied or 0), 0)
      cache.brakeCapacity = cache.brakeCapacity + brakeTorque
      cache.desiredBrake = cache.desiredBrake + desiredBrake
      cache.appliedBrake = cache.appliedBrake + appliedBrake
      if grounded and brakeTorque > 0 then cache.groundedBrakeCount = cache.groundedBrakeCount + 1 end

      local wheelABS = grounded and flag(w.absActive)
      local absCoef = clamp(num(w.lastABSCoef, wheelABS and 0.5 or 1), 0, 1)
      if wheelABS then
        cache.absWheelCount = cache.absWheelCount + 1
        cache.absSeverity = max(cache.absSeverity, 1 - absCoef)
        local absTime = num(w.absTime, 0)
        local controlHz = absTime > 0 and 1 / absTime or num(w.absFrequency, 0)
        if controlHz > 0 then
          cache.absControlHz = cache.absControlHz + controlHz
          cache.absControlCount = cache.absControlCount + 1
        end
      end

      if w.isPropulsed and grounded then
        cache.drivenCount = cache.drivenCount + 1
        cache.groundedDrivenCount = cache.groundedDrivenCount + 1
        cache.drivenSlip = cache.drivenSlip + abs(num(w.lastSlip, 0))
      end

      local radius = max(num(w.dynamicRadius, w.radius or 0.3), 0.05)
      local angularBrake = num(w.angularVelocityBrakeCouple, w.angularVelocity or 0)
      local wheelGroundSpeed = abs(angularBrake * radius)
      local slipRatio = speedMS > 1 and clamp01((speedMS - wheelGroundSpeed) / speedMS) or 0
      if grounded and lockNeeded and brakeTorque > 0 and not wheelABS and slipRatio >= (tc.lockSlipStart or 0.23) then
        cache.lockedWheelCount = cache.lockedWheelCount + 1
        cache.lockSeverity = max(cache.lockSeverity, clamp01((slipRatio - (tc.lockSlipStart or 0.23)) /
          max((tc.lockSlipFull or 0.68) - (tc.lockSlipStart or 0.23), 0.01)))
      end

    end
  end

  if cache.absControlCount > 0 then cache.absControlHz = cache.absControlHz / cache.absControlCount end
  cache.absControlCount = nil

  state.wheel = cache
end

local function triggerControlChanged(l2Effect, r2Effect)
  local changed = not TRIGGER.signatureEquals(l2Effect, state.lastL2Signature) or
    not TRIGGER.signatureEquals(r2Effect, state.lastR2Signature) or
    state.bodyEvent ~= state.lastSignatureBodyEvent or state.shiftEvent ~= state.lastSignatureShiftEvent
  TRIGGER.captureSignature(l2Effect, state.lastL2Signature)
  TRIGGER.captureSignature(r2Effect, state.lastR2Signature)
  state.lastSignatureBodyEvent = state.bodyEvent
  state.lastSignatureShiftEvent = state.shiftEvent
  return changed
end

local function publishDirectRPMLights(dt, rpm, maxRPM)
  local lightCfg = CONFIG.directRPMLights or {}
  if lightCfg.enabled == false or not obj or not obj.queueGameEngineLua then return end

  state.lightTelemetryTimer = state.lightTelemetryTimer + max(num(dt, 0), 0)
  local period = 1 / max(num(lightCfg.telemetryHz, 30), 5)
  if state.lightTelemetryTimer < period then return end
  state.lightTelemetryTimer = max(state.lightTelemetryTimer - period, 0)

  -- The GE writer receives only the values needed to draw the RPM lightbar.
  -- No gearbox mode, native shift-light hook or transmission state is involved.
  local command = string.format([[
    local e = extensions and extensions.dualsensePhysicsDirectRPMLights
    if e and e.onDualSensePhysicsRPMData then
      e.onDualSensePhysicsRPMData(%.6f, %.6f)
    end
  ]], max(num(rpm, 0), 0), max(num(maxRPM, 0), 0))
  pcall(obj.queueGameEngineLua, obj, command)
end

local function buildPacket(dt)
  local tc, bc = CONFIG.triggers, CONFIG.body
  state.seq = state.seq + 1
  state.bodyEventOccupiedThisPacket = false
  state.wheelTimer = state.wheelTimer + dt
  state.telemetryTimer = state.telemetryTimer + dt
  state.settingsHeartbeatTimer = state.settingsHeartbeatTimer + dt
  local currentUserSettings = USER_SETTINGS.read()

  local throttle = clamp01(num(input.state.throttle and input.state.throttle.val, electrics.values.throttle or 0))
  local brake = clamp01(num(input.state.brake and input.state.brake.val, electrics.values.brake or 0))
  local clutch = clamp01(num(electrics.values.clutchRatio,
    state.gearbox and state.gearbox.parent and state.gearbox.parent.clutchRatio or 1))
  local speedMS = abs(num(electrics.values.airspeed, 0))
  state.currentSpeedMS = speedMS
  local rpm = max(num(electrics.values.rpm, 0), 0)
  local maxRPM = max(num(electrics.values.maxrpm, 0), 1)
  local engineRunning = flag(electrics.values.engineRunning) or rpm > 120
  local gear = num(electrics.values.gearIndex, state.gearbox and state.gearbox.gearIndex or 0)
  local shifting = flag(electrics.values.isShifting) or flag(state.gearbox and state.gearbox.isShifting)
  local nativeABS = flag(electrics.values.absActive)
  local nativeTCS = flag(electrics.values.tcsActive)

  local gx2, gy2, gz2 = num(sensors.gx2, 0), num(sensors.gy2, 0), num(sensors.gz2, -9.81)
  local gravityAlpha = 1 - exp(-dt / max(bc.gravityBaselineTime or 0.95, 0.05))
  state.gravityZ = state.gravityZ + (gz2 - state.gravityZ) * gravityAlpha
  local verticalHP = abs(gz2 - state.gravityZ)
  local verticalJerk = abs(gz2 - state.lastGZ) / max(dt, 0.001)
  local horizontalJerk = sqrt((gx2 - state.lastGX) * (gx2 - state.lastGX) +
    (gy2 - state.lastGY) * (gy2 - state.lastGY)) / max(dt, 0.001)
  state.lastGZ, state.lastGX, state.lastGY = gz2, gx2, gy2
  local horizontalAccel = sqrt(gx2 * gx2 + gy2 * gy2)
  state.collisionCooldown = max(state.collisionCooldown - dt, 0)
  state.landingCooldown = max(state.landingCooldown - dt, 0)

  local currentDamage = max(num(beamstate and beamstate.damage, state.lastDamage), 0)
  if not state.damageBaselineReady or currentDamage + 0.001 < state.lastDamage then
    state.lastDamage = currentDamage
    state.damageBaselineReady = true
  end
  local damageDelta = max(currentDamage - state.lastDamage, 0)
  state.lastDamage = currentDamage
  state.damageTotal = currentDamage
  state.damageDelta = damageDelta

  local damageGate = sqrt(clamp01((damageDelta - (bc.damageDeltaThreshold or 0.010)) /
    max((bc.damageDeltaFull or 3.0) - (bc.damageDeltaThreshold or 0.010), 0.01)))
  local accelGate = clamp01((horizontalAccel - (bc.damageAccelThreshold or 12.0)) /
    max((bc.damageAccelFull or 50.0) - (bc.damageAccelThreshold or 12.0), 0.1))
  local hJerkGate = clamp01((horizontalJerk - (bc.damageJerkThreshold or 80.0)) /
    max((bc.damageJerkFull or 500.0) - (bc.damageJerkThreshold or 80.0), 0.1))
  local accelDominance = max(num(bc.collisionHorizontalAccelDominance, 1.35), 1.0)
  local jerkDominance = max(num(bc.collisionHorizontalJerkDominance, 1.15), 1.0)
  local horizontalDominant =
    horizontalAccel >= max(num(bc.collisionHorizontalMinAccel, 4.5), verticalHP * accelDominance) and
    horizontalJerk >= max(num(bc.collisionHorizontalMinJerk, 22.0), verticalJerk * jerkDominance)
  local severeDamage = damageDelta >= num(bc.collisionSevereDamageDelta, 0.10)

  local damageConfirmed = bc.collisionEnabled and speedMS >= (bc.collisionMinSpeedMS or 0.5) and
    damageDelta >= (bc.damageDeltaThreshold or 0.010) and (horizontalDominant or severeDamage)
  local damageScore = max(damageGate, min(accelGate, hJerkGate * 1.15) * 0.75)

  local collisionScore = damageScore
  local collisionDetected = state.collisionCooldown <= 0 and damageConfirmed
  state.collisionSource = damageConfirmed and (severeDamage and "severe_damage" or "horizontal_damage") or "none"

  if collisionDetected then
    local leftWeight, rightWeight = 1.0, 1.0
    local lateralMapped = false
    if abs(gx2) >= (bc.collisionSideMinAccel or 2.5) and
        abs(gx2) > abs(gy2) * (bc.collisionSideRatio or 0.55) then
      local lateralShare = clamp01(abs(gx2) / max(abs(gx2) + abs(gy2), 0.001))
      local opposite = lerp(bc.collisionCrossfeed or 0.04, 0.28, 1 - lateralShare)
      if gx2 > 0 then
        leftWeight, rightWeight = opposite, 1.0
      else
        leftWeight, rightWeight = 1.0, opposite
      end
      lateralMapped = true
    end
    if not lateralMapped and state.wheel then
      local priorL, priorR = state.wheel.candidateL or 0, state.wheel.candidateR or 0
      local strongest = max(priorL, priorR)
      if strongest >= 0.18 then
        if priorL > priorR * 1.25 then
          leftWeight, rightWeight = 1.0, bc.collisionCrossfeed or 0.04
        elseif priorR > priorL * 1.25 then
          leftWeight, rightWeight = bc.collisionCrossfeed or 0.04, 1.0
        end
      end
    end
    if emitCollisionStereo(leftWeight, rightWeight, collisionScore) then
      state.collisionCooldown = bc.collisionCooldown or 0.42
    end
  end

  local totalTorque, revLimiter = 0, false
  for _, engine in ipairs(state.engines) do
    if engine then
      totalTorque = totalTorque + abs(num(engine.outputTorque1, engine.combustionTorque or 0))
      revLimiter = revLimiter or flag(engine.revLimiterActive)
    end
  end
  revLimiter = revLimiter or flag(electrics.values.revLimiterActive)
  publishDirectRPMLights(dt, rpm, maxRPM)
  local torqueNorm = clamp01(totalTorque / max(state.maxTorque, 1))

  local wheelPeriod = 1 / max(CONFIG.wheelHz or 25, 2)
  local wheelDue = state.wheelTimer >= wheelPeriod
  if wheelDue then
    local wheelDt = min(max(state.wheelTimer, wheelPeriod), 0.20)
    state.wheelTimer = max(state.wheelTimer - wheelPeriod, 0)
    sampleWheels(wheelDt, speedMS, brake, nativeABS, nativeTCS)
  end

  local wc = state.wheel

  local landingThisFrame = false
  if wheelDue and bc.landingEnabled then
    local grounded = wc.groundedWheelCount > 0
    if not grounded and state.wheelCount > 0 then
      if not state.airborne then state.lastLandingAirTime = 0 end
      state.airborne = true
      state.airTime = state.airTime + dt
    elseif grounded and state.airborne then
      state.lastLandingAirTime = state.airTime
      local impulseSeverity = max(wc.candidateL, wc.candidateR, clamp01(verticalHP / 7.0))
      local landingEligible = state.airTime >= (bc.landingMinAirTime or 0.12) and
        (speedMS >= (bc.landingMinSpeedMS or 5.0) or impulseSeverity >= 0.35)
      if landingEligible and state.landingCooldown <= 0 and not collisionDetected then
        local airSeverity = clamp01((state.airTime - (bc.landingMinAirTime or 0.12)) /
          max((bc.landingFullAirTime or 0.85) - (bc.landingMinAirTime or 0.12), 0.05))
        if emitLanding(max(airSeverity * 0.75, impulseSeverity)) then
          landingThisFrame = true
          state.landingCooldown = bc.landingCooldown or 0.45
        end
      end
      state.airborne, state.airTime = false, 0
    elseif grounded then
      state.airTime = 0
    end
  end

  local groundControl = not state.airborne and wc.groundedWheelCount > 0
  if not groundControl then
    state.absSamples, state.absRelease = 0, 0
    state.lockSamples, state.lockRelease, state.lockPulsePhase = 0, 0, 0
    state.tcsSamples, state.tcsRelease = 0, 0
    state.wheelspinSamples, state.wheelspinRelease = 0, 0
  end
  local absRaw = groundControl and speedMS >= (tc.absMinSpeedMS or 1.5) and brake >= (tc.absMinBrakeInput or 0.12)
    and (nativeABS or wc.absWheelCount > 0)
  if absRaw then state.absSamples = state.absSamples + 1 else state.absSamples = 0 end
  if state.absSamples >= (tc.absDebounceSamples or 2) then
    state.absRelease = tc.absReleaseSeconds or 0.14
  else
    state.absRelease = max(state.absRelease - dt, 0)
  end
  local absActive = groundControl and brake >= (tc.absMinBrakeInput or 0.12) and
    (state.absSamples >= (tc.absDebounceSamples or 2) or state.absRelease > 0)

  local lockRaw = groundControl and wc.groundedBrakeCount > 0 and
    speedMS >= (tc.lockMinSpeedMS or 0.8) and brake >= (tc.absMinBrakeInput or 0.12) and
    not absActive and wc.lockSeverity > 0
  if lockRaw then state.lockSamples = state.lockSamples + 1 else state.lockSamples = 0 end
  if state.lockSamples >= (tc.lockDebounceSamples or 2) then
    state.lockRelease = tc.lockReleaseSeconds or 0.10
  else
    state.lockRelease = max(state.lockRelease - dt, 0)
  end
  local lockActive = groundControl and
    (state.lockSamples >= (tc.lockDebounceSamples or 2) or state.lockRelease > 0)

  local tcsRaw = groundControl and wc.groundedDrivenCount > 0 and
    speedMS >= (tc.tcsMinSpeedMS or 2.5) and throttle >= (tc.throttleActivation or 0.025) and nativeTCS
  if tcsRaw then state.tcsSamples = state.tcsSamples + 1 else state.tcsSamples = 0 end
  if state.tcsSamples >= (tc.tcsDebounceSamples or 2) then
    state.tcsRelease = tc.tcsReleaseSeconds or 0.13
  else
    state.tcsRelease = max(state.tcsRelease - dt, 0)
  end
  local tcsActive = groundControl and
    (state.tcsSamples >= (tc.tcsDebounceSamples or 2) or state.tcsRelease > 0)

  local meanDrivenSlip = wc.drivenCount > 0 and wc.drivenSlip / wc.drivenCount or 0
  local wheelspinRaw = groundControl and wc.groundedDrivenCount > 0 and not tcsActive and
    speedMS >= (tc.wheelspinMinSpeedMS or 3.0) and
    throttle >= (tc.wheelspinMinThrottle or 0.22) and
    meanDrivenSlip >= (tc.wheelspinSlipStart or 7.5)
  if wheelspinRaw then state.wheelspinSamples = state.wheelspinSamples + 1 else state.wheelspinSamples = 0 end
  if state.wheelspinSamples >= (tc.wheelspinDebounceSamples or 2) then
    state.wheelspinRelease = tc.wheelspinReleaseSeconds or 0.10
  else
    state.wheelspinRelease = max(state.wheelspinRelease - dt, 0)
  end
  local wheelspinActive = groundControl and
    (state.wheelspinSamples >= (tc.wheelspinDebounceSamples or 2) or state.wheelspinRelease > 0)

  local limiterRaw = throttle >= (tc.revLimiterMinThrottle or 0.65) and revLimiter
  if limiterRaw then state.limiterSamples = state.limiterSamples + 1 else state.limiterSamples = 0 end
  if state.limiterSamples >= (tc.revLimiterDebounceSamples or 1) then
    state.limiterRelease = tc.revLimiterReleaseSeconds or 0.11
  else
    state.limiterRelease = max(state.limiterRelease - dt, 0)
  end
  local limiterActive = state.limiterSamples >= (tc.revLimiterDebounceSamples or 1) or state.limiterRelease > 0
  if state.airborne then
    state.limiterRelease = 0
    limiterActive = limiterRaw
  end

  updateShift(dt, clutch, gear, shifting, speedMS, throttle, torqueNorm)

  local userTriggers = currentUserSettings.adaptiveTriggers
  local brakePrepared = tc.enabled and groundControl and tc.brakePreloadEnabled ~= false
    and userTriggers.l2BrakeEnabled ~= false
  if state.airborne and tc.airborneReleaseNormalTriggers ~= false then brakePrepared = false end
  local brakeActive = brakePrepared and brake >= (tc.brakeActivation or 0.005)
  local l2Effect = TRIGGER.off(state.l2Effect)
  if brakePrepared then
    local startForce = USER_SETTINGS.percentUnit(userTriggers.l2BrakeStartPercent, 25)
    local endForce = USER_SETTINGS.percentUnit(userTriggers.l2BrakeEndPercent, 75)
    if brakeActive then
      l2Effect = TRIGGER.resistance(tc.brakeStartPosition or 0, startForce, endForce, l2Effect)
      state.l2Level = lerp(startForce, endForce, brake)
    else
      -- The idle preload must follow the same UI resistance, not a fixed
      -- hard-coded force. Preserve the original stock preload ratio.
      local defaultStart = math.max(tc.brakeNormalStartForce or 0.25, 0.001)
      local preload = clamp01(startForce * (tc.brakePreloadForce or 0.125) / defaultStart)
      l2Effect = TRIGGER.resistance(tc.brakeStartPosition or 0, preload, preload, l2Effect)
      state.l2Level = preload
    end
  else
    state.l2Level = 0
  end

  if lockActive then
    l2Effect = TRIGGER.fine(tc.lockPosition or 0, tc.lockForce or 0.0208333333, l2Effect)
    state.l2Level = 0
    state.lockPulsePhase = 0
  else
    state.lockPulsePhase = 0
  end
  if absActive then
    local strongABS = wc.absSeverity >= (tc.absStrongSeverity or 0.55) or wc.absWheelCount >= 2
    l2Effect = TRIGGER.vibration(0,
      strongABS and (tc.absAmplitudeStrong or 0.625) or (tc.absAmplitudeSoft or 0.375),
      mapABSHapticHz(wc.absControlHz or 0, tc), l2Effect)
  elseif state.airborne and tc.airborneReducedTriggersEnabled ~= false then
    l2Effect = TRIGGER.fine(tc.airborneBrakePosition or 0, tc.airborneBrakeForce or 0.0208333333, l2Effect)
    state.l2Level = 0
  end

  local throttlePrepared = tc.enabled and tc.throttlePreloadEnabled ~= false
    and userTriggers.r2ThrottleEnabled ~= false
  if state.airborne and tc.airborneReleaseNormalTriggers ~= false then throttlePrepared = false end
  local r2Effect = TRIGGER.off(state.r2Effect)
  if throttlePrepared then
    local startForce = USER_SETTINGS.percentUnit(userTriggers.r2ThrottleStartPercent, 13)
    local endForce = USER_SETTINGS.percentUnit(userTriggers.r2ThrottleEndPercent, 13)
    r2Effect = TRIGGER.resistance(tc.throttleStartPosition or 0, startForce, endForce, r2Effect)
    state.r2Level = startForce
  else
    state.r2Level = 0
  end

  if state.airborne and tc.airborneReducedTriggersEnabled ~= false then
    r2Effect = TRIGGER.fine(tc.airborneThrottlePosition or 0, tc.airborneThrottleForce or 0.0208333333, r2Effect)
    state.r2Level = 0
  end

  if limiterActive then
    r2Effect = TRIGGER.vibration(0, tc.revLimiterAmplitude or 0.125, tc.revLimiterHz or 24, r2Effect)
  elseif state.shiftFineActive and tc.shiftFineReleaseEnabled ~= false then
    r2Effect = TRIGGER.fine(tc.shiftReleasePosition or 0, tc.shiftReleaseForce or 0.0208333333, r2Effect)
    state.r2Level = 0
  elseif tcsActive then
    local strongTCS = meanDrivenSlip >= (tc.tcsStrongSlip or 18.0)
    r2Effect = TRIGGER.vibration(0,
      strongTCS and (tc.tcsAmplitudeStrong or 0.25) or (tc.tcsAmplitudeSoft or 0.125),
      strongTCS and (tc.tcsHzStrong or 22) or (tc.tcsHzSoft or 19), r2Effect)
  elseif wheelspinActive then
    if userTriggers.r2ThrottleEnabled ~= false then
      r2Effect = TRIGGER.resistance(tc.throttleStartPosition or 0,
        USER_SETTINGS.percentUnit(userTriggers.r2ThrottleStartPercent, 13),
        USER_SETTINGS.percentUnit(userTriggers.r2ThrottleEndPercent, 13), r2Effect)
    end
  end

  -- User-facing dynamic-effects master. Older Bridges only understand the
  -- compatibility fields generated later, so suppress dynamic states here too.
  local userR2 = userTriggers
  if userR2.r2EffectsEnabled == false and
      (limiterActive or state.shiftFineActive or tcsActive or wheelspinActive or state.airborne) then
    if userR2.r2ThrottleEnabled ~= false then
      local startForce = USER_SETTINGS.percentUnit(userR2.r2ThrottleStartPercent or userR2.r2ThrottlePercent, 13)
      local endForce = USER_SETTINGS.percentUnit(userR2.r2ThrottleEndPercent or userR2.r2ThrottlePercent, 13)
      r2Effect = TRIGGER.resistance(tc.throttleStartPosition or 0, startForce, endForce, r2Effect)
      state.r2Level = startForce
    else
      r2Effect = TRIGGER.off(r2Effect)
      state.r2Level = 0
    end
  end

  updateNativeRumble(dt, speedMS)

  local nrCfg = CONFIG.nativeRumble or {}
  local shiftSurfaceDuck = state.nativeShiftPulseAge < (nrCfg.shiftSurfaceDuckSeconds or 0.095)
  local packetSurfaceMaterialL = shiftSurfaceDuck and -1 or (wc.surfaceMaterialL or -1)
  local packetSurfaceRoughnessL = shiftSurfaceDuck and 0 or (wc.roughnessL or 0)
  local packetSurfaceMaterialR = shiftSurfaceDuck and -1 or (wc.surfaceMaterialR or -1)
  local packetSurfaceRoughnessR = shiftSurfaceDuck and 0 or (wc.roughnessR or 0)
  local packetRoadExcitationL = shiftSurfaceDuck and 0 or (wc.roadExcitationL or 0)
  local packetRoadExcitationR = shiftSurfaceDuck and 0 or (wc.roadExcitationR or 0)
  local packetRoadRollingExcitationL = shiftSurfaceDuck and 0 or (wc.roadRollingExcitationL or 0)
  local packetRoadRollingExcitationR = shiftSurfaceDuck and 0 or (wc.roadRollingExcitationR or 0)

  if not shiftSurfaceDuck then
    local asphaltProfile = currentUserSettings.haptics.surfaceProfiles.asphalt or {}
    local asphaltReference = USER_SETTINGS.rollingReference("asphalt")
    local asphaltRollingGain = USER_SETTINGS.referenceGain(asphaltProfile.rollingPercent, asphaltReference)
    local asphaltBoosted = asphaltRollingGain > 1.0001
    local asphaltThreshold = asphaltBoosted and 0 or max(num(nrCfg.asphaltBridgeExcitationThreshold, 0.10), 0)
    local asphaltGain = asphaltBoosted and 1 or clamp(num(nrCfg.asphaltBridgeGain, 0.55), 0, 1)
    if packetSurfaceMaterialL == 10 then
      if packetRoadExcitationL < asphaltThreshold then
        packetSurfaceMaterialL, packetSurfaceRoughnessL, packetRoadExcitationL, packetRoadRollingExcitationL = -1, 0, 0, 0
      else
        packetSurfaceRoughnessL = packetSurfaceRoughnessL * asphaltGain
        packetRoadExcitationL = packetRoadExcitationL * asphaltGain
        packetRoadRollingExcitationL = packetRoadRollingExcitationL * asphaltGain
      end
    end
    if packetSurfaceMaterialR == 10 then
      if packetRoadExcitationR < asphaltThreshold then
        packetSurfaceMaterialR, packetSurfaceRoughnessR, packetRoadExcitationR, packetRoadRollingExcitationR = -1, 0, 0, 0
      else
        packetSurfaceRoughnessR = packetSurfaceRoughnessR * asphaltGain
        packetRoadExcitationR = packetRoadExcitationR * asphaltGain
        packetRoadRollingExcitationR = packetRoadRollingExcitationR * asphaltGain
      end
    end
  end

  local queuedBodyEvents = #(state.pendingBodyEvents or {})
  if queuedBodyEvents > 0 then
    -- Promote one queued impact before capturing the control signature so the
    -- new bodyEvent counter does not trigger a redundant packet next tick.
    flushPendingBodyEvent()
  end

  local controlChanged = triggerControlChanged(l2Effect, r2Effect)
  local telemetryPeriod = 1 / max(CONFIG.telemetryHz or 15, 2)
  local settingsHeartbeatPeriod = max(CONFIG.settingsHeartbeatSeconds or 1.0, 0.25)
  local settingsPacket, settingsRevision = USER_SETTINGS.packetSnapshot(currentUserSettings)
  local settingsChanged = settingsRevision ~= state.lastSettingsRevisionSent
  local settingsHeartbeatDue = state.settingsHeartbeatTimer >= settingsHeartbeatPeriod
  local sendDue = controlChanged or settingsChanged or
    state.telemetryTimer >= telemetryPeriod or queuedBodyEvents > 0
  if not sendDue then return nil end
  if state.telemetryTimer >= telemetryPeriod then state.telemetryTimer = state.telemetryTimer - telemetryPeriod end

  local settingsForPacket = nil
  if settingsChanged or settingsHeartbeatDue then
    settingsForPacket = settingsPacket
    state.lastSettingsRevisionSent = settingsRevision
    state.settingsHeartbeatTimer = 0
  end

  -- Protocol v43 is the V1.41 migration generation and carries the same semantic trigger effects. User settings are sparse:
  -- sent immediately when changed and periodically as a recovery heartbeat.
  local packet = {
    protocolId = COMPATIBILITY.protocolId,
    modVersion = COMPATIBILITY.modVersion,
    protocolMin = COMPATIBILITY.protocolMin,
    protocolMax = COMPATIBILITY.protocolMax,
    v = COMPATIBILITY.protocolVersion, active = true, seq = state.seq,
    -- Legacy protocol hint only; the lightbar does not use this value.
    shiftLEDsInUse = engineRunning and maxRPM > 1,
    userSettings = settingsForPacket,
    l2Effect = l2Effect, r2Effect = r2Effect,
    bodyEvent = state.bodyEvent,
    shiftEvent = state.shiftEvent, shiftStrength = tc.shiftStrength or 0.018,
    shiftDurationMS = tc.shiftDurationMS or 20,
    raw = CONFIG.includeRawTelemetry and {
      speed = speedMS, rpm = rpm, maxRPM = maxRPM, brake = brake, shifting = shifting,
      engineRunning = engineRunning, revLimiter = revLimiter,
      abs = absActive, absRaw = absRaw, absWheelCount = wc.absWheelCount, absSeverity = wc.absSeverity,
      absControlHz = wc.absControlHz or 0, lockHaptic = lockActive,
      tcs = tcsActive, tcsRaw = tcsRaw, wheelspin = wheelspinActive, drivenSlip = meanDrivenSlip,
      airborne = state.airborne, groundedWheels = wc.groundedWheelCount or 0,
      surfaceMaterialL = packetSurfaceMaterialL, surfaceRoughnessL = packetSurfaceRoughnessL,
      surfaceMaterialR = packetSurfaceMaterialR, surfaceRoughnessR = packetSurfaceRoughnessR,
      roadExcitationL = packetRoadExcitationL, roadExcitationR = packetRoadExcitationR,
      roadRollingExcitationValid = true,
      roadRollingExcitationL = packetRoadRollingExcitationL, roadRollingExcitationR = packetRoadRollingExcitationR,
      roadSlipL = wc.roadSlipL or 0, roadSlipR = wc.roadSlipR or 0,
      lock = wc.lockSeverity > 0, lockedWheelCount = wc.lockedWheelCount,
      candidateL = wc.candidateL, candidateR = wc.candidateR,
      peakImpulseL = wc.peakImpulseL or 0, peakImpulseR = wc.peakImpulseR or 0,
      nativeRumbleBaseForce = state.nativeRumbleBaseForce,
    } or nil,
  }

  -- Body-event metadata is emitted only for the packet that owns the event.
  -- The event counter remains continuous so a newly started Bridge can
  -- synchronize without replaying an old impact.
  if state.bodyEventOccupiedThisPacket then
    packet.bodySide, packet.bodyStrength = state.bodySide, state.bodyStrength
    packet.bodyLeftStrength, packet.bodyRightStrength = state.bodyLeftStrength, state.bodyRightStrength
    packet.bodyDurationMS, packet.bodyKind, packet.bodyProfile = state.bodyDurationMS, state.bodyKind, state.bodyProfile
    packet.bodySourceReason, packet.bodySourceConfidence = state.bodySourceReason, state.bodySourceConfidence
    packet.bodySourceLeftScore, packet.bodySourceRightScore = state.bodySourceLeftScore, state.bodySourceRightScore
    packet.bodySourceLeftContact, packet.bodySourceRightContact = state.bodySourceLeftContact, state.bodySourceRightContact
    packet.bodySourceLeftPeak, packet.bodySourceRightPeak = state.bodySourceLeftPeak, state.bodySourceRightPeak
    packet.bodySourceLeftEnergy, packet.bodySourceRightEnergy = state.bodySourceLeftEnergy, state.bodySourceRightEnergy
    packet.bodySourceLeftWheel, packet.bodySourceRightWheel = state.bodySourceLeftWheel, state.bodySourceRightWheel
    packet.bodySourceLeftAxle, packet.bodySourceRightAxle = state.bodySourceLeftAxle, state.bodySourceRightAxle
  end
  return packet
end

local function sendStop()
  local currentStop = {
    protocolId = COMPATIBILITY.protocolId,
    modVersion = COMPATIBILITY.modVersion,
    protocolMin = COMPATIBILITY.protocolMin,
    protocolMax = COMPATIBILITY.protocolMax,
    v = COMPATIBILITY.protocolVersion,
    active = false,
  }
  local payload = jsonEncode(currentStop)
  if udp then pcall(function() udp:send(payload) end) end
  if udpBluetooth then pcall(function() udpBluetooth:send(payload) end) end

  if legacyDualPublishEnabled() then
    local legacyStop = {
      protocolId = COMPATIBILITY.protocolId,
      modVersion = COMPATIBILITY.modVersion,
      protocolMin = COMPATIBILITY.protocolMin,
      protocolMax = COMPATIBILITY.protocolMax,
      legacyCompat = true,
      v = tonumber(COMPATIBILITY.legacyProtocolVersion) or 40,
      active = false,
    }
    local legacyPayload = jsonEncode(legacyStop)
    if udp then pcall(function() udp:send(legacyPayload) end) end
    if udpBluetooth then pcall(function() udpBluetooth:send(legacyPayload) end) end
  end
end

local function scheduleInit()
  sendStop()
  closeUDP()
  state.initialized = false
  state.initRetry = 0.20
  state.controlTimer = 0
end

local function updateGFX(dt)
  if not CONFIG.enabled or not playerInfo or not playerInfo.firstPlayerSeated then return end
  state.errorCooldown = max(state.errorCooldown - dt, 0)
  if not state.initialized then
    state.initRetry = state.initRetry - dt
    if state.initRetry <= 0 then
      state.initRetry = 0.75
      local ok, result = pcall(initialize)
      if not ok or not result then
        closeUDP()
        if not ok and state.errorCooldown <= 0 then
          log("E", "DualSensePhysicsTelemetry", "init isolated: " .. tostring(result))
          state.errorCooldown = 2
        end
      end
    end

    return
  end

  state.controlTimer = state.controlTimer + dt
  local period = 1 / max(CONFIG.controlHz or 30, 10)
  if state.controlTimer < period then return end
  local sampleDt = min(max(state.controlTimer, period), 0.10)
  state.controlTimer = max(state.controlTimer - period, 0)

  local ok, packet = pcall(buildPacket, sampleDt)
  if not ok then
    sendStop()
    closeUDP()
    state.initialized = false
    state.initRetry = 1
    if state.errorCooldown <= 0 then
      log("E", "DualSensePhysicsTelemetry", "runtime isolated: " .. tostring(packet))
      state.errorCooldown = 2
    end
    return
  end
  if not packet then return end

  local encodedOK, payload = pcall(jsonEncode, packet)
  if not encodedOK then
    packet = payload
  else
    -- Publish canonical migration generation 43. Legacy dual-publish remains disabled so
    -- migration lane for older Bridges and must be ignored by current Bridges.
    local sent, sendErr = pcall(function() return udp:send(payload) end)
    if udpBluetooth then
      pcall(function() udpBluetooth:send(payload) end)
    end
    if sent and legacyDualPublishEnabled() then
      local legacyPacket = buildLegacyV40Packet(packet)
      local legacyOK, legacyPayload = pcall(jsonEncode, legacyPacket)
      if legacyOK then
        pcall(function() udp:send(legacyPayload) end)
        if udpBluetooth then pcall(function() udpBluetooth:send(legacyPayload) end) end
      end
    end
    if sent then return end
    packet = sendErr
  end

  sendStop()
  closeUDP()
  state.initialized = false
  state.initRetry = 1
  if state.errorCooldown <= 0 then
    log("E", "DualSensePhysicsTelemetry", "send isolated: " .. tostring(packet))
    state.errorCooldown = 2
  end
end

local function onExtensionLoaded()
  scheduleInit()
end
local function onVehicleLoaded() scheduleInit() end
local function onReset() scheduleInit() end
local function onExtensionUnloaded()
  sendStop()
  closeUDP()
  state.initialized = false
end

M.onExtensionLoaded = onExtensionLoaded
M.onVehicleLoaded = onVehicleLoaded
M.onReset = onReset
M.updateGFX = updateGFX
M.onExtensionUnloaded = onExtensionUnloaded
return M
