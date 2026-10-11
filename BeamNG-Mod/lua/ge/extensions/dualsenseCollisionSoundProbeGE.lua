local M = {}
local socket = require('socket')

local HOST, PORT = '127.0.0.1', 6978
local udp = nil
local sequence = 0

local function clean(v)
  local s = tostring(v or ''):gsub('[\r\n\t]', ' ')
  if #s > 1200 then s = s:sub(1, 1200) end
  return s
end

local function num(v, fallback)
  v = tonumber(v)
  if not v or v ~= v or v == math.huge or v == -math.huge then return fallback or 0 end
  return v
end

local function ensureUDP()
  if udp then return true end
  udp = socket.udp()
  if not udp then return false end
  udp:settimeout(0)
  return true
end

function M.onVehicleCollisionEvent(vehicleId, eventPath, node, volume, pitch, color, texture, extra)
  local speaker = extensions and extensions.dualsenseControllerSpeaker
  if speaker and speaker.shouldSendCollision then
    local ok, enabled = pcall(speaker.shouldSendCollision)
    if ok and not enabled then return false end
  end
  sequence = sequence + 1

  local source = string.format(
    'veh=%d;node=%d;%s',
    math.floor(num(vehicleId, -1)),
    math.floor(num(node, -1)),
    clean(extra)
  )
  local packet = string.format(
    'DSE1\t1\toneshot\tcollision\t%d\t%s\t%.9g\t%.9g\t%.9g\t%.9g\t%s',
    sequence,
    clean(source),
    num(volume, 0),
    num(pitch, 1),
    num(color, 0),
    num(texture, 0),
    clean(eventPath)
  )

  return ensureUDP() and pcall(function()
    udp:sendto(packet, HOST, PORT)
  end)
end


function M.onVehicleCollisionAudioReset(vehicleId, reason)
  sequence = sequence + 1
  local source = string.format(
    'veh=%d;reason=%s',
    math.floor(num(vehicleId, -1)),
    clean(reason or 'reset')
  )
  local packet = string.format(
    'DSE1\t1\treset\tcollision\t%d\t%s\t0\t1\t0\t0\t-',
    sequence,
    clean(source)
  )
  return ensureUDP() and pcall(function()
    udp:sendto(packet, HOST, PORT)
  end)
end


function M.onExtensionLoaded()
  ensureUDP()
end

function M.onExtensionUnloaded()
  if udp then
    pcall(function() udp:close() end)
    udp = nil
  end
end

return M
