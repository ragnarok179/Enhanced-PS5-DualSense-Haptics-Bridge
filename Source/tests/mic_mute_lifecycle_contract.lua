-- GE lifecycle and strict read-only Mic Mute sync integration regression.
local saved, packets, sent, inputs, axes, flags = {}, {}, {}, {}, {}, {invalidate=0, enabled=nil}
package.preload['dualsenseSuite/settingsStore'] = function() return {
  get=function(key,default) if key=='dphTouchBindingMigrationStoreV1' then return true end
    if saved[key]==nil then return default end;return saved[key] end,
  set=function(key,value) saved[key]=value;return true end,
} end
package.preload['dualsenseSuite/virtualInputDevice'] = function() return {
  name='DualSense Extended Inputs',vidpid='dualsenseextendedv2',
  ensure=function()return 7 end,retireLegacy=function()end,releaseProducer=function()end,
  emitAxis=function(_,index,value)axes[#axes+1]={index=index,value=value}end,
  muteToggleAxisIndex=10,
} end
package.preload['dualsenseSuite/micMuteVehicleSync'] = function()return {
  reset=function()end, invalidate=function()flags.invalidate=flags.invalidate+1 end,
  update=function(_,_,enabled)flags.enabled=enabled end,
  accept=function()return true,true end,active=function()return false end,
  waitingForState=function()return false end,pulsedButton=function()return true end,
  notePhysicalPress=function()end,
} end
core_input_virtualInput={emit=function(_,kind,index,action,value)inputs[#inputs+1]={kind=kind,index=index,action=action,value=value}end}
core_input_bindings={bindings={{devname='vinput7',contents={vidpid='dualsenseextendedv2',name='DualSense Extended Inputs',bindings={}}}}}
local activeVehicle={id=101}
function activeVehicle:getID()return self.id end
getPlayerVehicle=function()return activeVehicle end
socket={udp=function()return {
  settimeout=function()end,setsockname=function()return true end,
  receivefrom=function()return table.remove(packets,1)end,
  sendto=function(_,data)sent[#sent+1]=data;return #data end,
  close=function()end,
}end}
function log()end
local function eq(a,b,why)assert(a==b,why..': '..tostring(a)..' != '..tostring(b))end
local M=dofile('BeamNG-Mod/lua/ge/extensions/dualsenseExtendedInputs.lua')
local function packet(mask)
  local b={string.byte('D'),string.byte('S'),string.byte('E'),string.byte('4'),4,mask or 0,0,0,1,0,0,0}
  for i=13,30 do b[i]=0 end
  return string.char((unpack or table.unpack)(b))
end
M.onFirstUpdate()
M.setMicMuteInputMode('toggle')
local original=flags.invalidate
M.onVehicleResetted(999);eq(flags.invalidate,original,'ignore unrelated Ctrl+R')
M.onVehicleResetted(101);eq(flags.invalidate,original+1,'active Ctrl+R')
M.onVehicleSpawned(999);eq(flags.invalidate,original+1,'ignore unrelated spawn')
M.onVehicleSpawned(101);eq(flags.invalidate,original+2,'active spawn')
M.onVehicleReplaced(101);eq(flags.invalidate,original+3,'same ID replacement')
M.onVehicleSwitched(101,102,1);eq(flags.invalidate,original+3,'ignore other player switch')
M.onVehicleSwitched(101,102,0);eq(flags.invalidate,original+4,'active player switch')
M.onClientEndMission();eq(flags.invalidate,original+5,'level unloading invalidates old replies')
M.onUpdate(0.02,0.02,0.02);eq(flags.enabled,false,'no queries while unloading')
M.onClientStartMission();eq(flags.invalidate,original+6,'new level invalidates')
packets[#packets+1]=packet(0)
M.onUpdate(0.02,0.02,0.02)
M.onUpdate(0.02,0.02,0.02);eq(flags.enabled,true,'new level probing resumes')
local beforeAxis=#axes
local beforeInput=#inputs
local beforeLED=#sent
M.onMicMuteVehicleState(101,1,'diff',true)
eq(#axes,beforeAxis,'passive state did not send axis')
eq(#inputs,beforeInput,'passive state did not send button')
assert(#sent>beforeLED,'passive state updated the LED')
eq(M.getMicMuteToggleState(),true,'state applied to local latch')
-- A genuine physical press must still send the virtual axis.
packets[#packets+1]=packet(4)
M.onUpdate(0.02,0.02,0.02)
assert(#axes>beforeAxis,'real Mic Mute press must send input')
eq(M.getMicMuteToggleState(),false,'physical click inverted the toggle')
M.onExtensionUnloaded()
print('Mic Mute GE lifecycle and passive LED contract: PASS')
