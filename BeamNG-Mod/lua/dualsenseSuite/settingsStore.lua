-- Enhanced PS5 DualSense Haptics
-- Mod-owned persistent settings store. Custom mod values deliberately stay out
-- of BeamNG's global settings registry so they do not generate unknown-setting warnings.

local M = {}

local STORE_PATH = "/settings/dualsense_haptics.json"
local LEGACY_PATH = "settings/dualsense_haptics.json"
-- os.clock() measures CPU time (not wall time). Cross-VM GE->vehicle updates
-- could remain invisible for minutes. Read the shared VFS once per wall second.
local CACHE_TTL = 1
local cache = nil
local lastReadAt = -999

local function now()
  if os and type(os.time) == "function" then return os.time() end
  if os and type(os.clock) == "function" then return os.clock() end
  return 0
end

local function readFile(force)
  local t = now()
  if not force and type(cache) == "table" and (t - lastReadAt) < CACHE_TTL then
    return cache
  end

  local data = {}
  if type(jsonReadFile) == "function" then
    local ok, value = pcall(jsonReadFile, STORE_PATH)
    if ok and type(value) == "table" then
      data = value
    else
      -- Migrate V1.42 relative-path data without overwriting valid canonical data.
      local oldOk, oldValue = pcall(jsonReadFile, LEGACY_PATH)
      if oldOk and type(oldValue) == "table" then data = oldValue end
    end
  end
  cache = data
  lastReadAt = t
  return cache
end

local function sameValue(a, b, depth)
  if type(a) ~= type(b) then return false end
  if type(a) ~= "table" then return a == b end
  if depth > 12 then return false end
  for k, v in pairs(a) do
    if not sameValue(v, b[k], depth + 1) then return false end
  end
  for k in pairs(b) do if a[k] == nil then return false end end
  return true
end

local function writeFile(changedKey)
  if type(jsonWriteFile) ~= "function" or type(jsonReadFile) ~= "function" then return false end
  local ok, result = pcall(jsonWriteFile, STORE_PATH, cache or {}, true)
  if not ok or result == false then return false end
  -- BeamNG's write routine may return nil. A real read-back is the only
  -- reliable confirmation that a UI change is persistent across menus/VMs.
  local readOk, stored = pcall(jsonReadFile, STORE_PATH)
  if not readOk or type(stored) ~= "table" then return false end
  if changedKey then return sameValue(stored[changedKey], cache[changedKey], 0) end
  return sameValue(stored, cache or {}, 0)
end

function M.reload()
  cache = nil
  lastReadAt = -999
  return readFile(true)
end

function M.has(key)
  return readFile(false)[tostring(key or "")] ~= nil
end

function M.get(key, fallback)
  local value = readFile(false)[tostring(key or "")]
  if value == nil then return fallback end
  return value
end

function M.set(key, value)
  key = tostring(key or "")
  if key == "" then return false end
  local data = readFile(false)
  data[key] = value
  cache = data
  lastReadAt = now()
  return writeFile(key)
end

function M.patch(values)
  if type(values) ~= "table" then return false end
  local data = readFile(false)
  for key, value in pairs(values) do data[tostring(key)] = value end
  cache = data
  lastReadAt = now()
  return writeFile()
end

function M.getJson(key)
  local value = M.get(key, nil)
  if value == nil or type(jsonEncode) ~= "function" then return "" end
  local ok, encoded = pcall(jsonEncode, value)
  if not ok then return "" end
  return tostring(encoded or "")
end

function M.setJson(key, encoded)
  if type(jsonDecode) ~= "function" then return false end
  local ok, value = pcall(jsonDecode, tostring(encoded or ""))
  if not ok then return false end
  return M.set(key, value)
end

M.path = STORE_PATH
return M
