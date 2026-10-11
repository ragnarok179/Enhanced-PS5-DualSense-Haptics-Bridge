<template>
  <section class="dse-settings" @pointerdown.capture="handleSettingsPointerDown" @keydown.capture="handleSettingsKeyCapture" @keyup.capture="handleSettingsKeyCapture">
    <header class="dse-header">
      <h2>DualSense Extended Inputs</h2>
      <BngButton :accent="ACCENTS.outlined" @click="closePanel">Close</BngButton>
    </header>

    <div v-if="loading" class="state-box">Loading…</div>
    <div v-else-if="loadError" class="state-box error-block">
      <div class="error-row"><span>{{ loadError }}</span><button type="button" class="small-button" @click="loadSettings">Retry</button></div>
    </div>

    <template v-else>
      <div class="group mapping-group">
        <div class="group-title-row">
          <h3>Touchpad</h3>
          <button type="button" class="toggle-button" :class="{ active: mouseMode }" @click="setMouseMode(!mouseMode)">
            Mouse {{ mouseMode ? "On" : "Off" }}
          </button>
        </div>

        <div class="mapping-divider"></div>
        <div class="custom-panel">
          <div v-for="group in customGroups" :key="group.key" class="finger-group">
            <div class="finger-title">{{ group.label }}</div>
            <div v-for="channel in group.channels" :key="channel.key" class="mapping-row">
              <span class="channel-name"><strong>{{ channel.label }}</strong><small>{{ channel.axis }}</small></span>
              <div class="mapping-segments">
                <button
                  v-for="option in mappingOptions"
                  :key="option.key"
                  type="button"
                  class="mapping-choice"
                  :class="{ active: customMapping[channel.key] === option.key }"
                  @click="setCustomChannel(channel.key, option.key)"
                >{{ option.label }}</button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="group motion-group">
        <div class="group-title-row"><h3>Motion</h3></div>

        <div class="sensor-slots">
          <div v-for="slot in motionSlots" :key="slot.index" class="sensor-slot">
            <div class="sensor-row">
              <div class="sensor-label">
                <strong>Motion {{ slot.index }}</strong>
                <span class="binding-label" :class="{ muted: !motionBinding(slot) }">{{ motionBinding(slot) || "Unbound" }}</span>
              </div>

              <div class="sensor-controls" :class="{ compact: slot.mode === 'off' }">
                <button type="button" class="picker-button" :class="{ open: isMotionMenuOpen(slot, 'mode') }" @click="toggleMotionMenu(slot, 'mode')">
                  <span>{{ optionLabel(motionModeOptions, slot.mode) }}</span><span>▾</span>
                </button>

                <template v-if="slot.mode !== 'off'">
                  <button type="button" class="picker-button" :class="{ open: isMotionMenuOpen(slot, 'axis') }" @click="toggleMotionMenu(slot, 'axis')">
                    <span>{{ optionLabel(motionAxisOptions(slot.mode), slot.axis) }}</span><span>▾</span>
                  </button>

                  <button
                    v-if="slot.mode === 'rotation' && slot.axis === 'auto'"
                    type="button"
                    class="small-button"
                    :class="{ active: slot.calibrating }"
                    :disabled="slot.calibrating || !sensorConnected"
                    @click="startMotionCalibration(slot)"
                  >{{ slot.calibrating ? `Turn both ways… ${Math.round(slot.progress * 100)}%` : (slot.learned ? "Recalibrate" : "Calibrate") }}</button>
                  <button
                    v-else-if="needsCenter(slot.mode)"
                    type="button"
                    class="small-button"
                    :disabled="!sensorConnected"
                    @click="recenterMotion(slot)"
                  >Center</button>

                  <button type="button" class="advanced-toggle" @click="slot.advancedOpen = !slot.advancedOpen">
                    Advanced <span>{{ slot.advancedOpen ? "▴" : "▾" }}</span>
                  </button>
                </template>
              </div>
            </div>

            <div v-if="isMotionMenuOpen(slot, 'mode')" class="inline-picker-panel">
              <div class="inline-picker-options mode-options">
                <button v-for="option in motionModeOptions" :key="option.key" type="button" class="inline-picker-choice" :class="{ active: slot.mode === option.key }" @click="chooseMotionOption(slot, 'mode', option.key)">{{ option.label }}</button>
              </div>
            </div>
            <div v-if="slot.mode !== 'off' && isMotionMenuOpen(slot, 'axis')" class="inline-picker-panel">
              <div class="inline-picker-options">
                <button v-for="option in motionAxisOptions(slot.mode)" :key="option.key" type="button" class="inline-picker-choice" :class="{ active: slot.axis === option.key }" @click="chooseMotionOption(slot, 'axis', option.key)">{{ option.label }}</button>
              </div>
            </div>

            <div v-if="slot.advancedOpen && slot.mode !== 'off'" class="advanced-panel">
              <div class="output-preview">
                <span>Live</span>
                <div class="output-meter"><span class="output-center"></span><span class="output-marker" :style="{ left: `${outputPosition(slot.output)}%` }"></span></div>
                <strong>{{ outputText(slot.output) }}</strong>
              </div>
              <div class="advanced-grid">
                <label class="advanced-field full-field">
                  <span>Output type</span>
                  <div class="segment-row two">
                    <button v-for="option in motionOutputOptions" :key="option.key" type="button" class="segment-choice" :class="{ active: slot.outputMode === option.key }" @click="setMotionOption(slot, 'outputMode', option.key)">{{ option.label }}</button>
                  </div>
                </label>

                <label v-if="slot.outputMode === 'axis'" class="advanced-field">
                  <span>Direction</span>
                  <button type="button" class="toggle-button mini" :class="{ active: slot.invert }" @click="setMotionOption(slot, 'invert', !slot.invert)">{{ slot.invert ? "Inverted" : "Normal" }}</button>
                </label>

                <label v-if="slot.outputMode === 'axis' && slot.mode !== 'gyro'" class="advanced-field">
                  <span>Deadzone</span>
                  <div class="unit-input number-stepper"><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button><input class="numeric-edit" type="number" min="0" max="30" step="1" :value="numericDraftValue(sensorEditKey('M', slot, 'deadzone'), Math.round(slot.deadzone * 100))" @focus="beginSensorEdit('M', slot, 'deadzone', $event)" @input="updateNumericDraft(sensorEditKey('M', slot, 'deadzone'), $event)" @blur="commitMotionPercent(slot, 'deadzone', $event, 30)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button><small>%</small></div>
                </label>

                <label v-if="slot.outputMode === 'axis'" class="advanced-field">
                  <span>Stability</span>
                  <div class="unit-input number-stepper"><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button><input class="numeric-edit" type="number" min="0" max="100" step="1" :value="numericDraftValue(sensorEditKey('M', slot, 'stability'), Math.round(slot.stability))" @focus="beginSensorEdit('M', slot, 'stability', $event)" @input="updateNumericDraft(sensorEditKey('M', slot, 'stability'), $event)" @blur="commitMotionNumber(slot, 'stability', $event, 0, 100)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button><small>%</small></div>
                </label>

                <label class="advanced-field">
                  <span>{{ motionRangeLabel(slot) }}</span>
                  <div class="unit-input number-stepper"><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button><input class="numeric-edit" type="number" min="1" :max="slot.mode === 'gyro' ? 2048 : 180" step="1" :value="numericDraftValue(sensorEditKey('M', slot, 'range'), Math.round(slot.range))" @focus="beginSensorEdit('M', slot, 'range', $event)" @input="updateNumericDraft(sensorEditKey('M', slot, 'range'), $event)" @blur="commitMotionNumber(slot, 'range', $event, 1, slot.mode === 'gyro' ? 2048 : 180)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button><small>{{ motionRangeUnit(slot) }}</small></div>
                </label>

                <label v-if="slot.outputMode === 'button'" class="advanced-field">
                  <span>Activation</span>
                  <div class="unit-input number-stepper"><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button><input class="numeric-edit" type="number" min="1" max="100" step="1" :value="numericDraftValue(sensorEditKey('M', slot, 'buttonThresholdPercent'), Math.round(slot.buttonThresholdPercent))" @focus="beginSensorEdit('M', slot, 'buttonThresholdPercent', $event)" @input="updateNumericDraft(sensorEditKey('M', slot, 'buttonThresholdPercent'), $event)" @blur="commitMotionNumber(slot, 'buttonThresholdPercent', $event, 1, 100)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button><small>%</small></div>
                </label>

                <label v-if="slot.outputMode === 'button'" class="advanced-field full-field">
                  <span>Direction</span>
                  <div class="segment-row three">
                    <button v-for="option in motionButtonDirectionOptions" :key="option.key" type="button" class="segment-choice" :class="{ active: slot.buttonDirection === option.key }" @click="setMotionOption(slot, 'buttonDirection', option.key)">{{ option.label }}</button>
                  </div>
                </label>

                <label v-if="slot.outputMode === 'button'" class="advanced-field full-field">
                  <span>Behavior</span>
                  <div class="segment-row two">
                    <button v-for="option in motionButtonBehaviorOptions" :key="option.key" type="button" class="segment-choice" :class="{ active: slot.buttonBehavior === option.key }" @click="setMotionOption(slot, 'buttonBehavior', option.key)">{{ option.label }}</button>
                  </div>
                </label>

                <label v-if="slot.mode === 'gyro' && (slot.axis === 'horizontal' || slot.axis === 'vertical')" class="advanced-field full-field">
                  <span>Reference</span>
                  <div class="segment-row three">
                    <button v-for="option in motionReferenceOptions" :key="option.key" type="button" class="segment-choice" :class="{ active: slot.reference === option.key }" @click="setMotionOption(slot, 'reference', option.key)">{{ option.label }}</button>
                  </div>
                </label>
              </div>

              <div v-if="slot.mode === 'rotation' && slot.axis === 'custom'" class="custom-axis-row">
                <span>Custom axis</span>
                <label>X <input class="numeric-edit" type="number" min="-1" max="1" step="0.01" :value="numericDraftValue(sensorEditKey('M', slot, 'customX'), slot.customX)" @focus="beginSensorEdit('M', slot, 'customX', $event)" @input="updateNumericDraft(sensorEditKey('M', slot, 'customX'), $event)" @blur="commitMotionNumber(slot, 'customX', $event, -1, 1)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop></label>
                <label>Y <input class="numeric-edit" type="number" min="-1" max="1" step="0.01" :value="numericDraftValue(sensorEditKey('M', slot, 'customY'), slot.customY)" @focus="beginSensorEdit('M', slot, 'customY', $event)" @input="updateNumericDraft(sensorEditKey('M', slot, 'customY'), $event)" @blur="commitMotionNumber(slot, 'customY', $event, -1, 1)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop></label>
                <label>Z <input class="numeric-edit" type="number" min="-1" max="1" step="0.01" :value="numericDraftValue(sensorEditKey('M', slot, 'customZ'), slot.customZ)" @focus="beginSensorEdit('M', slot, 'customZ', $event)" @input="updateNumericDraft(sensorEditKey('M', slot, 'customZ'), $event)" @blur="commitMotionNumber(slot, 'customZ', $event, -1, 1)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop></label>
              </div>

              <div class="advanced-footer">
                <button v-if="needsCenter(slot.mode)" type="button" class="reset-button" :disabled="!sensorConnected" @click="recenterMotion(slot)">Center</button>
                <button type="button" class="reset-button" @click="resetMotion(slot)">Reset</button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="group acceleration-group">
        <div class="group-title-row"><h3>Acceleration</h3></div>
        <div class="sensor-slots">
          <div v-for="slot in accelerationSlots" :key="slot.index" class="sensor-slot acceleration-slot">
            <div class="accel-row">
              <div class="sensor-label accel-label">
                <strong>{{ slot.label }}</strong>
                <span class="binding-label" :class="{ muted: !accelerationBinding(slot) }">{{ accelerationBinding(slot) || "Unbound" }}</span>
              </div>

              <div class="accel-controls">
                <div class="accel-mode-row">
                  <button v-for="option in accelerationModeOptions" :key="option.key" type="button" class="segment-choice" :class="{ active: slot.mode === option.key }" @click="setAccelerationMode(slot, option.key)">{{ option.label }}</button>
                </div>
                <button type="button" class="advanced-toggle accel-advanced" :disabled="slot.mode === 'off'" @click="slot.advancedOpen = !slot.advancedOpen"><span>Advanced</span><span>{{ slot.advancedOpen ? "▴" : "▾" }}</span></button>
              </div>
            </div>

            <div v-if="slot.advancedOpen && slot.mode !== 'off'" class="advanced-panel">
              <div class="output-preview">
                <span>Live</span>
                <div class="output-meter"><span class="output-center"></span><span class="output-marker" :style="{ left: `${outputPosition(slot.mode === 'button' ? (slot.buttonHigh ? 1 : 0) : slot.output)}%` }"></span></div>
                <strong>{{ outputText(slot.mode === 'button' ? (slot.buttonHigh ? 1 : 0) : slot.output) }}</strong>
              </div>
              <div class="advanced-grid">
                <label v-if="slot.mode === 'axis'" class="advanced-field">
                  <span>Direction</span>
                  <button type="button" class="toggle-button mini" :class="{ active: slot.invert }" @click="setAccelerationOption(slot, 'invert', !slot.invert)">{{ slot.invert ? "Inverted" : "Normal" }}</button>
                </label>

                <label v-if="slot.mode === 'axis'" class="advanced-field">
                  <span>Minimum</span>
                  <div class="unit-input number-stepper"><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button><input class="numeric-edit" type="number" min="0" max="99" step="1" :value="numericDraftValue(sensorEditKey('A', slot, 'minPercent'), formatAccelPercent(slot.minPercent))" @focus="beginSensorEdit('A', slot, 'minPercent', $event)" @input="updateNumericDraft(sensorEditKey('A', slot, 'minPercent'), $event)" @blur="commitAccelerationPercent(slot, 'minPercent', $event, 0, 99)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button><small>%</small></div>
                  <small class="g-value">{{ percentToG(slot.minPercent) }} g</small>
                </label>

                <label v-if="slot.mode === 'axis'" class="advanced-field">
                  <span>Full</span>
                  <div class="unit-input number-stepper"><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button><input class="numeric-edit" type="number" min="1" max="100" step="1" :value="numericDraftValue(sensorEditKey('A', slot, 'maxPercent'), formatAccelPercent(slot.maxPercent))" @focus="beginSensorEdit('A', slot, 'maxPercent', $event)" @input="updateNumericDraft(sensorEditKey('A', slot, 'maxPercent'), $event)" @blur="commitAccelerationPercent(slot, 'maxPercent', $event, 1, 100)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button><small>%</small></div>
                  <small class="g-value">{{ percentToG(slot.maxPercent) }} g</small>
                </label>

                <label v-if="slot.mode === 'button'" class="advanced-field">
                  <span>Activation</span>
                  <div class="unit-input number-stepper"><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button><input class="numeric-edit" type="number" min="1" max="100" step="1" :value="numericDraftValue(sensorEditKey('A', slot, 'thresholdPercent'), Math.round(slot.thresholdPercent))" @focus="beginSensorEdit('A', slot, 'thresholdPercent', $event)" @input="updateNumericDraft(sensorEditKey('A', slot, 'thresholdPercent'), $event)" @blur="commitAccelerationPercent(slot, 'thresholdPercent', $event, 1, 100)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop><button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button><small>%</small></div>
                  <small class="g-value">{{ percentToG(slot.thresholdPercent) }} g</small>
                </label>

                <label v-if="slot.mode === 'button'" class="advanced-field full-field">
                  <span>Direction</span>
                  <div class="segment-row three">
                    <button v-for="option in accelerationDirectionOptions" :key="option.key" type="button" class="segment-choice" :class="{ active: slot.direction === option.key }" @click="setAccelerationOption(slot, 'direction', option.key)">{{ option.label }}</button>
                  </div>
                </label>

                <label v-if="slot.mode === 'button'" class="advanced-field full-field">
                  <span>Behavior</span>
                  <div class="segment-row two">
                    <button v-for="option in accelerationBehaviorOptions" :key="option.key" type="button" class="segment-choice" :class="{ active: slot.behavior === option.key }" @click="setAccelerationOption(slot, 'behavior', option.key)">{{ option.label }}</button>
                  </div>
                </label>

                <label class="advanced-field full-field">
                  <span>Ignore stop rebound</span>
                  <button type="button" class="toggle-button mini" :class="{ active: slot.ignoreRebound }" @click="setAccelerationOption(slot, 'ignoreRebound', !slot.ignoreRebound)">{{ slot.ignoreRebound ? "On" : "Off" }}</button>
                </label>
              </div>

              <div class="advanced-footer"><button type="button" class="reset-button" @click="resetAcceleration(slot)">Reset</button></div>
            </div>
          </div>
        </div>
      </div>

      <div class="group sleep-group">
        <div class="group-title-row"><h3>Bluetooth sleep</h3></div>
        <div class="sleep-settings">
          <div class="sleep-row">
            <span>Sleep after</span>
            <div class="idle-row number-stepper sleep-stepper">
              <button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, -1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">−</button>
              <input class="idle-input numeric-edit" type="number" min="0" :max="BLUETOOTH_IDLE_MAX_MINUTES" step="1" :value="numericDraftValue('UI:bluetoothIdle', bluetoothIdleMinutes)" @focus="beginStandaloneNumberEdit('bluetoothIdle', $event)" @input="updateNumericDraft('UI:bluetoothIdle', $event)" @blur="commitBluetoothIdleTimeout($event)" @keydown="handleNumericKeydown" @keyup.stop @keypress.stop>
              <button type="button" class="stepper-button" @pointerdown.prevent="startNumericStep($event, 1)" @pointerup.prevent="stopNumericStep" @pointercancel="stopNumericStep" @lostpointercapture="stopNumericStep">+</button>
              <span class="idle-unit">min</span>
            </div>
          </div>
          <div class="sleep-row">
            <span>Controller movement resets timer</span>
            <button type="button" class="toggle-button sleep-toggle" :class="{ active: bluetoothIdleMotionActivity }" @click="setBluetoothIdleMotionActivity(!bluetoothIdleMotionActivity)">{{ bluetoothIdleMotionActivity ? "On" : "Off" }}</button>
          </div>
        </div>
      </div>

    </template>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from "vue"
