-- Regression: touchpad must emit vinput events BEFORE a first saved binding,
-- even when the Windows mouse mode is active or was explicitly turned off.
local events, packets, saved, statuses = {}, {}, {}, {}
package.preload['dualsenseSuite/settingsStore'] = function() return {
  get=function(k,default) if k=='dphTouchBindingMigrationStoreV1' then return true end
      if saved[k]==nil then return default end;return saved[k] end,
  set=function(k,v) saved[k]=v;return true end,
} end
package.preload['dualsenseSuite/virtualInputDevice'] = function() return {
  name='DualSense Extended Inputs',vidpid='dualsenseextendedv2',
  ensure=function() return 7 end,retireLegacy=function() end,releaseProducer=function() end,
  emitAxis=function() end,emitButton=function() end,muteToggleAxisIndex=10,
} end
package.preload['dualsenseSuite/micMuteVehicleSync'] = function() return {
  reset=function() end,invalidate=function() end,update=function() end,
  active=function() return false end,waitingForState=function() return false end,
  pulsedButton=function() return false end,notePhysicalPress=function() end,
} end
core_input_virtualInput={emit=function(id,kind,index,action,val)
  events[#events+1]={id=id,kind=kind,index=index,action=action,val=val}
end}
core_input_bindings={bindings={{devname='vinput7',contents={vidpid='dualsenseextendedv2',name='DualSense Extended Inputs',bindings={}}}}}
socket={udp=function() return {
  settimeout=function() end, setsockname=function() return true end,
  receivefrom=function() local p=table.remove(packets,1);return p end,
  sendto=function(_,p) if p:sub(1,4)=="DSM2" then statuses[#statuses+1]=p end;return #p end, close=function()end,
} end}
function log()end
local M=dofile('BeamNG-Mod/lua/ge/extensions/dualsenseExtendedInputs.lua')
local function frame(gesture,seq)
  local b={string.byte('D'),string.byte('S'),string.byte('E'),string.byte('4'),4,0,gesture,0,seq,0,0,0}
  for i=13,30 do b[i]=0 end
  local unpackValues = unpack or table.unpack; return string.char(unpackValues(b))
end
local function step() M.onUpdate(0.020,0.020,0.020) end
local function tapHasDown()
  for _,e in ipairs(events) do
    if e.kind=='button' and e.index==7 and e.action=='down' then return true end
  end
  return false
end
local function tapOnce(label,seq)
  events={}
  packets[#packets+1]=frame(0,seq);step()
  packets[#packets+1]=frame(1,seq+1);step()
  for i=1,9 do step() end
  assert(tapHasDown(),label..': no virtual button7 down')
  packets[#packets+1]=frame(0,seq+2);for i=1,10 do step() end
end
local function touchFrame(seq, count, x)
  local b={string.byte(frame(0,seq),1,30)}
  b[8]=count>0 and 1 or 0
  b[10]=count
  b[11]=1
  b[13]=x%256; b[14]=math.floor(x/256)
  b[15]=250%256; b[16]=math.floor(250/256)
  local unpackValues=unpack or table.unpack
  return string.char(unpackValues(b))
end
local function swipeRight(seq)
  events={}
  packets[#packets+1]=touchFrame(seq,1,250);step()
  packets[#packets+1]=touchFrame(seq+1,1,1550);step()
  for i=1,8 do step() end
  local found=false
  for _,e in ipairs(events) do
    if e.kind=='button' and e.index==10 and e.action=='down' then found=true end
  end
  assert(found,'swipe-right virtual button10 not available before binding')
  packets[#packets+1]=touchFrame(seq+2,0,0);step()
  for i=1,10 do step() end
end
M.onFirstUpdate()
assert(M.getTouchpadMouseMode()==true,'default auto mouse enabled')
tapOnce('AUTO, no binding',1)
M.setTouchpadMouseMode(false)
assert(M.getTouchpadMouseMode()==false,'explicit OFF')
tapOnce('OFF, no binding',11)
swipeRight(51)
M.setTouchpadMouseMode(true)
tapOnce('ON, before first binding',21)
-- The UI router may not report a nested EditBinding dialog. The native
-- input bindings service knows when it owns a modal capture popup.
local modal = false
core_input_bindings.isModalPopupOpen = function() return modal end
modal=true
for i=1,8 do step() end
assert(statuses[#statuses]:byte(7)==1,'modal binding editor must disable Bridge Windows mouse')
-- Must keep touch vinput events eligible while the popup is open.
tapOnce('modal binding editor',31)
modal=false
for i=1,8 do step() end
assert(statuses[#statuses]:byte(7)==0,'modal close restores ownership policy')
-- Once bound, the touch gesture is suspended only while mouse is explicitly ON.
core_input_bindings.bindings[1].contents.bindings={{control='button7',action='testTouch',player=0}}
M.onInputBindingsChanged()
assert(M.getTouchpadMouseMode()==false, 'new touchpad binding must disable mouse automatically')
-- Check native touchpad click index 3, distinct from ordinary button 4.
M.setTouchpadMouseMode(true)
core_input_bindings.bindings[1].contents.bindings={{control='button3',action='tapPhysicalPad',player=0}}
M.onInputBindingsChanged()
assert(M.getTouchpadMouseMode()==false, 'new physical touchpad click must disable mouse')
assert(statuses[#statuses]:byte(6)==1, 'physical click is a touch binding')
-- A manual ON after saving a binding still overrides its ownership.
M.setTouchpadMouseMode(true)
events={}
packets[#packets+1]=frame(0,71);step()
packets[#packets+1]=frame(1,72);step()
for i=1,8 do step() end
assert(not tapHasDown(),'saved touch action fired while mouse mode owns pad')
local function padClick(seq, down)
  local b={string.byte(frame(0,seq),1,30)}
  b[6]=down and 8 or 0
  local unpackValues=unpack or table.unpack
  packets[#packets+1]=string.char(unpackValues(b));step()
end
local function padClickFired()
  for _,e in ipairs(events) do
    if e.kind=='button' and e.index==3 and e.action=='down' then return true end
  end
  return false
end
padClick(74,true)
assert(not padClickFired(),'bound touchpad click fired during mouse mode')
padClick(75,false)
packets[#packets+1]=frame(0,73);for i=1,10 do step() end
M.setTouchpadMouseMode(false)
assert(M.getTouchpadMouseMode()==false,'off restores BeamNG binding ownership')
tapOnce('OFF, saved binding',81)
events={}
padClick(92,true)
assert(padClickFired(),'touchpad click did not return after mouse OFF')
padClick(93,false)
M.toggleTouchpadMouseMode()
assert(M.getTouchpadMouseMode()==true,'hotkey restores mouse without deleting binding')
assert(#core_input_bindings.bindings[1].contents.bindings==1,'binding must remain saved')
M.onExtensionUnloaded()
print('Touchpad first binding vinput contract: PASS')
