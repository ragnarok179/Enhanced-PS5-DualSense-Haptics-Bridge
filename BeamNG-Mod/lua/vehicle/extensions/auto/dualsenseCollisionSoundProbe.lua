local M = {}

local vehicleId = -1
local collisionEvents = {}
local originalPlay = nil
local collisionWrapper = nil
local originalAtNode = nil
local atNodeWrapper = nil
local systemCollisionSuppressed = false

local function finite(v, fallback)
  v = tonumber(v)
  if not v or v ~= v or v == math.huge or v == -math.huge then return fallback or 0 end
  return v
end

local function clean(v)
  local s = tostring(v or ''):gsub('[\r\n\t]', ' ')
  if #s > 900 then s = s:sub(1, 900) end
  return s
end

local function emit(eventPath, node, volume, pitch, color, texture, extra)
  if not obj or not obj.queueGameEngineLua then return end
  local cmd = string.format(
    'if extensions.dualsenseCollisionSoundProbeGE then extensions.dualsenseCollisionSoundProbeGE.onVehicleCollisionEvent(%d,%q,%d,%.9g,%.9g,%.9g,%.9g,%q) end',
    vehicleId, clean(eventPath), math.floor(finite(node, -1)), finite(volume, 0), finite(pitch, 1),
    finite(color, 0), finite(texture, 0), clean(extra))
  obj:queueGameEngineLua(cmd)
end

local function emitReset(reason)
  if not obj or not obj.queueGameEngineLua then return end
  local cmd = string.format(
    'if extensions.dualsenseCollisionSoundProbeGE then extensions.dualsenseCollisionSoundProbeGE.onVehicleCollisionAudioReset(%d,%q) end',
    vehicleId, clean(reason or 'reset'))
  obj:queueGameEngineLua(cmd)
end

local function requestSpeakerRouting()
  if not obj or not obj.queueGameEngineLua then return end
  local cmd = string.format(
    'if extensions.dualsenseControllerSpeaker and extensions.dualsenseControllerSpeaker.syncVehicleSpeakerRouting then extensions.dualsenseControllerSpeaker.syncVehicleSpeakerRouting(%d) end',
    vehicleId
  )
  obj:queueGameEngineLua(cmd)
end

local function addCollisionEvent(path)
  if path == nil or path == false then return end
  local p = tostring(path)
  if p ~= '' then collisionEvents[p] = true end
end

local function rebuildCollisionInventory()
  collisionEvents = {}
  local vs = (v and v.data and v.data.sounds) or {}
  addCollisionEvent(vs.impactGeneric or 'event:>Destruction>Vehicle>vehicle_part_impact')
  addCollisionEvent(vs.impactMetal or 'event:>Destruction>Vehicle>vehicle_part_impact')
  addCollisionEvent(vs.impactPlastic or 'event:>Destruction>Vehicle>vehicle_part_impact_plastic')
  addCollisionEvent(vs.breakGeneric or 'event:>Destruction>Vehicle>vehicle_part_break')
  addCollisionEvent(vs.breakPlastic or 'event:>Destruction>Vehicle>vehicle_part_break_plastic')

  for _, nodeData in pairs((v and v.data and v.data.nodes) or {}) do
    if type(nodeData) == 'table' and nodeData.cid ~= nil then
      for _, key in ipairs({'impactGenericEvent','impactMetalEvent','impactPlasticEvent','breakGenericEvent','breakPlasticEvent'}) do
        if nodeData[key] ~= nil and nodeData[key] ~= false then addCollisionEvent(nodeData[key]) end
      end
    end
  end
end

-- All recognized collision one-shots must obey the same routing. JBeam
-- overrides can use paths not included in our initial vehicle inventory.
local function isCollisionEvent(path)
  if collisionEvents[path] then return true end
  local name = tostring(path or ''):lower()
  local destruction = name:find('>destruction>', 1, true)
  local legacy = name:find('crash_generic', 1, true) or
    name:find('impact_vehicle_generic', 1, true)
  return (destruction and (name:find('impact', 1, true) or
    name:find('break', 1, true) or name:find('crash', 1, true) or
    name:find('shrapnel', 1, true))) or legacy or false
