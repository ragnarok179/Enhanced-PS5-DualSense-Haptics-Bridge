-- Persistent BeamNG settings and wire compatibility for Enhanced PS5 DualSense Haptics.
-- UI percentages live here; gameplay consumes normalized trigger intents and physical telemetry.
-- Haptic/light byte-scaled fields stay protocol-specific; adaptive-trigger strength uses the canonical 0..48 force lattice.

local M = {}
local SETTINGS_STORE = require("dualsenseSuite/settingsStore")
local min, max, floor = math.min, math.max, math.floor

local function num(v, fallback)
  return type(v) == "number" and v or (fallback or 0)
end

local function clamp(v, lo, hi)
  v = tonumber(v) or lo
  return min(max(v, lo), hi)
end

local USER_SETTINGS_KEY = "dphSettings"
local USER_SETTINGS_SURFACES = {
  "asphalt",
  "asphalt_wet",
  "slippery",
  "ice",
  "dirt",
  "dusty_dirt",
  "sandy_road",
  "sand",
  "mud",
  "gravel",
  "grass",
  "snow",
  "rock",
  "cobblestone",
  "rumble_strip",
}
local SURFACE_ROLLING_REFERENCES = {
  -- Displayed stock power, measured against the active RMS of the strongest
  -- calibrated primary suspension bump. These are labels only: at the shown
  -- default value the runtime gain is exactly 1.0, so the original feel is kept.
  asphalt = 10, asphalt_wet = 6, slippery = 3, ice = 3,
  dirt = 23, dusty_dirt = 22, sandy_road = 34, sand = 24, mud = 22,
  gravel = 62, grass = 12, snow = 8, rock = 31, cobblestone = 70, rumble_strip = 84,
}
local SURFACE_SLIP_REFERENCES = {
  -- Measured incremental slip RMS versus the strongest calibrated suspension
  -- bump in the standard reference scene. These are display values only; the
  -- wire control remains anchored to the schema-9 1% internal reference so the
  -- stock physical feel is unchanged at each displayed default.
  asphalt = 0.5, asphalt_wet = 2.6, slippery = 0.5, ice = 0.6,
  dirt = 0.5, dusty_dirt = 0.5, sandy_road = 0.7, sand = 1.0, mud = 0.5,
  gravel = 1.2, grass = 0.3, snow = 0.3, rock = 0.5, cobblestone = 0.4, rumble_strip = 0.1,
}
local SCHEMA9_SURFACE_SLIP_REFERENCES = {
  asphalt = 1, asphalt_wet = 1, slippery = 1, ice = 1,
  dirt = 1, dusty_dirt = 1, sandy_road = 1, sand = 1, mud = 1,
  gravel = 1, grass = 1, snow = 1, rock = 1, cobblestone = 1, rumble_strip = 1,
}
local SCHEMA8_SURFACE_ROLLING_REFERENCES = {
  asphalt = 29, asphalt_wet = 43, slippery = 43, ice = 43,
  dirt = 70, dusty_dirt = 70, sandy_road = 65, sand = 48, mud = 58,
  gravel = 100, grass = 45, snow = 39, rock = 100, cobblestone = 100, rumble_strip = 100,
}
local SCHEMA8_SURFACE_SLIP_REFERENCES = {
  asphalt = 100, asphalt_wet = 100, slippery = 100, ice = 100,
  dirt = 100, dusty_dirt = 100, sandy_road = 100, sand = 100, mud = 100,
  gravel = 100, grass = 100, snow = 100, rock = 100, cobblestone = 100, rumble_strip = 100,
}
local SCHEMA7_SURFACE_ROLLING_REFERENCES = {
  asphalt = 22, asphalt_wet = 26, slippery = 20, ice = 20,
  dirt = 50, dusty_dirt = 50, sandy_road = 50, sand = 28, mud = 32,
  gravel = 62, grass = 36, snow = 36, rock = 78, cobblestone = 74, rumble_strip = 88,
}
local SCHEMA7_SURFACE_SLIP_REFERENCES = {
  asphalt = 3, asphalt_wet = 7, slippery = 8, ice = 7,
  dirt = 8, dusty_dirt = 8, sandy_road = 5, sand = 3, mud = 4,
  gravel = 10, grass = 5, snow = 4, rock = 8, cobblestone = 6, rumble_strip = 4,
}
local USER_SETTINGS_SCHEMA = 12
local HAPTIC_REFERENCE_PERCENT = 100
local SURFACE_MASTER_REFERENCE_PERCENT = 100
local LED_REFERENCE_PERCENT = 86
local R2_JOLT_REFERENCE_PERCENT = 25
local userSettingsCache = {
  rawKind = nil,
  rawString = nil,
  rawTableSnapshot = nil,
  decoded = nil,
  generation = 0,
  packet = nil,
  packetGeneration = -1,
  packetRevision = 0,
}

