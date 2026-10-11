<template>
  <section class="dph-settings">
    <header class="dph-header">
      <h2>Enhanced PS5 DualSense</h2>
      <div class="header-actions">
        <BngButton :accent="ACCENTS.main" class="header-reset" @click="resetAllDefaults">Reset</BngButton>
        <button type="button" class="header-close" aria-label="Close" title="Close" @click="closePanel"><span aria-hidden="true">×</span></button>
      </div>
    </header>

    <div v-if="saveError" class="state-box error" role="alert">{{ saveError }}</div>

    <nav class="suite-tabs" aria-label="DualSense settings sections">
      <button type="button" class="suite-tab" :class="{ active: activeTab === 'haptics' }" @click="activeTab = 'haptics'">Haptics</button>
      <button type="button" class="suite-tab" :class="{ active: activeTab === 'inputs' }" @click="activeTab = 'inputs'">Inputs</button>
      <button type="button" class="suite-tab" :class="{ active: activeTab === 'speaker' }" @click="activeTab = 'speaker'">Speaker</button>
    </nav>

    <div v-if="activeTab === 'haptics'" class="suite-panel">
      <div v-if="loading" class="state-box">Loading settings…</div>
      <div v-else-if="loadError" class="state-box error">{{ loadError }}</div>

      <template v-else>
        <section class="group section-group">
          <h3>Haptics</h3>

          <div class="feature-card">
            <label class="feature-title">
              <input type="checkbox" v-model="values.haptics.masterEnabled" @change="scheduleSave" />
              <span>Haptics</span>
            </label>
            <SettingSlider
              label="Output level"
              :model-value="values.haptics.masterPercent"
              @update:model-value="v => updatePercent(values.haptics, 'masterPercent', v)"
            />
          </div>

          <div class="feature-card">
            <label class="feature-title">
              <input type="checkbox" v-model="values.haptics.surfaceEnabled" @change="scheduleSave" />
              <span>Road surfaces</span>
            </label>
            <SettingSlider
              label="All road surfaces"
              :model-value="values.haptics.surfacePercent"
              @update:model-value="v => updatePercent(values.haptics, 'surfacePercent', v)"
            />

            <button class="advanced-toggle" type="button" @click="advancedSurfacesOpen = !advancedSurfacesOpen">
              <span>Advanced surface settings</span>
              <span>{{ advancedSurfacesOpen ? '▲' : '▼' }}</span>
            </button>

            <div v-if="advancedSurfacesOpen" class="advanced-panel">
              <div v-for="surface in surfaces" :key="surface.key" class="surface-card">
                <div class="surface-title">{{ surface.label }}</div>
                <SettingSlider
                  label="Rolling power"
                  :model-value="values.haptics.surfaceProfiles[surface.key].rollingPercent"
                  compact
                  @update:model-value="v => updateSurface(surface.key, 'rollingPercent', v)"
                />
                <SettingSlider
                  label="Slip power"
                  :model-value="values.haptics.surfaceProfiles[surface.key].slipPercent"
                  compact
                  @update:model-value="v => updateSurface(surface.key, 'slipPercent', v)"
                />
              </div>
            </div>
          </div>

          <div class="feature-card">
            <label class="feature-title">
              <input type="checkbox" v-model="values.haptics.impactEnabled" @change="scheduleSave" />
              <span>Bumps / impacts</span>
            </label>
            <SettingSlider
              label="Strength"
              :model-value="values.haptics.impactPercent"
              @update:model-value="v => updatePercent(values.haptics, 'impactPercent', v)"
            />
          </div>
        </section>

        <section class="group">
          <h3>Adaptive triggers</h3>

          <div class="trigger-card">
            <label class="feature-title compact-title">
              <input type="checkbox" v-model="values.adaptiveTriggers.l2BrakeEnabled" @change="scheduleSave" />
              <span>L2 brake resistance</span>
            </label>
            <SettingSlider
              label="Start resistance"
              :model-value="values.adaptiveTriggers.l2BrakeStartPercent"
              compact
              @update:model-value="v => updateTriggerRange('l2', 'start', v)"
            />
            <SettingSlider
              label="End resistance"
              :model-value="values.adaptiveTriggers.l2BrakeEndPercent"
              compact
              @update:model-value="v => updateTriggerRange('l2', 'end', v)"
            />
          </div>

          <div class="trigger-card">
            <label class="feature-title compact-title">
              <input type="checkbox" v-model="values.adaptiveTriggers.absEnabled" @change="scheduleSave" />
              <span>L2 ABS pulse</span>
            </label>
            <SettingSlider
              label="ABS kick"
              :model-value="values.adaptiveTriggers.absPercent"
              compact
              @update:model-value="v => updatePercent(values.adaptiveTriggers, 'absPercent', v)"
            />
          </div>

          <div class="trigger-card">
            <label class="feature-title compact-title">
              <input type="checkbox" v-model="values.adaptiveTriggers.r2ThrottleEnabled" @change="scheduleSave" />
              <span>R2 throttle resistance</span>
            </label>
            <SettingSlider
              label="Start resistance"
              :model-value="values.adaptiveTriggers.r2ThrottleStartPercent"
              compact
              @update:model-value="v => updateTriggerRange('r2', 'start', v)"
            />
            <SettingSlider
              label="End resistance"
              :model-value="values.adaptiveTriggers.r2ThrottleEndPercent"
              compact
              @update:model-value="v => updateTriggerRange('r2', 'end', v)"
            />
          </div>

          <div class="trigger-card">
            <label class="feature-title compact-title">
              <input type="checkbox" v-model="values.adaptiveTriggers.r2EffectsEnabled" @change="scheduleSave" />
              <span>R2 dynamic effects</span>
            </label>
            <SettingSlider
              label="Jolt strength"
              :model-value="values.adaptiveTriggers.r2EffectsPercent"
              compact
              @update:model-value="v => updatePercent(values.adaptiveTriggers, 'r2EffectsPercent', v)"
            />
          </div>
        </section>

        <section class="group section-group">
          <h3>Lighting</h3>
          <div class="feature-card">
            <label class="feature-title">
              <input type="checkbox" v-model="values.lighting.enabled" @change="scheduleSave" />
              <span>LED lighting</span>
            </label>
            <SettingSlider
              label="Brightness"
              :model-value="values.lighting.brightnessPercent"
              @update:model-value="v => updatePercent(values.lighting, 'brightnessPercent', v)"
            />
          </div>
        </section>
      </template>
    </div>

    <div v-else-if="activeTab === 'inputs'" class="suite-panel embedded-panel">
      <DualSenseExtendedInputsSettings ref="inputsPanelRef" />
    </div>

    <div v-else class="suite-panel embedded-panel">
      <ControllerSpeakerSettings ref="speakerPanelRef" />
    </div>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from "vue"
