local M = {}
local socket = require('socket')
local SETTINGS_STORE = require('dualsenseSuite/settingsStore')

local HOST, PORT = '127.0.0.1', 6978
local udp = nil
local elapsed = 0

-- Public controller-speaker settings for the collision-only release profile.
local keys = {
  enabled = 'dseSpeakerEnabledV1',
  volume = 'dseSpeakerVolumeV1',
  collision = 'dseSpeakerCollisionV1',
  output = 'dseSpeakerOutputV1'
}

local defaults = {enabled=true, volume=100, collision=true, output=0}
local speakerKey = 'dseSpeakerSettingsV2'
local state = {enabled=true, volume=100, collision=true, output=0}

local function boolValue(v, fallback)
  if v == nil or v == '' then return fallback end
  return v == true or v == 1 or v == '1' or tostring(v):lower() == 'true'
end

local function readSetting(key, fallback)
  local value = SETTINGS_STORE.get(key, nil)
  if value == nil or value == '' then return fallback end
  return value
end

local function refreshState()
  local oldEnabled, oldCollision, oldOutput, oldVolume = state.enabled, state.collision, state.output, state.volume
  -- One atomic persisted snapshot owns all Speaker values. Four V1 keys
  -- are read only when upgrading an existing V1.42 installation.
  local saved = SETTINGS_STORE.get(speakerKey, nil)
  local function field(name, fallback)
    if type(saved) == 'table' then
      local v = saved[name]
      if v ~= nil and v ~= '' then return v end
      return fallback
    end
    return readSetting(keys[name], fallback)
  end
  state.enabled = boolValue(field('enabled', defaults.enabled), defaults.enabled)
  state.volume = math.max(0, math.min(100, math.floor((tonumber(field('volume', defaults.volume)) or defaults.volume) + 0.5)))
  state.collision = boolValue(field('collision', defaults.collision), defaults.collision)
  state.output = math.max(0, math.min(1, math.floor(tonumber(field('output', defaults.output)) or defaults.output)))
  return oldEnabled ~= state.enabled or oldCollision ~= state.collision or oldOutput ~= state.output or oldVolume ~= state.volume
end

local function suppressSystemCollision()
  return state.enabled and state.collision and state.volume > 0 and state.output == 0
end

local function routingCommand()
  return string.format(
    'if extensions.dualsenseCollisionSoundProbe and extensions.dualsenseCollisionSoundProbe.setSystemCollisionSuppressed then extensions.dualsenseCollisionSoundProbe.setSystemCollisionSuppressed(%s) end',
    suppressSystemCollision() and 'true' or 'false'
  )
end

local function sendRoutingToVehicle(vehicleId)
  if not be or not be.getObjectByID then return false end
  local veh = be:getObjectByID(math.floor(tonumber(vehicleId) or -1))
  if not veh or not veh.queueLuaCommand then return false end
  return pcall(function() veh:queueLuaCommand(routingCommand()) end)
end

local function broadcastRouting()
  if not be or not be.queueAllObjectLua then return false end
  return pcall(function() be:queueAllObjectLua(routingCommand()) end)
end


local function ensureUDP()
  if udp then return true end
  udp = socket.udp()
  if not udp then return false end
  udp:settimeout(0)
  return true
end

local function sendCurrentState()
  if not ensureUDP() then return false end
  -- DSS1 v2 uses only bit 0 in the collision-only profile.
  local flags = state.collision and 1 or 0
  local packet = 'DSS1' .. string.char(2, state.enabled and 1 or 0, state.volume, flags, 0, state.output)
  local ok = pcall(function() udp:sendto(packet, HOST, PORT) end)
  return ok
end

local function refreshAndSend(forceRouting)
  local wasAudible = state.enabled and state.collision and state.volume > 0
  local routingChanged = refreshState()
  local nowAudible = state.enabled and state.collision and state.volume > 0
  if forceRouting or routingChanged then broadcastRouting() end
  local sent = sendCurrentState()
  if wasAudible and not nowAudible and ensureUDP() then
    pcall(function()
      udp:sendto('DSE1\t1\treset\tcollision\t0\tveh=-1;reason=speaker_off\t0\t1\t0\t0\t-', HOST, PORT)
    end)
  end
  return sent
end

function M.applyFromSettings()
  return refreshAndSend(true)
end

function M.shouldSendCollision()
  refreshState()
  return state.enabled and state.collision and state.volume > 0
end


function M.onExtensionLoaded()
  refreshAndSend(true)
end

function M.syncVehicleSpeakerRouting(vehicleId)
  refreshState()
  return sendRoutingToVehicle(vehicleId)
end

function M.onUpdate(dt)
  elapsed = elapsed + (tonumber(dt) or 0)
  if elapsed >= 0.20 then
    elapsed = elapsed % 0.20
    refreshAndSend()
  end
end

function M.onExtensionUnloaded()
  if udp then pcall(function() udp:close() end); udp = nil end
end

return M