import { ACCENTS, BngButton } from "@/common/components/base"
import { runRaw } from "@/bridge/libs/Lua.js"
import { getModSetting } from "../settingsStore.js"
import {
  LEGACY_MODE_SETTINGS_KEY, CUSTOM_SETTING_KEYS, DEFAULT_CUSTOM_MAPPING, mappingOptions, customGroups,
  normalizeChannelMapping, legacyPresetMapping, MOUSE_MODE_SETTINGS_KEY, TWO_FINGER_MOUSE_TOGGLE_SETTINGS_KEY,
  normalizeBooleanSetting, BLUETOOTH_IDLE_SETTINGS_KEY, BLUETOOTH_IDLE_EXPLICIT_OFF_KEY, BLUETOOTH_IDLE_MOTION_ACTIVITY_KEY,
  DEFAULT_BLUETOOTH_IDLE_MINUTES, DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY, BLUETOOTH_IDLE_MAX_MINUTES, normalizeBluetoothIdleMinutes, resolveBluetoothIdleMinutes,
  motionModeOptions, motionAxisOptions, motionReferenceOptions, motionOutputOptions, motionButtonDirectionOptions, motionButtonBehaviorOptions, optionLabel,
  accelerationModeOptions, accelerationDirectionOptions, accelerationBehaviorOptions,
} from "./extendedInputsSettingsModel.js"
import { resetExtendedInputsDefaults } from "../suiteDefaults.js"