import { ACCENTS, BngButton } from "@/common/components/base"
import { runRaw } from "@/bridge/libs/Lua.js"
import { getModSetting, setModSetting, flushModSettings } from "./settingsStore.js"
import SettingSlider from "./SettingSlider.vue"
import DualSenseExtendedInputsSettings from "./components/DualSenseExtendedInputsSettings.vue"
import ControllerSpeakerSettings from "./components/ControllerSpeakerSettings.vue"
import { resetExtendedInputsDefaults, resetSpeakerDefaults } from "./suiteDefaults.js"
import {
  SETTINGS_KEY,
  SETTINGS_SCHEMA,
  surfaces,
  createDefaults,
  cloneSettings,
  decodeStored,
  clampPercent,
  clampFinePercent,
  normalizeSettings,
} from "./settingsModel.js"

const activeTab = ref("haptics")
const inputsPanelRef = ref(null)
const speakerPanelRef = ref(null)
const loading = ref(true)
const loadError = ref("")
const saveError = ref("")
const advancedSurfacesOpen = ref(false)
const values = reactive(createDefaults())
let saveTimer = null

function replaceValues(source) {
  const next = normalizeSettings(source)
  for (const key of Object.keys(values)) delete values[key]
  Object.assign(values, next)
}

function snapshot() {
  return cloneSettings(normalizeSettings(values))
}

function updatePercent(target, key, value) {
  target[key] = clampPercent(value)
  scheduleSave()
}

function updateSurface(key, field, value) {
  values.haptics.surfaceProfiles[key][field] = field === "slipPercent"
    ? clampFinePercent(value)
    : clampPercent(value)
  scheduleSave()
}

