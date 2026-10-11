local M = {}
local SETTINGS_STORE = require("dualsenseSuite/settingsStore")
local TOUCH_BINDING_MIGRATION_KEY = "dphTouchBindingMigrationStoreV1"
local initialized = false

local VINPUT = require("dualsenseSuite/virtualInputDevice")

local logTag = "DualSenseExtendedInputs"
local listenHost = "127.0.0.1"
local listenPort = 6977
local bridgeStatusHost = "127.0.0.1"
local bridgeStatusPort = 6978
local deviceName = VINPUT.name
-- Older virtual-input identities are retained only for binding migration.
-- BeamNG stores user bindings per device identifier, so source .diff files are
-- left untouched while compatible bindings are copied to the current device.
local activeDeviceVidPid = VINPUT.vidpid
local legacyTouchDeviceVidPids = {
  "dualsenseextunifiedv1", -- legacy unified touchpad-64 standalone touchpad device
  "dualsenseextswipesv1",   -- legacy touchpad-29 Swipes and legacy touchpad-32 unified identity
  "dualsenseextrelativev1", -- legacy touchpad-29 Relative
  "dualsenseextabsolutev1", -- legacy touchpad-29 Absolute
  "dualsenseextv7",
  "dualsenseextmousev1",
}
local obsoleteDeviceVidPids = legacyTouchDeviceVidPids
local axisCount = 4
local buttonCount = 17
local povCount = 0
local staleTimeout = 0.40
local maxPacketsPerUpdate = 64
local bindingPollPeriod = 0.10
local bindingHeartbeat = 0.50
local touchpadModeSettingKey = "dseTouchpadInputModeV2" -- compatibility preset migration only
local touchpadPolicyState = {
  mouseMode = false,
  twoTapToggle = false,
  twoTapHigh = false,
  lastMouseMode = nil,
  lastTwoTapToggle = nil,
  textEntryGuardAt = -999,
  mouseModeSettingKey = "dseTouchpadMouseModeV1",
  twoTapToggleSettingKey = "dseTouchpadTwoFingerMouseToggleV1",
}
local customTouchpadSettingKeys = {
  one_x = "dseTouchpadCustom1FXV1",
  one_y = "dseTouchpadCustom1FYV1",
  two_x = "dseTouchpadCustom2FXV1",
  two_y = "dseTouchpadCustom2FYV1",
}
local bluetoothIdleTimeoutSettingKey = "dseBluetoothIdleTimeoutMinutesV2"
local bluetoothIdleExplicitOffSettingKey = "dseBluetoothIdleExplicitOffV1"
local defaultTouchpadInputMode = "swipes"
local defaultCustomTouchpadMapping = { one_x="swipe", one_y="swipe", two_x="swipe", two_y="swipe" }
local defaultBluetoothIdleTimeoutMinutes = 10

local udp = nil
local statusUdp = nil
local deviceInstance = nil
local elapsed = 0
local lastPacketAt = -999
local staleResetDone = true
local bindingPollAt = -999
local bindingStatusAt = -999
local lastTouchBindingState = nil
local lastBindingCaptureState = nil
local bindingCaptureActive = false
local bindingPopupOpen = false
local bindingEditPopupOpen = false
local bindingRouteCaptureActive = false
local bindingInputCommandsDisabled = false
local bindingRawEventsForwarded = false
local settingsTextEntryActive = false
local settingsTextEntryOwnsActionMap = false
local actionMapObserverInstalled = false
local rawInputObserverInstalled = false
local inputObserverRetryAt = -999
local originalEnableInputCommands = nil
local wrappedEnableInputCommands = nil
local originalSetForwardRawEvents = nil
local wrappedSetForwardRawEvents = nil
local selectedTouchpadInputMode = "custom" -- current mapping: direct per-channel mapping only
local customTouchpadMapping = { one_x="swipe", one_y="swipe", two_x="swipe", two_y="swipe" }
local bluetoothIdleTimeoutMinutes = defaultBluetoothIdleTimeoutMinutes
local idleMotion = {
  settingKey="dseBluetoothIdleMotionActivityV1", default=false, enabled=false,
  resetSequence=0, activitySequence=0, lastSentAt=-999,
}
local sendTouchDebug = nil
local updateFrame = 0

local buttonState = {}
local axisState = {}

local touchState = {
  contacts = 0,
  id1 = nil, id2 = nil,
  touch1 = false, touch2 = false,
  abs1X = 0.5, abs1Y = 0.5,
  abs2X = 0.5, abs2Y = 0.5,
  oneDX = 0, oneDY = 0,
  twoDX = 0, twoDY = 0,
  pinch = 0,
  tap1 = false, tap2 = false,
  transport = "unknown",
  edge = false,
  inputMode = defaultTouchpadInputMode,
}

-- The current mapping exposes direct per-channel mapping only. Mouse mode is a
-- non-destructive override: saved BeamNG bindings remain on disk while touch
-- outputs are silenced. One-finger tap stays a normal digital control; two-
-- finger tap is also normal unless explicitly reserved as the Mouse-mode toggle.
-- Create/PS/Mute/Edge remain active and never claim touchpad ownership.
local commonTouchControls = { button7=true }
local validCustomChannels = { one_x=true, one_y=true, two_x=true, two_y=true }
local validChannelMappings = { off=true, swipe=true, relative=true, absolute=true }

-- Relative-axis mode is a position relative to the point where the gesture
-- started, not an instantaneous velocity pulse. 30% of the physical pad span
-- reaches full axis deflection; the value remains held while the finger stays
-- at that displacement and recentres only when the contact topology changes.
local relativeSpan = 0.30
local relativeMode = 0
local relativeID1, relativeID2 = nil, nil
local relativeOriginX, relativeOriginY = 0.5, 0.5
local relativeTarget = {0.5, 0.5, 0.5, 0.5}
local relativeNeedNeutral = true
local relativeNeutralFrame = -1
local relativeReleasePending = false
local relativeTargetEmittedFrame = -1
local relativeHasDeflection = false

-- Swipe mappings emit discrete virtual buttons; Relative and Absolute mappings
-- emit analog axes. One- and two-finger taps are independent digital controls.
local swipeThreshold = 0.10
local swipePulseDuration = 0.12
local swipeRearmDuration = 0.04
local swipePulseUntil = {}
local swipeRearmUntil = {}
local swipePending = {}
local swipeSequence = 0
local swipeMode = 0
local swipeFired = false
local swipeStartX, swipeStartY = 0.5, 0.5
local swipeID1, swipeID2 = nil, nil

-- One- and two-finger taps are digital controls independent of axis mapping.
-- DSE4 gesture bits can rise and fall several times while pollUDP() drains a
-- batch in one BeamNG update, so they are converted to queued button pulses
-- serviced once per game frame. This guarantees BeamNG sees a DOWN frame and
-- a later UP frame instead of both transitions being coalesced.
local tapPulseDuration = 0.12
local tapRearmDuration = 0.04
local tapPulseUntil = {}
local tapRearmUntil = {}
local tapPending = {}
local tapGestureHigh = {false, false}
local tapSequence = 0


local function virtualInput()
  return core_input_virtualInput or (extensions and extensions.core_input_virtualInput)
end

local function closeUDP()
  if udp then pcall(function() udp:close() end) end
  udp = nil
end

local function closeStatusUDP()
  if statusUdp then pcall(function() statusUdp:close() end) end
  statusUdp = nil
end

local function ensureStatusUDP()
  if statusUdp then return true end
  if not socket or not socket.udp then return false end
  local ok, s = pcall(socket.udp)
  if not ok or not s then return false end
  statusUdp = s
  pcall(function() statusUdp:settimeout(0) end)
  return true
end

local function deleteDevice()
  -- Release only this producer. The shared vinput remains registered while the
  -- motion producer is still active.
  deviceInstance = nil
  VINPUT.releaseProducer("touchpad")
end

local function configureDeviceShape()
  axisCount = 4
  buttonCount = 17
  buttonState = {}
  axisState = {}
  for i = 1, buttonCount do buttonState[i] = 0 end
  for i = 1, axisCount do axisState[i] = 0.5 end
end

local function removeInactiveDevices()
  VINPUT.retireLegacy()
end

local function ensureDevice()
  removeInactiveDevices()
  local id = VINPUT.ensure(logTag, "touchpad")
  if id == nil then
    deviceInstance = nil
    return false
  end
  deviceInstance = id
  return true
end

