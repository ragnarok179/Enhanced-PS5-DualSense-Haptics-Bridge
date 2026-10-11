import { setModSetting } from "./settingsStore.js"
import {
  CUSTOM_SETTING_KEYS,
  DEFAULT_CUSTOM_MAPPING,
  MOUSE_MODE_SETTINGS_KEY,
  TWO_FINGER_MOUSE_TOGGLE_SETTINGS_KEY,
  BLUETOOTH_IDLE_SETTINGS_KEY,
  BLUETOOTH_IDLE_EXPLICIT_OFF_KEY,
  BLUETOOTH_IDLE_MOTION_ACTIVITY_KEY,
  DEFAULT_BLUETOOTH_IDLE_MINUTES,
  DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY,
} from "./components/extendedInputsSettingsModel.js"

export const SPEAKER_SETTING_KEYS = Object.freeze({
  enabled: "dseSpeakerEnabledV1",
  volume: "dseSpeakerVolumeV1",
  collision: "dseSpeakerCollisionV1",
  output: "dseSpeakerOutputV1",
})

export const SPEAKER_SETTINGS_KEY = "dseSpeakerSettingsV2"

export const SPEAKER_DEFAULTS = Object.freeze({ enabled: true, volume: 100, collision: true, output: 0 })

function boolLua(value) { return value ? "true" : "false" }
function escapeLuaString(value) { return String(value).replace(/\\/g, "\\\\").replace(/'/g, "\\'") }

export function createExtendedInputsDefaultPatch() {
  const patch = {
    [MOUSE_MODE_SETTINGS_KEY]: false,
    [TWO_FINGER_MOUSE_TOGGLE_SETTINGS_KEY]: false,
    [BLUETOOTH_IDLE_SETTINGS_KEY]: DEFAULT_BLUETOOTH_IDLE_MINUTES,
    [BLUETOOTH_IDLE_EXPLICIT_OFF_KEY]: false,
    [BLUETOOTH_IDLE_MOTION_ACTIVITY_KEY]: DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY,
  }
  for (const [channel, key] of Object.entries(CUSTOM_SETTING_KEYS)) patch[key] = DEFAULT_CUSTOM_MAPPING[channel]
  return patch
}

export async function resetExtendedInputsDefaults(runRaw) {
  for (const [channel, mapping] of Object.entries(DEFAULT_CUSTOM_MAPPING)) {
    await runRaw(`extensions.dualsenseExtendedInputs.setTouchpadCustomMapping('${escapeLuaString(channel)}','${escapeLuaString(mapping)}')`)
  }
  await runRaw(`extensions.dualsenseExtendedInputs.setTouchpadMouseMode(${boolLua(false)})`)
  try { await runRaw("extensions.dualsenseExtendedInputs.setTwoFingerMouseToggleEnabled(false)") } catch {}
  await runRaw(`extensions.dualsenseExtendedInputs.setBluetoothIdleTimeoutMinutes(${DEFAULT_BLUETOOTH_IDLE_MINUTES})`)
  await runRaw(`extensions.dualsenseExtendedInputs.setBluetoothIdleMotionActivityEnabled(${boolLua(DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY)})`)
  await runRaw("extensions.dualsenseMotionInputs.resetSensorSettings()")
}

export async function resetSpeakerDefaults() {
  // Save a single snapshot instead of four independent writes.
  // Existing V1 keys remain readable for migration but no longer own the state.
  await setModSetting(SPEAKER_SETTINGS_KEY, { ...SPEAKER_DEFAULTS })
}
