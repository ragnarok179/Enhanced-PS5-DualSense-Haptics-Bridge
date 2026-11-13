-- The mod manager unmounts a disabled ZIP before notifying extensions.
-- These GE extensions are intentionally manual-lifetime across level changes,
-- so the mod itself must explicitly release them on onModDeactivated.
local M = {}

local MOD_ID = "ENHANCEDPS5DUALSENSEHAPTICS"
local LOG_TAG = "DualSenseModLifecycle"

-- The corresponding modScript.lua registers these extensions, including this
-- coordinator. Teardown keeps the settings store alive until all consumers
-- have closed their resources, then releases the coordinator itself.
local EXTENSIONS = {
  "dualsenseSettingsStore",
  "dualsenseNativeTriggerGuard",
  "dualsensePhysicsDirectRPMLights",
  "dualsenseExtendedInputs",
  "dualsenseMotionInputs",
  "dualsenseControllerSpeaker",
  "dualsenseCollisionSoundProbeGE",
  "dualsenseModLifecycle",
}

local function isThisMod(mod)
  if type(mod) ~= "table" then return false end
  -- BeamNG passes a copy of its modmanager entry. Its canonical identifier
  -- lives in mod.modData.tagid, not necessarily in a top-level modID field.
  -- This also works for ZIPs renamed by Windows (e.g. " (7).zip").
  local metadata = mod.modData
  if type(metadata) == "table" and metadata.tagid ~= nil then
    return tostring(metadata.tagid):upper() == MOD_ID
  end
  if mod.modID ~= nil then
    return tostring(mod.modID):upper() == MOD_ID
  end

  -- Fallback for older/local installations lacking mod metadata.
  local name = tostring(mod.modname or ""):lower()
  local path = tostring(mod.fullpath or ""):gsub("\\", "/"):lower()
  local filename = path:match("([^/]+)/?$") or ""
  return name:match("^enhanced_ps5_dualsense_haptics_v%d+%.%d+$") ~= nil
    or filename:match("^enhanced_ps5_dualsense_haptics_v%d+%.%d+%.zip$") ~= nil
end

local function logFailure(operation, name, reason)
  if type(log) == "function" then
    log("E", LOG_TAG, operation .. " " .. name .. ": " .. tostring(reason))
  end
end

local function releaseExtension(name)
  if type(extensions) ~= "table" or type(extensions.unload) ~= "function" then
    logFailure("unload unavailable for", name, "extensions.unload missing")
    return
  end
  local ok, err = pcall(extensions.unload, name)
  if not ok then logFailure("unload failed for", name, err) end
end

function M.onModDeactivated(mod)
  if not isThisMod(mod) then return end

  -- A manual-lifetime registration survives extensions.unload(). If these
  -- names stay registered, BeamNG will load the disabled ZIP's extensions at
  -- the next level transition. Remove registrations *before* unloading them.
  for _, name in ipairs(EXTENSIONS) do
    if type(setExtensionUnloadMode) == "function" then
      local ok, err = pcall(setExtensionUnloadMode, name, "auto")
      if not ok then logFailure("unregister failed for", name, err) end
    end
  end
  for index = #EXTENSIONS - 1, 1, -1 do
    releaseExtension(EXTENSIONS[index])
  end
  releaseExtension(EXTENSIONS[#EXTENSIONS])
end

return M