local function initUDP()
  closeUDP()
  if not socket or not socket.udp then
    log("E", logTag, "LuaSocket is unavailable")
    return false
  end
  local ok, s = pcall(socket.udp)
  if not ok or not s then
    log("E", logTag, "Unable to create UDP socket")
    return false
  end
  udp = s
  pcall(function() udp:settimeout(0) end)
  local bound, err = udp:setsockname(listenHost, listenPort)
  if not bound then
    log("E", logTag, string.format("Unable to bind %s:%d: %s", listenHost, listenPort, tostring(err)))
    closeUDP()
    return false
  end
  return true
end

local function hasBit(value, mask)
  return math.floor(value / mask) % 2 == 1
end

local function u16le(data, index)
  local lo, hi = data:byte(index, index + 1)
  if not lo or not hi then return 0 end
  return lo + hi * 256
end

local function i16le(data, index)
  local value = u16le(data, index)
  if value >= 32768 then value = value - 65536 end
  return value
end

local function clamp01(v)
  return math.max(0, math.min(1, tonumber(v) or 0.5))
end

local function absoluteX(raw)
  raw = math.max(0, math.min(1919, tonumber(raw) or 0))
  return raw / 1919
end

local function absoluteY(raw)
  raw = math.max(0, math.min(1079, tonumber(raw) or 0))
  return raw / 1079
end

local function q15(value)
  value = math.max(-32767, math.min(32767, tonumber(value) or 0))
  return value / 32767
end

local function centeredRaw(signedValue)
  return clamp01((math.max(-1, math.min(1, signedValue)) + 1) * 0.5)
end

local function normalizeTouchpadInputMode(mode)
  if mode == "relative_axes" or mode == "absolute_axes" or mode == "swipes" or mode == "custom" then
    return mode
  end
  -- Older settings stored "mouse" as a selectable mode. Mouse is now automatic,
  -- so migrate that old value to the normal default representation.
  return defaultTouchpadInputMode
end

local function normalizeChannelMapping(value)
  value = tostring(value or "")
  if validChannelMappings[value] then return value end
  return "swipe"
end

local function readStoredTouchpadInputMode()
  local value = SETTINGS_STORE.get(touchpadModeSettingKey, nil)
  return normalizeTouchpadInputMode(value)
end

local function mappingForLegacyPreset(mode)
  if mode == "relative_axes" then
    return { one_x="relative", one_y="relative", two_x="relative", two_y="relative" }
  elseif mode == "absolute_axes" then
    return { one_x="absolute", one_y="absolute", two_x="absolute", two_y="absolute" }
  end
  return { one_x="swipe", one_y="swipe", two_x="swipe", two_y="swipe" }
end

local function readStoredCustomTouchpadMapping()
  -- current mapping removes presets from the product UI. Existing users still migrate
  -- transparently: if no per-channel setting exists yet, seed it from the last
  -- Older preset values are used only as a fallback for partial maps.
  local legacy = mappingForLegacyPreset(readStoredTouchpadInputMode())
  local result = {}
  for channel, key in pairs(customTouchpadSettingKeys) do
    local value = SETTINGS_STORE.get(key, nil)
    if type(value) == "string" and validChannelMappings[value] then
      result[channel] = value
    else
      result[channel] = legacy[channel] or defaultCustomTouchpadMapping[channel]
    end
  end
  return result
end

local function effectiveTouchpadMapping()
  return customTouchpadMapping
end

local function channelForAxisIndex(index)
  if index == 0 then return "one_x" end
  if index == 1 then return "one_y" end
  if index == 2 then return "two_x" end
  if index == 3 then return "two_y" end
  return nil
end

local function mappingForAxisIndex(index)
  local channel = channelForAxisIndex(index)
  local mapping = effectiveTouchpadMapping()
  return channel and mapping[channel] or "off"
end

local function mappingSummary()
  local mapping = effectiveTouchpadMapping()
  return string.format("1x=%s,1y=%s,2x=%s,2y=%s", mapping.one_x, mapping.one_y, mapping.two_x, mapping.two_y)
end

-- BeamNG stores user input changes in per-device .diff files. legacy touchpad-29 used
-- three touchpad device identifiers, while legacy touchpad-32 reused the old Swipes id
-- for a differently-shaped unified device. Migrate those user files into a new
-- stable legacy unified touchpad id before the legacy virtual devices are unregistered. Source
-- .diff files are never removed or modified, so migration is reversible.
local function legacyBindingSourceOrder(control)
  local order = { "dualsenseextunifiedv1", "dualsenseextswipesv1" }
  local mapping = effectiveTouchpadMapping()
  local channelByControl = {
    xaxis="one_x", yaxis="one_y", zaxis="two_x", rxaxis="two_y"
  }
  local channel = channelByControl[tostring(control or "")]
  if channel and mapping[channel] == "absolute" then
    table.insert(order, "dualsenseextabsolutev1")
    table.insert(order, "dualsenseextrelativev1")
  else
    table.insert(order, "dualsenseextrelativev1")
    table.insert(order, "dualsenseextabsolutev1")
  end
  table.insert(order, "dualsenseextv7")
  table.insert(order, "dualsenseextmousev1")
  return order
end

local function readBindingDiff(path)
  if not FS then return nil end
  local okExists, exists = pcall(function() return FS:fileExists(path) end)
  if not okExists or not exists then return nil end
  local ok, data = pcall(jsonReadFile, path)
  if not ok or type(data) ~= "table" or type(data.bindings) ~= "table" then return nil end
  return data
end

local function migrateLegacyTouchpadBindingsInDir(dir)
  local targetPath = dir .. "/" .. activeDeviceVidPid .. ".diff"
  local existing = readBindingDiff(targetPath)

  local sources = {}
  local allControls = {}
  for _, vidpid in ipairs(legacyTouchDeviceVidPids) do
    local path = dir .. "/" .. vidpid .. ".diff"
    local data = readBindingDiff(path)
    if data and #data.bindings > 0 then
      sources[vidpid] = data
      for _, binding in ipairs(data.bindings) do
        if type(binding) == "table" and type(binding.control) == "string" and binding.control ~= "" then
          allControls[binding.control] = true
        end
      end
    end
  end
  if next(sources) == nil then return 0, 0, "none" end

  local migrated = {}
  local migratedKeys = {}
  if existing and type(existing.bindings) == "table" then
    for _, binding in ipairs(existing.bindings) do
      if type(binding) == "table" then
        local copy = deepcopy(binding)
        table.insert(migrated, copy)
        migratedKeys[tostring(copy.control or "") .. "\0" .. tostring(copy.action or "")] = true
      end
    end
  end
  local existingCount = #migrated
  local conflicts = 0
  for control in pairs(allControls) do
    local sourceCount = 0
    for _, vidpid in ipairs(legacyTouchDeviceVidPids) do
      local data = sources[vidpid]
      if data then
        for _, binding in ipairs(data.bindings) do
          if binding.control == control then sourceCount = sourceCount + 1; break end
        end
      end
    end
    if sourceCount > 1 then conflicts = conflicts + 1 end

    for _, vidpid in ipairs(legacyBindingSourceOrder(control)) do
      local data = sources[vidpid]
      local found = false
      if data then
        for _, binding in ipairs(data.bindings) do
          if binding.control == control then
            found = true
            local action = tostring(binding.action or "")
            local key = control .. "\0" .. action
            if not migratedKeys[key] then
              table.insert(migrated, deepcopy(binding))
              migratedKeys[key] = true
            end
          end
        end
      end
      if found then break end -- one historical profile owns each control
    end
  end

  if #migrated == 0 then return 0, conflicts, "empty" end
  if #migrated == existingCount then return 0, conflicts, existingCount > 0 and "existing" or "empty" end
  table.sort(migrated, function(a, b)
    local ac, bc = tostring(a.control or ""), tostring(b.control or "")
    if ac == bc then return tostring(a.action or "") < tostring(b.action or "") end
    return ac < bc
  end)
  local out = {
    version = 1,
    name = deviceName,
    devicetype = "vinput",
    vidpid = activeDeviceVidPid,
    bindings = migrated,
  }
  local ok = jsonWriteFile(targetPath, out, true)
  if not ok then
    log("E", logTag, "Could not migrate legacy touchpad bindings to " .. targetPath)
    return 0, conflicts, "write_failed"
  end
  local added = #migrated - existingCount
  return added, conflicts, "migrated"
end

local function migrateLegacyTouchpadBindings()
  if not FS or type(jsonReadFile) ~= "function" or type(jsonWriteFile) ~= "function" then
    log("W", logTag, "Binding migration skipped: filesystem/json helpers unavailable")
    return
  end
  local baseDir = "settings/inputmaps"
  local dirs = { baseDir }
  local entries = FS:findFiles(baseDir, "*", -1, false, true) or {}
  for _, entry in ipairs(entries) do
    local stat = FS:stat(entry)
    if stat and stat.filetype == "dir" then table.insert(dirs, entry) end
  end
  local total, conflicts = 0, 0
  for _, dir in ipairs(dirs) do
    local count, conflictCount = migrateLegacyTouchpadBindingsInDir(dir)
    total = total + (tonumber(count) or 0)
    conflicts = conflicts + (tonumber(conflictCount) or 0)
  end
