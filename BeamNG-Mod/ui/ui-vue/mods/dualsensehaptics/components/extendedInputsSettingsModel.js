export const LEGACY_MODE_SETTINGS_KEY = "dseTouchpadInputModeV2"
export const CUSTOM_SETTING_KEYS = {
  one_x: "dseTouchpadCustom1FXV1",
  one_y: "dseTouchpadCustom1FYV1",
  two_x: "dseTouchpadCustom2FXV1",
  two_y: "dseTouchpadCustom2FYV1",
}
export const DEFAULT_CUSTOM_MAPPING = { one_x: "swipe", one_y: "swipe", two_x: "swipe", two_y: "swipe" }
export const mappingOptions = [
  { key: "off", label: "Off" },
  { key: "swipe", label: "Swipe" },
  { key: "relative", label: "Relative" },
  { key: "absolute", label: "Absolute" },
]
export const customGroups = [
  { key: "one", label: "1 finger", channels: [
    { key: "one_x", label: "Horizontal", axis: "X" },
    { key: "one_y", label: "Vertical", axis: "Y" },
  ] },
  { key: "two", label: "2 fingers", channels: [
    { key: "two_x", label: "Horizontal", axis: "X" },
    { key: "two_y", label: "Vertical", axis: "Y" },
  ] },
]
export function normalizeChannelMapping(value, fallback = "swipe") {
  return mappingOptions.some(option => option.key === value) ? value : fallback
}
export function legacyPresetMapping(mode) {
  if (mode === "relative_axes") return { one_x: "relative", one_y: "relative", two_x: "relative", two_y: "relative" }
  if (mode === "absolute_axes") return { one_x: "absolute", one_y: "absolute", two_x: "absolute", two_y: "absolute" }
  return { ...DEFAULT_CUSTOM_MAPPING }
}

export const MOUSE_MODE_SETTINGS_KEY = "dseTouchpadMouseModeV1"
export const TWO_FINGER_MOUSE_TOGGLE_SETTINGS_KEY = "dseTouchpadTwoFingerMouseToggleV1"
export function normalizeBooleanSetting(value, fallback = false) {
  if (value === true || value === 1 || value === "1" || String(value).toLowerCase() === "true") return true
  if (value === false || value === 0 || value === "0" || String(value).toLowerCase() === "false") return false
  return fallback
}

export const BLUETOOTH_IDLE_SETTINGS_KEY = "dseBluetoothIdleTimeoutMinutesV2"
export const BLUETOOTH_IDLE_EXPLICIT_OFF_KEY = "dseBluetoothIdleExplicitOffV1"
export const BLUETOOTH_IDLE_MOTION_ACTIVITY_KEY = "dseBluetoothIdleMotionActivityV1"
export const DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY = false
export const DEFAULT_BLUETOOTH_IDLE_MINUTES = 10
export const BLUETOOTH_IDLE_MAX_MINUTES = 240
export function normalizeBluetoothIdleMinutes(value) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return DEFAULT_BLUETOOTH_IDLE_MINUTES
  return Math.max(0, Math.min(BLUETOOTH_IDLE_MAX_MINUTES, Math.round(parsed)))
}
export function bluetoothIdleExplicitOff(value) {
  return value === true || value === 1 || value === "1" || String(value).toLowerCase() === "true"
}
export function resolveBluetoothIdleMinutes(value, explicitOff) {
  if (bluetoothIdleExplicitOff(explicitOff)) return 0
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return DEFAULT_BLUETOOTH_IDLE_MINUTES
  return normalizeBluetoothIdleMinutes(parsed)
}

export const motionModeOptions = [
  { key: "off", label: "Off" },
  { key: "rotation", label: "Rotation angle" },
  { key: "gyro", label: "Rotation speed" },
  { key: "tilt", label: "Tilt" },
  { key: "orientation", label: "Orientation" },
]
export const motionAxisOptionsByMode = {
  off: [{ key: "auto", label: "—" }],
  rotation: [
    { key: "auto", label: "Auto" },
    { key: "roll", label: "Roll" },
    { key: "pitch", label: "Pitch" },
    { key: "yaw", label: "Yaw" },
    { key: "custom", label: "Custom" },
  ],
  gyro: [
    { key: "horizontal", label: "Horizontal" },
    { key: "vertical", label: "Vertical" },
    { key: "roll", label: "Roll" },
    { key: "pitch", label: "Pitch" },
    { key: "yaw", label: "Yaw" },
  ],
  tilt: [
    { key: "left_right", label: "Left / right" },
    { key: "forward_back", label: "Forward / back" },
  ],
  orientation: [
    { key: "roll", label: "Roll" },
    { key: "pitch", label: "Pitch" },
    { key: "yaw", label: "Yaw" },
  ],
}
export const motionOutputOptions = [
  { key: "axis", label: "Axis" },
  { key: "button", label: "Button" },
]
export const motionButtonDirectionOptions = [
  { key: "positive", label: "+" },
  { key: "negative", label: "−" },
  { key: "any", label: "Any" },
]
export const motionButtonBehaviorOptions = [
  { key: "hold", label: "Hold" },
  { key: "pulse", label: "Pulse" },
]
export const motionReferenceOptions = [
  { key: "adaptive", label: "Adaptive" },
  { key: "controller", label: "Controller" },
  { key: "world", label: "World" },
]
export function motionAxisOptions(mode) { return motionAxisOptionsByMode[mode] || motionAxisOptionsByMode.off }
export function optionLabel(options, key, fallback = "—") { return options.find(option => option.key === key)?.label || fallback }

export const accelerationModeOptions = [
  { key: "off", label: "Off" },
  { key: "axis", label: "Axis" },
  { key: "button", label: "Button" },
]
export const accelerationDirectionOptions = [
  { key: "any", label: "Any" },
  { key: "positive", label: "+" },
  { key: "negative", label: "−" },
]
export const accelerationBehaviorOptions = [
  { key: "pulse", label: "Pulse" },
  { key: "hold", label: "Hold" },
]