const loading = ref(true)
const loadError = ref("")
const customMapping = reactive({ ...DEFAULT_CUSTOM_MAPPING })
const mouseMode = ref(false)
const bluetoothIdleMinutes = ref(DEFAULT_BLUETOOTH_IDLE_MINUTES)
const bluetoothIdleMotionActivity = ref(DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY)
const openMotionMenu = ref("")
const sensorConnected = ref(false)
let stateTimer = null
let pollTick = 0
const editingField = ref("")
const numericDrafts = reactive({})
let numericStepDelay = null
let numericStepInterval = null
let numericStepButton = null
let numericStepDirection = 0

const motionSlots = reactive(Array.from({ length: 3 }, (_, i) => ({
  index: i + 1, mode: "off", axis: "auto", invert: false, deadzone: 0.02, stability: 8, range: 90,
  reference: "adaptive", customX: 0, customY: 0, customZ: 1, learned: false, learnedX: 0, learnedY: 0, learnedZ: 1,
  quality: 0, binding: "", buttonBinding: "", calibrating: false, progress: 0, output: 0, outputMode: "axis", buttonDirection: "positive", buttonThresholdPercent: 50, buttonBehavior: "hold", buttonHigh: false, advancedOpen: false,
})))
const accelerationSlots = reactive(["X", "Y", "Z"].map((label, i) => ({
  index: i + 1, label, mode: "off", invert: false, minPercent: 10, maxPercent: 100, thresholdPercent: 10,
  direction: "any", behavior: "pulse", ignoreRebound: true, axisBinding: "", buttonBinding: "", linear: 0, output: 0, buttonHigh: false, advancedOpen: false,
})))