end

local function hasMappingType(mappingType)
  local mapping = effectiveTouchpadMapping()
  return mapping.one_x == mappingType or mapping.one_y == mappingType or
    mapping.two_x == mappingType or mapping.two_y == mappingType
end

local function activeTouchControls()
  local controls = { button7=true }
  if not touchpadPolicyState.twoTapToggle then controls.button8 = true end
  local mapping = effectiveTouchpadMapping()
  if mapping.one_x == "swipe" then controls.button9=true; controls.button10=true
  elseif mapping.one_x == "relative" or mapping.one_x == "absolute" then controls.xaxis=true end
  if mapping.one_y == "swipe" then controls.button11=true; controls.button12=true
  elseif mapping.one_y == "relative" or mapping.one_y == "absolute" then controls.yaxis=true end
  if mapping.two_x == "swipe" then controls.button13=true; controls.button14=true
  elseif mapping.two_x == "relative" or mapping.two_x == "absolute" then controls.zaxis=true end
  if mapping.two_y == "swipe" then controls.button15=true; controls.button16=true
  elseif mapping.two_y == "relative" or mapping.two_y == "absolute" then controls.rxaxis=true end
  return controls
end

local function normalizeBluetoothIdleTimeoutMinutes(value)
  local minutes = tonumber(value) or defaultBluetoothIdleTimeoutMinutes
  minutes = math.floor(minutes + 0.5)
  return math.max(0, math.min(240, minutes))
end

local function settingIsExplicitTrue(value)
  return value == true or value == 1 or value == "1" or tostring(value):lower() == "true"
end

local function readStoredBluetoothIdleTimeoutMinutes()
  local value = SETTINGS_STORE.get(bluetoothIdleTimeoutSettingKey, nil)
  local explicitOff = SETTINGS_STORE.get(bluetoothIdleExplicitOffSettingKey, nil)
  if settingIsExplicitTrue(explicitOff) then return 0 end
  -- BeamNG can return numeric 0 for an unknown/uninitialized custom setting.
  -- Treat that ambiguous state as the intended 10-minute default. An actual
  -- user-selected Off is represented by the explicit-off companion key.
  local numeric = tonumber(value)
  if numeric == nil or numeric <= 0 then return defaultBluetoothIdleTimeoutMinutes end
  return normalizeBluetoothIdleTimeoutMinutes(numeric)
end

local function readStoredBoolSetting(key, defaultValue)
  local value = SETTINGS_STORE.get(key, nil)
  if value == true or value == 1 or value == "1" or tostring(value):lower() == "true" then return true end
  if value == false or value == 0 or value == "0" or tostring(value):lower() == "false" then return false end
  return defaultValue == true
end

local function persistSetting(key, value)
  SETTINGS_STORE.set(key, value)
end

local function touchControlsEnabled()
  if touchpadPolicyState.mouseMode then return false end
  -- While unbound, the Bridge owns the pad as the automatic mouse fallback.
  -- As soon as the binding editor starts listening, vinput temporarily owns it;
  -- after a real touch binding is saved, vinput keeps ownership permanently.
  return bindingCaptureActive or lastTouchBindingState == true
end

local function emitButton(index, value, force)
  if index < 0 or index >= buttonCount then return end
  local slot = index + 1
  value = value and 1 or 0
  if not force and buttonState[slot] == value then return end
  buttonState[slot] = value
  local vi = virtualInput()
  if vi and deviceInstance ~= nil then
    vi.emit(deviceInstance, "button", index, value > 0 and "down" or "up", value)
  end
end

local function emitAxis(index, value, force)
  if index < 0 or index >= axisCount then return end
  local slot = index + 1
  value = clamp01(value)
  if not force and math.abs((axisState[slot] or 0.5) - value) < 0.0005 then return end
  axisState[slot] = value
  local vi = virtualInput()
  if vi and deviceInstance ~= nil then
    vi.emit(deviceInstance, "axis", index, "change", value)
  end
end

local function swipeDirectionName(index)
  local names = {
    [9] = "left", [10] = "right", [11] = "up", [12] = "down",
    [13] = "two_left", [14] = "two_right", [15] = "two_up", [16] = "two_down",
  }
  return names[index] or ("button" .. tostring(index))
end

local function serviceSwipeButtons()
  -- A swipe is a real digital pulse. Repeated identical gestures are queued,
  -- and a short re-arm interval guarantees an observable UP state before the
  -- next DOWN. Do not emit synthetic UP events while already released: BeamNG
  -- binding capture counts raw button events, so neutral spam can capture the
  -- wrong control during a tap.
  for i = 9, 16 do
    local isDown = buttonState[i + 1] == 1
    if isDown then
      local untilAt = swipePulseUntil[i] or math.huge
      if elapsed >= untilAt then
        emitButton(i, false, true)
        swipePulseUntil[i] = nil
        swipeRearmUntil[i] = elapsed + swipeRearmDuration
        if sendTouchDebug then
          sendTouchDebug("SWIPE_UP", string.format("frame=%d button=%d dir=%s pending=%d", updateFrame, i, swipeDirectionName(i), swipePending[i] or 0))
        end
      end
    else
      local pending = swipePending[i] or 0
      local rearmAt = swipeRearmUntil[i] or -1
      if pending > 0 and elapsed >= rearmAt then
        swipePending[i] = pending - 1
        emitButton(i, true, true)
        swipePulseUntil[i] = elapsed + swipePulseDuration
        if sendTouchDebug then
          sendTouchDebug("SWIPE_DOWN", string.format("frame=%d button=%d dir=%s pending=%d", updateFrame, i, swipeDirectionName(i), swipePending[i] or 0))
        end
      end
    end
  end
end

local function tapName(index)
  if index == 7 then return "one_finger" end
  if index == 8 then return "two_finger" end
  return "tap" .. tostring(index)
end

local function queueTapButton(index)
  tapSequence = tapSequence + 1
  tapPending[index] = (tapPending[index] or 0) + 1
  if sendTouchDebug then
    sendTouchDebug("TAP_DETECTED", string.format("seq=%d frame=%d button=%d tap=%s pending=%d down=%s",
      tapSequence, updateFrame, index, tapName(index), tapPending[index] or 0,
      (buttonState[index + 1] == 1) and "1" or "0"))
  end
end

local function serviceTapButtons()
  for i = 7, 8 do
    local isDown = buttonState[i + 1] == 1
    if isDown then
      local untilAt = tapPulseUntil[i] or math.huge
      if elapsed >= untilAt then
        emitButton(i, false, true)
        tapPulseUntil[i] = nil
        tapRearmUntil[i] = elapsed + tapRearmDuration
        if sendTouchDebug then
          sendTouchDebug("TAP_UP", string.format("frame=%d button=%d tap=%s pending=%d",
            updateFrame, i, tapName(i), tapPending[i] or 0))
        end
      end
    else
      local pending = tapPending[i] or 0
      local rearmAt = tapRearmUntil[i] or -1
      if pending > 0 and elapsed >= rearmAt then
        tapPending[i] = pending - 1
        emitButton(i, true, true)
        tapPulseUntil[i] = elapsed + tapPulseDuration
        if sendTouchDebug then
          sendTouchDebug("TAP_DOWN", string.format("frame=%d button=%d tap=%s pending=%d",
            updateFrame, i, tapName(i), tapPending[i] or 0))
        end
      end
    end
  end
end

local function cancelTapButtonStates()
  for i = 7, 8 do
    tapPulseUntil[i] = nil
    tapRearmUntil[i] = nil
    tapPending[i] = nil
    if buttonState[i + 1] == 1 then emitButton(i, false, true) end
  end
  tapGestureHigh[1], tapGestureHigh[2] = false, false
end

local function cancelSwipeButtonStates()
  -- Used only when leaving swipe mode, losing touch ownership, reloading, or
  -- unloading. Never generate release events for buttons that are already up:
  -- raw binding capture treats every button event as meaningful.
  for i = 9, 16 do
    swipePulseUntil[i] = nil
    swipeRearmUntil[i] = nil
    swipePending[i] = nil
    if buttonState[i + 1] == 1 then emitButton(i, false, true) end
  end
end

local function queueSwipeButton(index, count, dx, dy)
  swipeSequence = swipeSequence + 1
  swipePending[index] = (swipePending[index] or 0) + 1
  if sendTouchDebug then
    sendTouchDebug("SWIPE_DETECTED", string.format(
      "seq=%d fingers=%d button=%d dir=%s dx=%.4f dy=%.4f pending=%d down=%s",
      swipeSequence, count or 0, index, swipeDirectionName(index), dx or 0, dy or 0,
      swipePending[index] or 0, (buttonState[index + 1] == 1) and "1" or "0"))
  end
