-- Regression: per-vehicle Mic Mute LED survives Ctrl+R and A -> B -> A switches.
-- An observation can update the stored state, but a switch must never operate a vehicle.
local saved, packets, sent, events, axes = {}, {}, {}, {}, {}
local vehicle = {id=101}
function vehicle:getID() return self.id end
getPlayerVehicle=function() return vehicle end
package.preload['dualsenseSuite/settingsStore']=function() return {
  get=function(key,default) if key=='dphTouchBindingMigrationStoreV1' then return true end
    return saved[key]==nil and default or saved[key] end,
  set=function(key,value) saved[key]=value return true end,
} end
package.preload['dualsenseSuite/virtualInputDevice']=function() return {
  name='DualSense Extended Inputs',vidpid='dualsenseextendedv2',muteToggleAxisIndex=10,
  ensure=function() return 7 end,retireLegacy=function()end,releaseProducer=function()end,
  emitAxis=function(_,index,value) axes[#axes+1]={index,value} end,
} end
local observed=nil
package.preload['dualsenseSuite/micMuteVehicleSync']=function() return {
  reset=function()end,invalidate=function()end,update=function()end,
  pulsedButton=function() return true end, notePhysicalPress=function()end,
  accept=function() return observed, true end,
} end
core_input_virtualInput={emit=function(_,kind,index,action,val)
  events[#events+1]={kind,index,action,val}
end}
core_input_bindings={bindings={{devname='vinput7',contents={bindings={}}}}}
socket={udp=function()return {
  settimeout=function()end,setsockname=function()return true end,
  receivefrom=function()return table.remove(packets,1) end,
  sendto=function(_,data)sent[#sent+1]=data return #data end,
  close=function()end,
}end}
function log()end
local M=dofile('BeamNG-Mod/lua/ge/extensions/dualsenseExtendedInputs.lua')
local function eq(a,b,why)assert(a==b,(why or 'assert')..': '..tostring(a)..' ~= '..tostring(b))end
local function drive(mute)
  local b={68,83,69,52,4,mute and 4 or 0,0,0,1,0,0,0}
  for i=13,30 do b[i]=0 end
  packets[#packets+1]=string.char((unpack or table.unpack)(b))
  M.onUpdate(.02,.02,.02)
end
local function led()
  for i=#sent,1,-1 do
    if sent[i]:sub(1,4)=='DMB2' then return sent[i]:byte(7) end
  end
end
local function physicalClick()
  drive(true);drive(false)
end
local function transition(from,to)
  vehicle.id=to
  M.onVehicleSwitched(from,to,0)
  drive(false)
end
M.onFirstUpdate();M.setMicMuteInputMode('toggle')
physicalClick()
eq(M.getMicMuteToggleState(),true,'A enabled');eq(led(),1,'A LED enabled')
local initialAxes=#axes
local initialEvents=#events
M.onVehicleResetted(101);M.onVehicleSpawned(101)
eq(M.getMicMuteToggleState(),true,'Ctrl+R retained');eq(led(),1,'Ctrl+R LED retained')
eq(#axes,initialAxes,'Ctrl+R no axis');eq(#events,initialEvents,'Ctrl+R no input')
M.onUpdate(.65,.65,.65) -- GE reload may pause UDP receive longer than 400ms.
eq(M.getMicMuteToggleState(),true,'brief Ctrl+R packet stall keeps A ON');eq(led(),1,'stall keeps LED ON')
transition(101,102)
eq(M.getMicMuteToggleState(),false,'B initially off');eq(led(),0,'B LED off')
transition(102,101)
eq(M.getMicMuteToggleState(),true,'return A restores ON');eq(led(),1,'return A LED ON')
eq(#axes,initialAxes,'switch is read-only')
transition(101,102)
physicalClick();eq(M.getMicMuteToggleState(),true,'B enabled');eq(led(),1,'B LED enabled')
transition(102,101)
eq(led(),1,'A ON -> B ON: A remains ON')
local statusesBefore=#sent
transition(101,102)
eq(led(),1,'A ON -> B ON: B remains ON')
for i=statusesBefore+1,#sent do
  if sent[i]:sub(1,4)=='DMB2' then eq(sent[i]:byte(7),1,'no intervening OFF status') end
end
-- An observation of a vehicle change made outside Mic Mute persists across switches.
observed=false;M.onMicMuteVehicleState(102,1,'diff',false)
eq(led(),0,'observed B OFF')
transition(102,101);eq(led(),1,'A still ON')
transition(101,102);eq(led(),0,'B observed OFF restored')
-- Explicit replacement can reuse the same numerical vehicle ID; discard old snapshot.
M.onVehicleReplaced(102);eq(led(),0,'replacement fresh neutral')
physicalClick();eq(led(),1,'replacement can enable')
M.onVehicleReplaced(102);eq(led(),0,'replacement must forget OLD state')
-- A destroyed vehicle must not leave an ON snapshot for a future reuse of ID 101.
transition(102,101);eq(led(),1,'A ON before removal')
transition(101,102)
M.onVehicleDestroyed(101)
transition(102,101);eq(led(),0,'reused destroyed ID starts unlatched')
-- Short pauses preserve the toggle, but real stream loss still clears it.
physicalClick();eq(led(),1,'fresh reused vehicle can enable')
M.onUpdate(.65,.65,.65);eq(led(),1,'short input gap retains Mic LED')
M.onUpdate(2.6,2.6,2.6);eq(led(),0,'extended stream disconnect clears Mic LED')
M.onExtensionUnloaded()
print('Mic Mute per-vehicle cache lifecycle: PASS')
