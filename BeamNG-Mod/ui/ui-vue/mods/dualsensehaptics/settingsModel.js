export const SETTINGS_KEY = "dphSettings"
export const SETTINGS_SCHEMA = 12

const HAPTIC_REFERENCE_PERCENT = 100
const SURFACE_MASTER_REFERENCE_PERCENT = 100
const LED_REFERENCE_PERCENT = 86
const R2_JOLT_REFERENCE_PERCENT = 25

export const surfaces = [
  { key: "asphalt", label: "Asphalt", rollingReference: 10, slipReference: 0.5, v149Slip: 1, v148Rolling: 29, v148Slip: 100, v147Rolling: 22, v147Slip: 3 },
  { key: "asphalt_wet", label: "Wet asphalt", rollingReference: 6, slipReference: 2.6, v149Slip: 1, v148Rolling: 43, v148Slip: 100, v147Rolling: 26, v147Slip: 7 },
  { key: "slippery", label: "Slippery", rollingReference: 3, slipReference: 0.5, v149Slip: 1, v148Rolling: 43, v148Slip: 100, v147Rolling: 20, v147Slip: 8 },
  { key: "ice", label: "Ice", rollingReference: 3, slipReference: 0.6, v149Slip: 1, v148Rolling: 43, v148Slip: 100, v147Rolling: 20, v147Slip: 7 },
  { key: "dirt", label: "Dirt", rollingReference: 23, slipReference: 0.5, v149Slip: 1, v148Rolling: 70, v148Slip: 100, v147Rolling: 50, v147Slip: 8 },
  { key: "dusty_dirt", label: "Dusty dirt", rollingReference: 22, slipReference: 0.5, v149Slip: 1, v148Rolling: 70, v148Slip: 100, v147Rolling: 50, v147Slip: 8 },
  { key: "sandy_road", label: "Sandy road", rollingReference: 34, slipReference: 0.7, v149Slip: 1, v148Rolling: 65, v148Slip: 100, v147Rolling: 50, v147Slip: 5 },
  { key: "sand", label: "Sand", rollingReference: 24, slipReference: 1.0, v149Slip: 1, v148Rolling: 48, v148Slip: 100, v147Rolling: 28, v147Slip: 3 },
  { key: "mud", label: "Mud", rollingReference: 22, slipReference: 0.5, v149Slip: 1, v148Rolling: 58, v148Slip: 100, v147Rolling: 32, v147Slip: 4 },
  { key: "gravel", label: "Gravel", rollingReference: 62, slipReference: 1.2, v149Slip: 1, v148Rolling: 100, v148Slip: 100, v147Rolling: 62, v147Slip: 10 },
  { key: "grass", label: "Grass", rollingReference: 12, slipReference: 0.3, v149Slip: 1, v148Rolling: 45, v148Slip: 100, v147Rolling: 36, v147Slip: 5 },
  { key: "snow", label: "Snow", rollingReference: 8, slipReference: 0.3, v149Slip: 1, v148Rolling: 39, v148Slip: 100, v147Rolling: 36, v147Slip: 4 },
  { key: "rock", label: "Rock", rollingReference: 31, slipReference: 0.5, v149Slip: 1, v148Rolling: 100, v148Slip: 100, v147Rolling: 78, v147Slip: 8 },
  { key: "cobblestone", label: "Cobblestone", rollingReference: 70, slipReference: 0.4, v149Slip: 1, v148Rolling: 100, v148Slip: 100, v147Rolling: 74, v147Slip: 6 },
  { key: "rumble_strip", label: "Rumble strip", rollingReference: 84, slipReference: 0.1, v149Slip: 1, v148Rolling: 100, v148Slip: 100, v147Rolling: 88, v147Slip: 4 },
]

function defaultSurfaceProfiles() {
  const profiles = {}
  for (const surface of surfaces) {
    profiles[surface.key] = {
      rollingPercent: surface.rollingReference,
      slipPercent: surface.slipReference,
    }
  }
  return profiles
}

export function createDefaults() {
  return {
    schema: SETTINGS_SCHEMA,
    haptics: {
      masterEnabled: true,
      masterPercent: HAPTIC_REFERENCE_PERCENT,
      surfaceEnabled: true,
      surfacePercent: SURFACE_MASTER_REFERENCE_PERCENT,
      impactEnabled: true,
      impactPercent: 100,
      surfaceProfiles: defaultSurfaceProfiles(),
    },
    adaptiveTriggers: {
      l2BrakeEnabled: true,
      l2BrakeStartPercent: 25,
      l2BrakeEndPercent: 75,
      absEnabled: true,
      absPercent: 75,
      r2ThrottleEnabled: true,
      r2ThrottleStartPercent: 13,
      r2ThrottleEndPercent: 13,
      r2EffectsEnabled: true,
      r2EffectsPercent: R2_JOLT_REFERENCE_PERCENT,
    },
    lighting: {
      enabled: true,
      brightnessPercent: LED_REFERENCE_PERCENT,
    },
  }
}