end

local function resetSwipeRecognizer()
  swipeMode = 0
  swipeFired = false
  swipeID1, swipeID2 = nil, nil
  swipeStartX, swipeStartY = 0.5, 0.5
end

local function releaseAllSwipeButtons()
  cancelSwipeButtonStates()
  resetSwipeRecognizer()
end

local function resetSwipeGesture(count, id1, id2, abs1X, abs1Y, abs2X, abs2Y)
  swipeMode = count or 0
  swipeFired = false
  swipeID1, swipeID2 = id1, id2
  if sendTouchDebug and swipeMode > 0 then
    sendTouchDebug("SWIPE_START", string.format("frame=%d fingers=%d id1=%s id2=%s", updateFrame, swipeMode, tostring(id1), tostring(id2)))
  end
  if swipeMode == 1 then
    swipeStartX, swipeStartY = abs1X, abs1Y
  elseif swipeMode == 2 then
    swipeStartX = (abs1X + abs2X) * 0.5
    swipeStartY = (abs1Y + abs2Y) * 0.5
  else
    swipeStartX, swipeStartY = 0.5, 0.5
  end
end

local function updateSwipeGesture(count, id1, id2, abs1X, abs1Y, abs2X, abs2Y)
  -- A swipe consumes only the channel(s) configured as Swipe. This makes mixed
  -- mappings possible (for example 1F X = Swipe, 1F Y = Relative) without the
  -- analog channel suppressing the digital gesture. If both X and Y are Swipe
  -- we preserve the familiar dominant-axis behavior and emit one direction.
  if count == 0 then
    if swipeMode ~= 0 and sendTouchDebug then
      sendTouchDebug("SWIPE_END", string.format("frame=%d fired=%s", updateFrame, swipeFired and "1" or "0"))
    end
    resetSwipeRecognizer()
    return
  end

  local idsChanged = (count == 1 and swipeID1 ~= id1) or
                     (count == 2 and (swipeID1 ~= id1 or swipeID2 ~= id2))
  if count ~= swipeMode or idsChanged then
    resetSwipeGesture(count, id1, id2, abs1X, abs1Y, abs2X, abs2Y)
    return
  end
  if swipeFired or (count ~= 1 and count ~= 2) then return end

  local mapping = effectiveTouchpadMapping()
  local useX, useY
  if count == 1 then
    useX, useY = mapping.one_x == "swipe", mapping.one_y == "swipe"
  else
    useX, useY = mapping.two_x == "swipe", mapping.two_y == "swipe"
  end
  if not useX and not useY then return end

  local x, y
  if count == 1 then
    x, y = abs1X, abs1Y
  else
    x = (abs1X + abs2X) * 0.5
    y = (abs1Y + abs2Y) * 0.5
  end

  local dx, dy = x - swipeStartX, y - swipeStartY
  local base = count == 1 and 9 or 13
  local button = nil
  if useX and useY then
    if math.max(math.abs(dx), math.abs(dy)) < swipeThreshold then return end
    if math.abs(dx) >= math.abs(dy) then
      button = dx < 0 and base or (base + 1)
    else
      button = dy < 0 and (base + 2) or (base + 3)
    end
  elseif useX then
    if math.abs(dx) < swipeThreshold then return end
    button = dx < 0 and base or (base + 1)
  elseif useY then
    if math.abs(dy) < swipeThreshold then return end
    button = dy < 0 and (base + 2) or (base + 3)
  end

  if button then
    queueSwipeButton(button, count, dx, dy)
    swipeFired = true
  end
end

local function resetRelativeGesture(count, id1, id2, abs1X, abs1Y, abs2X, abs2Y)
  relativeMode = count or 0
  relativeID1, relativeID2 = id1, id2
  if relativeMode == 1 then
    relativeOriginX, relativeOriginY = abs1X, abs1Y
  elseif relativeMode == 2 then
    relativeOriginX = (abs1X + abs2X) * 0.5
    relativeOriginY = (abs1Y + abs2Y) * 0.5
  else
    relativeOriginX, relativeOriginY = 0.5, 0.5
  end
  relativeTarget[1], relativeTarget[2], relativeTarget[3], relativeTarget[4] = 0.5, 0.5, 0.5, 0.5
  relativeNeedNeutral = true
  relativeReleasePending = false
  relativeTargetEmittedFrame = -1
  relativeHasDeflection = false
  if sendTouchDebug then
    sendTouchDebug("RELATIVE_NEUTRAL_REQUEST", string.format("frame=%d contacts=%d id1=%s id2=%s", updateFrame, relativeMode, tostring(id1), tostring(id2)))
  end
end

local function relativeSigned(delta)
  return math.max(-1, math.min(1, delta / relativeSpan))
end

local function updateRelativeAxes(count, id1, id2, abs1X, abs1Y, abs2X, abs2Y)
  -- Contact-up must not erase a short gesture before BeamNG has observed its
  -- directional value on a frame. Preserve the last target until the frame
  -- service has emitted it once; neutral is emitted on a later frame.
  if count == 0 and relativeMode ~= 0 then
    relativeMode = 0
    relativeID1, relativeID2 = nil, nil
    relativeReleasePending = true
    if sendTouchDebug then
      sendTouchDebug("RELATIVE_RELEASE_REQUEST", string.format("frame=%d deflected=%s", updateFrame, relativeHasDeflection and "1" or "0"))
    end
    return
  end

  local idsChanged = (count == 1 and relativeID1 ~= id1) or
                     (count == 2 and (relativeID1 ~= id1 or relativeID2 ~= id2))
  if count ~= relativeMode or idsChanged then
    resetRelativeGesture(count, id1, id2, abs1X, abs1Y, abs2X, abs2Y)
    return
  end

  if count == 1 then
    relativeTarget[1] = centeredRaw(relativeSigned(abs1X - relativeOriginX))
    relativeTarget[2] = centeredRaw(relativeSigned(abs1Y - relativeOriginY))
    relativeTarget[3], relativeTarget[4] = 0.5, 0.5
  elseif count == 2 then
    local cx = (abs1X + abs2X) * 0.5
    local cy = (abs1Y + abs2Y) * 0.5
    relativeTarget[1], relativeTarget[2] = 0.5, 0.5
    relativeTarget[3] = centeredRaw(relativeSigned(cx - relativeOriginX))
    relativeTarget[4] = centeredRaw(relativeSigned(cy - relativeOriginY))
  else
    relativeTarget[1], relativeTarget[2], relativeTarget[3], relativeTarget[4] = 0.5, 0.5, 0.5, 0.5
  end
  relativeHasDeflection = false
  for i = 1, 4 do
    if math.abs(relativeTarget[i] - 0.5) >= 0.01 then relativeHasDeflection = true; break end
  end
end

local function serviceRelativeAxes()
  if not touchControlsEnabled() or not hasMappingType("relative") then return end
  if relativeNeedNeutral then
    for i = 0, 3 do
      if mappingForAxisIndex(i) == "relative" then emitAxis(i, 0.5, true) end
    end
    relativeNeedNeutral = false
    relativeNeutralFrame = updateFrame
    if sendTouchDebug then
      sendTouchDebug("RELATIVE_NEUTRAL", string.format("frame=%d map=%s", updateFrame, mappingSummary()))
    end
    return
  end
  if updateFrame <= relativeNeutralFrame then return end

  -- For a very short gesture, contact-down, movement and contact-up may all be
  -- consumed from the 120 Hz UDP queue in one BeamNG frame. Keep the peak
  -- directional target for one full game frame before releasing to neutral.
  if relativeHasDeflection and relativeTargetEmittedFrame < relativeNeutralFrame then
    for i = 0, 3 do
      if mappingForAxisIndex(i) == "relative" then emitAxis(i, relativeTarget[i + 1], true) end
    end
    relativeTargetEmittedFrame = updateFrame
    if sendTouchDebug then
      sendTouchDebug("RELATIVE_TARGET", string.format("frame=%d x=%.4f y=%.4f z=%.4f rx=%.4f release=%s map=%s", updateFrame,
        relativeTarget[1], relativeTarget[2], relativeTarget[3], relativeTarget[4], relativeReleasePending and "1" or "0", mappingSummary()))
    end
    return
  end

  if relativeReleasePending then
    if relativeHasDeflection and updateFrame <= relativeTargetEmittedFrame then return end
    for i = 0, 3 do
      if mappingForAxisIndex(i) == "relative" then emitAxis(i, 0.5, true) end
    end
    relativeTarget[1], relativeTarget[2], relativeTarget[3], relativeTarget[4] = 0.5, 0.5, 0.5, 0.5
    relativeReleasePending = false
    relativeHasDeflection = false
    relativeNeutralFrame = updateFrame
    relativeTargetEmittedFrame = -1
    if sendTouchDebug then sendTouchDebug("RELATIVE_RELEASE", string.format("frame=%d map=%s", updateFrame, mappingSummary())) end
    return
  end

  for i = 0, 3 do
    if mappingForAxisIndex(i) == "relative" then emitAxis(i, relativeTarget[i + 1]) end
  end
  if relativeHasDeflection then relativeTargetEmittedFrame = updateFrame end
