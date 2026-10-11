-- Shared virtual-input ownership for Enhanced PS5 DualSense Haptics.
-- One BeamNG vinput device exposes both touchpad and motion/sensor controls.
local M = {}

M.name = "DualSense Extended Inputs"
M.vidpid = "dualsenseextendedv2"
M.axisCount = 10
M.buttonCount = 23
M.povCount = 0

M.touchAxisBase = 0
M.touchButtonBase = 0
M.sensorAxisBase = 4
M.sensorButtonBase = 17

-- BeamNG/Torque names the first joystick axes conventionally; higher virtual
-- axes continue with slider/slider2 and generic axisN names.
M.axisControlNames = {
  [0] = "xaxis",
  [1] = "yaxis",
  [2] = "zaxis",
  [3] = "rxaxis",
  [4] = "ryaxis",
  [5] = "rzaxis",
  [6] = "slider",
  [7] = "slider2",
  [8] = "axis8",
  [9] = "axis9",
}

M.legacyVidPids = {
  "dualsenseextunifiedv1",
  "dualsensesensorsv1",
  "dualsenseextswipesv1",
  "dualsenseextrelativev1",
  "dualsenseextabsolutev1",
  "dualsenseextv7",
  "dualsenseextmousev1",
  "dualsensemappedmotionv1",
  "dualsensemotionv2",
  "dualsenseorientationv2",
}

local deviceInstance = nil
local activeProducers = {}

local function vi()
  return core_input_virtualInput or (extensions and extensions.core_input_virtualInput)
end

local function validInfo(info)
  return type(info) == "table" and info[1] == M.name and
    tostring(info[2] or "") == M.vidpid and tonumber(info[3]) == M.axisCount and
    tonumber(info[4]) == M.buttonCount and tonumber(info[5]) == M.povCount
end

local function producerKey(producer, logTag)
  local key = tostring(producer or logTag or "anonymous")
  if key == "" then key = "anonymous" end
  return key
end

local function deleteInstance(v, id)
  if not v or id == nil then return end
  pcall(v.deleteDevice, id)
  if deviceInstance == id then deviceInstance = nil end
end

local function findCurrentDevice(v)
  -- core_input_virtualInput exposes its registered devices table. Older merged
  -- builds could leave several instances with the same vidpid after extension
  -- reloads. Keep one valid instance and unregister every duplicate.
  local candidates = {}
  if type(v.devices) == "table" then
    for id, info in pairs(v.devices) do
      if type(info) == "table" and tostring(info[2] or "") == M.vidpid then
        candidates[#candidates + 1] = {id = id, info = info}
      end
    end
  end
  table.sort(candidates, function(a, b) return (tonumber(a.id) or 999999) < (tonumber(b.id) or 999999) end)

  local keep = nil
  for _, entry in ipairs(candidates) do
    if keep == nil and validInfo(entry.info) then
      keep = entry.id
    else
      deleteInstance(v, entry.id)
    end
  end

  if keep ~= nil then return keep end

  -- Fallback for BeamNG versions where devices is not exposed.
  local existing, info = v.getDeviceInfo(M.vidpid)
  if existing ~= nil then
    if validInfo(info) then return existing end
    deleteInstance(v, existing)
  end
  return nil
end

function M.ensure(logTag, producer)
  local v = vi()
  if not v then
    log("E", logTag or "DualSenseVirtualInput", "core_input_virtualInput is unavailable")
    return nil
  end

  local producerId = producerKey(producer, logTag)

  -- Always rescan the BeamNG vinput registry. V1.2 could leave several valid
  -- instances behind, so returning a cached instance before this scan would
  -- preserve the inert duplicates in Available Devices.
  if deviceInstance ~= nil and type(v.devices) == "table" then
    local info = v.devices[deviceInstance]
    if info ~= nil and not validInfo(info) then deleteInstance(v, deviceInstance) end
  end

  local existing = findCurrentDevice(v)
  if existing ~= nil then
    deviceInstance = existing
    activeProducers[producerId] = true
    return existing
  end

  local id = v.createDevice(M.name, M.vidpid, M.axisCount, M.buttonCount, M.povCount)
  if id == nil or tonumber(id) == nil or tonumber(id) < 0 then
    log("E", logTag or "DualSenseVirtualInput", "Unable to create shared DualSense Extended Inputs virtual device")
    return nil
  end
  deviceInstance = id
  activeProducers[producerId] = true
  if core_input_bindings and type(core_input_bindings.onDeviceChanged) == "function" then
    pcall(core_input_bindings.onDeviceChanged)
  end
  return id
end

function M.retireLegacy()
  local v = vi()
  if not v then return end
  for _, vidpid in ipairs(M.legacyVidPids) do
    if vidpid ~= M.vidpid then
      local id = v.getDeviceInfo(vidpid)
      while id ~= nil do
        pcall(v.deleteDevice, id)
        id = v.getDeviceInfo(vidpid)
      end
    end
  end
end

function M.releaseProducer(producer)
  activeProducers[producerKey(producer)] = nil
  for _ in pairs(activeProducers) do return end

  local v = vi()
  if not v then
    deviceInstance = nil
    return
  end

  -- Delete every current mod instance when the final producer unloads. This
  -- makes full extension/mod reloads idempotent while preserving the device if
  -- only Touchpad or Motion is reloaded individually.
  if type(v.devices) == "table" then
    local ids = {}
    for id, info in pairs(v.devices) do
      if type(info) == "table" and tostring(info[2] or "") == M.vidpid then ids[#ids + 1] = id end
    end
    for _, id in ipairs(ids) do deleteInstance(v, id) end
  else
    local id = v.getDeviceInfo(M.vidpid)
    while id ~= nil do
      deleteInstance(v, id)
      id = v.getDeviceInfo(M.vidpid)
    end
  end
  deviceInstance = nil
end

function M.emitAxis(deviceInstance, index, raw)
  local v = vi()
  if not v or deviceInstance == nil then return end
  v.emit(deviceInstance, "axis", index, "change", raw)
end

function M.emitButton(deviceInstance, index, value)
  local v = vi()
  if not v or deviceInstance == nil then return end
  v.emit(deviceInstance, "button", index, value > 0 and "down" or "up", value)
end


return M