local function defaultSurfaceProfiles()
  local profiles = {}
  for _, name in ipairs(USER_SETTINGS_SURFACES) do
    profiles[name] = {
      rollingPercent = SURFACE_ROLLING_REFERENCES[name] or 100,
      slipPercent = SURFACE_SLIP_REFERENCES[name] or 1,
    }
  end
  return profiles
end

local function defaultUserSettings()
  return {
    schema = USER_SETTINGS_SCHEMA,
    haptics = {
      masterEnabled = true,
      masterPercent = HAPTIC_REFERENCE_PERCENT,
      surfaceEnabled = true,
      surfacePercent = SURFACE_MASTER_REFERENCE_PERCENT,
      impactEnabled = true,
      impactPercent = 100,
      surfaceProfiles = defaultSurfaceProfiles(),
    },
    adaptiveTriggers = {
      l2BrakeEnabled = true,
      l2BrakeStartPercent = 25,
      l2BrakeEndPercent = 75,
      absEnabled = true,
      absPercent = 75,
      r2ThrottleEnabled = true,
      r2ThrottleStartPercent = 13,
      r2ThrottleEndPercent = 13,
      r2EffectsEnabled = true,
      r2EffectsPercent = R2_JOLT_REFERENCE_PERCENT,
    },
    lighting = { enabled = true, brightnessPercent = LED_REFERENCE_PERCENT }
  }
end

local function clampPercent(value, fallback)
  value = tonumber(value)
  if value == nil then value = fallback or 100 end
  if value < 1 then return 1 end
  if value > 100 then return 100 end
  return floor(value + 0.5)
end

local function percentTo255(value)
  return clamp(floor(clampPercent(value) * 255 / 100 + 0.5), 1, 255)
end

local function percentToTrigger48(value)
  return clamp(floor(clampPercent(value) * 48 / 100 + 0.5), 1, 48)
end

local function clampFinePercent(value, fallback)
  value = tonumber(value)
  if value == nil then value = fallback or 1 end
  if value < 0.1 then return 0.1 end
  if value > 100 then return 100 end
  return floor(value * 10 + 0.5) / 10
end

local function percentTo255ZeroAllowed(value)
  value = tonumber(value) or 0
  if value <= 0 then return 0 end
  value = min(value, 100)
  return clamp(floor(value * 255 / 100 + 0.5), 1, 255)
end

local function slipDisplayToWirePercent(name, displayPercent)
  local displayRef = SURFACE_SLIP_REFERENCES[name] or 1
  local wireRef = SCHEMA9_SURFACE_SLIP_REFERENCES[name] or 1
  local value = clampFinePercent(displayPercent, displayRef)
  if value <= displayRef then
    return wireRef * value / max(displayRef, 0.1)
  end
  return wireRef + (100 - wireRef) * ((value - displayRef) / max(100 - displayRef, 0.1))
end

local function scaleOldGain(value, reference, fallback)
  return clampPercent(clampPercent(value, fallback or 100) * reference / 100, reference)