end

local function resetState()
  for i = 0, axisCount - 1 do emitAxis(i, 0.5) end
  for i = 0, buttonCount - 1 do emitButton(i, false, true) end
  touchState.contacts = 0
  touchState.id1, touchState.id2 = nil, nil
  touchState.touch1, touchState.touch2 = false, false
  touchState.abs1X, touchState.abs1Y = 0.5, 0.5
  touchState.abs2X, touchState.abs2Y = 0.5, 0.5
  touchState.oneDX, touchState.oneDY = 0, 0
  touchState.twoDX, touchState.twoDY = 0, 0
  touchState.pinch = 0
  touchState.tap1, touchState.tap2 = false, false
  swipeMode = 0
  swipeFired = false
  swipeID1, swipeID2 = nil, nil
  swipeStartX, swipeStartY = 0.5, 0.5
  swipePulseUntil = {}
  swipeRearmUntil = {}
  swipePending = {}
  tapPulseUntil = {}
  tapRearmUntil = {}
  tapPending = {}
  tapGestureHigh = {false, false}
  touchpadPolicyState.twoTapHigh = false
  relativeMode = 0
  relativeID1, relativeID2 = nil, nil
  relativeOriginX, relativeOriginY = 0.5, 0.5
  relativeTarget = {0.5, 0.5, 0.5, 0.5}
  relativeNeedNeutral = true
  relativeNeutralFrame = -1
  relativeReleasePending = false
  relativeTargetEmittedFrame = -1
  relativeHasDeflection = false
end

local function applyPacket(data)
  if type(data) ~= "string" or #data ~= 30 or data:sub(1, 4) ~= "DSE4" then return false end
  if data:byte(5) ~= 4 then return false end

  local flags = data:byte(6) or 0
  local gestures = data:byte(7) or 0
  local tf = data:byte(8) or 0
  local touch1 = hasBit(tf, 1)
  local touch2 = hasBit(tf, 2)

  emitButton(0, hasBit(flags, 1))
  emitButton(1, hasBit(flags, 2))
  emitButton(2, hasBit(flags, 4))
  emitButton(3, hasBit(flags, 8))
  emitButton(4, hasBit(flags, 16))
  emitButton(5, hasBit(flags, 32))
  emitButton(6, hasBit(flags, 64))
  local tap1 = hasBit(gestures, 1)
  local tap2 = hasBit(gestures, 2)

  -- A dedicated 2F tap toggle remains observable even while Mouse mode owns
  -- the pad. This is intentionally the only touch gesture that can restore
  -- saved bindings; swipes and 1F tap are fully suppressed in Mouse mode.
  if touchpadPolicyState.twoTapToggle and tap2 and not touchpadPolicyState.twoTapHigh then
    M.toggleTouchpadMouseMode()
  end
  touchpadPolicyState.twoTapHigh = tap2

  if touchControlsEnabled() then
    if tap1 and not tapGestureHigh[1] then queueTapButton(7) end
    if not touchpadPolicyState.twoTapToggle and tap2 and not tapGestureHigh[2] then queueTapButton(8) end
  else
    cancelTapButtonStates()
  end
  tapGestureHigh[1], tapGestureHigh[2] = tap1, tap2

  local count = data:byte(10) or 0
  local id1 = data:byte(11) or 0
  local id2 = data:byte(12) or 0
  local x1, y1 = u16le(data, 13), u16le(data, 15)
  local x2, y2 = u16le(data, 17), u16le(data, 19)
  local oneDX, oneDY = q15(i16le(data, 21)), q15(i16le(data, 23))
  local twoDX, twoDY = q15(i16le(data, 25)), q15(i16le(data, 27))
  local pinch = q15(i16le(data, 29))

  local abs1X = touch1 and absoluteX(x1) or 0.5
  local abs1Y = touch1 and absoluteY(y1) or 0.5
  local abs2X = touch2 and absoluteX(x2) or 0.5
  local abs2Y = touch2 and absoluteY(y2) or 0.5
  local touchEnabled = touchControlsEnabled()
  if not touchEnabled then
    for i = 0, axisCount - 1 do emitAxis(i, 0.5) end
    releaseAllSwipeButtons()
    relativeMode = 0; relativeID1, relativeID2 = nil, nil
    relativeTarget = {0.5, 0.5, 0.5, 0.5}; relativeNeedNeutral = true; relativeReleasePending = false; relativeHasDeflection = false; relativeTargetEmittedFrame = -1
  else
    if hasMappingType("relative") then
      updateRelativeAxes(count, touch1 and id1 or nil, touch2 and id2 or nil, abs1X, abs1Y, abs2X, abs2Y)
    else
      relativeMode = 0; relativeID1, relativeID2 = nil, nil
      relativeTarget = {0.5, 0.5, 0.5, 0.5}; relativeNeedNeutral = true; relativeReleasePending = false; relativeHasDeflection = false; relativeTargetEmittedFrame = -1
    end

    local absoluteValues = { abs1X, abs1Y, abs2X, abs2Y }
    for i = 0, 3 do
      if mappingForAxisIndex(i) == "absolute" then
        emitAxis(i, absoluteValues[i + 1])
      end
    end

    if hasMappingType("swipe") then
      updateSwipeGesture(count, touch1 and id1 or nil, touch2 and id2 or nil, abs1X, abs1Y, abs2X, abs2Y)
    else
      releaseAllSwipeButtons()
    end
  end

  touchState.contacts = count
  touchState.id1 = touch1 and id1 or nil
  touchState.id2 = touch2 and id2 or nil
  touchState.touch1, touchState.touch2 = touch1, touch2
  touchState.abs1X, touchState.abs1Y = abs1X, abs1Y
  touchState.abs2X, touchState.abs2Y = abs2X, abs2Y
  touchState.oneDX, touchState.oneDY = oneDX, oneDY
  touchState.twoDX, touchState.twoDY = twoDX, twoDY
  touchState.pinch = pinch
  touchState.tap1 = tap1
  touchState.tap2 = tap2
  touchState.transport = hasBit(tf, 8) and "Bluetooth" or "USB"
  touchState.edge = hasBit(tf, 4)
  touchState.inputMode = touchpadPolicyState.mouseMode and "mouse" or "custom"

  lastPacketAt = elapsed
  staleResetDone = false
  return true
end

local function pollUDP()
  if not udp then return end
  for _ = 1, maxPacketsPerUpdate do
    local data = udp:receivefrom()
    if not data then break end
    applyPacket(data)
  end
end

local function baseControl(control)
  if type(control) ~= "string" then return nil end
  return control:match("([^%s]+)$")
end

local function hasBeamNGTouchBindings()
  if touchpadPolicyState.mouseMode then return false end
  if deviceInstance == nil or not core_input_bindings or type(core_input_bindings.bindings) ~= "table" then
    return false
  end
  local devname = "vinput" .. tostring(deviceInstance)
  for _, device in ipairs(core_input_bindings.bindings) do
    if device.devname == devname and type(device.contents) == "table" and type(device.contents.bindings) == "table" then
      local activeControls = activeTouchControls()
      for _, binding in ipairs(device.contents.bindings) do
        local control = baseControl(binding.control)
        if commonTouchControls[control] or activeControls[control] then
          return true
        end
      end
    end
  end
  return false
end

-- VINPUT.ensure() performs the single binding refresh when it creates the
-- shared virtual device. Re-running core_input_bindings.onDeviceChanged here
-- caused redundant full input/FFB rebuilds, especially around controller
-- reconnects. Existing shared devices are already known to the binding system.

local function currentRouteName()
  local router = extensions and extensions.ui_router or ui_router
  if router and type(router.getCurrent) == "function" then
    local ok, current = pcall(router.getCurrent)
    if ok and type(current) == "table" then
      return current.resolved and current.resolved.name or current.name
    end
  end
  return nil
end

local function isTouchBindingsRoute(routeName)
  if type(routeName) ~= "string" then return false end
  local route = string.lower(routeName)
  local root = "menu.options.controls.bindings"
  return route == root or string.find(route, root .. ".", 1, true) == 1
