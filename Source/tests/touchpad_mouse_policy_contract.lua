-- Regression contract: current V1.51 BeamNG mouse UI/hotkey/bindings.
local saved = {}
local sent = {}
package.preload['dualsenseSuite/settingsStore'] = function() return {
  get = function(k, default) if k == 'dphTouchBindingMigrationStoreV1' then return true end
    local v=saved[k]; if v==nil then return default end;return v end,
  set = function(k,v) saved[k]=v;return true end,
} end
package.preload['dualsenseSuite/virtualInputDevice'] = function() return {
  name='DualSense Extended Inputs', vidpid='dualsenseextendedv2',
  ensure=function() return 7 end, retireLegacy=function() end, releaseProducer=function() end,
  emitAxis=function() end, emitButton=function() end,
  muteToggleAxisIndex=10,
} end
package.preload['dualsenseSuite/micMuteVehicleSync'] = function() return {
  reset=function() end, invalidate=function() end, update=function() end,
  active=function() return false end, waitingForState=function() return false end,
} end
socket={udp=function() return {
  settimeout=function() end, setsockname=function() return true end, receivefrom=function() return nil end,
  sendto=function(_,buf,_,_) sent[#sent+1]=buf;return #buf end,
} end}
function log() end
core_input_bindings={bindings={{devname='vinput7',contents={vidpid='dualsenseextendedv2',name='DualSense Extended Inputs',bindings={}}}}}
local M=dofile('BeamNG-Mod/lua/ge/extensions/dualsenseExtendedInputs.lua')
local function eq(actual,want,name) assert(actual==want, name..': got '..tostring(actual)..' want '..tostring(want)) end
local function lastDSM()
  for i=#sent,1,-1 do if sent[i]:sub(1,4)=='DSM2' then return sent[i] end end
end
M.onFirstUpdate()
eq(M.getTouchpadMouseMode(),true,'unbound auto mouse default')
eq(lastDSM():byte(5),3,'DSM2 version 3')
eq(lastDSM():byte(8),0,'automatic policy')
M.setTouchpadMouseMode(false)
eq(M.getTouchpadMouseMode(),false,'manual off without binding')
eq(lastDSM():byte(8),2,'explicit off wire policy')
eq(saved.dseTouchpadMouseModeV1,false,'off persistence')
M.toggleTouchpadMouseMode()
eq(M.getTouchpadMouseMode(),true,'BeamNG action toggle enables mouse')
eq(lastDSM():byte(8),1,'explicit on wire policy')
-- A regular DualSense button or sensor binding is not a touchpad binding.
core_input_bindings.bindings[1].contents.bindings={{control='button20',action='testMotion',player=0}}
M.onInputBindingsChanged()
eq(M.getTouchpadMouseMode(),true,'other virtual button must not disable mouse')
eq(lastDSM():byte(6),0,'other virtual button is not touchpad-bound')
core_input_bindings.bindings[1].contents.bindings={{control='button7',action='testTouch',player=0}}
M.onInputBindingsChanged()
eq(M.getTouchpadMouseMode(),false,'new touchpad binding automatically disables mouse')
eq(lastDSM():byte(6),1,'bound flag')
eq(lastDSM():byte(8),2,'auto-off policy after saved touchpad binding')
eq(saved.dseTouchpadMouseModeV1,false,'auto-off persists for later launches')
-- Once the user switches mouse back ON, the same saved binding may stay.
M.setTouchpadMouseMode(true)
M.onInputBindingsChanged()
eq(M.getTouchpadMouseMode(),true,'existing touchpad binding must not undo manual ON')
-- Changing the existing touchpad action counts as a newly assigned binding.
core_input_bindings.bindings[1].contents.bindings[1].action='testTouchDifferent'
M.onInputBindingsChanged()
eq(M.getTouchpadMouseMode(),false,'touchpad reassignment disables mouse')
M.toggleTouchpadMouseMode()
eq(M.getTouchpadMouseMode(),true,'hotkey can re-enable mouse after a new binding')
eq(lastDSM():byte(8),1,'manual on takes ownership from bindings')
M.toggleTouchpadMouseMode()
eq(M.getTouchpadMouseMode(),false,'hotkey off restores saved bindings')
core_input_bindings.bindings[1].contents.bindings={}
M.onInputBindingsChanged()
eq(M.getTouchpadMouseMode(),false,'removal of binding does not turn mouse on')
M.toggleTouchpadMouseMode()
eq(M.getTouchpadMouseMode(),true,'manual on after binding removal')
M.toggleTouchpadMouseMode()
eq(M.getTouchpadMouseMode(),false,'manual off after binding removal')
-- BeamNG may save bindings while its capture popup is open and skip the
-- onInputBindingsChanged callback. The periodic state comparison must still
-- disable mouse after capture closes, never during capture.
M.setTouchpadMouseMode(true)
local inCapture=false
core_input_bindings.isModalPopupOpen=function() return inCapture end
inCapture=true
M.onUpdate(0.15,0.15,0.15)
core_input_bindings.bindings[1].contents.bindings={{control='button9',action='swipeLeft',player=0}}
M.onInputBindingsChanged()
eq(M.getTouchpadMouseMode(),true,'binding editor capture does not change manual policy yet')
inCapture=false
M.onUpdate(0.15,0.15,0.15)
eq(M.getTouchpadMouseMode(),false,'capture close detects new touchpad binding without hook')
-- Changing only a different (motion) button during capture must not turn off mouse.
M.setTouchpadMouseMode(true)
inCapture=true
M.onUpdate(0.15,0.15,0.15)
core_input_bindings.bindings[1].contents.bindings[1]={control='button20',action='testMotion',player=0}
inCapture=false
M.onUpdate(0.15,0.15,0.15)
eq(M.getTouchpadMouseMode(),true,'non-touch binding during capture leaves manual mouse mode')
M.setTouchpadMouseMode(false)
M.onExtensionUnloaded()
print('BeamNG mouse policy contract: PASS')