export function cloneSettings(value) {
  return JSON.parse(JSON.stringify(value))
}

export function decodeStored(value) {
  if (value && typeof value === "object") return value
  if (typeof value === "string" && value.length) {
    try { return JSON.parse(value) } catch { return null }
  }
  return null
}

export function clampPercent(value, fallback = 100) {
  const number = Number(value)
  if (!Number.isFinite(number)) return fallback
  return Math.max(1, Math.min(100, Math.round(number)))
}

export function clampFinePercent(value, fallback = 1) {
  const number = Number(value)
  if (!Number.isFinite(number)) return fallback
  return Math.max(0.1, Math.min(100, Math.round(number * 10) / 10))
}

function rescaleReference(value, oldReference, newReference, fallback = oldReference) {
  const oldRef = Math.max(1, Number(oldReference) || 100)
  const newRef = Math.max(1, Number(newReference) || 100)
  const gain = clampPercent(value, fallback) / oldRef
  return clampPercent(Math.round(newRef * gain), newRef)
}

export function normalizeSettings(source) {
  const d = createDefaults()
  const s = source && typeof source === "object" ? source : {}
  const oldSchema = Number(s.schema) || 1
  const h = s.haptics && typeof s.haptics === "object" ? s.haptics : {}
  const t = s.adaptiveTriggers && typeof s.adaptiveTriggers === "object" ? s.adaptiveTriggers : {}
  const l = s.lighting && typeof s.lighting === "object" ? s.lighting : {}

  d.haptics.masterEnabled = h.masterEnabled !== false
  d.haptics.surfaceEnabled = h.surfaceEnabled !== false
  d.haptics.impactEnabled = h.impactEnabled !== false

  if (oldSchema >= 10 || oldSchema === 8 || oldSchema === 6 || oldSchema === 4) {
    d.haptics.masterPercent = clampPercent(h.masterPercent, 100)
    d.haptics.surfacePercent = clampPercent(h.surfacePercent, 100)
  } else if (oldSchema === 7 || oldSchema === 5) {
    d.haptics.masterPercent = rescaleReference(h.masterPercent, 65, 100, 65)
    d.haptics.surfacePercent = rescaleReference(h.surfacePercent, 88, 100, 88)
  } else {
    const p = clampPercent(h.masterPercent, 65)
    let gain = 1
    if (p <= 1) gain = 0.05
    else if (p <= 65) gain = 0.05 + (1 - 0.05) * ((p - 1) / 64)
    d.haptics.masterPercent = clampPercent(Math.round(100 * gain), 100)
    d.haptics.surfacePercent = oldSchema >= 3
      ? rescaleReference(h.surfacePercent, 88, 100, 88)
      : clampPercent(h.surfacePercent, 100)
  }

  d.haptics.impactPercent = clampPercent(h.impactPercent, 100)

  for (const surface of surfaces) {
    const stored = h.surfaceProfiles && h.surfaceProfiles[surface.key]
    if (oldSchema >= 10 && stored && typeof stored === "object") {
      d.haptics.surfaceProfiles[surface.key].rollingPercent = clampPercent(stored.rollingPercent, surface.rollingReference)
      d.haptics.surfaceProfiles[surface.key].slipPercent = clampFinePercent(stored.slipPercent, surface.slipReference)
    } else if (oldSchema === 9 && stored && typeof stored === "object") {
      d.haptics.surfaceProfiles[surface.key].rollingPercent = clampPercent(stored.rollingPercent, surface.rollingReference)
      const oldSlip = clampFinePercent(stored.slipPercent, surface.v149Slip)
      d.haptics.surfaceProfiles[surface.key].slipPercent = clampFinePercent(surface.slipReference * oldSlip / surface.v149Slip, surface.slipReference)
    } else if (oldSchema === 8 && stored && typeof stored === "object") {
      d.haptics.surfaceProfiles[surface.key].rollingPercent = rescaleReference(stored.rollingPercent, surface.v148Rolling, surface.rollingReference, surface.v148Rolling)
      d.haptics.surfaceProfiles[surface.key].slipPercent = clampFinePercent(surface.slipReference * clampPercent(stored.slipPercent, surface.v148Slip) / surface.v148Slip, surface.slipReference)
    } else if ((oldSchema === 7 || oldSchema === 5) && stored && typeof stored === "object") {
      d.haptics.surfaceProfiles[surface.key].rollingPercent = rescaleReference(stored.rollingPercent, surface.v147Rolling, surface.rollingReference, surface.v147Rolling)
      d.haptics.surfaceProfiles[surface.key].slipPercent = clampFinePercent(surface.slipReference * clampPercent(stored.slipPercent, surface.v147Slip) / surface.v147Slip, surface.slipReference)
    } else if ((oldSchema === 6 || oldSchema === 4) && stored && typeof stored === "object") {
      d.haptics.surfaceProfiles[surface.key].rollingPercent = rescaleReference(stored.rollingPercent, 100, surface.rollingReference, 100)
      d.haptics.surfaceProfiles[surface.key].slipPercent = clampFinePercent(surface.slipReference * clampPercent(stored.slipPercent, 100) / 100, surface.slipReference)
    } else if (oldSchema >= 3) {
      const oldValue = clampPercent(stored, surface.v147Rolling)
      const gain = oldValue / surface.v147Rolling
      d.haptics.surfaceProfiles[surface.key].rollingPercent = clampPercent(Math.round(surface.rollingReference * gain), surface.rollingReference)
      d.haptics.surfaceProfiles[surface.key].slipPercent = clampFinePercent(surface.slipReference * gain, surface.slipReference)
    }
  }

  d.adaptiveTriggers.l2BrakeEnabled = t.l2BrakeEnabled !== false
  if (oldSchema >= 12) {
    d.adaptiveTriggers.l2BrakeStartPercent = clampPercent(t.l2BrakeStartPercent, 25)
    d.adaptiveTriggers.l2BrakeEndPercent = clampPercent(t.l2BrakeEndPercent, 75)
  } else if (oldSchema >= 10) {
    const legacyStart = clampPercent(t.l2BrakeStartPercent, 13)
    const legacyEnd = clampPercent(t.l2BrakeEndPercent, 50)
    // Migrate only the historical default pair; preserve custom tuning.
    if (legacyStart === 13 && legacyEnd === 50) {
      d.adaptiveTriggers.l2BrakeStartPercent = 25
      d.adaptiveTriggers.l2BrakeEndPercent = 75
    } else {
      d.adaptiveTriggers.l2BrakeStartPercent = legacyStart
      d.adaptiveTriggers.l2BrakeEndPercent = legacyEnd
    }
  } else {
    const legacyL2End = clampPercent(t.l2BrakePercent, 50)
    d.adaptiveTriggers.l2BrakeEndPercent = legacyL2End
    d.adaptiveTriggers.l2BrakeStartPercent = Math.max(1, Math.min(legacyL2End, Math.round(legacyL2End * 0.25)))
  }
  d.adaptiveTriggers.absEnabled = t.absEnabled !== false
  d.adaptiveTriggers.absPercent = clampPercent(t.absPercent, 75)
  d.adaptiveTriggers.r2ThrottleEnabled = t.r2ThrottleEnabled !== false
  if (oldSchema >= 10) {
    d.adaptiveTriggers.r2ThrottleStartPercent = clampPercent(t.r2ThrottleStartPercent, 13)
    d.adaptiveTriggers.r2ThrottleEndPercent = clampPercent(t.r2ThrottleEndPercent, 13)
  } else {
    const legacyR2 = clampPercent(t.r2ThrottlePercent, 13)
    d.adaptiveTriggers.r2ThrottleStartPercent = legacyR2
    d.adaptiveTriggers.r2ThrottleEndPercent = legacyR2
  }
  d.adaptiveTriggers.r2EffectsEnabled = t.r2EffectsEnabled !== false
  const oldR2 = clampPercent(t.r2EffectsPercent, oldSchema >= 3 ? R2_JOLT_REFERENCE_PERCENT : 100)
  d.adaptiveTriggers.r2EffectsPercent = oldSchema >= 3
    ? oldR2
    : clampPercent(Math.round(oldR2 * R2_JOLT_REFERENCE_PERCENT / 100), R2_JOLT_REFERENCE_PERCENT)

  d.lighting.enabled = l.enabled !== false
  d.lighting.brightnessPercent = oldSchema >= 5
    ? clampPercent(l.brightnessPercent, LED_REFERENCE_PERCENT)
    : rescaleReference(l.brightnessPercent, 100, LED_REFERENCE_PERCENT, 100)

  d.schema = SETTINGS_SCHEMA
  return d
}