end

local function isTouchBindingCaptureRoute(routeName)
  if type(routeName) ~= "string" then return false end
  local route = string.lower(routeName)
  -- BeamNG 0.39 can present binding capture either as an edit/multistep
  -- child route or as a popup layered over the root bindings route.
  local edit = "menu.options.controls.bindings.edit"
  local multistep = "menu.options.controls.bindings.multistep"
  return route == edit or route == multistep or
         string.find(route, edit .. ".", 1, true) == 1 or
         string.find(route, multistep .. ".", 1, true) == 1
end

local function isPopupStateName(stateName)
  return type(stateName) == "string" and string.find(string.lower(stateName), "/popup/", 1, true) == 1
end

local function isEditBindingPopupStateName(stateName)
  if type(stateName) ~= "string" then return false end
  local state = string.lower(stateName)
  return string.find(state, "/popup/editbinding", 1, true) == 1
end

local function popupStateStackFlags(stateStack)
  local anyPopup, editBinding = false, false
  if type(stateStack) ~= "table" then return anyPopup, editBinding end
  for _, stateName in ipairs(stateStack) do
    if isPopupStateName(stateName) then anyPopup = true end
    if isEditBindingPopupStateName(stateName) then editBinding = true end
  end
  return anyPopup, editBinding
end

local function updateBindingPopupState(stateName, opened, stateStack)
  local popupOpen, editBindingOpen = bindingPopupOpen, bindingEditPopupOpen
  local stackPopup, stackEditBinding = popupStateStackFlags(stateStack)
  if stackPopup then popupOpen = true end
  if stackEditBinding then editBindingOpen = true end

  if isPopupStateName(stateName) then
    popupOpen = opened == true
  end
  if isEditBindingPopupStateName(stateName) then
    editBindingOpen = opened == true
  end

  local changed = popupOpen ~= bindingPopupOpen or editBindingOpen ~= bindingEditPopupOpen
  bindingPopupOpen = popupOpen
  bindingEditPopupOpen = editBindingOpen
  return changed
end

idleMotion.readStored = function()
  return readStoredBoolSetting(idleMotion.settingKey,idleMotion.default)
end

idleMotion.sendConfig = function()
  if not ensureStatusUDP() then return end
  local minutes=normalizeBluetoothIdleTimeoutMinutes(bluetoothIdleTimeoutMinutes)
  local flags=idleMotion.enabled and 1 or 0
  local seq=idleMotion.resetSequence%65536
  local packet="DSC2"..string.char(1,minutes%256,math.floor(minutes/256),flags,seq%256,math.floor(seq/256))
  pcall(function() statusUdp:sendto(packet,bridgeStatusHost,bridgeStatusPort) end)
end

idleMotion.noteActivity = function(gyroMagnitude,linearAccelerationMagnitude)
  if not idleMotion.enabled then return false end
  if elapsed-idleMotion.lastSentAt<0.20 then return false end
  idleMotion.lastSentAt=elapsed
  idleMotion.activitySequence=(idleMotion.activitySequence+1)%65536
  if ensureStatusUDP() then
    local seq=idleMotion.activitySequence
    local packet="DSA1"..string.char(1,seq%256,math.floor(seq/256))
    pcall(function() statusUdp:sendto(packet,bridgeStatusHost,bridgeStatusPort) end)
  end
  sendTouchDebug("IDLE_ACTIVITY",string.format("source=motion gyro=%.2f accel=%.3f",tonumber(gyroMagnitude) or 0,tonumber(linearAccelerationMagnitude) or 0))
  return true
end

local function sendTouchBindingStatus(force)
  local active = hasBeamNGTouchBindings()
  local capture = bindingCaptureActive
  local mouseForced = touchpadPolicyState.mouseMode
  local suppressTwoTapMouse = touchpadPolicyState.twoTapToggle
  if not force and active == lastTouchBindingState and capture == lastBindingCaptureState and
      mouseForced == touchpadPolicyState.lastMouseMode and suppressTwoTapMouse == touchpadPolicyState.lastTwoTapToggle and
      elapsed - bindingStatusAt < bindingHeartbeat then return end
  if not ensureStatusUDP() then return end
  -- DSM2 v2: bound, binding-capture, forced Mouse mode, suppress Windows 2F
  -- right-click. Bridge remains backward-compatible with DSM2 v1.
  local packet = "DSM2" .. string.char(2, active and 1 or 0, capture and 1 or 0, mouseForced and 1 or 0, suppressTwoTapMouse and 1 or 0)
  local ok = pcall(function() statusUdp:sendto(packet, bridgeStatusHost, bridgeStatusPort) end)
  local idleMinutes = normalizeBluetoothIdleTimeoutMinutes(bluetoothIdleTimeoutMinutes)
  local idlePacket = "DSC1" .. string.char(1, idleMinutes % 256, math.floor(idleMinutes / 256))
  pcall(function() statusUdp:sendto(idlePacket, bridgeStatusHost, bridgeStatusPort) end)
  idleMotion.sendConfig()
  if ok then
    if active ~= lastTouchBindingState or capture ~= lastBindingCaptureState or mouseForced ~= touchpadPolicyState.lastMouseMode or suppressTwoTapMouse ~= touchpadPolicyState.lastTwoTapToggle then
      if capture then
      elseif mouseForced then
      elseif active then
      else
      end
    end
    lastTouchBindingState = active
    lastBindingCaptureState = capture
    touchpadPolicyState.lastMouseMode = mouseForced
    touchpadPolicyState.lastTwoTapToggle = suppressTwoTapMouse
    bindingStatusAt = elapsed
  end
end

local function debugBool(value)
  return value and "1" or "0"
end

local function cleanDebugText(value)
  local text = tostring(value or "nil")
  text = string.gsub(text, "[\r\n\t]", " ")
  if #text > 700 then text = string.sub(text, 1, 700) end
  return text
end

sendTouchDebug = function(eventName, details)
  if not ensureStatusUDP() then return end
  local packet = "DSMD1\t" .. cleanDebugText(eventName) .. "\t" .. cleanDebugText(details)
  pcall(function() statusUdp:sendto(packet, bridgeStatusHost, bridgeStatusPort) end)
end

local function sendTouchBindingDebug()
  local devname = deviceInstance ~= nil and ("vinput" .. tostring(deviceInstance)) or "nil"
  sendTouchDebug("TOUCH_DEVICE", string.format("preset=%s map=%s device=%s vidpid=%s axes=%d buttons=%d",
    selectedTouchpadInputMode, mappingSummary(), devname, activeDeviceVidPid, axisCount, buttonCount))
  if deviceInstance == nil or not core_input_bindings or type(core_input_bindings.bindings) ~= "table" then
    sendTouchDebug("TOUCH_BINDINGS", "device/bindings unavailable")
    return
  end

  local currentFound, swipeFound = 0, 0
  for _, device in ipairs(core_input_bindings.bindings) do
    local contents = type(device.contents) == "table" and device.contents or nil
    local vidpid = contents and tostring(contents.vidpid or "") or ""
    local productName = contents and tostring(contents.name or "") or ""
    if string.find(string.lower(vidpid), "dualsenseext", 1, true) == 1 or productName == deviceName then
      sendTouchDebug("TOUCH_PROFILE_DEVICE", string.format("device=%s vidpid=%s name=%s current=%s",
        tostring(device.devname), vidpid, productName, device.devname == devname and "1" or "0"))
    end
    if device.devname == devname and contents and type(contents.bindings) == "table" then
      for _, binding in ipairs(contents.bindings) do
        local control = baseControl(binding.control)
        local kind = nil
        local index = control and tonumber(string.match(control, "^button(%d+)$")) or nil
        if index and index >= 9 and index <= 16 then kind = "swipe"; swipeFound = swipeFound + 1
        elseif control == "button7" or control == "button8" then kind = "tap"
        elseif control == "xaxis" or control == "yaxis" or control == "zaxis" or control == "rxaxis" then kind = "axis" end
        if kind then
          currentFound = currentFound + 1
          sendTouchDebug("TOUCH_BINDING", string.format("device=%s preset=%s map=%s kind=%s control=%s action=%s",
            devname, selectedTouchpadInputMode, mappingSummary(), kind, cleanDebugText(binding.control), cleanDebugText(binding.action)))
        end
      end
    end
  end
  if currentFound == 0 then sendTouchDebug("TOUCH_BINDINGS", "device=" .. devname .. " none") end
  if swipeFound == 0 then sendTouchDebug("SWIPE_BINDINGS", "device=" .. devname .. " none") end
end

