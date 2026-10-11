local M = {}

local SETTINGS_STORE = require("dualsenseSuite/settingsStore")
local CONFIG = require("dualsensePhysicsHaptics/config")
local cfg = CONFIG.directRPMLights or {}

local max, min, floor = math.max, math.min, math.floor
local elapsed = 0
local lastTelemetryAt = -999
local rpm = 0
local maxRPM = 0
local devices = {}
local deviceSignature = ""
local deviceRefreshTimer = 999
local reassertTimer = 999
local lastRGB = nil

local function clamp(v, lo, hi)
  v = tonumber(v) or lo
  return min(max(v, lo), hi)
end

local function clamp01(v) return clamp(v, 0, 1) end
local function round(v) return floor(clamp(v, 0, 255) + 0.5) end
local function lower(v) return string.lower(tostring(v or "")) end

local function lightingSettings()
  local enabled, brightness = true, 86
  local raw = SETTINGS_STORE.get("dphSettings", nil)
  local decoded = nil
  if type(raw) == "table" then
    decoded = raw
  elseif type(raw) == "string" and raw ~= "" then
    local ok, value = pcall(jsonDecode, raw)
    if ok and type(value) == "table" then decoded = value end
  end
  if decoded and type(decoded.lighting) == "table" then
    enabled = decoded.lighting.enabled ~= false
    brightness = tonumber(decoded.lighting.brightnessPercent) or 86
  end
  return enabled, clamp(brightness, 1, 100) / 86
end

local function isDualSense(deviceName, productName)
  local label = lower(deviceName) .. " " .. lower(productName)
  return label:find("dualsense", 1, true) ~= nil or
         label:find("wireless controller", 1, true) ~= nil or
         label:find("sony", 1, true) ~= nil
end

local function productName(deviceName)
  if not Input or not Input.getProductName then return "" end
  local ok, value = pcall(Input.getProductName, deviceName)
  return ok and tostring(value or "") or ""
end

local function refreshDevices()
  if not core_input_bindings then return end
  local assigned = core_input_bindings.assignedPlayers or {}
  local found, seen = {}, {}

  local function consider(deviceName)
    deviceName = tostring(deviceName or "")
    if deviceName == "" or seen[deviceName] or assigned[deviceName] ~= 0 then return end
    seen[deviceName] = true
    local product = productName(deviceName)
    if isDualSense(deviceName, product) then
      found[#found + 1] = {name = deviceName, product = product}
    end
  end

  for _, entry in ipairs(core_input_bindings.bindings or {}) do consider(entry.devname) end
  for deviceName, player in pairs(assigned) do
    if player == 0 then consider(deviceName) end
  end
  table.sort(found, function(a, b) return a.name < b.name end)

  local parts = {}
  for _, d in ipairs(found) do parts[#parts + 1] = d.name .. ":" .. d.product end
  local signature = table.concat(parts, "|")
  if signature ~= deviceSignature then
    devices = found
    deviceSignature = signature
    lastRGB = nil
  end
end

local function hsvToRGB(h, s, v)
  h, s, v = (tonumber(h) or 0) % 360, clamp01(s), clamp01(v)
  local c = v * s
  local x = c * (1 - math.abs(((h / 60) % 2) - 1))
  local m = v - c
  local r, g, b
  if h < 60 then r, g, b = c, x, 0
  elseif h < 120 then r, g, b = x, c, 0
  elseif h < 180 then r, g, b = 0, c, x
  elseif h < 240 then r, g, b = 0, x, c
  elseif h < 300 then r, g, b = x, 0, c
  else r, g, b = c, 0, x end
  return round((r + m) * 255), round((g + m) * 255), round((b + m) * 255)
end

local function targetRGB()
  local enabled, brightnessScale = lightingSettings()
  local stale = elapsed - lastTelemetryAt > max(tonumber(cfg.staleTelemetrySeconds) or 0.75, 0.20)
  if not enabled or stale or maxRPM < 500 then return 0, 0, 0 end

  local ratio = clamp01(rpm / maxRPM)
  local firstRatio = clamp01(cfg.firstRatio or 0.50)
  local redRatio = max(clamp01(cfg.redRatio or 0.95), firstRatio + 0.01)
  local blinkRatio = max(clamp01(cfg.blinkRatio or 0.985), redRatio)
  if ratio < firstRatio then return 0, 0, 0 end

  local maxBrightness = clamp(cfg.maxBrightness or 220, 1, 255)
  if ratio >= blinkRatio then
    local hz = max(tonumber(cfg.blinkHz) or 8, 1)
    if floor(elapsed * hz * 2) % 2 == 1 then return 0, 0, 0 end
    return round(maxBrightness * brightnessScale), 0, 0
  end
  if ratio >= redRatio then return round(maxBrightness * brightnessScale), 0, 0 end

  local t = clamp01((ratio - firstRatio) / (redRatio - firstRatio))
  local minBrightness = clamp(cfg.minBrightness or 48, 0, maxBrightness)
  local value = (minBrightness + (maxBrightness - minBrightness) * clamp01(t * 1.8)) / 255
  local r, g, b = hsvToRGB(120 * (1 - t), cfg.saturation or 1, value)
  return round(r * brightnessScale), round(g * brightnessScale), round(b * brightnessScale)
end

local function writeRGB(r, g, b, force)
  if not Device or not Device.setRGB then return end
  if not force and lastRGB and lastRGB[1] == r and lastRGB[2] == g and lastRGB[3] == b then return end
  local wrote = false
  for _, d in ipairs(devices) do
    wrote = pcall(Device.setRGB, d.name, r, g, b) or wrote
  end
  if wrote then lastRGB = {r, g, b} end
end

local function apply(force)
  local r, g, b = targetRGB()
  writeRGB(r, g, b, force)
end

function M.onDualSensePhysicsRPMData(newRPM, newMaxRPM)
  rpm = max(tonumber(newRPM) or 0, 0)
  maxRPM = max(tonumber(newMaxRPM) or 0, 0)
  lastTelemetryAt = elapsed
  apply(false)
end

local function refreshNow()
  deviceRefreshTimer = 999
  reassertTimer = 999
end

function M.onUpdate(dtReal, dtSim, dtRaw)
  local dt = max(tonumber(dtReal) or tonumber(dtRaw) or tonumber(dtSim) or 0, 0)
  elapsed = elapsed + dt
  deviceRefreshTimer = deviceRefreshTimer + dt
  reassertTimer = reassertTimer + dt

  if deviceRefreshTimer >= max(tonumber(cfg.deviceRefreshSeconds) or 0.50, 0.10) then
    deviceRefreshTimer = 0
    refreshDevices()
  end
  if reassertTimer >= max(tonumber(cfg.reassertSeconds) or 0.25, 0.10) then
    reassertTimer = 0
    apply(true)
  end
end

function M.onExtensionLoaded()
  refreshDevices()
  apply(true)
end

function M.onExtensionUnloaded()
  writeRGB(0, 0, 0, true)
end

M.onVehicleSwitched = refreshNow
M.onVehicleSpawned = refreshNow
M.onFFBConfigChanged = refreshNow
M.onDeviceChanged = refreshNow
M.onSettingsChanged = refreshNow

return M
