local M = {}

local VINPUT = require("dualsenseSuite/virtualInputDevice")
local SETTINGS_STORE = require("dualsenseSuite/settingsStore")
local MOTION_BINDING_MIGRATION_KEY = "dphMotionBindingMigrationStoreV1"
local initialized = false

local logTag = "DualSenseMotionInputs"
local listenHost = "127.0.0.1"
local listenPort = 6979
local staleTimeout = 0.40
local maxPacketsPerUpdate = 96

-- Keep the previous sensor+ vidpid stable so existing user .diff bindings remain valid.
-- stable sensor keeps the public Sensors identity and its stable 6-axis/6-button descriptor.
local deviceName = VINPUT.name
local deviceVidPid = VINPUT.vidpid
local axisCount = 6
local buttonCount = 6
local povCount = 0
local axisControls = {"ryaxis","rzaxis","slider","slider2","axis8","axis9"}

local legacyDeviceVidPids = {"dualsensesensorsv1","dualsensemappedmotionv1","dualsensemotionv2","dualsenseorientationv2"}
local settingsPrefixMotion = "dseSensorMotionV4_"
local legacySettingsPrefixMotion = "dseSensorMotionV3_"
local settingsPrefixAccel = "dseSensorAccelerationV3_"
local legacySettingsPrefixAccel = "dseSensorAccelerationV2_"
local olderSettingsPrefixAccel = "dseSensorAccelerationV1_"

local udp = nil
local statusUdp = nil
local deviceInstance = nil
local elapsed = 0
local lastPacketAt = -999
local staleResetDone = true
local streamSeen = false
local lastSensorTs = nil
local packetDt = 1/125
local lastRejectedPacketAt = -999
local rejectedPacketCount = 0
local lastMeaningfulMotionActivityAt = -999

local mappedAxisRaw = {0.5,0.5,0.5,0.5,0.5,0.5}
local mappedButtonRaw = {0,0,0,0,0,0}
local bindingMuteUntil = -999
local lastBindingChangeHandledAt = -999
local wrappedResetBindings = nil
local wrappedSaveBindings = nil
local originalResetBindings = nil
local originalSaveBindings = nil

local sensor = {
  protocol=0, transport="unknown", calibrated=false, timestamp=0,
  gyroX=0, gyroY=0, gyroZ=0,
  accelX=0, accelY=0, accelZ=0,
  linearX=0, linearY=0, linearZ=0,
  roll=0, pitch=0, yaw=0,
  qw=1,qx=0,qy=0,qz=0,
  gravityReady=false, gravityX=0, gravityY=0, gravityZ=0,
}

local calibrations = {}
local motionSlots = {}
local accelSlots = {}

local function clamp(v,lo,hi)
  v=tonumber(v) or 0
  if v<lo then return lo end
  if v>hi then return hi end
  return v
end

local function boolValue(v)
  if v==true or v==1 or v=="1" then return true end
  if type(v)=="string" and string.lower(v)=="true" then return true end
  return false
end

