<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { ElInput } from 'element-plus'
import { useTheme } from '../composables/useTheme'
import { SearchApps, LaunchApp, HideWindow, ResizeWindow, GetAppIcon, RunPlugins, AddPlugin } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'

const props = defineProps<{ manualResized?: boolean }>()

const promptText = ref<string>('')
const { isDark, toggleTheme } = useTheme()

// Search results
const results = ref<any[]>([])
const selectedIdx = ref<number>(-1)
const hasPluginResults = ref(false)

// Debounce timer
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const doSearch = async (q: string) => {
  // Run app search and plugin search in parallel
  const [appResults, pluginResults] = await Promise.allSettled([
    SearchApps(q),
    RunPlugins(q),
  ])

  const apps: any[] = (appResults.status === 'fulfilled' && appResults.value) ? appResults.value : []
  const plugins: any[] = (pluginResults.status === 'fulfilled' && pluginResults.value) ? pluginResults.value : []

  // Merge: plugin results first (tagged), then app results
  const merged: any[] = []

  // Add plugin entries with source tag
  for (const pe of plugins) {
    merged.push({
      name: pe.name,
      path: pe.path || '',
      icon: pe.icon || '',
      isPlugin: true,
      pluginSource: pe.source || 'plugin',
    })
  }

  // Add app entries (skip duplicates by path)
  const pluginPaths = new Set(plugins.map(p => p.path?.toLowerCase()).filter(Boolean))
  for (const ae of apps) {
    if (!pluginPaths.has(ae.path?.toLowerCase())) {
      merged.push({ ...ae, isPlugin: false, pluginSource: '' })
    }
  }

  results.value = merged
  selectedIdx.value = merged.length > 0 ? 0 : -1
  hasPluginResults.value = plugins.length > 0

  // Lazily load icons for results
  loadIcons(results.value)
}

// Icon cache and loader
const iconMap = ref<Record<string, string>>({})

const loadIcons = (items: any[]) => {
  for (const item of items) {
    if (iconMap.value[item.path]) continue // already cached
    GetAppIcon(item.path).then((b64: string) => {
      if (b64) {
        iconMap.value[item.path] = b64
      }
    }).catch(() => {})
  }
}

watch(promptText, (val) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (!val.trim()) {
    results.value = []
    selectedIdx.value = -1
    return
  }
  debounceTimer = setTimeout(() => doSearch(val), 120)
})

// 启动程序后：清空输入 + 隐藏窗口
const launchAndHide = async (idx: number) => {
  if (idx < 0 || idx >= results.value.length) return
  const item = results.value[idx]
  if (item.path) {
    await LaunchApp(item.path)
  }
  promptText.value = ''
  results.value = []
  selectedIdx.value = -1
  HideWindow()
}

const handleKeydown = (e: KeyboardEvent) => {
  if (!results.value.length) return

  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      selectedIdx.value = (selectedIdx.value + 1) % results.value.length
      scrollToSelected()
      break
    case 'ArrowUp':
      e.preventDefault()
      selectedIdx.value = selectedIdx.value <= 0
        ? results.value.length - 1
        : selectedIdx.value - 1
      scrollToSelected()
      break
    case 'Enter':
      e.preventDefault()
      launchAndHide(selectedIdx.value)
      break
    case 'Tab':
      if (results.value.length > 0) {
        e.preventDefault()
        selectedIdx.value = (selectedIdx.value + 1) % results.value.length
        scrollToSelected()
      }
      break
    case 'Escape':
      results.value = []
      selectedIdx.value = -1
      break
  }
}

const scrollToSelected = () => {
  const el = document.querySelector('.result-item.active')
  el?.scrollIntoView({ block: 'nearest' })
}

const getInitials = (name: string): string => {
  return name
    .split(/[\s\-_]+/)
    .map(w => w.charAt(0))
    .join('')
    .substring(0, 2)
    .toUpperCase()
}

// 结果变化后主动通知 Go 调整窗口大小
watch(results, () => {
  nextTick(() => {
    const container = document.querySelector('.app-container') as HTMLElement
    if (!container) return
    const rect = container.getBoundingClientRect()
    const padding = 16
    const width = Math.ceil(rect.width + padding)
    const height = Math.ceil(rect.height + padding)
    ResizeWindow(width, height)
  })
})

// 显示结果的条件：有结果 && 没有手动resize过
const showResults = () => results.value.length > 0 && !props.manualResized

// Drag-and-drop DLL import
const dragOverInput = ref(false)