function outputValue(value) { return Math.max(-1, Math.min(1, Number(value) || 0)) }
function outputPosition(value) { return 50 + outputValue(value) * 50 }
function outputText(value) { return `${Math.round(outputValue(value) * 100)}%` }
function boolLua(value) { return value ? "true" : "false" }
function normalizeLuaBoolean(value) { return value === "1" || value === 1 || value === true || String(value).toLowerCase() === "true" }
function escapeLuaString(value) { return String(value).replace(/\\/g, "\\\\").replace(/'/g, "\\'") }
function withTimeout(promise, ms, label) {
  let timer
  const timeout = new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`${label} timed out`)), ms) })
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer))
}

async function applyCustomToLua(channel, mapping) {
  await runRaw(`extensions.dualsenseExtendedInputs.setTouchpadCustomMapping('${escapeLuaString(channel)}','${escapeLuaString(mapping)}')`)
}
async function applyAllCustomToLua() { for (const channel of Object.keys(CUSTOM_SETTING_KEYS)) await applyCustomToLua(channel, customMapping[channel]) }
async function applyMouseModeToLua(value) { await runRaw(`extensions.dualsenseExtendedInputs.setTouchpadMouseMode(${boolLua(value)})`) }
async function applyBluetoothIdleToLua(minutes) { await runRaw(`extensions.dualsenseExtendedInputs.setBluetoothIdleTimeoutMinutes(${minutes})`) }
async function applyBluetoothIdleMotionToLua(value) { await runRaw(`extensions.dualsenseExtendedInputs.setBluetoothIdleMotionActivityEnabled(${boolLua(value)})`) }
async function disableLegacyTwoFingerShortcut() {
  try { await runRaw("extensions.dualsenseExtendedInputs.setTwoFingerMouseToggleEnabled(false)") } catch {}
}

