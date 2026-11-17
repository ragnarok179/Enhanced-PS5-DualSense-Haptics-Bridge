<template>
  <div class="setting-range" :class="{ compact, disabled }">
    <div class="range-label">{{ label }}</div>
    <div class="range-track-control">
      <div class="range-track" aria-hidden="true"><div class="range-active" :style="activeStyle" /></div>
      <input type="range" class="range-thumb start" :aria-label="`${label}: start`"
        :min="min" :max="Math.max(min, end - step)" :step="step" :value="start" :disabled="disabled" :style="{ zIndex: start > max - 12 ? 4 : 3 }"
        @input="setStart($event.target.value)" />
      <input type="range" class="range-thumb end" :aria-label="`${label}: end`"
        :min="Math.min(max, start + step)" :max="max" :step="step" :value="end" :disabled="disabled" @input="setEnd($event.target.value)" />
    </div>
    <div class="range-values">
      <label>Start <input type="number" :min="min" :max="Math.max(min, end - step)" :step="step" :value="start" :disabled="disabled" @change="setStart($event.target.value)" />%</label>
      <label>End <input type="number" :min="start + step" :max="max" :step="step" :value="end" :disabled="disabled" @change="setEnd($event.target.value)" />%</label>
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue"
const props = defineProps({
  label: { type: String, required: true },
  start: { type: Number, required: true },
  end: { type: Number, required: true },
  min: { type: Number, default: 0 },
  max: { type: Number, default: 100 },
  step: { type: Number, default: 1 },
  compact: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(["update:start", "update:end"])
const activeStyle = computed(() => {
  const span = Math.max(props.step, props.max - props.min)
  return {
    left: `${(props.start - props.min) / span * 100}%`,
    right: `${(props.max - props.end) / span * 100}%`,
  }
})
function clamp(v, lo, hi) { const n = Number(v); return Math.max(lo, Math.min(hi, Number.isFinite(n) ? n : lo)) }
function setStart(value) { if (!props.disabled) emit("update:start", Math.round(clamp(value, props.min, props.end - props.step))) }
function setEnd(value) { if (!props.disabled) emit("update:end", Math.round(clamp(value, props.start + props.step, props.max))) }
</script>

<style scoped>
.setting-range { width: 100%; margin: 1.16rem 0 0.6rem; color: #f4f4f4; }
.setting-range.compact { margin-top: 1.06rem; }
.range-label { font-size: 0.91rem; font-weight: 600; margin-bottom: 0.48rem; }
.range-track-control { height: 24px; position: relative; margin: 0 10px; }
.range-track { position: absolute; left: 0; right: 0; top: 10px; height: 5px; border-radius: 4px; background: #49494b; }
.range-active { position: absolute; top: 0; bottom: 0; border-radius: 4px; background: #ef6b1c; }
.range-thumb { appearance: none; -webkit-appearance: none; position: absolute; left: 0; top: 0; margin: 0; width: 100%; height: 24px; background: transparent; pointer-events: none; }
.range-thumb::-webkit-slider-thumb { -webkit-appearance: none; appearance: none; width: 18px; height: 18px; border-radius: 50%; border: 2px solid #f5f5f5; background: #ef6b1c; cursor: ew-resize; pointer-events: auto; }
.range-thumb::-moz-range-thumb { width: 16px; height: 16px; border-radius: 50%; border: 2px solid #f5f5f5; background: #ef6b1c; cursor: ew-resize; pointer-events: auto; }
.range-thumb:focus-visible { outline: none; }
.range-thumb:focus-visible::-webkit-slider-thumb { outline: 2px solid white; outline-offset: 3px; }
.setting-range.disabled .range-track-control,
.setting-range.disabled .range-values { opacity: 0.42; }
.range-thumb:disabled::-webkit-slider-thumb { cursor: not-allowed; }
.range-values { display: flex; justify-content: space-between; gap: 12px; margin-top: 4px; color: #d6d6d8; font-size: 0.9rem; }
.range-values label { display: flex; gap: 5px; align-items: center; }
.range-values input:focus-visible { outline: 2px solid #ef6b1c; outline-offset: 2px; }
.range-values input { box-sizing: border-box; width: 58px; padding: 4px 3px; color: white; text-align: center; background: #29292b; border: 1px solid #737373; border-radius: 4px; }
</style>
