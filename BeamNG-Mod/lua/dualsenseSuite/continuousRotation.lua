-- Track a signed steering rotation across the +/-180 degree quaternion wrap.
-- State belongs to a Motion slot, never to a shared controller/global axis.
local M = {}
local fullTurn = 2 * math.pi

local function shortestDelta(previousAngle, currentAngle)
  local delta = currentAngle - previousAngle
  if delta > math.pi then return delta - fullTurn end
  if delta < -math.pi then return delta + fullTurn end
  return delta
end

function M.twistAngle(w, x, y, z, axisX, axisY, axisZ)
  -- Swing/twist decomposition: retain only rotation along the selected axis.
  local projected = x * axisX + y * axisY + z * axisZ
  local magnitude = math.sqrt(w*w + projected*projected)
  if magnitude < 1e-8 then return 0 end
  local angle = 2 * math.atan2(projected / magnitude, w / magnitude)
  if angle > math.pi then return angle - fullTurn end
  if angle < -math.pi then return angle + fullTurn end
  return angle
end

function M.reset(slot)
  slot.previousTwistAngle = nil
  slot.continuousTwistAngle = nil
end

function M.advanceAndNormalize(slot, wrappedRadians, rangePerSideDegrees)
  if slot.previousTwistAngle == nil then
    slot.continuousTwistAngle = wrappedRadians
  else
    slot.continuousTwistAngle = slot.continuousTwistAngle + shortestDelta(slot.previousTwistAngle, wrappedRadians)
  end
  slot.previousTwistAngle = wrappedRadians

  -- Hold the steering end stop even when the hand moves beyond +/-180 degrees;
  -- the output can only leave its stop by rotating back toward the centre.
  local normalized = math.deg(slot.continuousTwistAngle) / math.max(0.1, rangePerSideDegrees)
  return math.max(-1, math.min(1, normalized))
end

return M
