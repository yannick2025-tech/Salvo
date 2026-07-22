<template>
  <div class="custom-select" ref="selectRef" :class="{ open: isOpen, disabled }">
    <button
      type="button"
      class="custom-select-trigger"
      @click="toggle"
      :disabled="disabled"
    >
      <span class="custom-select-value" :class="{ placeholder: !modelValue }">
        {{ displayLabel }}
      </span>
      <svg class="custom-select-arrow" width="10" height="6" viewBox="0 0 10 6" fill="none">
        <path d="M1 1l4 4 4-4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>
    <Teleport to="body">
      <div
        v-if="isOpen"
        class="custom-select-dropdown"
        :style="dropdownStyle"
      >
        <div
          v-for="opt in options"
          :key="opt.value"
          class="custom-select-option"
          :class="{ selected: opt.value === String(modelValue) }"
          @click="select(opt.value)"
        >
          <span v-if="opt.value === String(modelValue)" class="check-mark">✓</span>
          <span class="option-label">{{ opt.label }}</span>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

interface SelectOption {
  value: string
  label: string
}

const props = defineProps<{
  modelValue: string | number
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  minWidth?: string
  fontSize?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
  'openChange': [isOpen: boolean]
}>()

const selectRef = ref<HTMLElement>()
const isOpen = ref(false)
const dropdownStyle = ref<Record<string, string>>({})

const displayLabel = computed(() => {
  if (!props.modelValue) return props.placeholder || ''
  const opt = props.options.find(o => o.value === String(props.modelValue))
  return opt?.label || String(props.modelValue)
})

function toggle() {
  if (props.disabled) return
  isOpen.value = !isOpen.value
  emit('openChange', isOpen.value)
  if (isOpen.value) {
    nextTick(() => updateDropdownPosition())
  }
}

function select(value: string) {
  // Preserve original type: if modelValue is number, emit number
  if (typeof props.modelValue === 'number') {
    emit('update:modelValue', Number(value))
  } else {
    emit('update:modelValue', value)
  }
  isOpen.value = false
  emit('openChange', false)
}

function updateDropdownPosition() {
  if (!selectRef.value) return
  const rect = selectRef.value.getBoundingClientRect()
  dropdownStyle.value = {
    position: 'fixed',
    top: `${rect.bottom + 4}px`,
    left: `${rect.left}px`,
    minWidth: props.minWidth || `${rect.width}px`,
    zIndex: '10000',
  }
}

function handleClickOutside(e: MouseEvent) {
  if (selectRef.value && !selectRef.value.contains(e.target as Node)) {
    if (isOpen.value) {
      isOpen.value = false
      emit('openChange', false)
    }
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  window.addEventListener('scroll', () => { if (isOpen.value) updateDropdownPosition() }, true)
  window.addEventListener('resize', () => { if (isOpen.value) updateDropdownPosition() })
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.custom-select {
  position: relative;
  display: inline-block;
}

.custom-select.disabled {
  opacity: 0.5;
  pointer-events: none;
}

.custom-select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: v-bind("fontSize || '13px'");
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
  outline: none;
  text-align: left;
  font-family: var(--font-sans);
}

.custom-select-trigger:hover {
  border-color: var(--accent-primary);
}

.custom-select.open .custom-select-trigger {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(45, 212, 191, 0.15);
}

.custom-select-value {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.custom-select-value.placeholder {
  color: var(--text-tertiary);
}

.custom-select-arrow {
  color: var(--text-secondary);
  flex-shrink: 0;
  transition: transform 0.2s;
}

.custom-select.open .custom-select-arrow {
  transform: rotate(180deg);
}

.custom-select-dropdown {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
  max-height: 280px;
  overflow-y: auto;
  padding: 4px 0;
}

.custom-select-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  color: var(--text-primary);
  font-size: v-bind("fontSize || '13px'");
  transition: background 0.1s;
  white-space: nowrap;
}

.custom-select-option:hover {
  background: var(--bg-hover);
}

.custom-select-option.selected {
  color: var(--accent-primary);
  font-weight: 500;
}

.check-mark {
  color: var(--accent-primary);
  font-size: 12px;
  width: 14px;
  text-align: center;
  flex-shrink: 0;
}
</style>
