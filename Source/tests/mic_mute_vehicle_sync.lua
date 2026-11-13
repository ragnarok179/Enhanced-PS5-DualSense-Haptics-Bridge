-- Read-only Mic Mute state contract, with transient/stale vehicle states.
local root = os.getenv("BEAMNG_MOD_ROOT") or "BeamNG-Mod"
package.path = root .. "/lua/?.lua;" .. root .. "/lua/?/init.lua;" .. package.path
local function eq(a, b, name)
  assert(a == b, (name or "assertion") .. ": got " .. tostring(a) .. " expected " .. tostring(b))
end
local sent = {}
local vehicle = {id=101}
function vehicle:getID() return self.id end
function vehicle:queueLuaCommand(command) sent[#sent+1] = command end
getPlayerVehicle = function() return vehicle end
core_input_actions = {getActiveActions = function() return {
  toggleDiffMode={ctx="vlua", onDown="powertrain.toggleDefaultDiffs()"},
  toggle4WDStatus={ctx="vlua", onDown="controller.toggle4WD()"},
  parkingbrake={ctx="vlua", onChange="input.event('parkingbrake', VALUE, FILTERTYPE)"},
} end}
local bindings = {{control="button2",action="toggleDiffMode",player=0}}
core_input_bindings={bindings={{devname="vinput7",contents={bindings=bindings}}}}
local sync=require("dualsenseSuite/micMuteVehicleSync")
local function lastSeq()
  assert(#sent>0, "no vehicle query")
  return tonumber(sent[#sent]:match(",%s*(%d+)%)"))
end
local function sample(at, value, id)
  local before=#sent
  sync.update(at,7,true)
  assert(#sent>before,"sample: no query at "..at)
  return sync.accept(at+0.01,id or vehicle.id,lastSeq(),"diff",value)
end
sync.update(0,7,true)
eq(sync.pulsedButton(),true,"known onDown action uses pulses")
eq(sample(0.30,false),nil,"initial vehicle state requires a second observation")
eq(sample(0.50,true),nil,"changed startup sample starts new confirmation")
eq(sample(0.70,true),true,"stable new vehicle state confirmed")
eq(sync.active(),true,"confirmed state available")
-- Drive-side changes are confirmed, not guessed from one transient sample.
eq(sample(1.00,false),nil,"external change first observation")
eq(sample(1.20,false),false,"external change confirmed")
-- A physical press updates the latch immediately; replies from before it are stale.
sync.update(1.50,7,true)
local stale=lastSeq()
sync.notePhysicalPress(1.51,true)
eq(sync.accept(1.52,101,stale,"diff",false),nil,"old in-flight response ignored")
eq(sample(1.67,false),nil,"old state inside press guard ignored")
eq(sample(1.87,true),nil,"expected state still requires confirmation")
eq(sample(2.07,true),true,"post-press state confirmed")
-- A rejected action must eventually correct the local LED state.
sync.notePhysicalPress(2.20,false)
eq(sample(2.40,true),nil,"transient previous ON state ignored")
eq(sample(2.95,true),true,"still-confirmed old state reported after rejection")
-- Switched vehicle (new ID) discards old responses and initial observations.
vehicle.id=202
sync.update(3.20,7,true)
eq(sync.active(),false,"switch invalidates confirmation")
eq(sync.accept(3.21,101,stale,"diff",true),nil,"old vehicle response ignored")
eq(sample(3.50,false),nil,"new vehicle first sample")
eq(sample(3.70,false),false,"new vehicle confirmed")
-- Ctrl+R / replace can reuse the same ID, but invalidation rejects old sequence.
sync.update(3.95,7,true)
local oldSeq=lastSeq()
sync.invalidate()
eq(sync.accept(3.96,202,oldSeq,"diff",true),nil,"old-generation reply ignored")
sync.update(4.00,7,true)
eq(sample(4.30,true),nil,"same-ID reset first observation")
eq(sample(4.50,true),true,"same-ID reset confirmed")
-- Unknown reader must not be treated as false. Preserve manual latch and pulse.
sync.invalidate()
sync.update(5.00,7,true)
eq(sample(5.30,nil),nil,"unknown reader")
eq(sync.pulsedButton(),true,"recognized toggle still uses physical pulses")
eq(sync.active(),false,"unknown reader has no confirmed state")
-- Multiple bindings are ambiguous; unsafe parkingbrake axis is never claimed.
bindings[2]={control="button2",action="toggle4WDStatus",player=0}
sync.invalidate();sync.update(5.50,7,true)
eq(sync.pulsedButton(),false,"multiple actions not synchronized")
eq(sync.waitingForState(),false,"multiple actions use manual toggle")
core_input_bindings.bindings[1].contents.bindings={{control="axis10",action="parkingbrake",player=0}}
sync.invalidate();sync.update(5.70,7,true)
eq(sync.waitingForState(),false,"non-idempotent axis not synchronized")
-- Vehicle reader queries only physical state; never operates the vehicle.
local emitted={}
obj={getID=function()return 202 end,queueGameEngineLua=function(_,code)emitted[#emitted+1]=code end}
electrics={values={mode4WD=1,modeRangeBox=0,fog=true}}
controller={getControllersByType=function()return {{}} end,getController=function()return nil end}
local diffs={{defaultToggle=true,mode="locked"}}
powertrain={getDevicesByType=function()return diffs end}
local reader=dofile(root.."/lua/vehicle/extensions/auto/dualsenseMicMuteVehicleState.lua")
reader.query("diff",6);assert(emitted[#emitted]:find("true%)"),"locked diff")
diffs[1].mode="lsd"
reader.query("diff",7);assert(emitted[#emitted]:find("nil%)"),"ambiguous diff")
reader.query("4wd",8);assert(emitted[#emitted]:find("true%)"),"4wd")
reader.query("range",9);assert(emitted[#emitted]:find("false%)"),"range")
local n=#emitted
reader.query("unsupported",10);eq(#emitted,n,"unknown reader must not execute")
print("Mic Mute vehicle state contract: PASS")