function applySensorState(flat) {
  const rows = String(flat || "").split("\t")
  if (rows[0] !== "SENS4") throw new Error("Unsupported Sensors state")
  for (const row of rows.slice(1)) {
    const p = row.split("~")
    if (p[0] === "M") {
      const index = Number(p[1])
      const slot = motionSlots[index - 1]
      if (!slot || p.length < 27) continue
      const editing = editingField.value.startsWith(`M:${index}:`)
      if (!editing) {
        Object.assign(slot, {
          mode: p[2], axis: p[3], invert: normalizeLuaBoolean(p[4]), deadzone: Number(p[5]) || 0,
          stability: Number(p[6]) || 0, range: Number(p[7]) || 1, reference: p[8],
          customX: Number(p[9]) || 0, customY: Number(p[10]) || 0, customZ: Number(p[11]) || 0,
          learned: normalizeLuaBoolean(p[12]), learnedX: Number(p[13]) || 0, learnedY: Number(p[14]) || 0, learnedZ: Number(p[15]) || 0,
          quality: Number(p[16]) || 0, outputMode: p[21] || "axis", buttonDirection: p[22] || "positive", buttonThresholdPercent: Number.isFinite(Number(p[23])) ? Number(p[23]) : 50, buttonBehavior: p[24] || "hold",
        })
      }
      Object.assign(slot, {
        binding: p[17] || "", calibrating: normalizeLuaBoolean(p[18]), progress: Number(p[19]) || 0, output: Number(p[20]) || 0,
        buttonBinding: p[25] || "", buttonHigh: normalizeLuaBoolean(p[26]),
      })
    } else if (p[0] === "A") {
      const index = Number(p[1])
      const slot = accelerationSlots[index - 1]
      if (!slot || p.length < 15) continue
      const editing = editingField.value.startsWith(`A:${index}:`)
      if (!editing) {
        Object.assign(slot, {
          mode: p[2], invert: normalizeLuaBoolean(p[3]), minPercent: Number(p[4]) || 0, maxPercent: Number(p[5]) || 100,
          thresholdPercent: Number.isFinite(Number(p[6])) ? Number(p[6]) : 10, direction: p[7], behavior: p[8], ignoreRebound: normalizeLuaBoolean(p[14]),
        })
      }
      Object.assign(slot, {
        axisBinding: p[9] || "", buttonBinding: p[10] || "", linear: Number(p[11]) || 0, output: Number(p[12]) || 0, buttonHigh: normalizeLuaBoolean(p[13]),
      })
    } else if (p[0] === "S") {
      sensorConnected.value = normalizeLuaBoolean(p[3])
    }
  }
}