function updateTriggerRange(trigger, edge, value) {
  const t = values.adaptiveTriggers
  const next = clampPercent(value)
  if (trigger === "l2") {
    if (edge === "start") t.l2BrakeStartPercent = next
    else t.l2BrakeEndPercent = next
  } else {
    if (edge === "start") t.r2ThrottleStartPercent = next
    else t.r2ThrottleEndPercent = next
  }
  scheduleSave()
}

async function loadSettings() {
  loading.value = true
  loadError.value = ""
  try {
    const stored = decodeStored(await getModSetting(SETTINGS_KEY, null))
    const needsMigration = !stored || Number(stored.schema || 1) < SETTINGS_SCHEMA
    replaceValues(stored)
    if (needsMigration) await setModSetting(SETTINGS_KEY, snapshot())
  } catch (error) {
    loadError.value = `Could not load settings: ${error?.message || error}`
  } finally {
    loading.value = false
  }
}

async function saveNow() {
  try {
    await setModSetting(SETTINGS_KEY, snapshot())
    saveError.value = ""
    return true
  } catch (error) {
    saveError.value = "Unable to save DualSense settings — changes are not persistent"
    console.error("[Enhanced PS5 DualSense Haptics] Could not save settings", error)
    return false
  }
}

function scheduleSave() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    saveTimer = null
    void saveNow()
  }, 140)
}

async function resetAllDefaults() {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  try {
    // One reset action covers every setting exposed by the merged UI.
    replaceValues(createDefaults())
    await saveNow()
    await resetExtendedInputsDefaults(runRaw)
    await resetSpeakerDefaults()

    // Refresh whichever child panel is currently mounted so its controls update immediately.
    if (activeTab.value === "inputs" && inputsPanelRef.value?.reloadFromSettings) {
      await inputsPanelRef.value.reloadFromSettings()
    } else if (activeTab.value === "speaker" && speakerPanelRef.value?.reloadFromSettings) {
      await speakerPanelRef.value.reloadFromSettings()
    }
  } catch (error) {
    console.error("[Enhanced PS5 DualSense] Global reset failed", error)
  }
}

async function closePanel() {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
    if (!(await saveNow())) return
  }
  try {
    await flushModSettings()
    await runRaw("extensions.ui_menuManager.closeMenu()")
  } catch (error) {
    console.error("[Enhanced PS5 DualSense Haptics] Could not close settings", error)
    try { window.history.back() } catch {}
  }
}

onMounted(() => { void loadSettings() })
onBeforeUnmount(() => {
  if (saveTimer) clearTimeout(saveTimer)
})
</script>

<style scoped>
.dph-settings,
.dph-settings * { box-sizing: border-box; }

.dph-settings {
  --dph-orange: #ef6b1c;
  --dph-orange-hover: #ff7d2d;
  --dph-bg: rgba(18, 18, 19, 0.98);
  --dph-card: rgba(255, 255, 255, 0.055);
  --dph-card-strong: rgba(255, 255, 255, 0.088);
  --dph-control: rgba(255, 255, 255, 0.065);
  --dph-control-hover: rgba(255, 255, 255, 0.12);
  --dph-line: rgba(255, 255, 255, 0.11);
  --dph-line-soft: rgba(255, 255, 255, 0.07);
  --dph-text: #f5f5f5;
  --dph-muted: #b7bac0;
  --dph-dim: #94989f;

  width: 100%;
  max-width: 640px;
  min-width: 0;
  max-height: calc(100vh - 5.5rem);
  overflow-y: auto;
  overflow-x: hidden;
  margin: 0.35rem auto 0.35rem 0;
  align-self: flex-start;
  padding: 0.72rem;
  color: var(--dph-text);
  background: var(--dph-bg);
  border: 1px solid var(--dph-line);
  border-radius: 0 8px 8px 0;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.34);
  scrollbar-color: var(--dph-orange) rgba(0, 0, 0, 0.22);
  scrollbar-width: thin;
}

.dph-header {
  position: sticky;
  top: -0.72rem;
  z-index: 4;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
  min-height: 3rem;
  margin: -0.72rem -0.72rem 0;
  padding: 0.72rem;
  background: rgba(18, 18, 19, 0.99);
  border-bottom: 1px solid var(--dph-line-soft);
}

