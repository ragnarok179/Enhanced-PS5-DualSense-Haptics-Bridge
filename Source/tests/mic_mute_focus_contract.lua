-- Losing BeamNG focus clears the virtual latch when BeamNG releases actions.
-- The bridge carries a focus-loss epoch so even a missed focus transition resyncs.
local saved, packets, status, inputEvents, muteAxes = {}, {}, {}, {}, {}
local focused = true
Engine = {isProgramFocused = function() return focused end}
package.preload['dualsenseSuite/settingsStore'] = function() return {
  get = function(key, fallback) if key == 'dphTouchBindingMigrationStoreV1' then return true end
    return saved[key] == nil and fallback or saved[key] end,
  set = function(key, value) saved[key] = value; return true end,
} end
package.preload['dualsenseSuite/virtualInputDevice'] = function() return {
  name='DualSense Extended Inputs', vidpid='dualsenseextendedv2', muteToggleAxisIndex=10,
  ensure=function() return 7 end, retireLegacy=function() end, releaseProducer=function() end,
  emitAxis=function(_, index, value) muteAxes[#muteAxes+1]={index=index,value=value} end,
} end
package.preload['dualsenseSuite/micMuteVehicleSync'] = function() return {
  reset=function() end, invalidate=function() end, update=function() end,
  accept=function() return nil end, pulsedButton=function() return true end,
  notePhysicalPress=function() end,
} end
core_input_virtualInput={emit=function(_,kind,index,action,value)
  inputEvents[#inputEvents+1]={kind=kind,index=index,action=action,value=value}
end}
core_input_bindings={bindings={{devname='vinput7',contents={vidpid='dualsenseextendedv2',name='DualSense Extended Inputs',bindings={}}}}}
socket={udp=function() return {
  settimeout=function() end,setsockname=function() return true end,
  receivefrom=function() return table.remove(packets,1) end,
  sendto=function(_, data) status[#status+1]=data; return #data end,
  close=function() end,
} end}
function log() end
local M=dofile('BeamNG-Mod/lua/ge/extensions/dualsenseExtendedInputs.lua')
local function eq(got,want,why) assert(got==want,why..': '..tostring(got)..' != '..tostring(want)) end
local function packet(mask, epoch)
  local b={68,83,69,52,4,mask or 0,0,((epoch or 0) % 4) * 64,1,0,0,0}
  for i=13,30 do b[i]=0 end
  return string.char((unpack or table.unpack)(b))
end
local function latestStatus(prefix)
  for i=#status,1,-1 do if status[i]:sub(1,4)==prefix then return status[i] end end
  error('missing status '..prefix)
end
M.onFirstUpdate()
M.setMicMuteInputMode('toggle')
packets[#packets+1]=packet(4)
M.onUpdate(0.02,0.02,0.02)
packets[#packets+1]=packet(0)
M.onUpdate(0.02,0.02,0.02)
eq(M.getMicMuteToggleState(),true,'initial physical toggle')
local axisCount=#muteAxes
local function micDownCount()
  local n=0
  for _, event in ipairs(inputEvents) do
    if event.kind=='button' and event.index==2 and event.action=='down' then n=n+1 end
  end
  return n
end
local downs=micDownCount()
focused=false
M.onUpdate(0.5,0.5,0.5)
eq(M.getMicMuteToggleState(),false,'background focus loss releases virtual latch')
eq(#muteAxes,axisCount,'background timeout may not write a vehicle-setting axis')
eq(micDownCount(),downs,'background timeout may not synthesize a second onDown')
local bg=latestStatus('DMB2')
eq(bg:byte(8),0,'background focus propagated to Bridge')
focused=true
M.onUpdate(0.05,0.05,0.05)
eq(M.getMicMuteToggleState(),false,'focus return must not restore obsolete latch')
packets[#packets+1]=packet(0)
M.onUpdate(0.05,0.05,0.05)
eq(M.getMicMuteToggleState(),false,'heartbeat after focus restore stays neutral')
eq(#muteAxes,axisCount,'recovery does not emit synthetic Mic axis')
eq(micDownCount(),downs,'focus recovery may not synthesize a second onDown')
eq(latestStatus('DMB2'):byte(8),1,'foreground focus propagated to Bridge')
-- A missed focus event (Lua was suspended entirely) must be detected via
-- the epoch carried by the Bridge, even when BeamNG is foregrounded again.
packets[#packets+1]=packet(4)
M.onUpdate(0.02,0.02,0.02)
packets[#packets+1]=packet(0)
M.onUpdate(0.02,0.02,0.02)
eq(M.getMicMuteToggleState(),true,'second genuine Mic press')
local prevAxisCount=#muteAxes
packets[#packets+1]=packet(0,1)
M.onUpdate(0.02,0.02,0.02)
eq(M.getMicMuteToggleState(),false,'focus loss epoch clears missed Alt+Tab state')
eq(#muteAxes,prevAxisCount,'epoch must not synthesize an axis command')
eq(latestStatus('DMB2'):byte(7),0,'LED receives neutral latch on focus epoch')
-- A true stream loss while foregrounded must still clear all latched controls.
M.onUpdate(2.0,2.0,2.0)
eq(M.getMicMuteToggleState(),false,'foreground timeout clears latch')
M.onExtensionUnloaded()
print('Mic Mute focus-resynchronization lifecycle contract: PASS')