async function refreshSensorState() {
  try {
    const state = await withTimeout(runRaw("extensions.dualsenseMotionInputs.getSensorsUIStateFlat()"), 1000, "Sensors state")
    applySensorState(state)
  } catch (error) {
    console.error("[DualSense Extended Inputs] Sensors refresh failed", error)
  }
}

async function refreshTouchpadRuntime() {
  try {
    const value = await withTimeout(runRaw("extensions.dualsenseExtendedInputs.getTouchpadMouseMode()"), 800, "Mouse mode state")
    mouseMode.value = normalizeLuaBoolean(value)
  } catch {}
}

async function loadSettings() {
  loading.value = true
  loadError.value = ""
  try {
    const legacySeed = legacyPresetMapping(await getModSetting(LEGACY_MODE_SETTINGS_KEY, null))
    for (const [channel, key] of Object.entries(CUSTOM_SETTING_KEYS)) {
      const raw = await getModSetting(key, null)
      customMapping[channel] = normalizeChannelMapping(raw, legacySeed[channel])
    }
    await applyAllCustomToLua()

    mouseMode.value = normalizeBooleanSetting(await getModSetting(MOUSE_MODE_SETTINGS_KEY, false), false)
    await applyMouseModeToLua(mouseMode.value)
    await disableLegacyTwoFingerShortcut()

    const idle = resolveBluetoothIdleMinutes(
      await getModSetting(BLUETOOTH_IDLE_SETTINGS_KEY, null),
      await getModSetting(BLUETOOTH_IDLE_EXPLICIT_OFF_KEY, false)
    )
    bluetoothIdleMinutes.value = idle
    await applyBluetoothIdleToLua(idle)
    bluetoothIdleMotionActivity.value = normalizeBooleanSetting(
      await getModSetting(BLUETOOTH_IDLE_MOTION_ACTIVITY_KEY, DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY),
      DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY
    )
    await applyBluetoothIdleMotionToLua(bluetoothIdleMotionActivity.value)
    await refreshSensorState()
  } catch (error) {
    loadError.value = `Could not connect to DualSense settings: ${error?.message || error}`
  } finally {
    loading.value = false
  }
}

async function setCustomChannel(channel, mapping) {
  const normalized = normalizeChannelMapping(mapping)
  customMapping[channel] = normalized
  try { await applyCustomToLua(channel, normalized) } catch (error) { console.error(error) }
}
async function setMouseMode(value) {
  mouseMode.value = !!value
  try { await applyMouseModeToLua(mouseMode.value) } catch (error) { console.error(error) }
}