.dph-header h2 {
  margin: 0;
  font-size: 1.38rem;
  line-height: 1.15;
  font-weight: 700;
}

.header-actions {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 0.4rem;
}

.header-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.45rem;
  height: 2.45rem;
  padding: 0;
  border: 1px solid var(--dph-line);
  border-radius: 6px;
  background: var(--dph-control);
  color: var(--dph-text);
  font: inherit;
  font-size: 1.66rem;
  font-weight: 400;
  line-height: 1;
  cursor: pointer;
  transition: filter .12s ease, transform .12s ease, border-color .12s ease, background .12s ease;
}

.header-close:hover {
  filter: brightness(1.15);
  transform: translateY(-1px);
  border-color: rgba(255, 255, 255, 0.34);
}

.header-close:active {
  transform: translateY(0) scale(.985);
}

.header-close:focus-visible {
  outline: 2px solid var(--dph-orange);
  outline-offset: 2px;
}

.header-close > span {
  display: block;
  line-height: 1;
  transform: translateY(-1px);
}

.suite-tabs {
  position: sticky;
  top: 3rem;
  z-index: 3;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.34rem;
  margin: 0 -0.72rem 0.65rem;
  padding: 0.5rem 0.72rem;
  background: rgba(18, 18, 19, 0.98);
  border-bottom: 1px solid var(--dph-line-soft);
}

.suite-tab,
.advanced-toggle {
  min-height: 2.45rem;
  border: 1px solid transparent;
  border-radius: 6px;
  background: var(--dph-control);
  color: var(--dph-muted);
  font: inherit;
  font-size: 1.08rem;
  font-weight: 650;
  cursor: pointer;
  transition: background 0.14s ease, color 0.14s ease, border-color 0.14s ease, transform 0.10s ease;
}

.suite-tab:hover,
.advanced-toggle:hover {
  background: var(--dph-control-hover);
  color: #fff;
}

.suite-tab:active,
.advanced-toggle:active { transform: scale(0.99); }

.suite-tab:focus-visible,
.advanced-toggle:focus-visible {
  outline: 2px solid var(--dph-orange);
  outline-offset: 2px;
}

.suite-tab.active {
  background: var(--dph-orange);
  border-color: var(--dph-orange);
  color: #fff;
}

.suite-panel { min-width: 0; }

.group {
  margin-top: 0.72rem;
  padding: 0.86rem 0.9rem;
  border-radius: 8px;
  background: var(--dph-card);
  border: 1px solid var(--dph-line-soft);
}

.section-group {
  display: grid;
  gap: 0.68rem;
}

.group h3 {
  margin: 0 0 0.62rem;
  font-size: 1.28rem;
  line-height: 1.2;
  font-weight: 700;
  color: #f1f1f1;
}

.feature-title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0 0 0.42rem;
  cursor: pointer;
  font-size: 1.18rem;
  line-height: 1.28;
  font-weight: 700;
  color: #f5f5f5;
}

.feature-title input { width: 1.18rem; height: 1.18rem; margin: 0; accent-color: var(--dph-orange); flex: 0 0 auto; }

.sub-feature {
  margin-top: 0.72rem;
  padding-top: 0.65rem;
  border-top: 1px solid rgba(255, 255, 255, 0.055);
  font-size: 1.08rem;
}

.compact-title {
  margin-bottom: 0.36rem;
  font-size: 1.18rem;
}

.advanced-toggle {
  box-sizing: border-box;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  margin: 0.25rem 0 0.55rem;
  padding: 0 0.68rem;
}

.advanced-toggle span:last-child { color: var(--dph-orange); }

.feature-card,
.trigger-card {
  min-width: 0;
  padding: 0.74rem 0.78rem;
  background: var(--dph-card-strong);
  border-radius: 7px;
  border: 1px solid rgba(255, 255, 255, 0.09);
}

.feature-card + .feature-card,
.trigger-card + .trigger-card {
  margin-top: 0.58rem;
}

.feature-card .advanced-panel {
  margin-top: 0.58rem;
}

.advanced-panel {
  margin: -0.1rem 0 0.6rem;
  padding: 0.62rem;
  background: var(--dph-card-strong);
  border-radius: 6px;
  border: 1px solid var(--dph-line-soft);
}

