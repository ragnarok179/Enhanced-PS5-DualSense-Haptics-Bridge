-- Ctrl+R retains the same vehicle function; a real replacement must not inherit its latch.
local saved, packets, status, inputEvents, muteAxes = {}, {}, {}, {}, {}
package.preload['dualsenseSuite/settingsStore'] = function() return {
  get=function(key,default) if key=='dphTouchBindingMigrationStoreV1' then return true end
    if saved[key]==nil then return default end; return saved[key] end,
  set=function(key,value) saved[key]=value;return true end,
} end
package.preload['dualsenseSuite/virtualInputDevice'] = function() return {
  name='DualSense Extended Inputs', vidpid='dualsenseextendedv2', muteToggleAxisIndex=10,
  ensure=function()return 7 end, retireLegacy=function()end, releaseProducer=function()end,
  emitAxis=function(_,index,value) muteAxes[#muteAxes+1]={index=index,value=value}end,
} end
local invalidations=0
package.preload['dualsenseSuite/micMuteVehicleSync'] = function() return {
  reset=function() end,invalidate=function()invalidations=invalidations+1 end,
  update=function()end,accept=function()return nil end,pulsedButton=function()return true end,
  notePhysicalPress=function()end,
} end
core_input_virtualInput={emit=function(_,kind,index,action,value)
  inputEvents[#inputEvents+1]={kind=kind,index=index,action=action,value=value}
end}
core_input_bindings={bindings={{devname='vinput7',contents={vidpid='dualsenseextendedv2',name='DualSense Extended Inputs',bindings={}}}}}
local veh={id=101};function veh:getID()return self.id end;getPlayerVehicle=function()return veh end
socket={udp=function()return {
  settimeout=function()end,setsockname=function()return true end,
  receivefrom=function()return table.remove(packets,1)end,
  sendto=function(_,data)status[#status+1]=data;return #data end,close=function()end,
}end}
function log()end
local M=dofile('BeamNG-Mod/lua/ge/extensions/dualsenseExtendedInputs.lua')
local function eq(a,b,why)assert(a==b,why..': '..tostring(a)..' != '..tostring(b))end
local function packet(mask)
  local b={68,83,69,52,4,mask or 0,0,0,1,0,0,0};for i=13,30 do b[i]=0 end
  return string.char((unpack or table.unpack)(b))
end
local function drive(mask)
  packets[#packets+1]=packet(mask);M.onUpdate(0.02,0.02,0.02)
end
local function lastLED()
  for i=#status,1,-1 do if status[i]:sub(1,4)=='DMB2' then return status[i]:byte(7) end end
  error('Missing DMB2 status')
end
local function micDowns()
  local n=0
  for _,e in ipairs(inputEvents) do
    if e.kind=='button' and e.index==2 and e.action=='down' then n=n+1 end
  end
  return n
end
M.onFirstUpdate();M.setMicMuteInputMode('toggle')
drive(4);eq(M.getMicMuteToggleState(),true,'physical click ON')
eq(lastLED(),1,'LED ON')
local axisBefore=#muteAxes
local downsBefore=micDowns()
M.onVehicleResetted(999)
eq(M.getMicMuteToggleState(),true,'other vehicle reset should not touch player')
M.onVehicleResetted(101)
eq(M.getMicMuteToggleState(),true,'Ctrl+R preserves latched vehicle function')
eq(lastLED(),1,'Ctrl+R preserves LED')
M.onVehicleSpawned(101) -- Engine may emit spawn after a reset using the same ID.
eq(M.getMicMuteToggleState(),true,'spawn following Ctrl+R preserves latch')
eq(lastLED(),1,'spawn following Ctrl+R preserves LED')
eq(#muteAxes,axisBefore,'reset must not write virtual set-mode axis')
eq(micDowns(),downsBefore,'reset must not trigger button action')
-- Physical Mic was still held through reload; it must not be replayed as a new press.
drive(4);eq(M.getMicMuteToggleState(),true,'held mic after reload is ignored')
eq(micDowns(),downsBefore,'no synthetic down while held')
drive(0);drive(4);eq(M.getMicMuteToggleState(),false,'next physical press turns preserved toggle OFF')
drive(0)
axisBefore=#muteAxes;downsBefore=micDowns()
M.onVehicleReplaced(101)
eq(M.getMicMuteToggleState(),false,'replacement reuses vehicle ID but clears latch')
eq(lastLED(),0,'replacement LED neutral')
eq(#muteAxes,axisBefore,'replacement is passive')
eq(micDowns(),downsBefore,'replacement triggers no action')
drive(4);drive(0);eq(M.getMicMuteToggleState(),true,'physical press after replacement')
M.onVehicleSpawned(101)
eq(M.getMicMuteToggleState(),true,'spawn after replacement cannot erase a newly activated toggle')
eq(lastLED(),1,'spawn retains LED')
drive(4);drive(0);eq(M.getMicMuteToggleState(),false,'physical press after spawn can toggle OFF')
veh.id=102
M.onVehicleSwitched(101,102,0)
eq(M.getMicMuteToggleState(),false,'new vehicle clears old latch')
eq(lastLED(),0,'new vehicle LED neutral')
M.onExtensionUnloaded()
print('Mic Mute reset/reload/respawn latch contract: PASS')