async function setTextEntryCapture(active) {
  try { await runRaw(`extensions.dualsenseExtendedInputs.setSettingsTextEntryActive(${boolLua(active)})`) } catch {}
}
function isNumericEditor(target) { return !!target && target.tagName === "INPUT" && (target.type === "number" || target.classList?.contains("numeric-edit")) }
function handleSettingsPointerDown(event) {
  if (isNumericEditor(event.target)) void setTextEntryCapture(true)
}
function handleSettingsKeyCapture(event) {
  if (!isNumericEditor(event.target)) return
  event.stopImmediatePropagation?.()
  event.stopPropagation()
}
function sensorEditKey(kind, slot, key) { return `${kind}:${slot.index}:${key}` }
function numericDraftValue(key, fallback) { return Object.prototype.hasOwnProperty.call(numericDrafts, key) ? numericDrafts[key] : fallback }
function updateNumericDraft(key, event) { numericDrafts[key] = String(event?.target?.value ?? "") }
function beginSensorEdit(kind, slot, key, event) {
  const editKey = sensorEditKey(kind, slot, key)
  editingField.value = editKey
  if (!Object.prototype.hasOwnProperty.call(numericDrafts, editKey)) numericDrafts[editKey] = String(event?.target?.value ?? "")
  void setTextEntryCapture(true)
}
function beginStandaloneNumberEdit(key, event) {
  const editKey = `UI:${key}`
  editingField.value = editKey
  if (!Object.prototype.hasOwnProperty.call(numericDrafts, editKey)) numericDrafts[editKey] = String(event?.target?.value ?? "")
  void setTextEntryCapture(true)
}
function handleNumericKeydown(event) {
  event.stopImmediatePropagation?.()
  event.stopPropagation()
  if (event.key === "Enter") { event.preventDefault(); event.target.blur() }
}
function performNumericStep(button, direction) {
  const container = button?.parentElement
  const input = container?.querySelector?.('input[type="number"]')
  if (!input) return
  if (document.activeElement !== input) input.focus()
  try { direction > 0 ? input.stepUp() : input.stepDown() } catch { return }
  input.dispatchEvent(new Event("input", { bubbles: true }))
}
function stopNumericStep(commit = true) {
  const button = numericStepButton
  if (numericStepDelay) clearTimeout(numericStepDelay)
  if (numericStepInterval) clearInterval(numericStepInterval)
  numericStepDelay = null; numericStepInterval = null; numericStepButton = null; numericStepDirection = 0
  if (commit !== false && button) {
    const input = button.parentElement?.querySelector?.('input[type="number"]')
    if (input && document.activeElement === input) input.blur()
  }
}
function startNumericStep(event, direction) {
  stopNumericStep(false)
  numericStepButton = event?.currentTarget || null
  numericStepDirection = direction
  if (!numericStepButton) return
  try { numericStepButton.setPointerCapture?.(event.pointerId) } catch {}
  performNumericStep(numericStepButton, numericStepDirection)
  numericStepDelay = setTimeout(() => {
    numericStepInterval = setInterval(() => performNumericStep(numericStepButton, numericStepDirection), 75)
  }, 330)
}
function parseEditableNumber(event) {
  const raw = String(event?.target?.value ?? "").trim()
  if (raw === "") return null
  const n = Number(raw)
  return Number.isFinite(n) ? n : null
}
async function finishSensorEdit(expectedKey) {
  // Blur and focus fire back-to-back when moving between numeric fields. Do not
  // let the older async commit release the ActionMap guard or overwrite the
  // newer field's edit state.
  if (expectedKey && editingField.value !== expectedKey) return
  editingField.value = ""
  delete numericDrafts[expectedKey]
  await setTextEntryCapture(false)
  await refreshSensorState()
}
async function commitMotionNumber(slot, key, event, min, max) {
  const expectedKey = sensorEditKey("M", slot, key)
  const n = parseEditableNumber(event)
  try { if (n !== null) await setMotionOption(slot, key, Math.max(min, Math.min(max, n)), false) }
  finally { await finishSensorEdit(expectedKey) }
}
async function commitMotionPercent(slot, key, event, maxPercent) {
  const expectedKey = sensorEditKey("M", slot, key)
  const n = parseEditableNumber(event)
  try { if (n !== null) await setMotionOption(slot, key, Math.max(0, Math.min(maxPercent, n)) / 100, false) }
  finally { await finishSensorEdit(expectedKey) }
}
async function commitAccelerationPercent(slot, key, event, min, max) {
  const expectedKey = sensorEditKey("A", slot, key)
  const n = parseEditableNumber(event)
  try { if (n !== null) await setAccelerationOption(slot, key, Math.round(Math.max(min, Math.min(max, n))), false) }
  finally { await finishSensorEdit(expectedKey) }
}
function motionBinding(slot) { return slot.outputMode === "button" ? slot.buttonBinding : slot.binding }
function motionRangeLabel(slot) {
  if (slot.mode === "rotation") return "Angle per side"
  if (slot.mode === "gyro") return "Full speed"
  if (slot.mode === "tilt") return "Full tilt"
  return "Angle per side"
}
function motionRangeUnit(slot) { return slot.mode === "gyro" ? "°/s" : "°" }