const handleDrop = async (e: DragEvent) => {
  e.preventDefault()
  dragOverInput.value = false
  const files = e.dataTransfer?.files
  if (!files || files.length === 0) return
  for (let i = 0; i < files.length; i++) {
    const f = files[i]
    if (!f.name.toLowerCase().endsWith('.dll')) continue
    const pluginName = f.name.replace(/\.dll$/i, '')
    const filePath = (f as any).path || f.name
    await AddPlugin(pluginName, filePath)
  }
}

const handleDragOver = (e: DragEvent) => {
  e.preventDefault()
  dragOverInput.value = true
}

const handleDragLeave = () => {
  dragOverInput.value = false
}

onMounted(() => {
  // 窗口被隐藏时清空输入和结果
  EventsOn("windowHidden", () => {
    promptText.value = ''
    results.value = []
    selectedIdx.value = -1
  })
})

onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>

<template>
  <div class="launcher-container" @keydown="handleKeydown" @drop="handleDrop" @dragover="handleDragOver" @dragleave="handleDragLeave" :class="{ 'drag-over': dragOverInput }">
    <el-input
        v-model="promptText"
        placeholder="Hi, Quokka!"
        class="mainPrompt"
    >
      <template #prefix>
        <div class="prefix-icon">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="1.8"/>
            <path d="M16.5 16.5L21 21" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
          </svg>
        </div>
      </template>
      <template #suffix>
        <button class="theme-btn" @click="toggleTheme" :title="isDark ? 'Light mode' : 'Dark mode'">
          <transition name="icon-swap" mode="out-in">
            <svg v-if="isDark" key="moon" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <svg v-else key="sun" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="12" cy="12" r="4.5" stroke="currentColor" stroke-width="1.8"/>
              <path d="M12 2.5V4.5M12 19.5V21.5M4.5 12H2.5M21.5 12H19.5M5.85 5.85L4.44 4.44M19.56 19.56L18.15 18.15M5.85 18.15L4.44 19.56M19.56 4.44L18.15 5.85" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
            </svg>
          </transition>
        </button>
      </template>
    </el-input>

    <!-- Results dropdown (不依赖 focus，手动resize后隐藏) -->
    <transition name="results-list" appear>
    <div v-if="showResults()" class="results-list">
        <div
          v-for="(item, idx) in results"
          :key="item.path + idx"
          class="result-item"
          :class="{ active: idx === selectedIdx }"
          @mouseenter="selectedIdx = idx"
          @click="launchAndHide(idx)"
        >
          <div class="result-icon">
            <img
              v-if="iconMap[item.path]"
              :src="iconMap[item.path]"
              class="result-icon-img"
              alt=""
            />
            <span v-else>{{ getInitials(item.name) }}</span>
          </div>
          <div class="result-info">
            <span class="result-name">{{ item.name }}</span>
            <span class="result-path">{{ item.isPlugin ? `[${item.pluginSource}]` : item.path }}</span>
          </div>
          <kbd v-if="idx === selectedIdx" class="launch-hint">↵</kbd>
        </div>
      </div>
    </transition>
    <!-- Drag overlay hint -->
    <div class="drag-overlay" v-if="dragOverInput">
      <span class="drag-hint-text">Drop DLL to install plugin</span>
    </div>
    </div>
</template>

<style scoped>
.launcher-container {
  position: relative;
  --wails-draggable: drag;
}

.launcher-container.drag-over :deep(.el-input__wrapper) {
  border-color: rgba(var(--primary-rgb), 0.6) !important;
  box-shadow: 0 0 0 3px rgba(var(--primary-rgb), 0.12), var(--input-focus-shadow) !important;
}

.mainPrompt {
  width: 540px;
}

.mainPrompt :deep(.el-input__wrapper) {
  height: 52px;
  padding: 0 18px;
  background: var(--input-bg);
  border: 1.5px solid var(--input-border);
  border-radius: 16px;
  backdrop-filter: blur(24px) saturate(2.0);
  -webkit-backdrop-filter: blur(24px) saturate(2.0);
  box-shadow: var(--input-shadow) !important;
  transition: all 0.4s var(--ease-out-expo);
}

.mainPrompt :deep(.el-input__wrapper:hover) {
  border-color: var(--input-border-hover);
  box-shadow: var(--input-shadow-hover) !important;
  transform: translateY(-1px);
}

.mainPrompt :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(var(--primary-rgb), 0.45);
  background: var(--input-bg-solid);
  box-shadow: var(--input-focus-shadow) !important;
  transform: translateY(-2px);
}