.surface-card {
  min-width: 0;
  margin-top: 0.48rem;
  padding: 0.64rem;
  background: rgba(255, 255, 255, 0.062);
  border-radius: 6px;
  border: 1px solid var(--dph-line-soft);
}

.surface-card:first-child { margin-top: 0; }

.surface-title {
  margin-bottom: 0.25rem;
  font-size: 1.08rem;
  font-weight: 700;
  color: #f2f2f2;
}

.state-box {
  margin-top: 0.6rem;
  padding: 0.7rem;
  border-radius: 7px;
  background: var(--dph-card);
  border: 1px solid var(--dph-line-soft);
  font-size: 0.9rem;
  color: var(--dph-muted);
}

.state-box.error {
  color: #ffba96;
  border-color: rgba(255, 154, 105, 0.28);
  background: rgba(123, 45, 22, 0.25);
}

.embedded-panel :deep(.dse-settings),
.embedded-panel :deep(.speaker-settings) {
  width: 100%;
  max-width: none;
  max-height: none;
  margin: 0;
  padding: 0;
  overflow: visible;
  background: transparent;
  border: 0;
  border-radius: 0;
  box-shadow: none;
}

.embedded-panel :deep(.dse-header),
.embedded-panel :deep(.speaker-settings > .header) { display: none; }

@media (max-width: 560px) {
  .dph-settings { width: 100%; max-width: none; margin: 0; border-radius: 0; padding: 0.58rem; }
  .dph-header { top: -0.58rem; margin: -0.58rem -0.58rem 0; padding: 0.58rem; }
  .suite-tabs { margin-left: -0.58rem; margin-right: -0.58rem; padding-left: 0.58rem; padding-right: 0.58rem; }
}
</style>

<style>
/* Shared orange/black/white visual system for the embedded Inputs and Speaker panels.
   Content and behaviour stay owned by the original child components. */
.dph-settings .embedded-panel .dse-header,
.dph-settings .embedded-panel .speaker-settings > .header { display: none !important; }

.dph-settings .embedded-panel .dse-settings,
.dph-settings .embedded-panel .speaker-settings {
  width: 100% !important;
  min-width: 0 !important;
  max-width: 100% !important;
  max-height: none !important;
  margin: 0 !important;
  padding: 0 !important;
  overflow: visible !important;
  color: #f5f5f5 !important;
  background: transparent !important;
  border: 0 !important;
  box-shadow: none !important;
}

.dph-settings .embedded-panel .group {
  width: 100%;
  min-width: 0;
  margin: 0.72rem 0 0;
  padding: 0.86rem 0.9rem;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.055);
  border: 1px solid rgba(255, 255, 255, 0.07);
  overflow: hidden;
}

.dph-settings .embedded-panel .group h3 {
  margin: 0 0 0.58rem;
  font-size: 1.28rem;
  line-height: 1.2;
  font-weight: 700;
  color: #f1f1f1;
}

.dph-settings .embedded-panel .feature-title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0;
  cursor: pointer;
  font-size: 1.14rem;
  line-height: 1.28;
  font-weight: 700;
  color: #f4f4f4;
}

.dph-settings .embedded-panel input[type="checkbox"] { width: 1.18rem; height: 1.18rem; accent-color: #ef6b1c; flex: 0 0 auto; }

.dph-settings .embedded-panel button {
  min-height: 2.3rem;
  border: 1px solid transparent;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.06);
  color: #b7bac0;
  font: inherit;
  font-size: 0.9rem;
  font-weight: 650;
  cursor: pointer;
}

