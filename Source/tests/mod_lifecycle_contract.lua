-- Regression for BeamNG core/modmanager.lua onModDeactivated semantics.
local ROOT = "BeamNG-Mod/"
local registered, loaded, order, logMessages = {}, {}, {}, {}
local names = {
  "dualsenseSettingsStore", "dualsenseNativeTriggerGuard", "dualsensePhysicsDirectRPMLights",
  "dualsenseExtendedInputs", "dualsenseMotionInputs", "dualsenseControllerSpeaker",
  "dualsenseCollisionSoundProbeGE", "dualsenseModLifecycle",
}
local mock = {}
for _, name in ipairs(names) do
  registered[name] = true
  loaded[name] = true
end
setExtensionUnloadMode = function(name, mode)
  assert(mode == "auto", "only removes manual lifetime on deactivation")
  if registered[name] then
    registered[name] = nil
    order[#order + 1] = "unregister:" .. name
  end
end
extensions = {unload = function(name)
  assert(not registered[name], "must unregister manual mode before unload: " .. name)
  if loaded[name] then
    order[#order + 1] = "unload:" .. name
    loaded[name] = nil
  end
end}
log = function(_, _, message) logMessages[#logMessages + 1] = message end
local lifecycle = dofile(ROOT .. "lua/ge/extensions/dualsenseModLifecycle.lua")
local function count(value)
  local total = 0
  for _, entry in pairs(value) do if entry then total = total + 1 end end
  return total
end
local function eq(actual, expected, name)
  assert(actual == expected, name .. ": " .. tostring(actual) .. " ~= " .. tostring(expected))
end

-- Other mods and malformed events must never release our resources.
lifecycle.onModDeactivated(nil)
lifecycle.onModDeactivated({modID = "SOMEOTHERMOD", modname = "other.zip"})
lifecycle.onModDeactivated({modData = {tagid = "SOMEOTHERMOD"}, fullpath = "/mods/Enhanced_PS5_DualSense_Haptics_V1.50.zip"})
eq(count(loaded), #names, "unrelated mods keep all extensions")
eq(count(registered), #names, "unrelated mods keep manual registrations")

-- Level transitions do not dispatch onModDeactivated, so extensions stay alive.
eq(count(loaded), #names, "mission changes keep extensions alive")

-- Real BeamNG manager events expose modData.tagid, even for renamed ZIPs.
lifecycle.onModDeactivated({
  modData = {tagid = "ENHANCEDPS5DUALSENSEHAPTICS"},
  fullpath = "/mods/Enhanced_PS5_DualSense_Haptics_V1.50 (7).zip",
})
eq(count(loaded), 0, "all extensions unloaded")
eq(count(registered), 0, "all manual registrations removed")
eq(#order, #names * 2, "one unregister and one unload per extension")
for i, name in ipairs(names) do
  eq(order[i], "unregister:" .. name, "manual mode removed first")
  local expected = i == #names and names[#names] or names[#names - i]
  eq(order[#names + i], "unload:" .. expected, "resource teardown before coordinator")
end
lifecycle.onModDeactivated({modData = {tagid = "ENHANCEDPS5DUALSENSEHAPTICS"}})
eq(#order, #names * 2, "second teardown has no duplicate unload")

-- BeamNG loadManualUnloadExtensions() will only reload names still in the
-- manual list. This list must remain empty while our mod is disabled.
eq(count(registered), 0, "no automatic resurrection while disabled")

-- Simulate modScript registering the same names when user re-enables ZIP.
for _, name in ipairs(names) do registered[name] = true; loaded[name] = true end
lifecycle.onModDeactivated({fullpath="/mods/repo/Enhanced_PS5_DualSense_Haptics_V1.50.zip"})
eq(count(loaded), 0, "fallback filename-based ownership")
eq(count(registered), 0, "fallback unregisters manual modes")
assert(#logMessages == 0, table.concat(logMessages, "\n"))
print("Mod lifecycle deactivation and reactivation contract: PASS")