.mainPrompt :deep(.el-input__inner) {
  font-size: 16px;
  font-weight: 500;
  color: var(--input-text);
  letter-spacing: 0.15px;
  background: transparent !important;
  height: 100%;
  padding: 0;
  --wails-draggable: no-drag;
}

.mainPrompt :deep(.el-input__inner::placeholder) {
  color: var(--input-placeholder);
  font-weight: 400;
  font-size: 15px;
}

.mainPrompt :deep(.el-input__prefix) {
  margin-right: 8px;
}

.mainPrompt :deep(.el-input__suffix) {
  margin-left: 8px;
  gap: 6px;
}

/* Prefix icon */
.prefix-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(var(--primary-rgb), 0.55);
  transition: color 0.3s ease;
}

.mainPrompt :deep(.el-input__wrapper.is-focus) .prefix-icon {
  color: rgba(var(--primary-rgb), 0.95);
}

.prefix-icon svg {
  width: 18px;
  height: 18px;
}

/* Theme toggle button */
.theme-btn {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: rgba(var(--neutral-rgb), 0.05);
  border-radius: 7px;
  cursor: pointer;
  color: rgba(var(--neutral-rgb), 0.40);
  padding: 0;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  --wails-draggable: no-drag;
}

.theme-btn:hover {
  background: rgba(var(--primary-rgb), 0.10);
  color: rgba(var(--primary-rgb), 0.85);
  transform: scale(1.1);
}

.theme-btn:active {
  transform: scale(0.92);
}

.theme-btn svg {
  width: 14px;
  height: 14px;
}

/* Icon swap transition */
.icon-swap-enter-active,
.icon-swap-leave-active {
  transition: all 0.2s ease;
}

.icon-swap-enter-from {
  opacity: 0;
  transform: rotate(-45deg) scale(0.7);
}

.icon-swap-leave-to {
  opacity: 0;
  transform: rotate(45deg) scale(0.7);
}

/* ===== Results dropdown ===== */
.results-list {
  width: 540px;
  margin-top: 6px;
  max-height: 320px;
  overflow-y: auto;
  background: var(--input-bg);
  backdrop-filter: blur(24px) saturate(1.8);
  -webkit-backdrop-filter: blur(24px) saturate(1.8);
  border: 1.5px solid var(--input-border);
  border-radius: 14px;
  padding: 6px;
  box-shadow: 0 12px 48px rgba(var(--shadow-rgb), 0.12), 0 4px 12px rgba(var(--shadow-rgb), 0.06);
  --wails-draggable: no-drag;
}

.results-list::-webkit-scrollbar {
  width: 4px;
}

.results-list::-webkit-scrollbar-thumb {
  background: rgba(var(--neutral-rgb), 0.12);
  border-radius: 4px;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.15s ease, transform 0.1s ease;
}

.result-item:hover,
.result-item.active {
  background: rgba(var(--primary-rgb), 0.08);
}

.result-item.active {
  background: rgba(var(--primary-rgb), 0.12);
}

.result-item:active {
  transform: scale(0.985);
}

.result-icon {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  background: rgba(var(--primary-rgb), 0.10);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.result-icon span {
  font-size: 11px;
  font-weight: 700;
  color: rgba(var(--primary-rgb), 0.8);
  letter-spacing: 0.5px;
}

.result-icon-img {
  width: 24px;
  height: 24px;
  object-fit: contain;
}

.result-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.result-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--input-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.result-path {
  font-size: 11px;
  color: var(--input-placeholder);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.launch-hint {
  font-size: 11px;
  font-weight: 600;
  color: rgba(var(--primary-rgb), 0.6);
  background: rgba(var(--primary-rgb), 0.06);
  border: 1px solid rgba(var(--primary-rgb), 0.12);
  border-radius: 4px;
  padding: 1px 5px;
  font-family: inherit;
  flex-shrink: 0;
}

/* Results animation */
.results-list-enter-active,
.results-list-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.results-list-enter-from {
  opacity: 0;
  transform: translateY(-8px) scale(0.98);
}

.results-list-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.99);
}

/* Drag overlay hint */
.drag-overlay {
  position: absolute;
  inset: 0;
  background: rgba(var(--primary-rgb), 0.08);
  border: 2px dashed rgba(var(--primary-rgb), 0.4);
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.launcher-container.drag-over .drag-overlay {
  opacity: 1;
}

.drag-hint-text {
  font-size: 13px;
  color: rgba(var(--primary-rgb), 0.7);
  font-weight: 500;
  background: rgba(var(--primary-rgb), 0.1);
  padding: 6px 12px;
  border-radius: 8px;
  backdrop-filter: blur(8px);
}
</style>