end

local function scaleBetweenReferences(value, oldReference, newReference, fallback)
  oldReference = max(num(oldReference, 100), 1)
  newReference = max(num(newReference, 100), 1)
  local gain = clampPercent(value, fallback or oldReference) / oldReference
  return clampPercent(newReference * gain, newReference)
end

local function normalizeUserSettings(data)
  local d = defaultUserSettings()
  local src = type(data) == "table" and data or {}
  local h = type(src.haptics) == "table" and src.haptics or {}
  local a = type(src.adaptiveTriggers) == "table" and src.adaptiveTriggers or {}
  local l = type(src.lighting) == "table" and src.lighting or {}
  local sourceSchema = floor(num(src.schema, 1))

  d.haptics.masterEnabled = h.masterEnabled ~= false
  d.haptics.surfaceEnabled = h.surfaceEnabled ~= false
  d.haptics.impactEnabled = h.impactEnabled ~= false

  if sourceSchema >= 10 then
    d.haptics.masterPercent = clampPercent(h.masterPercent, HAPTIC_REFERENCE_PERCENT)
    d.haptics.surfacePercent = clampPercent(h.surfacePercent, SURFACE_MASTER_REFERENCE_PERCENT)
  elseif sourceSchema == 8 then
    -- Schema 8 stored master/surface at 100% for unity gain.
    d.haptics.masterPercent = clampPercent(h.masterPercent, 100)
    d.haptics.surfacePercent = scaleBetweenReferences(h.surfacePercent, 100, SURFACE_MASTER_REFERENCE_PERCENT, 100)
  elseif sourceSchema == 7 then
    d.haptics.masterPercent = scaleBetweenReferences(h.masterPercent, 65, HAPTIC_REFERENCE_PERCENT, 65)
    d.haptics.surfacePercent = scaleBetweenReferences(h.surfacePercent, 88, SURFACE_MASTER_REFERENCE_PERCENT, 88)
  elseif sourceSchema == 6 then
    d.haptics.masterPercent = scaleBetweenReferences(h.masterPercent, 100, HAPTIC_REFERENCE_PERCENT, 100)
    d.haptics.surfacePercent = scaleBetweenReferences(h.surfacePercent, 100, SURFACE_MASTER_REFERENCE_PERCENT, 100)
  elseif sourceSchema == 5 then
    d.haptics.masterPercent = scaleBetweenReferences(h.masterPercent, 65, HAPTIC_REFERENCE_PERCENT, 65)
    d.haptics.surfacePercent = scaleBetweenReferences(h.surfacePercent, 88, SURFACE_MASTER_REFERENCE_PERCENT, 88)
  elseif sourceSchema == 4 then
    d.haptics.masterPercent = clampPercent(h.masterPercent, HAPTIC_REFERENCE_PERCENT)
    d.haptics.surfacePercent = scaleBetweenReferences(h.surfacePercent, 100, SURFACE_MASTER_REFERENCE_PERCENT, 100)
  else
    local p = clampPercent(h.masterPercent, 65)
    local gain = 1
    if p <= 1 then gain = 0.05
    elseif p <= 65 then gain = 0.05 + (1 - 0.05) * ((p - 1) / 64) end
    d.haptics.masterPercent = clampPercent(HAPTIC_REFERENCE_PERCENT * gain, HAPTIC_REFERENCE_PERCENT)
    if sourceSchema >= 3 then
      d.haptics.surfacePercent = scaleBetweenReferences(h.surfacePercent, 88, SURFACE_MASTER_REFERENCE_PERCENT, 88)
    else
      d.haptics.surfacePercent = scaleBetweenReferences(h.surfacePercent, 100, SURFACE_MASTER_REFERENCE_PERCENT, 100)
    end
  end

  d.haptics.impactPercent = clampPercent(h.impactPercent, 100)

  for _, name in ipairs(USER_SETTINGS_SURFACES) do
    local newRollingRef = SURFACE_ROLLING_REFERENCES[name] or 100
    local newSlipRef = SURFACE_SLIP_REFERENCES[name] or 1
    local stored = type(h.surfaceProfiles) == "table" and h.surfaceProfiles[name] or nil
    if sourceSchema >= 10 and type(stored) == "table" then
      d.haptics.surfaceProfiles[name].rollingPercent = clampPercent(stored.rollingPercent, newRollingRef)
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(stored.slipPercent, newSlipRef)
    elseif sourceSchema == 9 and type(stored) == "table" then
      d.haptics.surfaceProfiles[name].rollingPercent = clampPercent(stored.rollingPercent, newRollingRef)
      local oldSlipRef = SCHEMA9_SURFACE_SLIP_REFERENCES[name] or 1
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(newSlipRef * clampPercent(stored.slipPercent, oldSlipRef) / oldSlipRef, newSlipRef)
    elseif sourceSchema == 8 and type(stored) == "table" then
      d.haptics.surfaceProfiles[name].rollingPercent = scaleBetweenReferences(
        stored.rollingPercent, SCHEMA8_SURFACE_ROLLING_REFERENCES[name] or 100, newRollingRef,
        SCHEMA8_SURFACE_ROLLING_REFERENCES[name] or 100)
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(newSlipRef * clampPercent(stored.slipPercent, SCHEMA8_SURFACE_SLIP_REFERENCES[name] or 100) / (SCHEMA8_SURFACE_SLIP_REFERENCES[name] or 100), newSlipRef)
    elseif sourceSchema == 7 and type(stored) == "table" then
      d.haptics.surfaceProfiles[name].rollingPercent = scaleBetweenReferences(
        stored.rollingPercent, SCHEMA7_SURFACE_ROLLING_REFERENCES[name] or 100, newRollingRef,
        SCHEMA7_SURFACE_ROLLING_REFERENCES[name] or 100)
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(newSlipRef * clampPercent(stored.slipPercent, SCHEMA7_SURFACE_SLIP_REFERENCES[name] or 100) / (SCHEMA7_SURFACE_SLIP_REFERENCES[name] or 100), newSlipRef)
    elseif sourceSchema == 6 and type(stored) == "table" then
      d.haptics.surfaceProfiles[name].rollingPercent = scaleBetweenReferences(stored.rollingPercent, 100, newRollingRef, 100)
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(newSlipRef * clampPercent(stored.slipPercent, 100) / 100, newSlipRef)
    elseif sourceSchema == 5 and type(stored) == "table" then
      d.haptics.surfaceProfiles[name].rollingPercent = scaleBetweenReferences(
        stored.rollingPercent, SCHEMA7_SURFACE_ROLLING_REFERENCES[name] or 100, newRollingRef,
        SCHEMA7_SURFACE_ROLLING_REFERENCES[name] or 100)
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(newSlipRef * clampPercent(stored.slipPercent, SCHEMA7_SURFACE_SLIP_REFERENCES[name] or 100) / (SCHEMA7_SURFACE_SLIP_REFERENCES[name] or 100), newSlipRef)
    elseif sourceSchema == 4 and type(stored) == "table" then
      d.haptics.surfaceProfiles[name].rollingPercent = scaleBetweenReferences(stored.rollingPercent, 100, newRollingRef, 100)
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(newSlipRef * clampPercent(stored.slipPercent, 100) / 100, newSlipRef)
    elseif sourceSchema >= 3 then
      local oldRef = SCHEMA7_SURFACE_ROLLING_REFERENCES[name] or 100
      local oldValue = clampPercent(stored, oldRef)
      local gain = oldValue / oldRef
      d.haptics.surfaceProfiles[name].rollingPercent = clampPercent(newRollingRef * gain, newRollingRef)
      d.haptics.surfaceProfiles[name].slipPercent = clampFinePercent(newSlipRef * gain, newSlipRef)
    end
  end

  d.adaptiveTriggers.l2BrakeEnabled = a.l2BrakeEnabled ~= false
  if sourceSchema >= 12 then
    d.adaptiveTriggers.l2BrakeStartPercent = clampPercent(a.l2BrakeStartPercent, 25)
    d.adaptiveTriggers.l2BrakeEndPercent = clampPercent(a.l2BrakeEndPercent, 75)
  elseif sourceSchema >= 10 then
    local legacyStart = clampPercent(a.l2BrakeStartPercent, 13)
    local legacyEnd = clampPercent(a.l2BrakeEndPercent, 50)
    -- Migrate only the historical default pair; preserve custom brake tuning.
    if legacyStart == 13 and legacyEnd == 50 then
      d.adaptiveTriggers.l2BrakeStartPercent = 25
      d.adaptiveTriggers.l2BrakeEndPercent = 75
    else
      d.adaptiveTriggers.l2BrakeStartPercent = legacyStart
      d.adaptiveTriggers.l2BrakeEndPercent = legacyEnd
    end
  else
    local legacyEnd = clampPercent(a.l2BrakePercent, 50)
    d.adaptiveTriggers.l2BrakeEndPercent = legacyEnd
    d.adaptiveTriggers.l2BrakeStartPercent = max(1, min(legacyEnd, floor(legacyEnd * 0.25 + 0.5)))
  end
  d.adaptiveTriggers.absEnabled = a.absEnabled ~= false
  d.adaptiveTriggers.absPercent = clampPercent(a.absPercent, 75)
  d.adaptiveTriggers.r2ThrottleEnabled = a.r2ThrottleEnabled ~= false
  if sourceSchema >= 10 then
    d.adaptiveTriggers.r2ThrottleStartPercent = clampPercent(a.r2ThrottleStartPercent, 13)
    d.adaptiveTriggers.r2ThrottleEndPercent = clampPercent(a.r2ThrottleEndPercent, 13)
  else
    local legacyR2 = clampPercent(a.r2ThrottlePercent, 13)
    d.adaptiveTriggers.r2ThrottleStartPercent = legacyR2
    d.adaptiveTriggers.r2ThrottleEndPercent = legacyR2
  end
  d.adaptiveTriggers.r2EffectsEnabled = a.r2EffectsEnabled ~= false
  local oldR2 = clampPercent(a.r2EffectsPercent, sourceSchema >= 3 and R2_JOLT_REFERENCE_PERCENT or 100)
  d.adaptiveTriggers.r2EffectsPercent = sourceSchema >= 3 and oldR2 or clampPercent(oldR2 * R2_JOLT_REFERENCE_PERCENT / 100, R2_JOLT_REFERENCE_PERCENT)

  d.lighting.enabled = l.enabled ~= false
  if sourceSchema >= 5 then
    d.lighting.brightnessPercent = clampPercent(l.brightnessPercent, LED_REFERENCE_PERCENT)
  else
    d.lighting.brightnessPercent = scaleOldGain(l.brightnessPercent, LED_REFERENCE_PERCENT, 100)
  end
  d.schema = USER_SETTINGS_SCHEMA
  return d
