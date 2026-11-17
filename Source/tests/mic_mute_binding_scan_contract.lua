-- A Mic Mute binding scan must not fetch the entire action catalogue when
-- there is no relevant virtual binding (or more than one ambiguous binding).
local root = os.getenv('BEAMNG_MOD_ROOT') or 'BeamNG-Mod'
package.path = root .. '/lua/?.lua;' .. root .. '/lua/?/init.lua;' .. package.path
local function eq(actual, expected, reason)
  assert(actual == expected, reason .. ': got ' .. tostring(actual) .. ', expected ' .. tostring(expected))
end
local actionsRead = 0
core_input_actions = {getActiveActions = function()
  actionsRead = actionsRead + 1
  return {customToggle = {ctx='vlua', onDown='controller.toggleSomething()'}}
end}
local binds = {}
core_input_bindings = {bindings = {{devname='vinput7',contents={bindings=binds}}}}
local vehicle = {getID = function() return 101 end, queueLuaCommand = function() error('unexpected vehicle state probe') end}
getPlayerVehicle = function() return vehicle end
local sync = dofile(root .. '/lua/dualsenseSuite/micMuteVehicleSync.lua')
sync.update(0,7,true)
eq(actionsRead,0,'unbound controller does not need full action catalogue')
binds[1]={control='button2', action='customToggle', player=0}
binds[2]={control='axis10', action='customToggle', player=0}
sync.invalidate(); sync.update(.25,7,true)
eq(actionsRead,0,'ambiguous bindings do not need action catalogue')
eq(sync.pulsedButton(),false,'ambiguous bindings cannot emit a pulse')
binds[2]=nil
binds[1].action=nil
sync.invalidate(); sync.update(.40,7,true)
eq(actionsRead,0,'malformed binding does not need action catalogue')
binds[1].action='customToggle'
sync.invalidate(); sync.update(.50,7,true)
eq(actionsRead,1,'single Mic binding examines action catalogue once')
eq(sync.pulsedButton(),true,'unobservable onDown toggle retains a physical-only pulse')
print('Mic Mute binding scan: PASS')
