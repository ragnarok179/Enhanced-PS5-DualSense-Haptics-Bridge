-- Optional Mic Mute/vehicle-state synchronisation.
-- Only inspect known, binary vehicle actions. Unknown/custom actions deliberately
-- keep the existing manual toggle: an action name alone is not state telemetry.
local M = {}

local supportedActions = {
  toggleDiffMode = "diff",
  toggle4WDStatus = "4wd",
  toggleRangeStatus = "range",
  toggle_foglights = "fog",
  toggle_left_signal = "left",
  toggle_right_signal = "right",
  toggle_hazard_signal = "hazard",
  toggle_underglow = "underglow",
  parkingbrake_toggle = "parkingbrake",
}

local state = {
  target = nil, control = nil, action = nil,
  vehicleId = nil, deviceInstance = nil, dirty = true, confirmed = false, unavailable = false,
  sequence = 0, pending = nil, nextPollAt = 0,
  lastPressAt = -100, expectedAfterPress = nil, pressGuardUntil = 0,
  confirmedValue = nil, candidateValue = nil, candidateAt = nil,
}

local function playerVehicle()
  if type(getPlayerVehicle) ~= "function" then return nil end
  local ok, veh = pcall(getPlayerVehicle, 0)
  return ok and veh or nil
end

local function matchingBinding(instance)
  if instance == nil or not core_input_bindings or type(core_input_bindings.bindings) ~= "table" then return nil end
  local actions = core_input_actions and core_input_actions.getActiveActions and core_input_actions.getActiveActions()
  if type(actions) ~= "table" then return nil end
  local expectedName = "vinput" .. tostring(instance)
  local match, count = nil, 0
  for _, device in ipairs(core_input_bindings.bindings) do
    if device.devname == expectedName and type(device.contents) == "table" then
      for _, binding in ipairs(device.contents.bindings or {}) do
        local control = type(binding.control) == "string" and binding.control:match("([^%s]+)$") or nil
        if (control == "button2" or control == "axis10") and not binding.unused and
            (binding.player == nil or tonumber(binding.player) == 0) then
          -- Ambiguous/multiple actions must never control a single LED.
          count = count + 1
          local action = actions[binding.action]
          local target = supportedActions[binding.action]
          local valid = target and type(action) == "table" and
            ((control == "button2" and type(action.onDown) == "string" and
              type(action.onUp) ~= "string" and type(action.onChange) ~= "string") or
             (control == "axis10" and type(action.onChange) == "string")) and
            (action.ctx == "vlua" or action.ctx == nil) and
            action.cat ~= "vehicle_specific_unusable"
          if valid then
            match = {target = target, action = binding.action, control = control}
          end
        end
      end
    end
  end
  return count == 1 and match or nil
end

function M.invalidate()
  state.dirty = true
  state.confirmed = false
  state.unavailable = false
  state.pending = nil
  state.confirmedValue, state.candidateValue, state.candidateAt = nil, nil, nil
  state.sequence = state.sequence + 1
  state.expectedAfterPress = nil
  state.pressGuardUntil = 0
  state.nextPollAt = 0
end

function M.reset()
  state.target, state.control, state.action = nil, nil, nil
  state.vehicleId, state.deviceInstance = nil, nil
  state.lastPressAt = -100
  M.invalidate()
end

function M.active() return state.confirmed end
-- While the first observation is pending, also use short button pulses: an
-- old held state must not toggle the freshly loaded vehicle before we read it.
-- If the vehicle cannot expose the state, restore the original manual latch.
function M.pulsedButton()
  -- Keep true for recognized onDown actions even when a state probe fails.
  -- Re-latching a held button after a timeout would synthesize an unwanted
  -- second onDown event and toggle the in-game function back.
  return state.target ~= nil and state.control == "button2"
end
function M.waitingForState()
  return state.target ~= nil and not state.confirmed and not state.unavailable
end

function M.notePhysicalPress(now, expectedAfterPress)
  state.lastPressAt = now
  if type(expectedAfterPress) == "boolean" then
    state.expectedAfterPress = expectedAfterPress
  else
    state.expectedAfterPress = nil
  end
  -- BeamNG may publish the PREVIOUS vehicle state for a few frames after an
  -- onDown toggle. Do not flash the LED back to that stale state: keep the
  -- optimistic local state briefly and only correct it if BeamNG still
  -- disagrees after the action has had time to take effect.
  state.pressGuardUntil = now + 0.65
  state.pending = nil -- a reply from before this press must not undo it
  state.candidateValue, state.candidateAt = nil, nil
  state.nextPollAt = now + 0.10