.dph-settings .embedded-panel button:hover { background: rgba(255, 255, 255, 0.105); color: #fff; }
.dph-settings .embedded-panel button:focus-visible { outline: 2px solid #ef6b1c; outline-offset: 2px; }
.dph-settings .embedded-panel button:disabled { opacity: 0.42; cursor: default; }

.dph-settings .dse-settings .toggle-button.active,
.dph-settings .dse-settings .small-button.active,
.dph-settings .dse-settings .segment-choice.active,
.dph-settings .dse-settings .inline-picker-choice.active,
.dph-settings .dse-settings .mapping-choice.active,
.dph-settings .dse-settings .picker-button.open,
.dph-settings .speaker-settings .choice.active {
  background: #ef6b1c !important;
  border-color: #ef6b1c !important;
  color: #fff !important;
}

.dph-settings .dse-settings .state-box {
  margin: 0.55rem 0;
  padding: 0.7rem;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.055);
  border: 1px solid rgba(255, 255, 255, 0.07);
  font-size: 0.9rem;
  color: #b7bac0;
}

.dph-settings .dse-settings .error-block { color: #ffba96; }
.dph-settings .dse-settings .error-row,
.dph-settings .dse-settings .group-title-row,
.dph-settings .dse-settings .section-heading { display: flex; align-items: center; justify-content: space-between; gap: 0.55rem; min-width: 0; }
.dph-settings .dse-settings .group-title-row h3 { margin: 0; }
.dph-settings .dse-settings .mapping-divider { height: 1px; margin: 0.72rem 0; background: rgba(255, 255, 255, 0.07); }

.dph-settings .dse-settings .custom-panel { display: grid; gap: 0.58rem; }
.dph-settings .dse-settings .finger-group { display: grid; gap: 0.44rem; padding: 0.72rem 0.76rem; border-radius: 7px; background: rgba(255, 255, 255, 0.078); border: 1px solid rgba(255, 255, 255, 0.085); }
.dph-settings .dse-settings .finger-title { margin-bottom: 0.1rem; font-size: 0.9rem; font-weight: 700; color: #a8abb1; text-transform: uppercase; letter-spacing: 0.04em; }
.dph-settings .dse-settings .mapping-row { display: grid; grid-template-columns: minmax(6.3rem, 0.82fr) minmax(0, 2.18fr); align-items: center; gap: 0.58rem; min-width: 0; }
.dph-settings .dse-settings .mapping-row + .mapping-row { padding-top: 0.42rem; border-top: 1px solid rgba(255, 255, 255, 0.06); }
.dph-settings .dse-settings .channel-name { display: flex; align-items: baseline; gap: 0.35rem; min-width: 0; font-size: 1.08rem; color: #d6d8dc; }
.dph-settings .dse-settings .channel-name small { font-size: 0.83rem; color: #999da4; }
.dph-settings .dse-settings .mapping-segments { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 0.3rem; min-width: 0; }
.dph-settings .dse-settings .mapping-choice { padding: 0 0.38rem; font-size: 0.86rem; }

.dph-settings .dse-settings .sensor-slots { display: grid; gap: 0.58rem; margin-top: 0.58rem; min-width: 0; }
.dph-settings .dse-settings .sensor-slot,
.dph-settings .dse-settings .acceleration-slot { min-width: 0; padding: 0.72rem 0.76rem; border-radius: 7px; background: rgba(255, 255, 255, 0.078); border: 1px solid rgba(255, 255, 255, 0.085); overflow: hidden; }
.dph-settings .dse-settings .sensor-row,
.dph-settings .dse-settings .accel-row { display: grid; grid-template-columns: minmax(6.8rem, 0.72fr) minmax(0, 2.28fr); gap: 0.7rem; align-items: center; min-width: 0; }
.dph-settings .dse-settings .sensor-label,
.dph-settings .dse-settings .accel-label { display: flex; flex-direction: column; min-width: 0; }
.dph-settings .dse-settings .sensor-label strong,
.dph-settings .dse-settings .accel-label strong { font-size: 1.14rem; line-height: 1.24; color: #f2f2f2; }
.dph-settings .dse-settings .binding-label { margin-top: 0.16rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 0.82rem; color: #b8bbc1; }
.dph-settings .dse-settings .binding-label.muted { color: #858990; }
.dph-settings .dse-settings .sensor-controls { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.36rem; align-items: center; min-width: 0; }
.dph-settings .dse-settings .sensor-controls.compact { grid-template-columns: minmax(0, 1fr); }
.dph-settings .dse-settings .picker-button,
.dph-settings .dse-settings .advanced-toggle { width: 100%; min-width: 0; font-size: 0.88rem; }
.dph-settings .dse-settings .picker-button { display: flex; align-items: center; justify-content: space-between; gap: 0.4rem; text-align: left; }
.dph-settings .dse-settings .advanced-toggle { text-align: center; white-space: nowrap; overflow: hidden; }

.dph-settings .dse-settings .inline-picker-panel,
.dph-settings .dse-settings .advanced-panel { width: 100%; min-width: 0; margin-top: 0.56rem; padding: 0.66rem; border-radius: 7px; background: rgba(255, 255, 255, 0.054); border: 1px solid rgba(255, 255, 255, 0.076); }
.dph-settings .dse-settings .inline-picker-options { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0.3rem; min-width: 0; }
.dph-settings .dse-settings .inline-picker-choice { min-width: 0; font-size: 0.86rem; }
.dph-settings .dse-settings .output-preview { display: grid; grid-template-columns: 4.2rem minmax(4rem, 1fr) 3.6rem; align-items: center; gap: 0.55rem; margin-bottom: 0.6rem; font-size: 0.83rem; color: #b4b7bd; }
.dph-settings .dse-settings .output-preview strong { text-align: right; font-size: 0.84rem; color: #fff; }
.dph-settings .dse-settings .output-meter { position: relative; height: 0.36rem; border-radius: 999px; background: rgba(255, 255, 255, 0.09); }
.dph-settings .dse-settings .output-center { position: absolute; left: 50%; top: -0.14rem; bottom: -0.14rem; width: 1px; background: rgba(255, 255, 255, 0.25); }
.dph-settings .dse-settings .output-marker { position: absolute; top: 50%; width: 0.62rem; height: 0.62rem; border-radius: 50%; background: #ef6b1c; transform: translate(-50%, -50%); transition: left 70ms linear; }

.dph-settings .dse-settings .advanced-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.62rem 0.72rem; min-width: 0; }
.dph-settings .dse-settings .advanced-field { display: flex; flex-direction: column; gap: 0.3rem; min-width: 0; font-size: 0.89rem; color: #c1c3c8; }
.dph-settings .dse-settings .advanced-field > span,
.dph-settings .dse-settings .custom-axis-row > span { font-size: 0.84rem; color: #9fa2a8; }
.dph-settings .dse-settings .full-field { grid-column: 1 / -1; }
.dph-settings .dse-settings .advanced-field input,
.dph-settings .dse-settings .custom-axis-row input,
.dph-settings .dse-settings .idle-input { width: 100%; min-width: 0; min-height: 2.45rem; padding: 0.42rem 0.55rem; border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 6px; color: #fff; background: rgba(0, 0, 0, 0.28); font: inherit; font-size: 0.9rem; text-align: center; }
.dph-settings .dse-settings .advanced-field input:focus,
.dph-settings .dse-settings .custom-axis-row input:focus,
.dph-settings .dse-settings .idle-input:focus { outline: 2px solid #ef6b1c; outline-offset: 1px; border-color: #ef6b1c; }
.dph-settings .dse-settings .unit-input { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 0.32rem; min-width: 0; }
.dph-settings .dse-settings .unit-input small,
.dph-settings .dse-settings .g-value { font-size: 0.82rem; color: #9a9da3; }
.dph-settings .dse-settings .segment-row { display: grid; gap: 0.3rem; min-width: 0; }
.dph-settings .dse-settings .segment-row.three { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.dph-settings .dse-settings .segment-row.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.dph-settings .dse-settings .segment-choice { min-width: 0; padding: 0 0.38rem; font-size: 0.86rem; text-align: center; overflow: hidden; text-overflow: ellipsis; }
.dph-settings .dse-settings .custom-axis-row { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: center; gap: 0.4rem; margin-top: 0.52rem; min-width: 0; font-size: 0.84rem; }
.dph-settings .dse-settings .custom-axis-row > span { grid-column: 1 / -1; }
.dph-settings .dse-settings .custom-axis-row label { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 0.28rem; min-width: 0; }
.dph-settings .dse-settings .advanced-footer { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 0.4rem; margin-top: 0.55rem; }
.dph-settings .dse-settings .reset-button { background: transparent; color: #b7bac0; font-size: 0.88rem; }
.dph-settings .dse-settings .accel-controls { display: grid; grid-template-columns: 1fr; gap: 0.36rem; align-items: center; min-width: 0; }
.dph-settings .dse-settings .accel-advanced { display: flex; align-items: center; justify-content: center; gap: 0.4rem; }
.dph-settings .dse-settings .accel-mode-row { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0.32rem; min-width: 0; }
.dph-settings .dse-settings .idle-row { display: flex; align-items: center; justify-content: flex-end; gap: 0.38rem; min-width: 0; }
.dph-settings .dse-settings .idle-input { width: 5.8rem; }
.dph-settings .dse-settings .idle-unit,
.dph-settings .dse-settings .range-hint { font-size: 0.82rem; color: #9fa2a8; }
.dph-settings .dse-settings .number-stepper { display: grid; grid-template-columns: 2.35rem minmax(4rem, 1fr) 2.35rem auto; align-items: center; gap: 0.34rem; min-width: 0; }
.dph-settings .dse-settings .stepper-button { width: 2.45rem; min-width: 2.45rem; height: 2.45rem; padding: 0; font-size: 1.28rem; line-height: 1; user-select: none; touch-action: none; }
.dph-settings .dse-settings .number-stepper input[type="number"] { appearance: textfield; -moz-appearance: textfield; }
.dph-settings .dse-settings .number-stepper input[type="number"]::-webkit-inner-spin-button,
.dph-settings .dse-settings .number-stepper input[type="number"]::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }
.dph-settings .dse-settings .sleep-settings { display: grid; gap: 0.58rem; margin-top: 0.58rem; }
.dph-settings .dse-settings .sleep-toggle { flex: 0 0 auto; }
.dph-settings .dse-settings .sleep-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 0.7rem; min-width: 0; font-size: 0.9rem; color: #d1d3d7; }
.dph-settings .dse-settings .sleep-stepper { width: min(100%, 16rem); grid-template-columns: 2.35rem minmax(4.6rem, 1fr) 2.35rem auto; }

.dph-settings .speaker-settings .group { margin: 0.72rem 0 0; padding: 0.86rem 0.9rem; border-radius: 8px; background: rgba(255, 255, 255, 0.055); border: 1px solid rgba(255, 255, 255, 0.07); }
.dph-settings .speaker-settings .group h3 { margin: 0 0 0.58rem; font-size: 1.28rem; font-weight: 700; color: #f1f1f1; }
.dph-settings .speaker-settings .feature-title { display: flex; align-items: center; gap: 0.5rem; margin: 0; cursor: pointer; font-size: 1.14rem; font-weight: 700; line-height: 1.28; }
.dph-settings .speaker-settings .choice-row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.34rem; }
.dph-settings .speaker-settings .choice { min-height: 2.45rem; padding: 0 0.56rem; font-size: 0.9rem; }
.dph-settings .speaker-settings .stepper { display: grid; grid-template-columns: 2.35rem minmax(5rem, 8rem) 2.35rem; align-items: center; gap: 0.34rem; }
.dph-settings .speaker-settings .step { width: 2.45rem; height: 2.45rem; padding: 0; font-size: 1.28rem; user-select: none; touch-action: none; }
.dph-settings .speaker-settings .value { height: 2.45rem; display: flex; align-items: center; justify-content: center; border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 6px; background: rgba(0, 0, 0, 0.28); font-size: 1.08rem; color: #fff; }
.dph-settings .speaker-settings .status-row { min-height: 1rem; margin-top: 0.45rem; color: #999da4; font-size: 0.82rem; }
.dph-settings .speaker-settings .error { margin-top: 0.55rem; padding: 0.7rem; border-radius: 6px; background: rgba(123, 45, 22, 0.25); border: 1px solid rgba(255, 154, 105, 0.28); color: #ffba96; }

@media (max-width: 600px) {
  .dph-settings .dse-settings .sensor-row,
  .dph-settings .dse-settings .accel-row { grid-template-columns: 1fr; gap: 0.4rem; }
  .dph-settings .dse-settings .sensor-controls { grid-template-columns: repeat(2, minmax(0, 1fr)); width: 100%; }
  .dph-settings .dse-settings .advanced-grid { grid-template-columns: 1fr; }
  .dph-settings .speaker-settings .choice-row { grid-template-columns: 1fr; }
}

@media (max-width: 430px) {
  .dph-settings .dse-settings .mapping-row { grid-template-columns: 1fr; gap: 0.34rem; }
  .dph-settings .dse-settings .mapping-segments { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .dph-settings .dse-settings .inline-picker-options,
  .dph-settings .dse-settings .custom-axis-row { grid-template-columns: 1fr; }
  .dph-settings .dse-settings .full-field,
  .dph-settings .dse-settings .custom-axis-row > span { grid-column: auto; }
}
</style>