function isMotionMenuOpen(slot, kind) { return openMotionMenu.value === `${slot.index}:${kind}` }
function toggleMotionMenu(slot, kind) { openMotionMenu.value = isMotionMenuOpen(slot, kind) ? "" : `${slot.index}:${kind}` }
async function chooseMotionOption(slot, kind, value) {
  openMotionMenu.value = ""
  try {
    if (kind === "mode") await runRaw(`extensions.dualsenseMotionInputs.setMotionMode(${slot.index},'${escapeLuaString(value)}')`)
    else await runRaw(`extensions.dualsenseMotionInputs.setMotionAxis(${slot.index},'${escapeLuaString(value)}')`)
    await refreshSensorState()
  } catch (error) { console.error(error) }
}
function needsCenter(mode) { return mode === "rotation" || mode === "tilt" || mode === "orientation" }
async function startMotionCalibration(slot) {
  try { await runRaw(`extensions.dualsenseMotionInputs.startMotionCalibration(${slot.index})`); await refreshSensorState() } catch (error) { console.error(error) }
}
async function recenterMotion(slot) {
  try { await runRaw(`extensions.dualsenseMotionInputs.recenterMotion(${slot.index})`); await refreshSensorState() } catch (error) { console.error(error) }
}
async function setMotionOption(slot, key, value, refresh = true) {
  const luaValue = typeof value === "boolean" ? boolLua(value) : (typeof value === "number" ? String(value) : `'${escapeLuaString(value)}'`)
  try { await runRaw(`extensions.dualsenseMotionInputs.setMotionOption(${slot.index},'${key}',${luaValue})`); if (refresh) await refreshSensorState() } catch (error) { console.error(error) }
}
async function resetMotion(slot) { try { await runRaw(`extensions.dualsenseMotionInputs.resetMotionSlot(${slot.index})`); await refreshSensorState() } catch (error) { console.error(error) } }

function accelerationBinding(slot) { return slot.mode === "button" ? slot.buttonBinding : slot.axisBinding }
async function setAccelerationMode(slot, mode) {
  try { await runRaw(`extensions.dualsenseMotionInputs.setAccelerationMode(${slot.index},'${escapeLuaString(mode)}')`); if (mode === "off") slot.advancedOpen = false; await refreshSensorState() } catch (error) { console.error(error) }
}
async function setAccelerationOption(slot, key, value, refresh = true) {
  const luaValue = typeof value === "boolean" ? boolLua(value) : (typeof value === "number" ? String(value) : `'${escapeLuaString(value)}'`)
  try { await runRaw(`extensions.dualsenseMotionInputs.setAccelerationOption(${slot.index},'${key}',${luaValue})`); if (refresh) await refreshSensorState() } catch (error) { console.error(error) }
}
async function resetAcceleration(slot) { try { await runRaw(`extensions.dualsenseMotionInputs.resetAccelerationSlot(${slot.index})`); await refreshSensorState() } catch (error) { console.error(error) } }
function formatAccelPercent(value) { return String(Math.round(Math.max(0, Math.min(100, Number(value) || 0)))) }
function percentToG(value) { return (Math.max(0, Math.min(100, Number(value) || 0)) * 0.04).toFixed(2) }

async function applyBluetoothIdleTimeout() {
  const normalized = normalizeBluetoothIdleMinutes(bluetoothIdleMinutes.value)
  bluetoothIdleMinutes.value = normalized
  try { await applyBluetoothIdleToLua(normalized) } catch (error) { console.error(error) }
}
async function commitBluetoothIdleTimeout(event) {
  const expectedKey = "UI:bluetoothIdle"
  const n = parseEditableNumber(event)
  try {
    if (n !== null) bluetoothIdleMinutes.value = normalizeBluetoothIdleMinutes(n)
    await applyBluetoothIdleTimeout()
  } finally {
    if (editingField.value === expectedKey) {
      editingField.value = ""
      delete numericDrafts[expectedKey]
      await setTextEntryCapture(false)
    }
  }
}

async function setBluetoothIdleMotionActivity(value) {
  bluetoothIdleMotionActivity.value = !!value
  try { await applyBluetoothIdleMotionToLua(bluetoothIdleMotionActivity.value) } catch (error) { console.error(error) }
}

async function resetAllSettings() {
  try {
    await resetExtendedInputsDefaults(runRaw)
    Object.assign(customMapping, DEFAULT_CUSTOM_MAPPING)
    mouseMode.value = false
    bluetoothIdleMinutes.value = DEFAULT_BLUETOOTH_IDLE_MINUTES
    bluetoothIdleMotionActivity.value = DEFAULT_BLUETOOTH_IDLE_MOTION_ACTIVITY
    await refreshSensorState()
  } catch (error) { console.error("[DualSense Extended Inputs] Reset failed", error) }
}

async function reloadFromSettings() { await loadSettings() }
defineExpose({ resetAllSettings, reloadFromSettings })

async function closePanel() {
  try { await runRaw("extensions.ui_menuManager.closeMenu()") }
  catch { try { window.history.back() } catch {} }
}

onMounted(async () => {
  await loadSettings()
  stateTimer = setInterval(() => {
    if (loading.value || editingField.value) return
    void refreshSensorState()
    pollTick += 1
    if (pollTick % 3 === 0) void refreshTouchpadRuntime()
  }, 180)
})
onBeforeUnmount(() => {
  stopNumericStep(false)
  if (stateTimer) clearInterval(stateTimer)
  if (editingField.value) void setTextEntryCapture(false)
})
</script>
