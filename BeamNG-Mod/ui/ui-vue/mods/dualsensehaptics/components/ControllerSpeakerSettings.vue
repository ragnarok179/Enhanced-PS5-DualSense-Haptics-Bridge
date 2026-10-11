<template>
  <section class="speaker-settings">
    <header class="header">
      <div>
        <h2>DualSense Speaker</h2>
        <p>BeamNG collision audio through the controller speaker.</p>
      </div>
    </header>

    <div v-if="loadError" class="error">{{ loadError }}</div>

    <div class="group">
      <label class="feature-title">
        <input type="checkbox" :checked="state.enabled" :disabled="loading" @change="setBoolean('enabled', $event.target.checked)" />
        <span>Controller speaker</span>
      </label>
    </div>

    <div class="group">
      <h3>Volume</h3>
      <div class="stepper">
        <button type="button" class="step" :disabled="loading || !state.enabled" @pointerdown="startVolumeStep($event, -1)" @pointerup="stopVolumeStep" @pointercancel="stopVolumeStep" @lostpointercapture="stopVolumeStep">−</button>
        <div class="value">{{ state.volume }}%</div>
        <button type="button" class="step" :disabled="loading || !state.enabled" @pointerdown="startVolumeStep($event, 1)" @pointerup="stopVolumeStep" @pointercancel="stopVolumeStep" @lostpointercapture="stopVolumeStep">+</button>
      </div>
    </div>

    <div class="group">
      <h3>Output</h3>
      <div class="choice-row">
        <button type="button" class="choice" :disabled="loading || !state.enabled" :class="{ active: state.output === 0 }" @click="setOutput(0)">Controller only</button>
        <button type="button" class="choice" :disabled="loading || !state.enabled" :class="{ active: state.output === 1 }" @click="setOutput(1)">Controller + PC</button>
      </div>
    </div>

    <div v-if="saveError" class="status-row error" aria-live="polite">
      <span>{{ saveError }}</span>
    </div>
  </section>
</template>

<script setup>
import { reactive, ref, onMounted, onBeforeUnmount } from "vue"
import { SPEAKER_DEFAULTS, SPEAKER_SETTING_KEYS, SPEAKER_SETTINGS_KEY, resetSpeakerDefaults } from "../suiteDefaults.js"
import { getModSetting, setModSetting, flushModSettings } from "../settingsStore.js"

const SETTING_KEYS = SPEAKER_SETTING_KEYS // V1.42 legacy migration only.
const state = reactive({ ...SPEAKER_DEFAULTS })
const loading = ref(true), loadError = ref(""), saveError = ref("")
let repeatDelay = null, repeatTimer = null, revision = 0
const clamp = (v,min,max) => Math.max(min, Math.min(max,v))

function boolValue(value, fallback) {
  if (value === undefined || value === null || value === "") return fallback
  return value === true || value === 1 || value === "1" || String(value).toLowerCase() === "true"
}
function normalizeSpeaker(raw) {
  const s = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const volume = Number(s.volume)
  return {
    enabled: boolValue(s.enabled, SPEAKER_DEFAULTS.enabled),
    volume: Number.isFinite(volume) ? clamp(Math.round(volume), 0, 100) : SPEAKER_DEFAULTS.volume,
    collision: boolValue(s.collision, SPEAKER_DEFAULTS.collision),
    output: Number(s.output) === 1 ? 1 : 0,
  }
}
function sameSpeaker(a,b) {
  return a && b && a.enabled === b.enabled && a.collision === b.collision
    && a.volume === b.volume && a.output === b.output
}

async function readLegacyState() {
  const [enabled, volume, collision, output] = await Promise.all([
    getModSetting(SETTING_KEYS.enabled, SPEAKER_DEFAULTS.enabled),
    getModSetting(SETTING_KEYS.volume, SPEAKER_DEFAULTS.volume),
    getModSetting(SETTING_KEYS.collision, SPEAKER_DEFAULTS.collision),
    getModSetting(SETTING_KEYS.output, SPEAKER_DEFAULTS.output),
  ])
  return normalizeSpeaker({ enabled, volume, collision, output })
}

async function loadState() {
  const openedRevision = revision
  loading.value = true
  loadError.value = ""
  try {
    // Haptics already persists one JSON object reliably. Speaker now uses
    // the same transaction shape, with the old four keys as read-only fallback.
    await flushModSettings()
    const saved = await getModSetting(SPEAKER_SETTINGS_KEY, null)
    const next = saved && typeof saved === "object" && !Array.isArray(saved)
      ? normalizeSpeaker(saved) : await readLegacyState()
    if (revision === openedRevision) Object.assign(state, next)
  } catch (error) {
    loadError.value = "Unable to load speaker settings"
    console.error("[DualSense Speaker] load failed", error)
  } finally {
    loading.value = false
  }
}

async function commit(change) {
  if (loading.value) return
  const previous = { ...state }
  Object.assign(state, change)
  const snapshot = { ...state }
  const myRevision = ++revision
  try {
    await setModSetting(SPEAKER_SETTINGS_KEY, snapshot)
    if (revision !== myRevision) return // a newer user edit has superseded this write
    // An acknowledged write is not enough: detect VFS/Lua failures instead
    // of silently showing defaults after the next page mount.
    const stored = await getModSetting(SPEAKER_SETTINGS_KEY, null)
    if (revision !== myRevision) return
    if (!sameSpeaker(normalizeSpeaker(stored), snapshot) || !stored || typeof stored !== "object") {
      throw new Error("Speaker settings read-back mismatch")
    }
    if (revision === myRevision) saveError.value = ""
  } catch (error) {
    if (revision === myRevision) {
      Object.assign(state, previous)
      saveError.value = "Unable to save speaker settings"
    }
    console.error("[DualSense Speaker] persistence failed", error)
  }
}

function setBoolean(key, value) { void commit({[key]: !!value}) }
function setOutput(value) { if (state.enabled) void commit({output: value === 1 ? 1 : 0}) }
function volumeStep(direction) {
  if (state.enabled) void commit({volume: clamp(state.volume + direction * 5, 0, 100)})
}
function stopVolumeStep(){ clearTimeout(repeatDelay); clearInterval(repeatTimer); repeatDelay=null; repeatTimer=null }
function startVolumeStep(event,direction){
  event.preventDefault(); stopVolumeStep(); volumeStep(direction)
  if(event.currentTarget?.setPointerCapture && event.pointerId!=null) try{event.currentTarget.setPointerCapture(event.pointerId)}catch{}
  repeatDelay=setTimeout(()=>{ repeatTimer=setInterval(()=>volumeStep(direction),90) },330)
}

async function resetDefaults() {
  stopVolumeStep()
  await flushModSettings()
  await resetSpeakerDefaults()
  const stored = await getModSetting(SPEAKER_SETTINGS_KEY, null)
  if (!stored || typeof stored !== "object" || !sameSpeaker(normalizeSpeaker(stored), SPEAKER_DEFAULTS)) {
    throw new Error("Speaker defaults were not saved")
  }
  revision++
  Object.assign(state, SPEAKER_DEFAULTS)
  saveError.value = ""
}
async function reloadFromSettings() { await loadState() }
defineExpose({ resetDefaults, reloadFromSettings })
onMounted(loadState)
onBeforeUnmount(stopVolumeStep)
</script>