end

local HAPTIC_CACHE_KEYS = {
  "masterEnabled", "masterPercent", "surfaceEnabled", "surfacePercent",
  "impactEnabled", "impactPercent",
}
local TRIGGER_CACHE_KEYS = {
  "l2BrakeEnabled", "l2BrakeStartPercent", "l2BrakeEndPercent", "l2BrakePercent",
  "absEnabled", "absPercent",
  "r2ThrottleEnabled", "r2ThrottleStartPercent", "r2ThrottleEndPercent", "r2ThrottlePercent",
  "r2EffectsEnabled", "r2EffectsPercent",
}
local LIGHT_CACHE_KEYS = { "enabled", "brightnessPercent" }

local EMPTY_TABLE = {}

local function sectionOrEmpty(value)
  return type(value) == "table" and value or EMPTY_TABLE
end

local function sameKeys(current, cached, keys)
  current = sectionOrEmpty(current)
  cached = sectionOrEmpty(cached)
  for _, key in ipairs(keys) do
    if current[key] ~= cached[key] then return false end
  end
  return true
end

local function rawTableUnchanged(raw)
  local cached = userSettingsCache.rawTableSnapshot
  if not cached or cached.schema ~= raw.schema then return false end
  local h, a, l = sectionOrEmpty(raw.haptics), sectionOrEmpty(raw.adaptiveTriggers), sectionOrEmpty(raw.lighting)
  if not sameKeys(h, cached.haptics, HAPTIC_CACHE_KEYS) or
     not sameKeys(a, cached.adaptiveTriggers, TRIGGER_CACHE_KEYS) or
     not sameKeys(l, cached.lighting, LIGHT_CACHE_KEYS) then
    return false
  end
  local currentProfiles = sectionOrEmpty(h.surfaceProfiles)
  local cachedProfiles = sectionOrEmpty(cached.haptics and cached.haptics.surfaceProfiles)
  for _, name in ipairs(USER_SETTINGS_SURFACES) do
    local current = sectionOrEmpty(currentProfiles[name])
    local previous = sectionOrEmpty(cachedProfiles[name])
    if current.rollingPercent ~= previous.rollingPercent or current.slipPercent ~= previous.slipPercent then
      return false
    end
  end
  return true
