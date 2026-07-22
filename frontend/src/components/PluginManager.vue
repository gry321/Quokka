<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ListPlugins, RemovePlugin, TogglePlugin, AddPlugin } from '../../wailsjs/go/main/App'

const emit = defineEmits<{ (e: 'close'): void }>()

interface PluginItem {
  name: string
  path: string
  enabled: boolean
}

const plugins = ref<PluginItem[]>([])
const dragOver = ref(false)
const loading = ref(false)

const loadPlugins = async () => {
  try {
    const list = await ListPlugins()
    plugins.value = list || []
  } catch { plugins.value = [] }
}

const removePlugin = async (idx: number) => {
  await RemovePlugin(idx)
  await loadPlugins()
}

const togglePlugin = async (idx: number) => {
  await TogglePlugin(idx)
  await loadPlugins()
}

const handleDrop = async (e: DragEvent) => {
  e.preventDefault()
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (!files || files.length === 0) return
  loading.value = true
  for (let i = 0; i < files.length; i++) {
    const f = files[i]
    if (!f.name.toLowerCase().endsWith('.dll')) continue
    // Use the webkitRelativePath or name as plugin name
    const pluginName = f.name.replace(/\.dll$/i, '')
    // We need the real path — Wails provides file:// URIs
    const filePath = (f as any).path || f.name
    await AddPlugin(pluginName, filePath)
  }
  loading.value = false
  await loadPlugins()
}

const handleDragOver = (e: DragEvent) => {
  e.preventDefault()
  dragOver.value = true
}

const handleDragLeave = () => {
  dragOver.value = false
}

onMounted(loadPlugins)
</script>

<template>
  <Teleport to="body">
    <div class="plugin-overlay" @click.self="emit('close')">
      <div class="plugin-panel">
        <div class="plugin-header">
          <span class="plugin-title">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" width="16" height="16">
              <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            Plugins
          </span>
          <button class="close-btn" @click="emit('close')">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" width="14" height="14">
              <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </button>
        </div>

        <!-- Drop zone -->
        <div
          class="drop-zone"
          :class="{ 'drag-over': dragOver }"
          @drop="handleDrop"
          @dragover="handleDragOver"
          @dragleave="handleDragLeave"
        >
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" width="22" height="22">
            <path d="M12 16V4m0 0L8 8m4-4l4 4M4 14v4a2 2 0 002 2h12a2 2 0 002-2v-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <span>Drag &amp; drop <b>.dll</b> plugins here</span>
          <span v-if="loading" class="loading-hint">Importing...</span>
        </div>

        <!-- Plugin list -->
        <div class="plugin-list" v-if="plugins.length > 0">
          <div
            v-for="(p, idx) in plugins"
            :key="p.path + idx"
            class="plugin-item"
            :class="{ disabled: !p.enabled }"
          >
            <div class="plugin-info">
              <span class="plugin-name">{{ p.name }}</span>
              <span class="plugin-path">{{ p.path }}</span>
            </div>
            <div class="plugin-actions">
              <button
                class="action-btn toggle-btn"
                :class="{ active: p.enabled }"
                @click="togglePlugin(idx)"
                :title="p.enabled ? 'Disable' : 'Enable'"
              >
                <svg v-if="p.enabled" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" width="14" height="14">
                  <path d="M20 6L9 17l-5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                <svg v-else viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" width="14" height="14">
                  <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                </svg>
              </button>
              <button class="action-btn delete-btn" @click="removePlugin(idx)" title="Remove">
                <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" width="14" height="14">
                  <path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2m3 0v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6h14z" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </button>
            </div>
          </div>
        </div>

        <div class="plugin-empty" v-else-if="!loading">
          No plugins installed yet
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.plugin-overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  justify-content: center;
  align-items: center;
  background: rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(4px);
  --wails-draggable: no-drag;
}

.plugin-panel {
  width: 480px;
  max-height: 480px;
  background: var(--input-bg);
  backdrop-filter: blur(24px) saturate(1.8);
  -webkit-backdrop-filter: blur(24px) saturate(1.8);
  border: 1.5px solid var(--input-border);
  border-radius: 16px;
  padding: 18px;
  box-shadow: 0 20px 60px rgba(var(--shadow-rgb), 0.18);
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow: hidden;
}

.plugin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.plugin-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--input-text);
}

.close-btn {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: rgba(var(--neutral-rgb), 0.06);
  border-radius: 7px;
  cursor: pointer;
  color: rgba(var(--neutral-rgb), 0.45);
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: rgba(255, 80, 80, 0.12);
  color: rgba(255, 80, 80, 0.85);
}

/* Drop zone */
.drop-zone {
  border: 2px dashed rgba(var(--primary-rgb), 0.2);
  border-radius: 12px;
  padding: 22px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: rgba(var(--neutral-rgb), 0.35);
  font-size: 13px;
  transition: all 0.25s ease;
  cursor: default;
}

.drop-zone svg {
  color: rgba(var(--primary-rgb), 0.4);
}

.drop-zone.drag-over {
  border-color: rgba(var(--primary-rgb), 0.6);
  background: rgba(var(--primary-rgb), 0.06);
  color: rgba(var(--primary-rgb), 0.8);
}

.drop-zone b {
  color: rgba(var(--primary-rgb), 0.7);
  font-weight: 600;
}

.loading-hint {
  font-size: 11px;
  color: rgba(var(--primary-rgb), 0.6);
  animation: pulse 1s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 1; }
}

/* Plugin list */
.plugin-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  overflow-y: auto;
  max-height: 280px;
  padding-right: 2px;
}

.plugin-list::-webkit-scrollbar {
  width: 4px;
}

.plugin-list::-webkit-scrollbar-thumb {
  background: rgba(var(--neutral-rgb), 0.12);
  border-radius: 4px;
}

.plugin-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(var(--neutral-rgb), 0.03);
  transition: all 0.2s ease;
}

.plugin-item:hover {
  background: rgba(var(--neutral-rgb), 0.06);
}

.plugin-item.disabled {
  opacity: 0.5;
}

.plugin-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.plugin-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--input-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.plugin-path {
  font-size: 10px;
  color: var(--input-placeholder);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.plugin-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.action-btn {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: rgba(var(--neutral-rgb), 0.05);
  border-radius: 7px;
  cursor: pointer;
  color: rgba(var(--neutral-rgb), 0.4);
  transition: all 0.2s ease;
}

.toggle-btn.active {
  color: rgba(var(--primary-rgb), 0.8);
  background: rgba(var(--primary-rgb), 0.1);
}

.toggle-btn:hover {
  background: rgba(var(--primary-rgb), 0.12);
  color: rgba(var(--primary-rgb), 0.9);
}

.delete-btn:hover {
  background: rgba(255, 80, 80, 0.12);
  color: rgba(255, 80, 80, 0.85);
}

.plugin-empty {
  text-align: center;
  padding: 16px;
  color: rgba(var(--neutral-rgb), 0.3);
  font-size: 13px;
}
</style>
