-- Enhanced PS5 DualSense Haptics
--
-- BeamNG 0.39 exposes native force-feedback routes for the DualSense brake and
-- accelerator axes. The external Bridge already owns the adaptive triggers for
-- this mod. If another controller is connected, BeamNG can rebuild those native
-- trigger FFB routes and both systems may write to L2/R2 at the same time.
--
-- This extension does not rewrite inputmaps or rebind controls. Instead, after
-- BeamNG has configured FFB for a vehicle driven by a DualSense, it clears only
-- the vehicle-side accelerator/brake FFB IDs used by input_haptics. Steering FFB
-- and the normal accelerator/brake input bindings remain untouched.

local M = {}

local INITIAL_DELAY = 0.25
local RETRY_DELAY = 0.45
local DEFAULT_ATTEMPTS = 2

local elapsed = 0
local nextApplyAt = nil
local attemptsRemaining = 0
local unloading = false

local CLEAR_TRIGGER_FFB_COMMAND = [[
if input_haptics and hydros and input_haptics.setFFBIDs and hydros.getFFBID then
  input_haptics.setFFBIDs(hydros.getFFBID(), -1, -1)
end
]]

local function lower(value)
  return string.lower(tostring(value or ""))
end

local function contains(text, needle)
  return lower(text):find(needle, 1, true) ~= nil
end

local function isDualSenseEntry(entry)
  if type(entry) ~= "table" then return false end

  local deviceName = lower(entry.devname)
  if not deviceName:match("^gamepad%d+$") then return false end

  local contents = type(entry.contents) == "table" and entry.contents or {}
  local name = lower(contents.name)
  local vendor = lower(contents.vendorName)
  local guid = lower(contents.guid)
  local vidpid = lower(contents.vidpid)
  local pidvid = lower(contents.pidvid)
  local imagePack = lower(contents.imagePack)

  if contains(name, "dualsense") then return true end
  if vidpid == "sce_pad_standard" or pidvid == "sce_pad_standard" then return true end
  if imagePack == "ps5" and vendor == "sony" then return true end
  if contains(guid, "0ce6054c") or contains(guid, "0df2054c") then return true end
  if contains(vidpid, "0ce6054c") or contains(vidpid, "0df2054c") then return true end
  if contains(pidvid, "0ce6054c") or contains(pidvid, "0df2054c") then return true end

  return false
end

local function playerForDevice(deviceName)
  if core_input_bindings and type(core_input_bindings.assignedPlayers) == "table" then
    local player = tonumber(core_input_bindings.assignedPlayers[deviceName])
    if player ~= nil then return player end
  end
  return 0
end

local function collectDualSensePlayers()
  local result = {}
  if not core_input_bindings or type(core_input_bindings.bindings) ~= "table" then
    return result
  end

  for _, entry in ipairs(core_input_bindings.bindings) do
    if isDualSenseEntry(entry) then
      local deviceName = tostring(entry.devname or "")
      result[playerForDevice(deviceName)] = true
    end
  end
  return result
end

local function applyGuard()
  if unloading then return end

  for player, _ in pairs(collectDualSensePlayers()) do
    local vehicle = getPlayerVehicle and getPlayerVehicle(player) or nil
    if vehicle and type(vehicle.queueLuaCommand) == "function" then
      vehicle:queueLuaCommand(CLEAR_TRIGGER_FFB_COMMAND)
    end
  end
end

local function schedule(delay, attempts)
  if unloading then return end
  attemptsRemaining = math.max(attemptsRemaining, tonumber(attempts) or DEFAULT_ATTEMPTS)
  local target = elapsed + math.max(tonumber(delay) or 0, 0)
  if nextApplyAt == nil or target < nextApplyAt then nextApplyAt = target end
end

local function onUpdate(dtReal)
  elapsed = elapsed + math.max(tonumber(dtReal) or 0, 0)
  if nextApplyAt == nil or elapsed < nextApplyAt then return end

  nextApplyAt = nil
  applyGuard()
  attemptsRemaining = math.max(attemptsRemaining - 1, 0)
  if attemptsRemaining > 0 then nextApplyAt = elapsed + RETRY_DELAY end
end

local function onInit()
  if setExtensionUnloadMode then setExtensionUnloadMode(M, "manual") end
end

local function onExtensionLoaded()
  schedule(INITIAL_DELAY, 3)
end

-- core_input_bindings queues hydros.onFFBConfigChanged() before firing this hook,
-- so our vehicle command is queued afterwards and wins without touching bindings.
local function onFFBConfigChanged()
  schedule(0, DEFAULT_ATTEMPTS)
end

-- Device and binding changes already flow through BeamNG's FFB rebuild and
-- then emit onFFBConfigChanged. Handling those hooks a second time can queue
-- trigger-guard writes while the input registry is still being rebuilt.
local function onVehicleSwitched()
  schedule(0.08, 3)
end

local function onVehicleSpawned()
  schedule(0.12, 3)
end

local function onExtensionUnloaded()
  unloading = true
  nextApplyAt = nil
  attemptsRemaining = 0

  -- Do not force a device/input rebuild here. BeamNG owns controller discovery
  -- and will restore its native trigger route on the next normal FFB rebuild.
end

M.onInit = onInit
M.onExtensionLoaded = onExtensionLoaded
M.onFFBConfigChanged = onFFBConfigChanged
M.onVehicleSwitched = onVehicleSwitched
M.onVehicleSpawned = onVehicleSpawned
M.onUpdate = onUpdate
M.onExtensionUnloaded = onExtensionUnloaded

return M
