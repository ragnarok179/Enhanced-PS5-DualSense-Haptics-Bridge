<template>
  <div class="setting-slider" :class="{ compact }">
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
})

const emit = defineEmits(["update:modelValue"])

const sliderValue = computed({
  get() {
    const value = Number(props.modelValue)
    return Number.isFinite(value) ? value : 0
  },
  set(value) {
    const numeric = Number(value)
    if (Number.isFinite(numeric)) emit("update:modelValue", numeric)
  },
})
</script>

<style scoped>
.setting-slider {
  width: 100%;
  min-width: 0;
  margin: 0.48rem 0 0.82rem;
}

.setting-slider.compact { margin: 0.36rem 0 0.58rem; }

.setting-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
  margin-bottom: 0.34rem;
}

.setting-copy { min-width: 0; }

.setting-title {
  font-size: 1.08rem;
  line-height: 1.25;
  font-weight: 700;
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
  font-size: 1.02rem;
  line-height: 1;
  text-align: left;
}
</style>