end

local function copyKeys(source, keys)
  source = sectionOrEmpty(source)
  local out = {}
  for _, key in ipairs(keys) do out[key] = source[key] end
  return out
end

local function snapshotRawTable(raw)
  local h = sectionOrEmpty(raw.haptics)
  local snapshot = {
    schema = raw.schema,
    haptics = copyKeys(h, HAPTIC_CACHE_KEYS),
    adaptiveTriggers = copyKeys(raw.adaptiveTriggers, TRIGGER_CACHE_KEYS),
    lighting = copyKeys(raw.lighting, LIGHT_CACHE_KEYS),
  }
  snapshot.haptics.surfaceProfiles = {}
  local profiles = sectionOrEmpty(h.surfaceProfiles)
  for _, name in ipairs(USER_SETTINGS_SURFACES) do
    local profile = sectionOrEmpty(profiles[name])
    snapshot.haptics.surfaceProfiles[name] = {
      rollingPercent = profile.rollingPercent,
      slipPercent = profile.slipPercent,
    }
  end
  return snapshot
end

local function cacheDecoded(rawKind, rawString, rawTableSnapshot, normalized)
  userSettingsCache.rawKind = rawKind
  userSettingsCache.rawString = rawString
  userSettingsCache.rawTableSnapshot = rawTableSnapshot
  userSettingsCache.decoded = normalized
  userSettingsCache.generation = userSettingsCache.generation + 1
  return normalized