local function shouldCaptureTouchBinding(routeName)
  -- BeamNG 0.39's Vue controls editor is an EditBinding fullscreen popup
  -- layered over pause.options. The popup itself is therefore authoritative;
  -- requiring the legacy bindings route keeps capture false on the live UI.
  return isTouchBindingCaptureRoute(routeName) or bindingEditPopupOpen
end

local function captureDebugSummary(routeName)
  local route = routeName or currentRouteName()
  return string.format(
    "route=%s popup=%s editBindingPopup=%s routeCapture=%s actionCommandsDisabled=%s rawEvents=%s finalCapture=%s bound=%s mouse=%s twoTapToggle=%s map=%s dev=%s vidpid=%s idleBT=%dmin motionIdle=%s",
    cleanDebugText(route), debugBool(bindingPopupOpen), debugBool(bindingEditPopupOpen), debugBool(bindingRouteCaptureActive),
    debugBool(bindingInputCommandsDisabled), debugBool(bindingRawEventsForwarded),
    debugBool(bindingCaptureActive), debugBool(hasBeamNGTouchBindings()), debugBool(touchpadPolicyState.mouseMode), debugBool(touchpadPolicyState.twoTapToggle), mappingSummary(),
    deviceInstance ~= nil and ("vinput" .. tostring(deviceInstance)) or "nil", activeDeviceVidPid,
    normalizeBluetoothIdleTimeoutMinutes(bluetoothIdleTimeoutMinutes), debugBool(idleMotion.enabled))
end

local function refreshBindingCapture(routeName)
  local route = routeName or currentRouteName()
  bindingRouteCaptureActive = shouldCaptureTouchBinding(route)

  -- BeamNG's controls UI does not always expose binding capture as a distinct
  -- Lua-router route. Its actual capture path disables input commands and
  -- forwards RawInputChanged events. Observe those engine calls directly and
  -- use route/popup detection only as a fallback.
  local observedCapture = (bindingInputCommandsDisabled and not settingsTextEntryActive) or bindingEditPopupOpen or
    (bindingRawEventsForwarded and (bindingEditPopupOpen or isTouchBindingsRoute(route)))
  local capture = bindingRouteCaptureActive or observedCapture
  if capture ~= bindingCaptureActive then
    bindingCaptureActive = capture
    sendTouchDebug("CAPTURE", captureDebugSummary(route))
    -- Immediate ownership hand-off: a tap used as the new BeamNG binding must
    -- never also become mouse0/click in Windows.
    sendTouchBindingStatus(true)
  end
end

local function installInputCaptureObservers()
  if not actionMapObserverInstalled and ActionMap and type(ActionMap.enableInputCommands) == "function" then
    local original = ActionMap.enableInputCommands
    if original ~= wrappedEnableInputCommands then
      originalEnableInputCommands = original
      wrappedEnableInputCommands = function(enabled, ...)
        local result = originalEnableInputCommands(enabled, ...)
        bindingInputCommandsDisabled = enabled == false
        sendTouchDebug("ACTION_COMMANDS", "enabled=" .. debugBool(enabled ~= false))
        refreshBindingCapture()
        return result
      end
      local ok = pcall(function() ActionMap.enableInputCommands = wrappedEnableInputCommands end)
      actionMapObserverInstalled = ok and ActionMap.enableInputCommands == wrappedEnableInputCommands
      sendTouchDebug("OBSERVER", "ActionMap.enableInputCommands=" .. (actionMapObserverInstalled and "installed" or "FAILED"))
    end
  end

  if not rawInputObserverInstalled and WinInput and type(WinInput.setForwardRawEvents) == "function" then
    local original = WinInput.setForwardRawEvents
    if original ~= wrappedSetForwardRawEvents then
      originalSetForwardRawEvents = original
      wrappedSetForwardRawEvents = function(enabled, ...)
        local result = originalSetForwardRawEvents(enabled, ...)
        bindingRawEventsForwarded = enabled == true
        sendTouchDebug("RAW_EVENTS", "enabled=" .. debugBool(enabled == true))
        refreshBindingCapture()
        return result
      end
      local ok = pcall(function() WinInput.setForwardRawEvents = wrappedSetForwardRawEvents end)
      rawInputObserverInstalled = ok and WinInput.setForwardRawEvents == wrappedSetForwardRawEvents
      sendTouchDebug("OBSERVER", "WinInput.setForwardRawEvents=" .. (rawInputObserverInstalled and "installed" or "FAILED"))
    end
  end

  return actionMapObserverInstalled or rawInputObserverInstalled
end

local function setSettingsTextEntryActive(active)
  active=active==true
  if active then
    settingsTextEntryActive=true
    touchpadPolicyState.textEntryGuardAt=-999
    installInputCaptureObservers()
    -- Retry ownership on every call. previous implementation could enter text-entry mode before
    -- ActionMap became available, then permanently early-return without ever
    -- disabling gameplay bindings.
    if not settingsTextEntryOwnsActionMap and not bindingInputCommandsDisabled and ActionMap and type(ActionMap.enableInputCommands)=="function" then
      local ok=pcall(ActionMap.enableInputCommands,false)
      settingsTextEntryOwnsActionMap=ok
    end
    sendTouchDebug("TEXT_ENTRY","active=1 ownsActionMap="..debugBool(settingsTextEntryOwnsActionMap).." disabled="..debugBool(bindingInputCommandsDisabled))
  else
    local owned=settingsTextEntryOwnsActionMap
    settingsTextEntryActive=false; settingsTextEntryOwnsActionMap=false; touchpadPolicyState.textEntryGuardAt=-999
    if owned and ActionMap and type(ActionMap.enableInputCommands)=="function" then pcall(ActionMap.enableInputCommands,true) end
    sendTouchDebug("TEXT_ENTRY","active=0")
  end
  refreshBindingCapture()
  return true
end

local function restoreInputCaptureObservers()
  if actionMapObserverInstalled and ActionMap and ActionMap.enableInputCommands == wrappedEnableInputCommands and originalEnableInputCommands then
    pcall(function() ActionMap.enableInputCommands = originalEnableInputCommands end)
  end
  if rawInputObserverInstalled and WinInput and WinInput.setForwardRawEvents == wrappedSetForwardRawEvents and originalSetForwardRawEvents then
    pcall(function() WinInput.setForwardRawEvents = originalSetForwardRawEvents end)
  end
  actionMapObserverInstalled = false
  rawInputObserverInstalled = false
end

local function onAfterRouteChange(context)
  local toRoute = context and context.toRoute or nil
  local routeName = toRoute and toRoute.name or nil
  if routeName and string.find(string.lower(routeName), "options", 1, true) == nil then
    bindingPopupOpen = false
    bindingEditPopupOpen = false
  end
  sendTouchDebug("ROUTE", "to=" .. cleanDebugText(routeName))
  refreshBindingCapture(routeName)
end

local function onUIStateTriggered(stateName, opened, stateStack)
  local changed = updateBindingPopupState(stateName, opened, stateStack)
  sendTouchDebug("UI_STATE", string.format("state=%s opened=%s popup=%s editBindingPopup=%s", cleanDebugText(stateName), debugBool(opened == true), debugBool(bindingPopupOpen), debugBool(bindingEditPopupOpen)))
  if changed then refreshBindingCapture() end
end

local function resetTouchpadOutputs()
  for i = 0, axisCount - 1 do emitAxis(i, 0.5) end
  cancelTapButtonStates()
  releaseAllSwipeButtons()
end

local function resetTouchpadMappingState()
  resetTouchpadOutputs()
  resetSwipeRecognizer()
  relativeMode = 0
  relativeID1, relativeID2 = nil, nil
  relativeOriginX, relativeOriginY = 0.5, 0.5
  relativeTarget = {0.5, 0.5, 0.5, 0.5}
  relativeNeedNeutral = true
  relativeNeutralFrame = -1
  relativeReleasePending = false
  relativeHasDeflection = false
  relativeTargetEmittedFrame = -1
end

local function setTouchpadCustomMapping(channel, mapping)
  channel = tostring(channel or "")
  if not validCustomChannels[channel] then return nil end
  local normalized = normalizeChannelMapping(mapping)
  if customTouchpadMapping[channel] == normalized then
    sendTouchBindingStatus(true)
    return normalized
  end
  resetTouchpadMappingState()
  customTouchpadMapping[channel] = normalized
  persistSetting(customTouchpadSettingKeys[channel], normalized)
  persistSetting(touchpadModeSettingKey, "custom")
  selectedTouchpadInputMode = "custom"
  lastTouchBindingState, lastBindingCaptureState = nil, nil
  sendTouchDebug("MAPPING", string.format("map=%s changed=%s", mappingSummary(), channel))
  sendTouchBindingStatus(true)
  return normalized
end

