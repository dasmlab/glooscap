<template>
  <Teleport to="body">
    <div
      v-if="drawerOpen"
      class="console-drawer"
      style="position: fixed; bottom: 0; left: 0; right: 0; height: 300px; max-height: 50vh; z-index: 2000; background: #1e1e1e; border-top: 2px solid #007acc; display: flex; flex-direction: column;"
    >
    <q-toolbar class="bg-grey-9 text-white">
      <q-toolbar-title>
        <q-icon name="terminal" class="q-mr-sm" />
        {{ $t('console.title') }}
      </q-toolbar-title>
      <q-btn flat dense icon="delete_sweep" :title="$t('console.clear')" @click="clearLogs" />
      <q-btn flat dense icon="close" :title="$t('console.close')" @click="closeDrawer" />
    </q-toolbar>
    <q-scroll-area style="height: calc(100% - 50px); max-height: calc(50vh - 50px);" class="console-scroll">
      <div class="console-logs q-pa-sm">
        <div
          v-for="(log, index) in logs"
          :key="index"
          :class="['log-entry', `log-${log.level}`]"
        >
          <div class="log-entry-header">
            <span class="log-time">{{ formatTime(log.timestamp) }}</span>
            <span class="log-level">{{ log.level.toUpperCase() }}</span>
            <span class="log-message">{{ log.message }}</span>
          </div>
          <pre v-if="log.data" class="log-data">{{ formatData(log.data) }}</pre>
        </div>
      </div>
    </q-scroll-area>
  </div>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue'])

const drawerOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const logs = ref([])
const maxLogs = 500

function addLog(level, message, data = null) {
  logs.value.push({
    timestamp: new Date(),
    level,
    message,
    data,
  })
  // Keep only last maxLogs entries
  if (logs.value.length > maxLogs) {
    logs.value = logs.value.slice(-maxLogs)
  }
}

function clearLogs() {
  logs.value = []
}

function formatTime(date) {
  return date.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
  })
}

function formatData(data) {
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data)
      return JSON.stringify(parsed, null, 2)
    } catch {
      return data
    }
  }
  return JSON.stringify(data, null, 2)
}

function closeDrawer() {
  drawerOpen.value = false
}

// Expose methods for parent components
defineExpose({
  addLog,
  clearLogs,
})
</script>

<style scoped>
.console-drawer {
  background: #1e1e1e;
  color: #d4d4d4;
}

.console-scroll {
  background: #1e1e1e;
}

.console-logs {
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.5;
}

.log-entry {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 4px 0;
  border-bottom: 1px solid #2d2d2d;
}

.log-entry-header {
  display: flex;
  gap: 8px;
  align-items: baseline;
}

.log-time {
  color: #858585;
  min-width: 100px;
  flex-shrink: 0;
}

.log-level {
  min-width: 60px;
  font-weight: bold;
  flex-shrink: 0;
}

.log-level.log-INFO {
  color: #4ec9b0;
}

.log-level.log-ERROR {
  color: #f48771;
}

.log-level.log-WARN {
  color: #dcdcaa;
}

.log-level.log-DEBUG {
  color: #9cdcfe;
}

.log-message {
  flex: 1;
  color: #d4d4d4;
  word-break: break-word;
}

.log-data {
  margin: 4px 0 0 0;
  padding: 8px;
  background: #252526;
  border-left: 3px solid #007acc;
  color: #ce9178;
  font-size: 11px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
  width: 100%;
  box-sizing: border-box;
}
</style>