end

local function readUserSettings()
  local raw = SETTINGS_STORE.get(USER_SETTINGS_KEY, nil)
  local decoded = nil

  if type(raw) == "table" then
    -- BeamNG may mutate and return the same table object. Compare only the
    -- settings consumed by this mod; unchanged frames reuse the normalized
    -- object and generate no migration/profile tables.
    if userSettingsCache.rawKind == "table" and userSettingsCache.decoded and rawTableUnchanged(raw) then
      return userSettingsCache.decoded
    end
    decoded = raw
    return cacheDecoded("table", nil, snapshotRawTable(raw), normalizeUserSettings(decoded))
  end

  if type(raw) == "string" and raw ~= "" then
    if userSettingsCache.rawKind == "string" and raw == userSettingsCache.rawString and userSettingsCache.decoded then
      return userSettingsCache.decoded
    end
    local ok, value = pcall(jsonDecode, raw)
    if ok and type(value) == "table" then decoded = value end
    return cacheDecoded("string", raw, nil, normalizeUserSettings(decoded))
  end

  if userSettingsCache.rawKind == "default" and userSettingsCache.decoded then
    return userSettingsCache.decoded
  end
  return cacheDecoded("default", nil, nil, normalizeUserSettings(nil))
end

local function buildUserSettingsPacket(s)
  s = s or readUserSettings()
  local rollingStrengths, slipStrengths = {}, {}
  for _, name in ipairs(USER_SETTINGS_SURFACES) do
    local profile = s.haptics.surfaceProfiles[name] or { rollingPercent = SURFACE_ROLLING_REFERENCES[name] or 100, slipPercent = SURFACE_SLIP_REFERENCES[name] or 1 }
    rollingStrengths[name] = percentTo255(profile.rollingPercent)
    slipStrengths[name] = percentTo255ZeroAllowed(slipDisplayToWirePercent(name, profile.slipPercent))
  end
  return {
    schema = USER_SETTINGS_SCHEMA,
    triggerForceScale = 48,
    masterEnabled = s.haptics.masterEnabled,
    masterStrength = percentTo255(s.haptics.masterPercent),
    surfaceEnabled = s.haptics.surfaceEnabled,
    surfaceStrength = percentTo255(s.haptics.surfacePercent),
    impactEnabled = s.haptics.impactEnabled,
    impactStrength = percentTo255(s.haptics.impactPercent),
    l2BrakeEnabled = s.adaptiveTriggers.l2BrakeEnabled,
    l2BrakeStartStrength = percentToTrigger48(s.adaptiveTriggers.l2BrakeStartPercent),
    l2BrakeEndStrength = percentToTrigger48(s.adaptiveTriggers.l2BrakeEndPercent),
    l2BrakeStrength = percentToTrigger48(s.adaptiveTriggers.l2BrakeEndPercent),
    absEnabled = s.adaptiveTriggers.absEnabled,
    absStrength = percentToTrigger48(s.adaptiveTriggers.absPercent),
    r2ThrottleEnabled = s.adaptiveTriggers.r2ThrottleEnabled,
    r2ThrottleStartStrength = percentToTrigger48(s.adaptiveTriggers.r2ThrottleStartPercent),
    r2ThrottleEndStrength = percentToTrigger48(s.adaptiveTriggers.r2ThrottleEndPercent),
    r2ThrottleStrength = percentToTrigger48(s.adaptiveTriggers.r2ThrottleStartPercent),
    r2EffectsEnabled = s.adaptiveTriggers.r2EffectsEnabled,
    r2EffectsStrength = percentToTrigger48(s.adaptiveTriggers.r2EffectsPercent),
    lightingEnabled = s.lighting.enabled,
    lightingBrightness = percentTo255(s.lighting.brightnessPercent),
    surfaceRollingStrengths = rollingStrengths,
    surfaceSlipStrengths = slipStrengths,
  }
end


function M.read()
  return readUserSettings()
end

function M.packetSnapshot(s)
  s = s or readUserSettings()
  if userSettingsCache.packetGeneration ~= userSettingsCache.generation or not userSettingsCache.packet then
    userSettingsCache.packet = buildUserSettingsPacket(s)
    userSettingsCache.packetGeneration = userSettingsCache.generation
    userSettingsCache.packetRevision = userSettingsCache.packetRevision + 1
  end
  return userSettingsCache.packet, userSettingsCache.packetRevision
end

function M.packet()
  local packet = M.packetSnapshot()
  return packet
end

function M.rollingReference(name)
  return SURFACE_ROLLING_REFERENCES[name] or 100
end

function M.slipReference(name)
  return SURFACE_SLIP_REFERENCES[name] or 1
end

-- UI percentages are converted to normalized gameplay values here so vehicle
-- telemetry never needs to know the persistence/validation rules.
function M.percentUnit(value, fallback)
  return clampPercent(value, fallback) / 100
end

function M.referenceGain(value, reference)
  reference = max(num(reference, 100), 0.1)
  return clampPercent(value, reference) / reference
end

return M
