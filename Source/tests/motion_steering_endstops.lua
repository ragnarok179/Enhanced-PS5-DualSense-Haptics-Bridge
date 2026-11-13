-- Rotation Angle must retain steering direction after the +/-180 degree wrap.
-- Run with Lua 5.4, independent of BeamNG or physical controller hardware.
package.path = "BeamNG-Mod/lua/?.lua;" .. package.path
local rotation = require("dualsenseSuite/continuousRotation")

local function approximately(actual, expected, context)
  assert(math.abs(actual-expected) < 1e-6,
    string.format("%s: expected %.6f, got %.6f", context, expected, actual))
end

-- Test the same quaternion twist decomposition used by Motion Inputs.
-- Quaternion signs may switch after 180 degrees; both represent the same pose.
local function output(slot, degrees, range)
  local halfAngle = math.rad(degrees) / 2
  local angle = rotation.twistAngle(math.cos(halfAngle), math.sin(halfAngle), 0, 0, 1, 0, 0)
  return rotation.advanceAndNormalize(slot, angle, range)
end

local slot = {}
for _, sample in ipairs({
  {0,0}, {45,0.25}, {135,0.75}, {179,179/180},
  {181,1}, {200,1}, {270,1}, {359,1}, {360,1}, {420,1},
  {270,1}, {200,1}, {181,1}, {179,179/180}, {90,0.5}, {0,0},
  {-90,-0.5}, {-179,-179/180}, {-181,-1}, {-270,-1},
  {-360,-1}, {-420,-1}, {-300,-1}, {-181,-1}, {-179,-179/180}, {0,0},
}) do
  approximately(output(slot, sample[1], 180), sample[2], "rotation "..sample[1])
end

-- The chosen per-side range, rather than 180 degrees, determines the stop.
for _, range in ipairs({90,135,180}) do
  local independent = {}
  output(independent,0,range)
  approximately(output(independent,range/2,range),0.5,"half of "..range)
  approximately(output(independent,range+30,range),1,"stop of "..range)
  approximately(output(independent,range+90,range),1,"further past stop of "..range)
  approximately(output(independent,range+180,range),1,"past stop of "..range)
  approximately(output(independent,range+120,range),1,"return above stop "..range)
  approximately(output(independent,range+60,range),1,"return past stop "..range)
  approximately(output(independent,range/2,range),0.5,"return within "..range)
end

-- A separate Motion slot is independent; recentering resets phase history.
local other={}
approximately(output(other,-45,90),-0.5,"other independent slot")
rotation.reset(slot)
approximately(output(slot,0,180),0,"recenter neutral")
approximately(output(slot,20,180),20/180,"after recenter")

-- +/-180-degree jitter never flips a full-lock steering axis.
local jitter={}
for _, degrees in ipairs({0,175,179,180,181,179.9,180.1,182,185}) do
  local value=output(jitter,degrees,180)
  if degrees>=175 then
    assert(value>0.90, "full-lock jitter must not invert at "..degrees)
  end
end

-- Quaternion signs have no effect on the extracted twist phase.
local half = math.rad(181) / 2
approximately(rotation.twistAngle(math.cos(half),math.sin(half),0,0,1,0,0),
  rotation.twistAngle(-math.cos(half),-math.sin(half),0,0,1,0,0), "q and -q")
-- A rotation on another physical axis must not accidentally turn the steering wheel.
approximately(rotation.twistAngle(math.cos(math.rad(35)),0,math.sin(math.rad(35)),0,1,0,0),0,
  "off-axis swing")

print("Motion steering end stops: PASS")
