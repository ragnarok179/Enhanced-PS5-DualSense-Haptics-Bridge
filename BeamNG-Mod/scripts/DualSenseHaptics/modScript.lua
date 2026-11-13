-- Enhanced PS5 DualSense Haptics
-- Register the GE extensions as manual-lifetime extensions. BeamNG loads every
-- registered manual extension after all modScript.lua files have finished.
-- Do not call extensions.load() here: BeamNG deliberately warns about that
-- legacy modScript pattern.

local function keepLoaded(name)
  if type(setExtensionUnloadMode) == "function" then
    setExtensionUnloadMode(name, "manual")
  end
end

-- Keep the registration order deterministic. The settings store is available
-- before feature extensions consume mod-owned settings. The lifecycle extension
-- is last so other extensions release their resources before it is unloaded.
keepLoaded("dualsenseSettingsStore")
keepLoaded("dualsenseNativeTriggerGuard")
keepLoaded("dualsensePhysicsDirectRPMLights")
keepLoaded("dualsenseExtendedInputs")
keepLoaded("dualsenseMotionInputs")
keepLoaded("dualsenseControllerSpeaker")
keepLoaded("dualsenseCollisionSoundProbeGE")
keepLoaded("dualsenseModLifecycle")
