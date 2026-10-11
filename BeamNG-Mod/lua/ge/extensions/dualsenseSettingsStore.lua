-- Enhanced PS5 DualSense Haptics
-- GE-facing access to the mod-owned settings store and one-time migration away
-- from BeamNG's global settings registry.

local M = {}
local STORE = require("dualsenseSuite/settingsStore")

local legacyKeys = {
  "dphSettings",
  "dphBridgeUpdateNoticeSeen",
  "dualsenseExtendedInputsSettings",
  "dseUnifiedTouchpadBindingsMigratedV1",
  "dseTouchpadInputMode",
  "dseTouchpadInputModeV2",
  "dseTouchpadMouseModeV1",
  "dseTouchpadTwoFingerMouseToggleV1",
  "dseTouchpadCustom1FXV1",
  "dseTouchpadCustom1FYV1",
  "dseTouchpadCustom2FXV1",
  "dseTouchpadCustom2FYV1",
  "dseBluetoothIdleTimeoutMinutesV1",
  "dseBluetoothIdleTimeoutMinutesV2",
  "dseBluetoothIdleExplicitOffV1",
  "dseBluetoothIdleMotionActivityV1",
  "dseSpeakerEnabledV1",
  "dseSpeakerVolumeV1",
  "dseSpeakerCollisionV1",
  "dseSpeakerOutputV1",
  "dseSpeakerSuspensionV1",
  "dseSpeakerTireBurstV1",
  "dseSpeakerLandingV1",
  "dseSpeakerGearGrindV1",
  "dseSpeakerFireIgnitionV1",
  "dseSpeakerEngineFailureV1",
  "dseSpeakerShiftV1",
  "dseSpeakerCouplingV1",
  "dseSpeakerCompressorPurgeV1",
}

for i = 1, 6 do
  legacyKeys[#legacyKeys + 1] = "dualsenseMappedMotionSlotV1_" .. tostring(i)
  legacyKeys[#legacyKeys + 1] = "dseMappedMotionSlotV1_" .. tostring(i)
end
for i = 1, 3 do
  legacyKeys[#legacyKeys + 1] = "dseSensorMotionV3_" .. tostring(i)
  legacyKeys[#legacyKeys + 1] = "dseSensorMotionV4_" .. tostring(i)
  legacyKeys[#legacyKeys + 1] = "dseSensorAccelerationV1_" .. tostring(i)
  legacyKeys[#legacyKeys + 1] = "dseSensorAccelerationV2_" .. tostring(i)
  legacyKeys[#legacyKeys + 1] = "dseSensorAccelerationV3_" .. tostring(i)
end

local function legacyValue(key)
  if settings and type(settings.getValue) == "function" then
    local ok, value = pcall(settings.getValue, key)
    if ok then return value end
  end
  return nil
end

local function cleanPersistedFile(path)
  if type(jsonReadFile) ~= "function" or type(jsonWriteFile) ~= "function" then return end
  local ok, data = pcall(jsonReadFile, path)
  if not ok or type(data) ~= "table" then return end
  local changed = false
  for _, key in ipairs(legacyKeys) do
    if data[key] ~= nil then data[key] = nil; changed = true end
  end
  if changed then pcall(jsonWriteFile, path, data, true) end
end

local function migrateLegacySettings()
  local patch = {}
  for _, key in ipairs(legacyKeys) do
    local value = legacyValue(key)
    if value ~= nil and not STORE.has(key) then patch[key] = value end
  end
  if next(patch) ~= nil then STORE.patch(patch) end

  -- Remove only this mod's historical keys from BeamNG's global settings state.
  if settings and type(settings.setValue) == "function" then
    for _, key in ipairs(legacyKeys) do pcall(settings.setValue, key, nil) end
  end
  cleanPersistedFile("/settings/settings.json")
  cleanPersistedFile("/settings/cloud/settings.json")
end

function M.onInit()
  if setExtensionUnloadMode then setExtensionUnloadMode(M, "manual") end
end

function M.onExtensionLoaded()
  migrateLegacySettings()
end

function M.getJson(key) return STORE.getJson(key) end
function M.setJson(key, encoded)
  local saved = STORE.setJson(key, encoded)
  if saved and type(key) == "string" and key:sub(1, 10) == "dseSpeaker" then
    local speaker = extensions and extensions.dualsenseControllerSpeaker
    if speaker and speaker.applyFromSettings then pcall(speaker.applyFromSettings) end
  end
  return saved
end
function M.get(key, fallback) return STORE.get(key, fallback) end
function M.set(key, value) return STORE.set(key, value) end
function M.reload() return STORE.reload() end

return M
