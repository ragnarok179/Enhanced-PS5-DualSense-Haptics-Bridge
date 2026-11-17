<template>
  <div class="setting-slider" :class="{ compact, disabled }">
    <div class="setting-head">
      <div class="setting-copy">
        <div class="setting-title">{{ label }}</div>
        <div v-if="description" class="setting-description">{{ description }}</div>
      </div>
    </div>

    <div class="setting-control">
      <BngSlider
        v-model="sliderValue"
        class="beamng-slider"
        :min="min"
        :max="max"
        :step="step"
        with-input
        :disabled="disabled"
        :aria-label="label"
      />
      <strong class="setting-unit">%</strong>
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue"
import { BngSlider } from "@/common/components/base"

const props = defineProps({
  label: { type: String, required: true },
  description: { type: String, default: "" },
  modelValue: { type: Number, required: true },
  min: { type: Number, default: 1 },
  max: { type: Number, default: 100 },
  step: { type: Number, default: 1 },
  compact: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(["update:modelValue"])

const sliderValue = computed({
  get() {
    const value = Number(props.modelValue)
    return Number.isFinite(value) ? value : 0
  },
  set(value) {
    const numeric = Number(value)
    if (!props.disabled && Number.isFinite(numeric)) emit("update:modelValue", numeric)
  },
})
</script>

<style scoped>
.setting-slider {
  width: 100%;
  min-width: 0;
  margin: 1.18rem 0 0.64rem;
}

.setting-slider.disabled .setting-control { opacity: 0.45; }
.setting-slider.compact { margin: 1.06rem 0 0.54rem; }

.setting-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
  margin-bottom: 0.40rem;
}

.setting-copy { min-width: 0; }

.setting-title {
  font-size: 0.91rem;
  line-height: 1.3;
  font-weight: 600;
  color: #f4f4f4;
}

.setting-description {
  margin-top: 0.14rem;
  color: #b1b4ba;
  font-size: 0.9rem;
  line-height: 1.4;
}

.setting-control {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.42rem;
  min-width: 0;
}

.beamng-slider {
  width: 100%;
  min-width: 0;
}

.setting-unit {
  min-width: 1.1rem;
  color: #f4f4f4;
  font-size: 0.9rem;
  line-height: 1;
  text-align: left;
}
</style>
