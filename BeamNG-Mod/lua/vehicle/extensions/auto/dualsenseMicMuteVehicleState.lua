-- Read-only, conservative state probe for the Mic Mute virtual toggle.
-- Only the active player vehicle is queried. Unknown states return nil.
local M = {}

local function booleanState(value)
  if type(value) == "boolean" then return value end
  if type(value) == "number" then return value > 0.5 end
  return nil
end

local function electricState(key)
  local v = electrics and electrics.values
  if type(v) ~= "table" then return nil end
  return booleanState(v[key])
end

local function diffState()
  if not powertrain or type(powertrain.getDevicesByType) ~= "function" then return nil end
  -- 4wd controller may delegate to multi-state driveModes. We cannot infer a
  -- binary diff lock from those drive modes, so leave it manual in that case.
  if controller and controller.getController and controller.getController("driveModes") then return nil end
  local diffs = powertrain.getDevicesByType("differential")
  if type(diffs) ~= "table" then return nil end
  local result = nil
  for _, diff in pairs(diffs) do
    if diff.defaultToggle then
      if diff.mode ~= "locked" and diff.mode ~= "open" then return nil end
      local locked = diff.mode == "locked"
      if result ~= nil and result ~= locked then return nil end
      result = locked
    end
  end
  return result
end

local readers = {
  diff = diffState,
  ["4wd"] = function()
    if not controller or not controller.getControllersByType or
       #(controller.getControllersByType("4wd") or {}) == 0 then return nil end
    return electricState("mode4WD")
  end,
  range = function()
    if not controller or not controller.getControllersByType or
       #(controller.getControllersByType("4wd") or {}) == 0 then return nil end
    return electricState("modeRangeBox")
  end,
  fog = function() return electricState("fog") end,
  left = function() return electricState("signal_left_input") end,
  right = function() return electricState("signal_right_input") end,
  hazard = function() return electricState("hazard_enabled") end,
  underglow = function() return electricState("underglow") end,
  parkingbrake = function() return electricState("parkingbrake") end,
}

function M.query(target, sequence)
  local reader = readers[target]
  if not reader or type(sequence) ~= "number" or sequence < 0 then return end
  local ok, value = pcall(reader)
  if not ok or type(value) ~= "boolean" then value = nil end
  local encoded = value == nil and "nil" or tostring(value)
  obj:queueGameEngineLua(string.format(
    "if extensions.dualsenseExtendedInputs then extensions.dualsenseExtendedInputs.onMicMuteVehicleState(%d,%d,%q,%s) end",
    obj:getID(), sequence, target, encoded))
end

return M