local function getTouchpadCustomMapping()
  return {
    one_x = customTouchpadMapping.one_x, one_y = customTouchpadMapping.one_y,
    two_x = customTouchpadMapping.two_x, two_y = customTouchpadMapping.two_y,
  }
end

local function setTouchpadMouseMode(enabled)
  local value = enabled == true or enabled == 1 or enabled == "1" or tostring(enabled):lower() == "true"
  if value == touchpadPolicyState.mouseMode then
    sendTouchBindingStatus(true)
    return value
  end
  resetTouchpadMappingState()
  touchpadPolicyState.mouseMode = value
  touchState.inputMode = value and "mouse" or "custom"
  persistSetting(touchpadPolicyState.mouseModeSettingKey, value)
  lastTouchBindingState, lastBindingCaptureState = nil, nil
  touchpadPolicyState.lastMouseMode = nil
  sendTouchDebug("MOUSE_MODE", string.format("enabled=%s bindingsPreserved=1 map=%s", debugBool(value), mappingSummary()))
  sendTouchBindingStatus(true)
  return value
end

local function getTouchpadMouseMode()
  return touchpadPolicyState.mouseMode
end

local function toggleTouchpadMouseMode()
  return setTouchpadMouseMode(not touchpadPolicyState.mouseMode)
end

local function setTwoFingerMouseToggleEnabled(enabled)
  -- Compatibility API only: previous implementation permanently removed the dedicated 2F
  -- Mouse-mode shortcut. button8 is always a normal touchpad button.
  touchpadPolicyState.twoTapToggle = false
  touchpadPolicyState.twoTapHigh = false
  persistSetting(touchpadPolicyState.twoTapToggleSettingKey, false)
  touchpadPolicyState.lastTwoTapToggle = nil
  sendTouchBindingStatus(true)
  return false
end

local function getTwoFingerMouseToggleEnabled()
  return false
end

local function setBluetoothIdleTimeoutMinutes(minutes)
  local normalized = normalizeBluetoothIdleTimeoutMinutes(minutes)
  if normalized == bluetoothIdleTimeoutMinutes then
    sendTouchBindingStatus(true)
    return normalized
  end
  bluetoothIdleTimeoutMinutes = normalized
  idleMotion.resetSequence=(idleMotion.resetSequence+1)%65536
  persistSetting(bluetoothIdleTimeoutSettingKey, normalized)
  persistSetting(bluetoothIdleExplicitOffSettingKey, normalized == 0)
  sendTouchDebug("IDLE_CONFIG",string.format("minutes=%d resetSeq=%d motion=%s",normalized,idleMotion.resetSequence,debugBool(idleMotion.enabled)))
  sendTouchBindingStatus(true)
  return normalized
end

idleMotion.setEnabled = function(enabled)
  idleMotion.enabled=(enabled==true or enabled==1 or enabled=="1" or tostring(enabled):lower()=="true")
  persistSetting(idleMotion.settingKey,idleMotion.enabled)
  idleMotion.resetSequence=(idleMotion.resetSequence+1)%65536
  sendTouchDebug("IDLE_CONFIG",string.format("minutes=%d resetSeq=%d motion=%s",normalizeBluetoothIdleTimeoutMinutes(bluetoothIdleTimeoutMinutes),idleMotion.resetSequence,debugBool(idleMotion.enabled)))
  sendTouchBindingStatus(true)
  return idleMotion.enabled
end

idleMotion.getEnabled = function() return idleMotion.enabled end

local function getBluetoothIdleTimeoutMinutes()
  return bluetoothIdleTimeoutMinutes
end

local function initializeRuntime()
  if initialized then return end
  initialized = true
  customTouchpadMapping = readStoredCustomTouchpadMapping()
  selectedTouchpadInputMode = "custom"
  touchpadPolicyState.mouseMode = readStoredBoolSetting(touchpadPolicyState.mouseModeSettingKey, false)
  -- The dedicated 2F mouse-mode shortcut was removed from the public design.
  -- Keep button8 as a normal touchpad button; Mouse mode is toggled by the
  -- global BeamNG action, which can be assigned to any regular button/key.
  touchpadPolicyState.twoTapToggle = false
  persistSetting(touchpadPolicyState.twoTapToggleSettingKey, false)
  bluetoothIdleTimeoutMinutes = readStoredBluetoothIdleTimeoutMinutes()
  idleMotion.enabled = idleMotion.readStored()
  touchState.inputMode = touchpadPolicyState.mouseMode and "mouse" or "custom"
  if not SETTINGS_STORE.get(TOUCH_BINDING_MIGRATION_KEY, false) then
    migrateLegacyTouchpadBindings()
    SETTINGS_STORE.set(TOUCH_BINDING_MIGRATION_KEY, true)
  end
  configureDeviceShape()
  ensureDevice()
  initUDP()
  ensureStatusUDP()
  resetState()
  installInputCaptureObservers()
  refreshBindingCapture(currentRouteName())
  sendTouchDebug("LOAD", captureDebugSummary())
  sendTouchBindingDebug()
  sendTouchBindingStatus(true)
end

function M.onExtensionLoaded()
  -- Keep extension loading lightweight; initialization runs on the first update.
end

function M.onFirstUpdate()
  initializeRuntime()
end

function M.onUpdate(dtReal, dtSim, dtRaw)
  if not initialized then initializeRuntime() end
  updateFrame = updateFrame + 1
  elapsed = elapsed + math.max(0, tonumber(dtReal) or tonumber(dtSim) or tonumber(dtRaw) or 0)
  if deviceInstance == nil then ensureDevice() end
  if udp == nil then initUDP() end
  pollUDP()
  serviceTapButtons()
  serviceSwipeButtons()
  serviceRelativeAxes()
  -- Keep the keyboard guard asserted for the whole lifetime of a focused
  -- numeric editor. Some BeamNG UI transitions can re-enable input commands
  -- after focus; previous implementation only disabled them once, so typing could again fire
  -- gameplay bindings. Reassert at low frequency while preserving ownership.
  if settingsTextEntryActive and elapsed-(touchpadPolicyState.textEntryGuardAt or -999)>=0.08 then
    touchpadPolicyState.textEntryGuardAt=elapsed
    installInputCaptureObservers()
    if not bindingInputCommandsDisabled and ActionMap and type(ActionMap.enableInputCommands)=="function" then
      local ok=pcall(ActionMap.enableInputCommands,false)
      if ok then settingsTextEntryOwnsActionMap=true end
    end
  end
  if not staleResetDone and elapsed - lastPacketAt > staleTimeout then
    resetState()
    staleResetDone = true
  end
  if elapsed - bindingPollAt >= bindingPollPeriod then
    bindingPollAt = elapsed
    if elapsed - inputObserverRetryAt >= 0.50 then
      inputObserverRetryAt = elapsed
      installInputCaptureObservers()
    end
    -- Router polling remains a fallback. Engine-call observers are authoritative
    -- when BeamNG's binding editor is an overlay rather than a distinct route.
    refreshBindingCapture()
    sendTouchBindingStatus(false)
  end
end

function M.onInputBindingsChanged()
  -- BeamNG writes user changes as the regular device .diff inputmap. Re-scan
  -- immediately so mouse fallback stops as soon as a touchpad control is bound.
  sendTouchBindingDebug()
  sendTouchBindingStatus(true)
end

function M.onExtensionUnloaded()
  if settingsTextEntryActive then setSettingsTextEntryActive(false) end
  resetState()
  restoreInputCaptureObservers()
  closeUDP()
  closeStatusUDP()
  deleteDevice()
end

function M.getTouchState()
  return deepcopy(touchState)
end

M.onAfterRouteChange = onAfterRouteChange
M.onUIStateTriggered = onUIStateTriggered
M.setTouchpadCustomMapping = setTouchpadCustomMapping
M.getTouchpadCustomMapping = getTouchpadCustomMapping
M.setTouchpadMouseMode = setTouchpadMouseMode
M.getTouchpadMouseMode = getTouchpadMouseMode
M.toggleTouchpadMouseMode = toggleTouchpadMouseMode
M.setTwoFingerMouseToggleEnabled = setTwoFingerMouseToggleEnabled
M.getTwoFingerMouseToggleEnabled = getTwoFingerMouseToggleEnabled
M.setBluetoothIdleTimeoutMinutes = setBluetoothIdleTimeoutMinutes
M.getBluetoothIdleTimeoutMinutes = getBluetoothIdleTimeoutMinutes
M.setBluetoothIdleMotionActivityEnabled = idleMotion.setEnabled
M.getBluetoothIdleMotionActivityEnabled = idleMotion.getEnabled
M.noteControllerMotionActivity = idleMotion.noteActivity
M.setSettingsTextEntryActive = setSettingsTextEntryActive

return M