local function splitPlain(s,sep)
  local out={}
  s=tostring(s or "")
  local start=1
  while true do
    local a,b=string.find(s,sep,start,true)
    if not a then out[#out+1]=string.sub(s,start); break end
    out[#out+1]=string.sub(s,start,a-1); start=b+1
  end
  return out
end

local function cleanText(v)
  local s=tostring(v or "")
  s=s:gsub("[\r\n\t~|]"," ")
  if #s>120 then s=s:sub(1,120) end
  return s
end

local function vi()
  return core_input_virtualInput or (extensions and extensions.core_input_virtualInput)
end

local function sendMotionDebug(eventName,details)
  if not statusUdp and socket and socket.udp then
    local ok,s=pcall(socket.udp)
    if ok and s then statusUdp=s; pcall(function() statusUdp:settimeout(0) end) end
  end
  if not statusUdp then return end
  pcall(function()
    statusUdp:sendto("DSMM1\t"..cleanText(eventName).."\t"..cleanText(details),"127.0.0.1",6978)
  end)
end

local function ensureDevice()
  local id = VINPUT.ensure(logTag, "motion")
  if id == nil then return nil end
  deviceInstance = id
  return id
end

local function readBindingDiff(path)
  if not FS or type(jsonReadFile)~="function" then return nil end
  local okExists,exists=pcall(function() return FS:fileExists(path) end)
  if not okExists or not exists then return nil end
  local ok,data=pcall(jsonReadFile,path)
  if not ok or type(data)~="table" or type(data.bindings)~="table" then return nil end
  return data
end

local function legacyMappedSlotMode(index)
  local keys={"dualsenseMappedMotionSlotV1_"..tostring(index),"dseMappedMotionSlotV1_"..tostring(index)}
  for _,key in ipairs(keys) do
    local v=SETTINGS_STORE.get(key,nil)
    if type(v)=="string" and v~="" then
      local p=splitPlain(v,"|")
      return p[1],p[2]
    end
  end
  return nil,nil
end

local sensorAxisTarget = {
  xaxis="ryaxis", yaxis="rzaxis", zaxis="slider",
  rxaxis="slider2", ryaxis="axis8", rzaxis="axis9",
}

local function standaloneToSharedControl(control)
  control=tostring(control or "")
  if sensorAxisTarget[control] then return sensorAxisTarget[control] end
  local n=tonumber(control:match("^button(%d+)$"))
  if n and n>=0 and n<=5 then return "button"..tostring(VINPUT.sensorButtonBase+n) end
  return nil
end

local function safeLegacyControlMap(sourceVidPid,control)
  control=tostring(control or "")
  if sourceVidPid=="dualsensesensorsv1" then
    return standaloneToSharedControl(control)
  end
  if sourceVidPid=="dualsensemappedmotionv1" then
    local slotByControl={xaxis=1,yaxis=2,zaxis=3,rxaxis=4,ryaxis=5,rzaxis=6}
    local oldSlot=slotByControl[control]
    if not oldSlot then return nil end
    local mode,axis=legacyMappedSlotMode(oldSlot)
    local standalone=nil
    if mode=="acceleration" then
      if axis=="x" then standalone="rxaxis"
      elseif axis=="y" then standalone="ryaxis"
      elseif axis=="z" then standalone="rzaxis"
      else
        local fallback={ [1]="rxaxis",[2]="ryaxis",[3]="rzaxis",[4]="rxaxis",[5]="ryaxis",[6]="rzaxis" }
        standalone=fallback[oldSlot]
      end
    elseif oldSlot<=3 then
      standalone=control
    end
    return standaloneToSharedControl(standalone)
  end
  if sourceVidPid=="dualsensemotionv2" then
    local map={xaxis="rxaxis",yaxis="ryaxis",zaxis="rzaxis",rxaxis="xaxis",ryaxis="yaxis",rzaxis="zaxis"}
    return standaloneToSharedControl(map[control])
  end
  if sourceVidPid=="dualsenseorientationv2" then
    local map={xaxis="xaxis",yaxis="yaxis",zaxis="zaxis"}
    return standaloneToSharedControl(map[control])
  end
  return nil
end

local function migrateBindingsInDir(dir)
  local targetPath=dir.."/"..deviceVidPid..".diff"
  local existing=readBindingDiff(targetPath)

  local order={"dualsensesensorsv1","dualsensemappedmotionv1","dualsenseorientationv2","dualsensemotionv2"}
  local migrated,seen={},{}
  if existing and type(existing.bindings)=="table" then
    for _,b in ipairs(existing.bindings) do
      if type(b)=="table" then
        local copy=deepcopy(b)
        migrated[#migrated+1]=copy
        seen[tostring(copy.control or "").."\0"..tostring(copy.action or "")]=true
      end
    end
  end
  local existingCount=#migrated
  local targetControlClaimed={}
  if existing and type(existing.bindings)=="table" then
    for _,b in ipairs(existing.bindings) do
      if type(b)=="table" and b.control then targetControlClaimed[tostring(b.control)]="existing" end
    end
  end
  for _,sourceVidPid in ipairs(order) do
    local data=readBindingDiff(dir.."/"..sourceVidPid..".diff")
    if data then
      for _,b in ipairs(data.bindings) do
        if type(b)=="table" then
          local mapped=safeLegacyControlMap(sourceVidPid,b.control)
          local action=tostring(b.action or "")
          if mapped and mapped~="" and action~="" then
            local key=mapped.."\0"..action
            -- The first source in priority order owns a target control. Keep
            -- multiple actions from that same source/control, but do not mix
            -- orientation and gyro semantics onto the same migrated target axis.
            local owner=targetControlClaimed[mapped]
            if owner==nil or owner==sourceVidPid then
              targetControlClaimed[mapped]=sourceVidPid
              if not seen[key] then
                local copy=deepcopy(b); copy.control=mapped
                migrated[#migrated+1]=copy; seen[key]=true
              end
            end
          end
        end
      end
    end
  end
  if #migrated==0 then return 0,"none" end
  if #migrated==existingCount then return 0,existingCount>0 and "existing" or "none" end
  table.sort(migrated,function(a,b)
    if tostring(a.control)==tostring(b.control) then return tostring(a.action)<tostring(b.action) end
    return tostring(a.control)<tostring(b.control)
  end)
  local out={version=1,name=deviceName,devicetype="vinput",vidpid=deviceVidPid,bindings=migrated}
  local ok=jsonWriteFile(targetPath,out,true)
  if not ok then return 0,"write_failed" end
  local added=#migrated-existingCount
  return added,"migrated"
end

local function migrateLegacyBindings()
  if not FS or type(jsonReadFile)~="function" or type(jsonWriteFile)~="function" then return end
  local dirs={"settings/inputmaps"}
  local entries=FS:findFiles("settings/inputmaps","*",-1,false,true) or {}
  for _,entry in ipairs(entries) do
    local stat=FS:stat(entry)
    if stat and stat.filetype=="dir" then dirs[#dirs+1]=entry end
  end
  local total=0
  for _,dir in ipairs(dirs) do local n=migrateBindingsInDir(dir); total=total+(tonumber(n) or 0) end
  sendMotionDebug("MIGRATION",string.format("bindings=%d target=%s",total,deviceVidPid))
end

local function retireLegacyDevices()
  VINPUT.retireLegacy()
  local v=vi(); if not v then return end
  for _,vidpid in ipairs(legacyDeviceVidPids) do
    local id=v.getDeviceInfo(vidpid)
    if id~=nil then pcall(v.deleteDevice,id) end
  end
end

local function emitAxis(index,signed)
  local v=vi(); if not v or deviceInstance==nil then return end
  signed=clamp(signed,-1,1)
  local raw=(signed+1)*0.5
  local slot=index+1
  if math.abs((mappedAxisRaw[slot] or 0.5)-raw)<0.00035 then return end
  mappedAxisRaw[slot]=raw
  VINPUT.emitAxis(deviceInstance,VINPUT.sensorAxisBase+index,raw)
end

local function emitButton(index,value)
  local v=vi(); if not v or deviceInstance==nil then return end
  value=value and 1 or 0
  local slot=index+1
  if (mappedButtonRaw[slot] or 0)==value then return end
  mappedButtonRaw[slot]=value
  VINPUT.emitButton(deviceInstance,VINPUT.sensorButtonBase+index,value)
  if index<=2 then
    sendMotionDebug("ACCEL_BUTTON",string.format("axis=%s state=%d",({"X","Y","Z"})[slot] or tostring(index),value))
  else
    sendMotionDebug("MOTION_BUTTON",string.format("slot=%d state=%d",index-2,value))
  end
end

local function neutralAllOutputs()
  for i=0,5 do emitAxis(i,0) end
  for i=0,5 do emitButton(i,false) end
end

local function u16le(data,index)
  local lo,hi=data:byte(index,index+1); if not lo or not hi then return 0 end
  return lo+hi*256
end
local function i16le(data,index)
  local v=u16le(data,index); if v>=32768 then v=v-65536 end; return v
end
local function u32le(data,index)
  local b1,b2,b3,b4=data:byte(index,index+3); if not b4 then return 0 end
  return b1+b2*256+b3*65536+b4*16777216
end
local function signedUnit(q)
  q=clamp(q,-32767,32767); return q/32767
end

local function quatNormalize(w,x,y,z)
  local n=math.sqrt(w*w+x*x+y*y+z*z)
  if n<1e-9 then return 1,0,0,0 end
  return w/n,x/n,y/n,z/n
end

local function quatFromEuler(roll,pitch,yaw)
  -- Bridge orientation is exposed as Roll/Pitch/Yaw degrees. Keep this
  -- conversion internally consistent; auto calibration learns the user's real
  -- hand axis, so it does not rely on a guessed steering convention.
  local r=math.rad(roll or 0)*0.5
  local p=math.rad(pitch or 0)*0.5
  local y=math.rad(yaw or 0)*0.5
  local cr,sr=math.cos(r),math.sin(r)
  local cp,sp=math.cos(p),math.sin(p)
  local cy,sy=math.cos(y),math.sin(y)
  return quatNormalize(
    cr*cp*cy + sr*sp*sy,
    sr*cp*cy - cr*sp*sy,
    cr*sp*cy + sr*cp*sy,
    cr*cp*sy - sr*sp*cy
  )
end

local function quatConj(w,x,y,z) return w,-x,-y,-z end
local function quatMul(aw,ax,ay,az,bw,bx,by,bz)
  return aw*bw-ax*bx-ay*by-az*bz,
         aw*bx+ax*bw+ay*bz-az*by,
         aw*by-ax*bz+ay*bw+az*bx,
         aw*bz+ax*by-ay*bx+az*bw
end

local function quatDelta(nw,nx,ny,nz,cw,cx,cy,cz)
  local iw,ix,iy,iz=quatConj(nw,nx,ny,nz)
  local w,x,y,z=quatMul(iw,ix,iy,iz,cw,cx,cy,cz)
  w,x,y,z=quatNormalize(w,x,y,z)
  if w<0 then w,x,y,z=-w,-x,-y,-z end
  return w,x,y,z
end

local function quatToRotVec(w,x,y,z)
  w=clamp(w,-1,1)
  local s=math.sqrt(math.max(0,1-w*w))
  local angle=2*math.acos(w)
  if angle>math.pi then angle=angle-2*math.pi end
  if s<1e-7 then return x*2,y*2,z*2 end
  local k=angle/s
  return x*k,y*k,z*k
end

local function rotateVector(qw,qx,qy,qz,vx,vy,vz)
  local rw,rx,ry,rz=quatMul(qw,qx,qy,qz,0,vx,vy,vz)
  local cw,cx,cy,cz=quatConj(qw,qx,qy,qz)
  local _,ox,oy,oz=quatMul(rw,rx,ry,rz,cw,cx,cy,cz)
  return ox,oy,oz
end

local function angleDiffDeg(a,b)
  local d=(tonumber(a) or 0)-(tonumber(b) or 0)
  while d>180 do d=d-360 end
  while d<-180 do d=d+360 end
  return d
end

local function newMotionSlot(index)
  return {
    index=index, mode="off", axis="auto", invert=false,
    deadzone=0.02, stability=8, range=180, reference="adaptive",
    customX=0,customY=0,customZ=1,
    learned=false,learnedX=0,learnedY=0,learnedZ=1,learnQuality=0,
    outputMode="axis",buttonDirection="positive",buttonThresholdPercent=50,buttonBehavior="hold",
    buttonHigh=false,buttonArmed=true,buttonPulseUntil=-999,
    neutralQw=nil,neutralQx=nil,neutralQy=nil,neutralQz=nil,
    neutralRoll=nil,neutralPitch=nil,neutralYaw=nil,
    neutralTiltLR=nil,neutralTiltFB=nil,
    filtered=0,output=0,
  }
end

local function normalizeMotion(s)
  local validMode={off=true,rotation=true,gyro=true,tilt=true,orientation=true}
  if not validMode[s.mode] then s.mode="off" end
  local validAxis={auto=true,roll=true,pitch=true,yaw=true,horizontal=true,vertical=true,left_right=true,forward_back=true,custom=true,x=true,y=true,z=true}
  if not validAxis[s.axis] then s.axis="auto" end
  s.invert=boolValue(s.invert)
  s.deadzone=clamp(s.deadzone,0,0.30)
  s.stability=clamp(s.stability,0,100)
  local maxRange=(s.mode=="gyro") and 2048 or 180
  s.range=clamp(tonumber(s.range) or 180,1,maxRange)
  if s.reference~="adaptive" and s.reference~="controller" and s.reference~="world" then s.reference="adaptive" end
  s.customX=clamp(s.customX,-1,1); s.customY=clamp(s.customY,-1,1); s.customZ=clamp(s.customZ,-1,1)
  s.learned=boolValue(s.learned)
  s.learnedX=tonumber(s.learnedX) or 0; s.learnedY=tonumber(s.learnedY) or 0; s.learnedZ=tonumber(s.learnedZ) or 1
  s.learnQuality=clamp(s.learnQuality,0,1)
  if s.outputMode~="button" then s.outputMode="axis" end
  if s.buttonDirection~="negative" and s.buttonDirection~="any" then s.buttonDirection="positive" end
  s.buttonThresholdPercent=clamp(s.buttonThresholdPercent,1,100)
  if s.buttonBehavior~="pulse" then s.buttonBehavior="hold" end
  return s
end

local function defaultRangeForMode(mode)
  if mode=="gyro" then return 180 end
  if mode=="tilt" then return 45 end
  if mode=="orientation" then return 90 end
  if mode=="rotation" then return 180 end
  return 90
end

local function newAccelSlot(index)
  return {
    index=index, mode="off", invert=false,
    -- Public percentages refer directly to the DualSense accelerometer's
    -- physical +/-4 g per-axis range. Fresh defaults deliberately expose the
    -- complete range: Minimum 10% (=0.40 g), Full 100% (=4.00 g).
    minPercent=10,maxPercent=100,
    -- Button Activation is also an absolute percentage of +/-4 g, not a
    -- percentage between Minimum and Full.
    thresholdPercent=10,direction="any",behavior="pulse",ignoreRebound=true,
    output=0,filtered=0,buttonHigh=false,pulseUntil=-999,armed=true,
    reboundSign=0,reboundUntil=-999,reboundPeak=0,reboundLogged=false,
    oppositeSince=-999,oppositeStart=0,oppositePeak=0,
    buttonGestureSign=0,buttonGesturePhase="idle",buttonGestureLastAt=-999,buttonGestureNeutralAt=-999,
  }
end

local function normalizeAccel(s)
  if s.mode~="axis" and s.mode~="button" then s.mode="off" end
  s.invert=boolValue(s.invert)
  s.minPercent=math.floor(clamp(tonumber(s.minPercent) or 10,0,99)+0.5)
  s.maxPercent=math.floor(clamp(tonumber(s.maxPercent) or 100,1,100)+0.5)
  if s.maxPercent<=s.minPercent then s.maxPercent=math.min(100,s.minPercent+1) end
  s.thresholdPercent=math.floor(clamp(tonumber(s.thresholdPercent) or 10,1,100)+0.5)
  if s.direction~="positive" and s.direction~="negative" then s.direction="any" end
  if s.behavior~="hold" then s.behavior="pulse" end
  s.ignoreRebound=s.ignoreRebound==nil and true or boolValue(s.ignoreRebound)
  return s
end

local function motionSettingKey(i) return settingsPrefixMotion..tostring(i) end
local function accelSettingKey(i) return settingsPrefixAccel..tostring(i) end

local function serializeMotion(s)
  return table.concat({s.mode,s.axis,s.invert and "1" or "0",string.format("%.6f",s.deadzone),string.format("%.3f",s.stability),string.format("%.6f",s.range),s.reference,string.format("%.6f",s.customX),string.format("%.6f",s.customY),string.format("%.6f",s.customZ),s.learned and "1" or "0",string.format("%.8f",s.learnedX),string.format("%.8f",s.learnedY),string.format("%.8f",s.learnedZ),string.format("%.5f",s.learnQuality),s.outputMode,s.buttonDirection,string.format("%.3f",s.buttonThresholdPercent),s.buttonBehavior},"|")
end

local function deserializeMotion(index,text)
  local s=newMotionSlot(index)
  if type(text)~="string" or text=="" then return s end
  local p=splitPlain(text,"|")
  s.mode=p[1] or s.mode; s.axis=p[2] or s.axis; s.invert=p[3]
  s.deadzone=tonumber(p[4]) or s.deadzone; s.stability=tonumber(p[5]) or s.stability; s.range=tonumber(p[6]) or s.range
  s.reference=p[7] or s.reference; s.customX=tonumber(p[8]) or 0; s.customY=tonumber(p[9]) or 0; s.customZ=tonumber(p[10]) or 1
  s.learned=p[11]; s.learnedX=tonumber(p[12]) or 0; s.learnedY=tonumber(p[13]) or 0; s.learnedZ=tonumber(p[14]) or 1; s.learnQuality=tonumber(p[15]) or 0
  s.outputMode=p[16] or s.outputMode; s.buttonDirection=p[17] or s.buttonDirection; s.buttonThresholdPercent=tonumber(p[18]) or s.buttonThresholdPercent; s.buttonBehavior=p[19] or s.buttonBehavior
  return normalizeMotion(s)
end

local function serializeAccel(s)
  -- Field 9 marks the current sensor protocol semantics: all percentages are integer percentages
  -- of the physical +/-4 g sensor range and thresholdPercent is absolute.
  return table.concat({s.mode,s.invert and "1" or "0",string.format("%.0f",s.minPercent),string.format("%.0f",s.maxPercent),string.format("%.0f",s.thresholdPercent),s.direction,s.behavior,s.ignoreRebound and "1" or "0","2"},"|")
end

local function deserializeAccel(index,text,legacyOneGScale)
  local s=newAccelSlot(index)
  if type(text)~="string" or text=="" then return s end
  local p=splitPlain(text,"|")
  s.mode=p[1] or s.mode; s.invert=p[2]
  local oldMin=tonumber(p[3]); local oldMax=tonumber(p[4]); local oldThreshold=tonumber(p[5])
  if p[9]=="2" then
    s.minPercent=oldMin or s.minPercent; s.maxPercent=oldMax or s.maxPercent; s.thresholdPercent=oldThreshold or s.thresholdPercent
  else
    -- previous implementation-47 stored Trigger as a percentage *between* Minimum and Full.
    -- Convert it once to an absolute percentage of the physical +/-4 g range.
    local minPct=oldMin or s.minPercent; local maxPct=oldMax or s.maxPercent; local relative=clamp(oldThreshold or 50,0,100)
    if legacyOneGScale then minPct=minPct/4; maxPct=maxPct/4 end
    s.minPercent=minPct; s.maxPercent=maxPct
    s.thresholdPercent=minPct+(maxPct-minPct)*(relative/100)
  end
  s.direction=p[6] or s.direction; s.behavior=p[7] or s.behavior
  if p[8]~=nil and p[8]~="" then s.ignoreRebound=boolValue(p[8]) end
  return normalizeAccel(s)
end

local function persistMotion(s)
  SETTINGS_STORE.set(motionSettingKey(s.index),serializeMotion(s))
end
local function persistAccel(s)
  SETTINGS_STORE.set(accelSettingKey(s.index),serializeAccel(s))
end

local function readSetting(key)
  return SETTINGS_STORE.get(key,nil)
end

local function readLegacyMappedSlot(index)
  for _,prefix in ipairs({"dualsenseMappedMotionSlotV1_","dseMappedMotionSlotV1_"}) do
    local raw=readSetting(prefix..tostring(index))
    if type(raw)=="string" and raw~="" then return splitPlain(raw,"|") end
  end
  return nil
end

local function migrateLegacySlotIntoMotion(index,p)
  if not p then return nil end
  local mode=p[1] or "off"
  if mode=="acceleration" then return nil end
  local s=newMotionSlot(index)
  if mode=="rotation" or mode=="gyro" or mode=="tilt" or mode=="orientation" then s.mode=mode else s.mode="off" end
  s.axis=p[2] or s.axis; s.invert=boolValue(p[3])
  local oldSensitivity=clamp(tonumber(p[4]) or 1,0.1,4)
  s.deadzone=tonumber(p[5]) or s.deadzone; s.stability=tonumber(p[6]) or s.stability; s.range=tonumber(p[7]) or defaultRangeForMode(s.mode)
  if math.abs(oldSensitivity-1)>0.0001 then s.range=s.range/oldSensitivity end
  local ref=p[8] or "adaptive"; if ref=="player" then ref="adaptive" elseif ref=="local" then ref="controller" end; s.reference=ref
  s.customX=tonumber(p[10]) or 0; s.customY=tonumber(p[11]) or 0; s.customZ=tonumber(p[12]) or 1
  s.learned=boolValue(p[13]); s.learnedX=tonumber(p[14]) or 0; s.learnedY=tonumber(p[15]) or 0; s.learnedZ=tonumber(p[16]) or 1; s.learnQuality=tonumber(p[17]) or 0
  return normalizeMotion(s)
end

local function legacyAccelerationTarget(p,fallback)
  if not p or p[1]~="acceleration" then return nil end
  if p[2]=="x" then return 1 end
  if p[2]=="y" then return 2 end
  if p[2]=="z" then return 3 end
  return fallback
end

local function loadSettings()
  local motionPresent,accelPresent={},{ }
  for i=1,3 do
    local raw=readSetting(motionSettingKey(i))
    local migratedLegacy=false
    if type(raw)~="string" or raw=="" then
      raw=readSetting(legacySettingsPrefixMotion..tostring(i))
      migratedLegacy=type(raw)=="string" and raw~=""
    end
    motionPresent[i]=type(raw)=="string" and raw~=""
    motionSlots[i]=deserializeMotion(i,raw)
    -- previous implementation displayed 90 degrees while its projected rotation representation
    -- could require roughly 180 physical degrees. previous implementation uses a true
    -- swing/twist angle, so migrate the old scale to physical degrees/side.
    if migratedLegacy and motionSlots[i].mode=="rotation" then
      motionSlots[i].range=clamp(motionSlots[i].range*2,1,180)
    end
    if motionPresent[i] then persistMotion(motionSlots[i]) end
  end
  for i=1,3 do
    local raw=readSetting(accelSettingKey(i))
    local migratedOneGScale=false
    if type(raw)~="string" or raw=="" then
      raw=readSetting(legacySettingsPrefixAccel..tostring(i))
      migratedOneGScale=type(raw)=="string" and raw~=""
    end
    if type(raw)~="string" or raw=="" then
      raw=readSetting(olderSettingsPrefixAccel..tostring(i))
      migratedOneGScale=type(raw)=="string" and raw~=""
    end
    accelPresent[i]=type(raw)=="string" and raw~=""
    accelSlots[i]=deserializeAccel(i,raw,migratedOneGScale)
    if accelPresent[i] then persistAccel(accelSlots[i]) end
  end

  -- One-time previous sensor-42 settings migration. It is intentionally conservative:
  -- rotational slots 1-3 remain Motion 1-3; legacy acceleration slots are
  -- moved to the matching X/Y/Z acceleration channel. Source settings remain.
  local legacy={}
  for i=1,6 do legacy[i]=readLegacyMappedSlot(i) end
  for i=1,3 do
    if not motionPresent[i] then
      local migrated=migrateLegacySlotIntoMotion(i,legacy[i])
      if migrated then motionSlots[i]=migrated; persistMotion(migrated) end
    end
  end
  for oldIndex=1,6 do
    local target=legacyAccelerationTarget(legacy[oldIndex],oldIndex<=3 and oldIndex or oldIndex-3)
    if target and target>=1 and target<=3 and not accelPresent[target] then
      local a=newAccelSlot(target); a.mode="axis"
      local lp=legacy[oldIndex]
      a.invert=boolValue(lp[3]); accelSlots[target]=normalizeAccel(a); persistAccel(accelSlots[target]); accelPresent[target]=true
    end
  end
end

local function captureNeutral(s)
  s.neutralQw,s.neutralQx,s.neutralQy,s.neutralQz=sensor.qw,sensor.qx,sensor.qy,sensor.qz
  s.neutralRoll,s.neutralPitch,s.neutralYaw=sensor.roll,sensor.pitch,sensor.yaw
  -- Tilt uses the gravity estimate expressed in controller axes.
  local gx,gy,gz=sensor.gravityX,sensor.gravityY,sensor.gravityZ
  s.neutralTiltLR=math.deg(math.atan2(gx,math.sqrt(gy*gy+gz*gz)))
  s.neutralTiltFB=math.deg(math.atan2(gz,math.sqrt(gx*gx+gy*gy)))
  s.filtered=0; s.output=0
end

local function ensureNeutral(s)
  if s.neutralQw==nil then captureNeutral(s) end
end

local function shapeMotion(s,value,dt)
  value=tonumber(value) or 0
  local dz=(s.mode=="gyro") and 0 or s.deadzone
  local a=math.abs(value)
  if a<=dz then value=0 elseif dz<0.999 then value=(value<0 and -1 or 1)*((a-dz)/(1-dz)) end
  value=clamp(value,-1,1)
  if s.invert then value=-value end

  local stability=s.stability/100
  if stability<=0 or dt<=0 then
    s.filtered=value
  else
    local previous=tonumber(s.filtered) or 0
    local threshold=0.04+0.26*stability
    local activity=math.max(math.abs(value),math.abs(previous))
    local gate=clamp(1-(activity/math.max(0.001,threshold)),0,1)
    if gate<=0.001 then s.filtered=value
    else
      local tau=(0.012+0.09*stability)*gate
      local alpha=1-math.exp(-dt/math.max(0.001,tau))
      s.filtered=previous+alpha*(value-previous)
    end
  end
  s.output=clamp(s.filtered,-1,1)
  return s.output
end

local function controllerDeltaQuat(s)
  ensureNeutral(s)
  local w,x,y,z=quatDelta(s.neutralQw,s.neutralQx,s.neutralQy,s.neutralQz,sensor.qw,sensor.qx,sensor.qy,sensor.qz)
  if sensor.protocol==2 then
    -- Bridge v2 filter frame: filter(X,Y,Z)=controller(X,-Z,Y).
    -- Re-express the quaternion vector part in controller axes.
    return w,x,z,-y
  end
  -- v1 fallback quaternion is conventional Euler (X=Roll,Y=Pitch,Z=Yaw).
  return w,y,z,x
end

local function relativeRotVec(s)
  local w,x,y,z=controllerDeltaQuat(s)
  return quatToRotVec(w,x,y,z)
end

local function normalizedMotionAxis(s)
  local ax,ay,az=1,0,0
  if s.axis=="auto" then
    if not s.learned then return nil end
    ax,ay,az=s.learnedX,s.learnedY,s.learnedZ
  elseif s.axis=="custom" then
    ax,ay,az=s.customX,s.customY,s.customZ
  elseif s.axis=="yaw" or s.axis=="y" then ax,ay,az=0,1,0
  elseif s.axis=="roll" or s.axis=="z" then ax,ay,az=0,0,1
  else ax,ay,az=1,0,0 end
  local n=math.sqrt(ax*ax+ay*ay+az*az)
  if n<1e-7 then return nil end
  return ax/n,ay/n,az/n
end

local function twistAngleFromQuat(w,x,y,z,ax,ay,az)
  local projection=x*ax+y*ay+z*az
  local n=math.sqrt(w*w+projection*projection)
  if n<1e-8 then return 0 end
  local tw=w/n
  local tp=projection/n
  local angle=2*math.atan2(tp,tw)
  while angle>math.pi do angle=angle-2*math.pi end
  while angle<-math.pi do angle=angle+2*math.pi end
  return angle
end

local function processRotation(s)
  local ax,ay,az=normalizedMotionAxis(s)
  if not ax then return 0 end
  local w,x,y,z=controllerDeltaQuat(s)
  -- Swing/twist decomposition isolates rotation around the chosen physical
  -- axis. This restores the steering-style calibration behavior and makes
  -- Range a real per-side angle instead of a projected 3D rotation vector.
  local value=twistAngleFromQuat(w,x,y,z,ax,ay,az)
  return math.deg(value)/math.max(0.1,s.range)
end

local function processGyro(s)
  local gx,gy,gz=sensor.gyroX,sensor.gyroY,sensor.gyroZ
  local v=0
  if s.axis=="pitch" or s.axis=="x" then v=gx
  elseif s.axis=="yaw" or s.axis=="y" then v=gy
  elseif s.axis=="roll" or s.axis=="z" then v=gz
  else
    -- Reference only affects the general Horizontal/Vertical abstractions.
    -- Explicit Pitch/Yaw/Roll always means the controller's physical gyro axis.
    if s.reference=="world" then
      gx,gy,gz=rotateVector(sensor.qw,sensor.qx,sensor.qy,sensor.qz,gx,gy,gz)
    elseif s.reference=="adaptive" then
      -- Remove controller roll/pitch coupling while deliberately ignoring world
      -- yaw. This behaves like a player-space frame rather than a fixed world axis.
      local aw,ax,ay,az=quatFromEuler(sensor.roll,sensor.pitch,0)
      gx,gy,gz=rotateVector(aw,ax,ay,az,gx,gy,gz)
    end
    if s.axis=="vertical" then v=gx else v=gy end
  end
  return v/math.max(0.1,s.range)
end

local function processTilt(s)
  ensureNeutral(s)
  local gx,gy,gz=sensor.gravityX,sensor.gravityY,sensor.gravityZ
  local current
  if s.axis=="forward_back" or s.axis=="pitch" then
    current=math.deg(math.atan2(gz,math.sqrt(gx*gx+gy*gy)))
    return angleDiffDeg(current,s.neutralTiltFB)/math.max(0.1,s.range)
  end
  current=math.deg(math.atan2(gx,math.sqrt(gy*gy+gz*gz)))
  return angleDiffDeg(current,s.neutralTiltLR)/math.max(0.1,s.range)
end

local function processOrientation(s)
  ensureNeutral(s)
  local v=0
  if s.axis=="pitch" then v=angleDiffDeg(sensor.pitch,s.neutralPitch)
  elseif s.axis=="yaw" then v=angleDiffDeg(sensor.yaw,s.neutralYaw)
  else v=angleDiffDeg(sensor.roll,s.neutralRoll) end
  return v/math.max(0.1,s.range)
end

local function rawMotionValue(s)
  if calibrations[s.index] or s.mode=="off" then return 0 end
  if s.mode=="rotation" then return processRotation(s) end
  if s.mode=="gyro" then return processGyro(s) end
  if s.mode=="tilt" then return processTilt(s) end
  if s.mode=="orientation" then return processOrientation(s) end
  return 0
end

local function accelerationComponent(i)
  if i==1 then return sensor.linearX end
  if i==2 then return sensor.linearY end
  return sensor.linearZ
end

local function accelBounds(s)
  -- DualSense accelerometer hardware range is +/-4 g. Public percentages map
  -- to that physical range: 25%=1 g and 100%=4 g.
  return (s.minPercent/100)*4,(s.maxPercent/100)*4
end

local function accelAxisValue(s,v)
  if s.invert then v=-v end
  local minG,maxG=accelBounds(s)
  local a=math.abs(v)
  if a<=minG then return 0 end
  local out=(a-minG)/math.max(0.001,maxG-minG)
  return clamp((v<0 and -1 or 1)*out,-1,1)
end

local function accelButtonMagnitude(s,v)
  -- Button direction is explicit (+/-/Any). Axis inversion is preserved but
  -- intentionally ignored here so switching modes does not expose hidden state.
  if s.direction=="positive" then return v
  elseif s.direction=="negative" then return -v
  else return math.abs(v) end
end

local function resetAccelReboundState(s)
  s.reboundSign=0; s.reboundUntil=-999; s.reboundPeak=0; s.reboundLogged=false
  s.oppositeSince=-999; s.oppositeStart=0; s.oppositePeak=0
  s.buttonGestureSign=0; s.buttonGesturePhase="idle"; s.buttonGestureLastAt=-999; s.buttonGestureNeutralAt=-999
end

local function beginAccelReboundGesture(s,sign,a)
  s.reboundSign=sign
  s.reboundPeak=a
  s.reboundUntil=elapsed+0.12
  s.reboundLogged=false
  s.oppositeSince=-999; s.oppositeStart=0; s.oppositePeak=0
end

local function applyAccelReboundFilter(s,v)
  if not s.ignoreRebound then
    resetAccelReboundState(s)
    return v
  end

  local minG=accelBounds(s)
  local armThreshold=math.max(0.04,minG)
  local a=math.abs(v)
  if a<armThreshold then
    -- Quiet is no longer required to unlock another gesture. It only clears a
    -- pending opposite candidate so ordinary hand motion cannot leave a latch.
    s.oppositeSince=-999; s.oppositeStart=0; s.oppositePeak=0
    return v
  end
  local sign=(v<0) and -1 or 1

  if s.reboundSign==0 or elapsed>=s.reboundUntil then
    beginAccelReboundGesture(s,sign,a)
    return v
  end

  if sign==s.reboundSign then
    if a>s.reboundPeak then s.reboundPeak=a end
    s.oppositeSince=-999; s.oppositeStart=0; s.oppositePeak=0
    return v
  end

  -- The first short opposite lobe after a shove is usually the controller
  -- stopping. Suppress it, but accept a deliberate reversal without requiring
  -- the controller to become perfectly still: either immediately when the
  -- reversal is clearly stronger, or after ~45 ms when it is sustained rather
  -- than rapidly decaying.
  if s.oppositeSince<0 then
    s.oppositeSince=elapsed; s.oppositeStart=a; s.oppositePeak=a
  elseif a>s.oppositePeak then
    s.oppositePeak=a
  end

  local strongThreshold=math.max(armThreshold*1.6,s.reboundPeak*1.15)
  local sustained=(elapsed-s.oppositeSince)>=0.045 and a>=armThreshold*1.10 and a>=s.oppositeStart*0.78
  if a>=strongThreshold or sustained then
    beginAccelReboundGesture(s,sign,a)
    return v
  end

  if not s.reboundLogged then
    s.reboundLogged=true
    sendMotionDebug("ACCEL_REBOUND",string.format("axis=%s suppressed=1 sourceSign=%d windowMs=%d",({"X","Y","Z"})[s.index] or tostring(s.index),s.reboundSign,math.max(0,math.floor((s.reboundUntil-elapsed)*1000))))
  end
  return 0
end

local function applyAccelButtonGestureGuard(s,v)
  if not s.ignoreRebound then
    s.buttonGestureSign=0; s.buttonGesturePhase="idle"; s.buttonGestureLastAt=-999; s.buttonGestureNeutralAt=-999
    return v
  end

  local triggerG=(s.thresholdPercent/100)*4
  -- A button gesture is treated as one physical shove: the first meaningful
  -- acceleration lobe defines its direction. The opposite lobe created by
  -- braking/stopping the hand is never allowed to become a new +/- button
  -- event. This specifically fixes e.g. a downward move firing a + binding
  -- when the controller decelerates on the way back.
  -- Do not let tiny pre-motion jitter choose the gesture sign. Arm only once
  -- the intended motion reaches a meaningful fraction of the button threshold.
  local onset=math.max(0.08,triggerG*0.55)
  local quiet=math.max(0.08,math.min(0.20,onset*0.55))
  local a=math.abs(v)
  local sign=(v<0) and -1 or 1

  if s.buttonGestureSign==0 then
    if a>=onset then
      s.buttonGestureSign=sign; s.buttonGesturePhase="primary"; s.buttonGestureLastAt=elapsed; s.buttonGestureNeutralAt=-999
    end
    return v
  end

  if s.buttonGesturePhase=="primary" then
    if a>=onset then
      if sign==s.buttonGestureSign then
        s.buttonGestureLastAt=elapsed
        return v
      end
      -- Opposite-sign acceleration after the primary lobe is the stop/rebound
      -- phase. Lock the whole remainder of this gesture, not just 120 ms.
      s.buttonGesturePhase="rebound"; s.buttonGestureLastAt=elapsed; s.buttonGestureNeutralAt=-999
      sendMotionDebug("ACCEL_BUTTON_REBOUND",string.format("axis=%s primarySign=%d suppressedSign=%d",({"X","Y","Z"})[s.index] or tostring(s.index),s.buttonGestureSign,sign))
      return 0
    end
    -- Constant-velocity movement can contain a long near-zero acceleration gap
    -- before braking. Keep the sign lock through that gap, but expire after a
    -- generous 1.25 s if no stop lobe ever arrives. The old 450 ms timeout
    -- could expire during an ordinary hand translation, allowing the braking
    -- lobe to become a false new gesture in the opposite direction.
    if elapsed-s.buttonGestureLastAt>=1.25 then
      s.buttonGestureSign=0; s.buttonGesturePhase="idle"; s.buttonGestureNeutralAt=-999
    end
    return v
  end

  -- Once the stop/rebound phase starts, suppress all ringing until it settles
  -- below a tolerant quiet threshold for only 70 ms. Hand jitter therefore does
  -- not keep the button locked for seconds, but the braking lobe cannot fire the
  -- opposite direction.
  if a<=quiet then
    if s.buttonGestureNeutralAt<0 then s.buttonGestureNeutralAt=elapsed end
    if elapsed-s.buttonGestureNeutralAt>=0.07 then
      s.buttonGestureSign=0; s.buttonGesturePhase="idle"; s.buttonGestureLastAt=-999; s.buttonGestureNeutralAt=-999
    end
  else
    s.buttonGestureNeutralAt=-999
  end
  return 0
end

local function updateAccelButton(s,v)
  local trigger=(s.thresholdPercent/100)*4
  local hysteresis=math.max(0.04,math.min(0.20,trigger*0.15))
  local magnitude=accelButtonMagnitude(s,v)
  local on=magnitude>=trigger
  local release=magnitude<=math.max(0,trigger-hysteresis)

  -- In button mode the dedicated gesture guard owns rearming. A button may
  -- fire again only after the complete primary+stop waveform has ended.
  local canRearm=(not s.ignoreRebound) or s.buttonGestureSign==0

  if s.behavior=="hold" then
    if s.armed and not s.buttonHigh and on then
      s.buttonHigh=true; s.armed=false
      sendMotionDebug("ACCEL_TRIGGER",string.format("axis=%s g=%.3f activation=%d%% direction=%s behavior=%s",({"X","Y","Z"})[s.index] or tostring(s.index),v,s.thresholdPercent,s.direction,s.behavior))
    end
    if s.buttonHigh and release then s.buttonHigh=false end
    if not s.buttonHigh and release and canRearm then s.armed=true end
    return s.buttonHigh
  end

  if s.armed and on then
    s.armed=false; s.pulseUntil=elapsed+0.18
    sendMotionDebug("ACCEL_TRIGGER",string.format("axis=%s g=%.3f activation=%d%% direction=%s behavior=%s",({"X","Y","Z"})[s.index] or tostring(s.index),v,s.thresholdPercent,s.direction,s.behavior))
  end
  if release and canRearm then s.armed=true end
  s.buttonHigh=elapsed<s.pulseUntil
  return s.buttonHigh
end

local function motionButtonMagnitude(s,v)
  if s.buttonDirection=="negative" then return -v end
  if s.buttonDirection=="any" then return math.abs(v) end
  return v
end

local function updateMotionButton(s,v)
  local trigger=clamp(s.buttonThresholdPercent/100,0,1)
  local hysteresis=0.08
  local magnitude=motionButtonMagnitude(s,v)
  local on=magnitude>=trigger
  local release=magnitude<=math.max(0,trigger-hysteresis)
  if s.buttonBehavior=="hold" then
    if not s.buttonHigh and on then s.buttonHigh=true end
    if s.buttonHigh and release then s.buttonHigh=false end
    return s.buttonHigh
  end
  if s.buttonArmed and on then s.buttonArmed=false; s.buttonPulseUntil=elapsed+0.10 end
  if release then s.buttonArmed=true end
  s.buttonHigh=elapsed<s.buttonPulseUntil
  return s.buttonHigh
end

local function processMotion(dt)
  for i=1,3 do
    local s=motionSlots[i]
    local raw=clamp(rawMotionValue(s),-1,1)
    if s.outputMode=="button" and s.mode~="off" then
      -- Button thresholds operate on the physical normalized value directly.
      -- Axis-only Deadzone/Stability/Invert must not silently affect a button
      -- while those controls are hidden from the UI.
      s.filtered=raw; s.output=raw
      emitAxis(i-1,0)
      emitButton(i+2,updateMotionButton(s,raw))
    else
      local v=shapeMotion(s,raw,dt)
      s.buttonHigh=false; s.buttonArmed=true; s.buttonPulseUntil=-999
      emitButton(i+2,false)
      emitAxis(i-1,v)
    end
  end
end

local function processAcceleration(dt)
  for i=1,3 do
    local s=accelSlots[i]
    local raw=accelerationComponent(i)
    if s.mode=="axis" then
      local v=applyAccelReboundFilter(s,raw)
      emitButton(i-1,false)
      s.output=accelAxisValue(s,v)
      emitAxis(i+2,s.output)
    elseif s.mode=="button" then
      local v=applyAccelButtonGestureGuard(s,raw)
      s.output=0; emitAxis(i+2,0)
      emitButton(i-1,updateAccelButton(s,v))
    else
      s.output=0; s.buttonHigh=false; s.armed=true; s.pulseUntil=-999
      resetAccelReboundState(s)
      emitAxis(i+2,0); emitButton(i-1,false)
    end
  end
end

local function notifyMeaningfulMotionActivity()
  -- Stationary residual gyro is far below this threshold, so this does not
  -- keep Bluetooth alive from sensor noise alone. The extended-inputs module
  -- throttles/forwards the activity pulse to the Bridge when the user enables
  -- the corresponding sleep option.
  local gm=math.sqrt(sensor.gyroX*sensor.gyroX+sensor.gyroY*sensor.gyroY+sensor.gyroZ*sensor.gyroZ)
  local am=math.sqrt(sensor.linearX*sensor.linearX+sensor.linearY*sensor.linearY+sensor.linearZ*sensor.linearZ)
  if gm>=5.0 or am>=0.06 then
    if elapsed-lastMeaningfulMotionActivityAt>=0.20 then
      lastMeaningfulMotionActivityAt=elapsed
      -- rawget avoids BeamNG's lazy extension loader here. Motion packets arrive
      -- continuously, so a temporarily unavailable companion extension must not
      -- trigger a load attempt (and error log) on every packet.
      local ext=type(extensions)=="table" and rawget(extensions,"dualsenseExtendedInputs") or nil
      if ext and type(ext.noteControllerMotionActivity)=="function" then pcall(ext.noteControllerMotionActivity,gm,am) end
    end
  end
end

local function updateGravityAndLinear(dt)
  local ax,ay,az=sensor.accelX,sensor.accelY,sensor.accelZ
  if not sensor.gravityReady then
    sensor.gravityX,sensor.gravityY,sensor.gravityZ=ax,ay,az
    sensor.gravityReady=true
  else
    local tau=0.45
    local alpha=1-math.exp(-math.max(0,dt)/tau)
    -- During a strong transient, slow gravity adaptation further so a real
    -- shake/impact is not immediately absorbed into the baseline.
    local dx,dy,dz=ax-sensor.gravityX,ay-sensor.gravityY,az-sensor.gravityZ
    local transient=math.sqrt(dx*dx+dy*dy+dz*dz)
    alpha=alpha/(1+3*transient)
    sensor.gravityX=sensor.gravityX+alpha*dx
    sensor.gravityY=sensor.gravityY+alpha*dy
    sensor.gravityZ=sensor.gravityZ+alpha*dz
  end
  sensor.linearX=ax-sensor.gravityX
  sensor.linearY=ay-sensor.gravityY
  sensor.linearZ=az-sensor.gravityZ
end

local function processOutputs(dt)
  if elapsed<bindingMuteUntil then neutralAllOutputs(); return end
  processMotion(dt)
  processAcceleration(dt)
end

local function initUDP()
  if udp then pcall(function() udp:close() end) end
  udp=nil
  if not socket or not socket.udp then return false end
  local ok,s=pcall(socket.udp); if not ok or not s then return false end
  udp=s; pcall(function() udp:settimeout(0) end)
  local bound,err=udp:setsockname(listenHost,listenPort)
  if not bound then
    log("E",logTag,string.format("Unable to bind %s:%d: %s",listenHost,listenPort,tostring(err)))
    pcall(function() udp:close() end); udp=nil; return false
  end
  return true
end

local function updatePacketDt(timestamp)
  if lastSensorTs~=nil then
    local d=timestamp-lastSensorTs
    if d<0 then d=d+4294967296 end
    -- Sony sensor timestamp units in the current Bridge are not guaranteed to
    -- be host seconds here; reject implausible conversions and use frame dt.
    if d>0 and d<100000000 then packetDt=clamp(packetDt,1/500,1/30) end
  end
  lastSensorTs=timestamp
end

local function rejectPacket(data,reason)
  rejectedPacketCount=rejectedPacketCount+1
  if elapsed-lastRejectedPacketAt>=2.0 then
    lastRejectedPacketAt=elapsed
    local size=type(data)=="string" and #data or -1
    local version=(type(data)=="string" and #data>=5) and (data:byte(5) or -1) or -1
    log("W",logTag,string.format("Rejected DualSense sensor packet: %s (size=%d version=%d count=%d)",tostring(reason),size,version,rejectedPacketCount))
    sendMotionDebug("STREAM_REJECT",string.format("reason=%s size=%d version=%d count=%d",cleanText(reason),size,version,rejectedPacketCount))
  end
  return false
end

local function parsePacketV1(data)
  if #data~=30 then return nil end
  local q={}; local off=9
  for i=1,9 do q[i]=i16le(data,off); off=off+2 end
  return {q=q,timestamp=u32le(data,27)}
end

local function parsePacketV2(data)
  if #data~=56 then return nil end
  local q={}; local off=9
  for i=1,9 do q[i]=i16le(data,off); off=off+2 end
  local correctedGyro,gravity,linearAccel,quat={},{},{},{}
  for i=1,3 do correctedGyro[i]=i16le(data,off); off=off+2 end
  for i=1,3 do gravity[i]=i16le(data,off); off=off+2 end
  for i=1,3 do linearAccel[i]=i16le(data,off); off=off+2 end
  for i=1,4 do quat[i]=i16le(data,off); off=off+2 end
  return {q=q,correctedGyro=correctedGyro,gravity=gravity,linearAccel=linearAccel,quat=quat,timestamp=u32le(data,53)}
end

local function applyPacket(data)
  if type(data)~="string" then return rejectPacket(data,"not_string") end
  if data:sub(1,4)~="DSMO" then return rejectPacket(data,"bad_magic") end
  local version=data:byte(5) or 0
  local parsed
  if version==2 then parsed=parsePacketV2(data)
  elseif version==1 then parsed=parsePacketV1(data)
  else return rejectPacket(data,"unsupported_version") end
  if not parsed then return rejectPacket(data,"bad_size") end

  ensureDevice()
  local flags=data:byte(6) or 0
  local q=parsed.q
  sensor.protocol=version
  sensor.accelX=signedUnit(q[4])*4
  sensor.accelY=signedUnit(q[5])*4
  sensor.accelZ=signedUnit(q[6])*4
  sensor.roll=signedUnit(q[7])*90
  sensor.pitch=signedUnit(q[8])*90
  sensor.yaw=signedUnit(q[9])*180

  if version==2 then
    -- Protocol v2 already carries the Bridge's residual-bias-corrected gyro,
    -- fused gravity, gravity-removed linear acceleration and fused quaternion.
    -- Use those directly instead of reconstructing lower-quality substitutes.
    local cg,gr,la,quat=parsed.correctedGyro,parsed.gravity,parsed.linearAccel,parsed.quat
    sensor.gyroX=signedUnit(cg[1])*2048
    sensor.gyroY=signedUnit(cg[2])*2048
    sensor.gyroZ=signedUnit(cg[3])*2048
    sensor.gravityX=signedUnit(gr[1])
    sensor.gravityY=signedUnit(gr[2])
    sensor.gravityZ=signedUnit(gr[3])
    sensor.gravityReady=true
    sensor.linearX=signedUnit(la[1])*4
    sensor.linearY=signedUnit(la[2])*4
    sensor.linearZ=signedUnit(la[3])*4
    sensor.qw,sensor.qx,sensor.qy,sensor.qz=quatNormalize(signedUnit(quat[1]),signedUnit(quat[2]),signedUnit(quat[3]),signedUnit(quat[4]))
  else
    -- Backward-compatible legacy/v1 fallback.
    sensor.gyroX=signedUnit(q[1])*2048
    sensor.gyroY=signedUnit(q[2])*2048
    sensor.gyroZ=signedUnit(q[3])*2048
    sensor.qw,sensor.qx,sensor.qy,sensor.qz=quatFromEuler(sensor.roll,sensor.pitch,sensor.yaw)
    updateGravityAndLinear(packetDt)
  end

  -- Motion activity is transport/protocol independent. previous implementation accidentally
  -- called this only from the v1 gravity fallback, so protocol-v2 controllers
  -- could never reset Bluetooth idle from controller movement.
  notifyMeaningfulMotionActivity()

  sensor.transport=(flags%2==1) and "Bluetooth" or "USB"
  sensor.calibrated=(math.floor(flags/2)%2==1)
  sensor.timestamp=parsed.timestamp
  updatePacketDt(sensor.timestamp)

  local reconnecting=staleResetDone or (elapsed-lastPacketAt)>staleTimeout
  lastPacketAt=elapsed; staleResetDone=false
  if reconnecting then
    for i=1,3 do
      local m=motionSlots[i]
      if m then captureNeutral(m); m.filtered=0; m.output=0 end
    end
    for i=1,3 do
      local a=accelSlots[i]
      if a then a.output=0; a.filtered=0; a.buttonHigh=false; a.pulseUntil=-999; a.armed=true; resetAccelReboundState(a) end
    end
    local wasConnectedBefore=streamSeen
    streamSeen=true
    sendMotionDebug("STREAM",string.format("connected=1 protocol=%d transport=%s calibrated=%d reconnect=%d",version,sensor.transport,sensor.calibrated and 1 or 0,wasConnectedBefore and 1 or 0))
  end
  return true
end

local function pollUDP()
  if not udp then return end
  for _=1,maxPacketsPerUpdate do local data=udp:receivefrom(); if not data then break end; applyPacket(data) end
end

local function resetTransientRuntime()
  for i=1,3 do
    local s=motionSlots[i]
    if s then
      s.filtered=0; s.output=0; s.buttonHigh=false; s.buttonArmed=true; s.buttonPulseUntil=-999
    end
  end
  for i=1,3 do
    local s=accelSlots[i]
    if s then
      s.output=0; s.filtered=0; s.buttonHigh=false; s.pulseUntil=-999; s.armed=true; resetAccelReboundState(s)
    end
  end
end

local function beginBindingMutation(reason,duration)
  bindingMuteUntil=math.max(bindingMuteUntil,elapsed+(tonumber(duration) or 0.75))
  neutralAllOutputs(); resetTransientRuntime()
  sendMotionDebug("BINDING_GUARD",string.format("phase=begin reason=%s muteMs=%d",cleanText(reason),math.floor(math.max(0,bindingMuteUntil-elapsed)*1000)))
end

local function installBindingMutationGuard()
  if not core_input_bindings then return end
  if type(core_input_bindings.resetBindings)=="function" and not wrappedResetBindings then
    originalResetBindings=core_input_bindings.resetBindings
    wrappedResetBindings=function(...)
      beginBindingMutation("resetBindings",1.00)
      return originalResetBindings(...)
    end
    core_input_bindings.resetBindings=wrappedResetBindings
  end
  if type(core_input_bindings.saveBindingsToDisk)=="function" and not wrappedSaveBindings then
    originalSaveBindings=core_input_bindings.saveBindingsToDisk
    wrappedSaveBindings=function(...)
      beginBindingMutation("saveBindingsToDisk",0.55)
      return originalSaveBindings(...)
    end
    core_input_bindings.saveBindingsToDisk=wrappedSaveBindings
  end
end

local function uninstallBindingMutationGuard()
  if core_input_bindings then
    if wrappedResetBindings and core_input_bindings.resetBindings==wrappedResetBindings then core_input_bindings.resetBindings=originalResetBindings end
    if wrappedSaveBindings and core_input_bindings.saveBindingsToDisk==wrappedSaveBindings then core_input_bindings.saveBindingsToDisk=originalSaveBindings end
  end
  wrappedResetBindings,wrappedSaveBindings,originalResetBindings,originalSaveBindings=nil,nil,nil,nil
end

local function onInputBindingsChanged()
  -- BeamNG may emit this hook while the extension is loaded but before its first
  -- update. At that point the configured slot tables do not exist yet.
  if not initialized then return end
  bindingMuteUntil=math.max(bindingMuteUntil,elapsed+0.55)
  local coalesced=(elapsed-lastBindingChangeHandledAt)<0.08
  if not coalesced then
    lastBindingChangeHandledAt=elapsed; neutralAllOutputs(); resetTransientRuntime()
  end
  sendMotionDebug("BINDINGS_CHANGED",string.format("mappedDevice=%s muteMs=%d coalesced=%d",deviceInstance and ("vinput"..tostring(deviceInstance)) or "nil",math.floor(math.max(0,bindingMuteUntil-elapsed)*1000),coalesced and 1 or 0))
end

local function baseControl(control)
  if type(control)~="string" then return nil end
  return control:match("([^%s]+)$")
end

local function bindingInfo()
  local actionsByControl={}
  if deviceInstance==nil or not core_input_bindings or type(core_input_bindings.bindings)~="table" then return actionsByControl end
  local devname="vinput"..tostring(deviceInstance)
  for _,device in ipairs(core_input_bindings.bindings) do
    if device.devname==devname and type(device.contents)=="table" and type(device.contents.bindings)=="table" then
      for _,b in ipairs(device.contents.bindings) do
        local c=baseControl(b.control)
        if c then
          actionsByControl[c]=actionsByControl[c] or {}
          actionsByControl[c][#actionsByControl[c]+1]=tostring(b.action or "")
        end
      end
    end
  end
  return actionsByControl
end

local function bindingLabel(actions)
  if not actions or #actions==0 then return "" end
  if #actions==1 then return actions[1] end
  return actions[1].." +"..tostring(#actions-1)
end

local function startMotionCalibration(index)
  index=math.floor(tonumber(index) or 0)
  local s=motionSlots[index]
  if not s or s.mode~="rotation" or s.axis~="auto" then return false end
  -- The current natural holding position is the neutral reference. The user can
  -- then turn in either/both directions; opposite samples must reinforce the
  -- same learned axis rather than cancel each other.
  captureNeutral(s)
  calibrations[index]={startedAt=elapsed,duration=2.4,samples={}}
  sendMotionDebug("CALIBRATION",string.format("slot=%d phase=start independent=1 neutral=1 principalAxis=1",index))
  return true
end

local function principalAxisFromSamples(samples)
  local xx,xy,xz,yy,yz,zz=0,0,0,0,0,0
  local totalEnergy=0
  local firstX,firstY,firstZ=nil,nil,nil
  local count=0
  for _,q in ipairs(samples or {}) do
    local rx,ry,rz=quatToRotVec(q[1],q[2],q[3],q[4])
    local a2=rx*rx+ry*ry+rz*rz
    if a2>math.rad(2)^2 then
      xx=xx+rx*rx; xy=xy+rx*ry; xz=xz+rx*rz
      yy=yy+ry*ry; yz=yz+ry*rz; zz=zz+rz*rz
      totalEnergy=totalEnergy+a2; count=count+1
      if not firstX then firstX,firstY,firstZ=rx,ry,rz end
    end
  end
  if count<8 or totalEnergy<math.rad(8)^2 then return nil,nil,nil,0 end

  -- Power iteration of the 3x3 covariance matrix gives the dominant physical
  -- rotation axis. Because covariance uses outer products, left/right samples
  -- reinforce one another instead of cancelling.
  local ax,ay,az=firstX or 1,firstY or 0,firstZ or 0
  local n=math.sqrt(ax*ax+ay*ay+az*az); ax,ay,az=ax/n,ay/n,az/n
  for _=1,12 do
    local nx=xx*ax+xy*ay+xz*az
    local ny=xy*ax+yy*ay+yz*az
    local nz=xz*ax+yz*ay+zz*az
    local nn=math.sqrt(nx*nx+ny*ny+nz*nz)
    if nn<1e-10 then return nil,nil,nil,0 end
    ax,ay,az=nx/nn,ny/nn,nz/nn
  end
  -- Keep a deterministic sign: the first deliberate gesture is positive.
  if firstX and (ax*firstX+ay*firstY+az*firstZ)<0 then ax,ay,az=-ax,-ay,-az end

  local projectedEnergy=0
  for _,q in ipairs(samples or {}) do
    local rx,ry,rz=quatToRotVec(q[1],q[2],q[3],q[4])
    local d=rx*ax+ry*ay+rz*az
    projectedEnergy=projectedEnergy+d*d
  end
  return ax,ay,az,clamp(projectedEnergy/math.max(1e-12,totalEnergy),0,1)
end

local function updateCalibrations()
  for index,c in pairs(calibrations) do
    local s=motionSlots[index]
    if s then
      local qw,qx,qy,qz=controllerDeltaQuat(s)
      local rx,ry,rz=quatToRotVec(qw,qx,qy,qz)
      local a=math.sqrt(rx*rx+ry*ry+rz*rz)
      if a>math.rad(2) and #c.samples<420 then c.samples[#c.samples+1]={qw,qx,qy,qz} end

      if elapsed-c.startedAt>=c.duration then
        local ax,ay,az,quality=principalAxisFromSamples(c.samples)
        if ax then
          local maxTwist=0
          for _,q in ipairs(c.samples) do
            local angle=math.abs(twistAngleFromQuat(q[1],q[2],q[3],q[4],ax,ay,az))
            if angle>maxTwist then maxTwist=angle end
          end
          if maxTwist>=math.rad(8) then
            s.learned=true; s.learnedX=ax; s.learnedY=ay; s.learnedZ=az; s.learnQuality=quality
            s.range=clamp(math.deg(maxTwist),20,180)
            persistMotion(s)
            sendMotionDebug("CALIBRATION",string.format("slot=%d phase=done axis=%+.3f,%+.3f,%+.3f quality=%.3f rangePerSide=%.1f",index,ax,ay,az,quality,s.range))
          else
            sendMotionDebug("CALIBRATION",string.format("slot=%d phase=failed reason=range quality=%.3f",index,quality))
          end
        else
          sendMotionDebug("CALIBRATION",string.format("slot=%d phase=failed reason=axis quality=0",index))
        end
        calibrations[index]=nil
        -- Keep the neutral captured at calibration START. The user may finish
        -- the gesture at a steering endpoint; recentering there would destroy
        -- the natural holding position that calibration was meant to learn.
        s.filtered=0; s.output=0
      end
    else
      calibrations[index]=nil
    end
  end
end

local function recenterMotion(index)
  index=math.floor(tonumber(index) or 0); local s=motionSlots[index]; if not s then return false end
  captureNeutral(s); return true
end

local function setMotionMode(index,mode)
  index=math.floor(tonumber(index) or 0); local s=motionSlots[index]; if not s then return false end
  local valid={off=true,rotation=true,gyro=true,tilt=true,orientation=true}; if not valid[mode] then return false end
  emitAxis(index-1,0); emitButton(index+2,false)
  s.mode=mode; s.range=defaultRangeForMode(mode); s.buttonHigh=false; s.buttonArmed=true; s.buttonPulseUntil=-999
  if mode=="rotation" then s.axis="auto"
  elseif mode=="gyro" then s.axis="horizontal"; s.reference="adaptive"
  elseif mode=="tilt" then s.axis="left_right"
  elseif mode=="orientation" then s.axis="roll" end
  captureNeutral(s); persistMotion(s)
  sendMotionDebug("MOTION_MODE",string.format("slot=%d mode=%s",index,s.mode))
  return true
end

local function setMotionAxis(index,axis)
  index=math.floor(tonumber(index) or 0); local s=motionSlots[index]; if not s then return false end
  s.axis=tostring(axis or ""); normalizeMotion(s); captureNeutral(s); persistMotion(s)
  sendMotionDebug("MOTION_AXIS",string.format("slot=%d axis=%s",index,s.axis))
  return true
end

local function setMotionOption(index,key,value)
  index=math.floor(tonumber(index) or 0); local s=motionSlots[index]; if not s then return false end
  if key=="invert" then s.invert=boolValue(value)
  elseif key=="reference" then s.reference=tostring(value or "")
  elseif key=="deadzone" then s.deadzone=tonumber(value) or s.deadzone
  elseif key=="stability" then s.stability=tonumber(value) or s.stability
  elseif key=="range" then s.range=tonumber(value) or s.range
  elseif key=="customX" then s.customX=tonumber(value) or s.customX
  elseif key=="customY" then s.customY=tonumber(value) or s.customY
  elseif key=="customZ" then s.customZ=tonumber(value) or s.customZ
  elseif key=="outputMode" then
    s.outputMode=tostring(value or "axis")
    emitAxis(index-1,0); emitButton(index+2,false); s.buttonHigh=false; s.buttonArmed=true; s.buttonPulseUntil=-999
  elseif key=="buttonDirection" then s.buttonDirection=tostring(value or "positive")
  elseif key=="buttonThresholdPercent" then s.buttonThresholdPercent=tonumber(value) or s.buttonThresholdPercent
  elseif key=="buttonBehavior" then s.buttonBehavior=tostring(value or "hold")
  else return false end
  normalizeMotion(s); persistMotion(s); sendMotionDebug("MOTION_SETTING",string.format("slot=%d key=%s",index,cleanText(key))); return true
end

local function setAccelerationMode(index,mode)
  index=math.floor(tonumber(index) or 0); local s=accelSlots[index]; if not s then return false end
  if mode~="off" and mode~="axis" and mode~="button" then return false end
  emitAxis(index+2,0); emitButton(index-1,false)
  s.mode=mode
  s.buttonHigh=false; s.armed=true; s.pulseUntil=-999; s.output=0
  resetAccelReboundState(s)
  persistAccel(s)
  sendMotionDebug("ACCEL_MODE",string.format("axis=%s mode=%s",({"X","Y","Z"})[index] or tostring(index),s.mode))
  return true
end

local function setAccelerationOption(index,key,value)
  index=math.floor(tonumber(index) or 0); local s=accelSlots[index]; if not s then return false end
  if key=="invert" then s.invert=boolValue(value)
  elseif key=="minPercent" then s.minPercent=tonumber(value) or s.minPercent
  elseif key=="maxPercent" then s.maxPercent=tonumber(value) or s.maxPercent
  elseif key=="thresholdPercent" then s.thresholdPercent=tonumber(value) or s.thresholdPercent
  elseif key=="direction" then s.direction=tostring(value or "")
  elseif key=="behavior" then s.behavior=tostring(value or "")
  elseif key=="ignoreRebound" then s.ignoreRebound=boolValue(value); resetAccelReboundState(s)
  else return false end
  normalizeAccel(s); persistAccel(s)
  sendMotionDebug("ACCEL_SETTING",string.format("axis=%s key=%s min=%d%% full=%d%% activation=%d%% direction=%s behavior=%s rebound=%d",({"X","Y","Z"})[index] or tostring(index),cleanText(key),s.minPercent,s.maxPercent,s.thresholdPercent,s.direction,s.behavior,s.ignoreRebound and 1 or 0))
  return true
end

local function resetMotionSlot(index)
  index=math.floor(tonumber(index) or 0); if index<1 or index>3 then return false end
  calibrations[index]=nil; motionSlots[index]=newMotionSlot(index); persistMotion(motionSlots[index]); captureNeutral(motionSlots[index]); emitAxis(index-1,0); emitButton(index+2,false); return true
end

local function resetAccelerationSlot(index)
  index=math.floor(tonumber(index) or 0); if index<1 or index>3 then return false end
  accelSlots[index]=newAccelSlot(index); persistAccel(accelSlots[index]); emitAxis(index+2,0); emitButton(index-1,false); return true
end

local function resetSensorSettings()
  for i=1,3 do resetMotionSlot(i); resetAccelerationSlot(i) end
  return true
end

local function getSensorsUIStateFlat()
  local bind=bindingInfo()
  local rows={"SENS4"}
  for i=1,3 do
    local s=motionSlots[i]
    local c=calibrations[i]
    local progress=c and clamp((elapsed-c.startedAt)/c.duration,0,1) or 0
    local axisControl=axisControls[i]
    local buttonControl="button"..tostring(VINPUT.sensorButtonBase+i+2)
    rows[#rows+1]=table.concat({
      "M",tostring(i),s.mode,s.axis,s.invert and "1" or "0",string.format("%.5f",s.deadzone),string.format("%.2f",s.stability),string.format("%.3f",s.range),s.reference,
      string.format("%.4f",s.customX),string.format("%.4f",s.customY),string.format("%.4f",s.customZ),s.learned and "1" or "0",string.format("%.4f",s.learnedX),string.format("%.4f",s.learnedY),string.format("%.4f",s.learnedZ),string.format("%.3f",s.learnQuality),
      bindingLabel(bind[axisControl]),c and "1" or "0",string.format("%.3f",progress),string.format("%.5f",s.output or 0),s.outputMode,s.buttonDirection,string.format("%.2f",s.buttonThresholdPercent),s.buttonBehavior,bindingLabel(bind[buttonControl]),s.buttonHigh and "1" or "0"
    },"~")
  end
  for i=1,3 do
    local s=accelSlots[i]
    local axisControl=axisControls[i+3]
    local buttonControl="button"..tostring(VINPUT.sensorButtonBase+i-1)
    rows[#rows+1]=table.concat({
      "A",tostring(i),s.mode,s.invert and "1" or "0",string.format("%.0f",s.minPercent),string.format("%.0f",s.maxPercent),string.format("%.0f",s.thresholdPercent),s.direction,s.behavior,
      bindingLabel(bind[axisControl]),bindingLabel(bind[buttonControl]),string.format("%.5f",accelerationComponent(i)),string.format("%.5f",s.output or 0),s.buttonHigh and "1" or "0",s.ignoreRebound and "1" or "0"
    },"~")
  end
  rows[#rows+1]=table.concat({"S",sensor.transport,sensor.calibrated and "1" or "0",(elapsed-lastPacketAt)<=staleTimeout and "1" or "0",string.format("%.4f",sensor.linearX),string.format("%.4f",sensor.linearY),string.format("%.4f",sensor.linearZ),tostring(sensor.protocol or 0),tostring(rejectedPacketCount)},"~")
  return table.concat(rows,"\t")
end

local function initializeRuntime()
  if initialized then return end
  initialized = true
  loadSettings()
  if not SETTINGS_STORE.get(MOTION_BINDING_MIGRATION_KEY, false) then
    migrateLegacyBindings()
    SETTINGS_STORE.set(MOTION_BINDING_MIGRATION_KEY, true)
  end
  retireLegacyDevices(); ensureDevice(); initUDP(); neutralAllOutputs(); installBindingMutationGuard()
  for i=1,3 do captureNeutral(motionSlots[i]) end
end

local function onExtensionLoaded()
  -- Keep extension loading lightweight; initialization runs on the first update.
end

local function onFirstUpdate()
  initializeRuntime()
end

local function onUpdate(dtReal,dtSim,dtRaw)
  if not initialized then initializeRuntime() end
  local dt=math.max(0,tonumber(dtReal) or tonumber(dtSim) or tonumber(dtRaw) or 0)
  elapsed=elapsed+dt
  packetDt=(dt>0 and dt<0.1) and dt or packetDt
  ensureDevice(); installBindingMutationGuard(); if not udp then initUDP() end; pollUDP(); updateCalibrations()
  if elapsed-lastPacketAt<=staleTimeout then processOutputs(dt)
  elseif not staleResetDone then
    neutralAllOutputs(); resetTransientRuntime(); sensor.gravityReady=false; staleResetDone=true
    sendMotionDebug("STREAM","connected=0 reason=stale outputsNeutral=1")
  end
end

local function onExtensionUnloaded()
  neutralAllOutputs(); uninstallBindingMutationGuard()
  if udp then pcall(function() udp:close() end) end; udp=nil
  if statusUdp then pcall(function() statusUdp:close() end) end; statusUdp=nil
  deviceInstance=nil; VINPUT.releaseProducer("motion")
end

M.onExtensionLoaded=onExtensionLoaded
M.onFirstUpdate=onFirstUpdate
M.onUpdate=onUpdate
M.onExtensionUnloaded=onExtensionUnloaded
M.onInputBindingsChanged=onInputBindingsChanged
M.getSensorsUIStateFlat=getSensorsUIStateFlat
M.startMotionCalibration=startMotionCalibration
M.recenterMotion=recenterMotion
M.setMotionMode=setMotionMode
M.setMotionAxis=setMotionAxis
M.setMotionOption=setMotionOption
M.setAccelerationMode=setAccelerationMode
M.setAccelerationOption=setAccelerationOption
M.resetMotionSlot=resetMotionSlot
M.resetAccelerationSlot=resetAccelerationSlot
M.resetSensorSettings=resetSensorSettings
M.getMotionState=function() return deepcopy(sensor) end

return M
