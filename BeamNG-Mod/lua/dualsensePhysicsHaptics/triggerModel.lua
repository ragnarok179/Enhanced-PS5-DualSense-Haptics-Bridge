-- Enhanced PS5 DualSense Haptics
-- Transport-neutral trigger intents with one canonical 48-step force lattice.

local M = {}
local min, max, floor = math.min, math.max, math.floor

local TRIGGER_FORCE_STEPS = 48

local function number(v, fallback)
  return type(v) == "number" and v or (fallback or 0)
end

local function clamp(v, lo, hi)
  return min(max(number(v, lo), lo), hi)
end

local function unit(v)
  return clamp(v, 0, 1)
end

-- Fine Feedback exposes the highest trigger-force resolution used by this mod.
-- Quantize every force/amplitude once to that common lattice. Positions remain
-- normalized separately because their final HID field genuinely has byte
-- precision and is not a force quantity.
local function forceUnit(v)
  v = unit(v)
  if v <= 0 then return 0 end
  local level = clamp(floor(v * TRIGGER_FORCE_STEPS + 0.5), 1, TRIGGER_FORCE_STEPS)
  return level / TRIGGER_FORCE_STEPS
end

local function effectTarget(out)
  return type(out) == "table" and out or {}
end

local function writeEffect(out, kind, startPosition, startForce, endForce, amplitude, frequencyHz)
  out = effectTarget(out)
  out.kind = kind
  out.startPosition = startPosition
  out.startForce = startForce
  out.endForce = endForce
  out.amplitude = amplitude
  out.frequencyHz = frequencyHz
  return out
end

function M.off(out)
  return writeEffect(out, "off", 0, 0, 0, 0, 0)
end

function M.resistance(startPosition, startForce, endForce, out)
  return writeEffect(out, "resistance", unit(startPosition), forceUnit(startForce), forceUnit(endForce), 0, 0)
end

function M.vibration(startPosition, amplitude, frequencyHz, out)
  return writeEffect(out, "vibration", unit(startPosition), 0, 0,
    forceUnit(amplitude), max(number(frequencyHz, 0), 0))
end

function M.fine(startPosition, force, out)
  force = forceUnit(force)
  return writeEffect(out, "fine", unit(startPosition), force, force, 0, 0)
end


local function positionByte(value)
  return floor(unit(value) * 255 + 0.5)
end

local function frequencyByte(value)
  return floor(clamp(number(value, 0), 0, 255) + 0.5)
end

-- Allocation-free change detection at the real HID-significant resolutions.
-- The state table is allocated once by vehicle telemetry and mutated in place.
function M.signatureEquals(effect, state)
  if not state or not state.valid then return false end
  effect = effect or {}
  return state.kind == (effect.kind or "off") and
    state.position == positionByte(effect.startPosition) and
    state.startForce == M.forceLevel(effect.startForce) and
    state.endForce == M.forceLevel(effect.endForce) and
    state.amplitude == M.forceLevel(effect.amplitude) and
    state.frequency == frequencyByte(effect.frequencyHz)
end

function M.captureSignature(effect, state)
  state = state or {}
  effect = effect or {}
  state.kind = effect.kind or "off"
  state.position = positionByte(effect.startPosition)
  state.startForce = M.forceLevel(effect.startForce)
  state.endForce = M.forceLevel(effect.endForce)
  state.amplitude = M.forceLevel(effect.amplitude)
  state.frequency = frequencyByte(effect.frequencyHz)
  state.valid = true
  return state
end


function M.forceUnit(value)
  return forceUnit(value)
end

function M.forceLevel(value)
  return floor(forceUnit(value) * TRIGGER_FORCE_STEPS + 0.5)
end

M.forceSteps = TRIGGER_FORCE_STEPS

return M