end

function M.update(now, deviceInstance, enabled)
  if not enabled or deviceInstance == nil then
    if state.target or state.pending then M.reset() end
    return
  end
  local veh = playerVehicle()
  local id = nil
  if veh then
    -- A stale player-vehicle handle may be invalidated during a scene switch.
    local okRead, value = pcall(function() return veh:getID() end)
    if okRead then id = tonumber(value) end
  end
  if state.vehicleId ~= id or state.deviceInstance ~= deviceInstance then
    state.vehicleId, state.deviceInstance = id, deviceInstance
    M.invalidate()
  end
  if state.dirty then
    state.dirty = false
    local selected = id and matchingBinding(deviceInstance) or nil
    state.target = selected and selected.target or nil
    state.action = selected and selected.action or nil
    state.control = selected and selected.control or nil
    state.nextPollAt = now + 0.25 -- give new vehicle's action maps time to load
  end
  if not state.target then
    -- Vehicle-specific actions can appear a little after the vehicle spawns.
    if id and now >= state.nextPollAt + 1.5 then state.dirty = true end
    return
  end
  if not veh or now < state.nextPollAt then return end
  if state.pending then
    if now - state.pending.at < 0.80 then return end
    state.pending = nil -- no reply: never assume a vehicle state
    state.confirmed = false
    state.confirmedValue, state.candidateValue, state.candidateAt = nil, nil, nil
    state.unavailable = true
    state.nextPollAt = now + 2
    return
  end
  state.sequence = state.sequence + 1
  local seq = state.sequence
  state.pending = {id = id, seq = seq, target = state.target, at = now}
  state.nextPollAt = now + 0.25
  local command = string.format(
    "if extensions.dualsenseMicMuteVehicleState then extensions.dualsenseMicMuteVehicleState.query(%q, %d) end",
    state.target, seq)
  local ok = pcall(function() veh:queueLuaCommand(command) end)
  if not ok then state.pending = nil end
end

function M.accept(now, id, sequence, target, value)
  local pending = state.pending
  if not pending or id ~= pending.id or sequence ~= pending.seq or target ~= pending.target then return nil end
  state.pending = nil
  if now - state.lastPressAt < 0.10 then return nil end
  if type(value) == "boolean" and state.expectedAfterPress ~= nil then
    if value ~= state.expectedAfterPress and now < state.pressGuardUntil then
      -- Recheck soon, without overwriting the LED/axis with a transient old
      -- value. An action rejected by the vehicle will still resync after the
      -- guard expires.
      state.nextPollAt = now + 0.10
      return nil
    end
    -- Keep the guard until the expected state is actually confirmed. A
    -- single matching frame followed by an older frame is not confirmation.
    if now >= state.pressGuardUntil then state.expectedAfterPress = nil end
  end
  if type(value) ~= "boolean" then
    -- A temporarily missing reader is unknown, not OFF. Keep the LED and
    -- manual latch unchanged, and retry after the vehicle finishes loading.
    state.confirmed = false
    state.confirmedValue, state.candidateValue, state.candidateAt = nil, nil, nil
    state.unavailable = true
    state.nextPollAt = now + 2
    return nil
  end
  state.unavailable = false
  if state.confirmed and value == state.confirmedValue then
    state.candidateValue, state.candidateAt = nil, nil
    return value, false
  end
  -- At startup, after a reset/replacement, and when the vehicle changes
  -- state externally, a single reading may still describe an old Lua frame.
  -- Accept only two matching observations separated in time.
  if state.candidateValue ~= value then
    state.candidateValue, state.candidateAt = value, now
    state.nextPollAt = now + 0.12
    return nil
  end
  if now - state.candidateAt < 0.12 then
    state.nextPollAt = state.candidateAt + 0.12
    return nil
  end
  local firstObservation = not state.confirmed
  state.confirmed, state.confirmedValue = true, value
  if state.expectedAfterPress == value then state.expectedAfterPress = nil end
  state.candidateValue, state.candidateAt = nil, nil
  return value, firstObservation
end

return M