end

local function routeOneShot(original, source, soundName, nodeID, volume, pitch, color, texture, ...)
  local path = tostring(soundName or '')
  local vol = finite(volume, 0)
  local pit = finite(pitch, 1)
  local collision = isCollisionEvent(path) and vol >= 0.01 and pit >= 0.0039
  if collision and systemCollisionSuppressed then
    emit(path, nodeID, vol, pit, color or 0, texture or 0,
      'source=' .. source .. ';route=controller')
    return nil
  end
  local result = original(soundName, nodeID, volume, pitch, color, texture, ...)
  if collision then
    emit(path, nodeID, vol, pit, color or 0, texture or 0,
      'source=' .. source .. ';route=system+controller')
  end
  return result
end

local function installCollisionHook()
  if not sounds then return false end
  local installed = false
  if type(sounds.playSoundOnceFollowNode) == 'function' and
     sounds.playSoundOnceFollowNode ~= collisionWrapper then
    originalPlay = sounds.playSoundOnceFollowNode
    collisionWrapper = function(soundName, nodeID, volume, pitch, color, texture, ...)
      return routeOneShot(originalPlay, 'sounds.playSoundOnceFollowNode',
        soundName, nodeID, volume, pitch, color, texture, ...)
    end
    sounds.playSoundOnceFollowNode = collisionWrapper
  end
  installed = collisionWrapper ~= nil

  -- Older vehicle implementations use the sibling one-shot API. Its shorter
  -- argument list is kept intact by forwarding variadic arguments unchanged.
  if type(sounds.playSoundOnceAtNode) == 'function' and
     sounds.playSoundOnceAtNode ~= atNodeWrapper then
    originalAtNode = sounds.playSoundOnceAtNode
    atNodeWrapper = function(soundName, nodeID, volume, pitch, color, texture, ...)
      return routeOneShot(originalAtNode, 'sounds.playSoundOnceAtNode',
        soundName, nodeID, volume, pitch, color, texture, ...)
    end
    sounds.playSoundOnceAtNode = atNodeWrapper
  end
  return installed or atNodeWrapper ~= nil
end

local function uninstallCollisionHook()
  if sounds then
    if collisionWrapper and sounds.playSoundOnceFollowNode == collisionWrapper and originalPlay then
      sounds.playSoundOnceFollowNode = originalPlay
    end
    if atNodeWrapper and sounds.playSoundOnceAtNode == atNodeWrapper and originalAtNode then
      sounds.playSoundOnceAtNode = originalAtNode
    end
  end
  originalPlay, collisionWrapper, originalAtNode, atNodeWrapper = nil, nil, nil, nil
end

local function rebuild()
  rebuildCollisionInventory()
  installCollisionHook()
  requestSpeakerRouting()
end

function M.setSystemCollisionSuppressed(value)
  systemCollisionSuppressed = value == true or value == 1 or tostring(value):lower() == 'true'
end

function M.onExtensionLoaded()
  vehicleId = obj and obj.getId and obj:getId() or -1
  rebuild()
end

function M.onVehicleLoaded() emitReset('vehicleLoaded'); rebuild() end
function M.onReset() emitReset('reset'); rebuild() end

function M.updateGFX()
  if not sounds or
    (type(sounds.playSoundOnceFollowNode) == 'function' and sounds.playSoundOnceFollowNode ~= collisionWrapper) or
    (type(sounds.playSoundOnceAtNode) == 'function' and sounds.playSoundOnceAtNode ~= atNodeWrapper) then
    installCollisionHook()
  end
end

function M.onExtensionUnloaded() emitReset('extensionUnloaded'); uninstallCollisionHook() end

return M
