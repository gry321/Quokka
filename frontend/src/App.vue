<script lang="ts" setup>
import MainPrompt from "./components/MainPrompt.vue";
import PluginManager from "./components/PluginManager.vue";
import { ElInput } from "element-plus";
import { nextTick, onMounted, ref, onUnmounted } from "vue";
import { ResizeWindow } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";

const prompt = ref<InstanceType<typeof ElInput> | null>(null);

const doFocus = () => {
  prompt.value?.focus?.();
};

/* ---- 动态窗口适配（带防抖 + 手动resize检测） ---- */
let resizeTimer: ReturnType<typeof setTimeout> | null = null;
let fittingContent = false;
let manualResized = false;

const fitWindow = () => {
  if (manualResized) return; // 用户手动调整过，不再自动适配
  if (resizeTimer) clearTimeout(resizeTimer);
  resizeTimer = setTimeout(() => {
    const container = document.querySelector('.app-container') as HTMLElement;
    if (!container) return;
    const rect = container.getBoundingClientRect();
    const padding = 16;
    const width = Math.ceil(rect.width + padding);
    const height = Math.ceil(rect.height + padding);
    fittingContent = true;
    ResizeWindow(width, height);
    setTimeout(() => { fittingContent = false; }, 100);
  }, 30);
};

let resizeObserver: ResizeObserver | null = null;
let windowResizeHandler: (() => void) | null = null;

onMounted(() => {
  nextTick(fitWindow);

  const container = document.querySelector('.app-container');
  if (container) {
    resizeObserver = new ResizeObserver(() => fitWindow());
    resizeObserver.observe(container);
  }

  // 检测手动窗口resize（非 fitWindow 触发的）
  windowResizeHandler = () => {
    if (!fittingContent) {
      manualResized = true;
    }
  };
  window.addEventListener('resize', windowResizeHandler);

  // 窗口重新显示时重置手动resize状态
  EventsOn("windowShown", () => {
    manualResized = false;
    nextTick(fitWindow);
    doFocus();
  });

  doFocus();
});

onUnmounted(() => {
  resizeObserver?.disconnect();
  if (resizeTimer) clearTimeout(resizeTimer);
  if (windowResizeHandler) {
    window.removeEventListener('resize', windowResizeHandler);
  }
});

window.addEventListener('load', () => {
  setTimeout(fitWindow, 50);
});

/* 暴露给 MainPrompt 查询 */
const isManualResized = () => manualResized;
defineExpose({ isManualResized });

/* Plugin manager panel */
const showPlugins = ref(false);
</script>

<template>
  <!-- Plugin toggle button — outside inline-block container, absolute to #app -->
  <button class="plugin-toggle-btn" @click="showPlugins = !showPlugins" title="Plugins">
    <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" width="15" height="15">
      <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
    </svg>
  </button>

  <div class="app-container">
    <transition name="fade-up" appear>
      <div class="prompt-wrapper">
        <MainPrompt ref="prompt" :manual-resized="manualResized" />
      </div>
    </transition>
  </div>

  <!-- Plugin manager overlay — Teleported to body to escape inline-block -->
  <PluginManager v-if="showPlugins" @close="showPlugins = false" />
</template>

<style scoped>
.app-container {
  display: inline-block;
  padding: 8px;
  position: relative;
  z-index: 1;
}

.prompt-wrapper {
  position: relative;
}

/* 入场动画 */
.fade-up-enter-active {
  transition: opacity 0.5s ease, transform 0.5s cubic-bezier(0.16, 1, 0.3, 1);
}

.fade-up-enter-from {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}

.fade-up-enter-to {
  opacity: 1;
  transform: translateY(0) scale(1);
}

/* Plugin toggle button — positioned absolute to #app (which has position: relative) */
.plugin-toggle-btn {
  position: absolute;
  top: 10px;
  right: 12px;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1.5px solid rgba(var(--neutral-rgb), 0.08);
  background: rgba(var(--neutral-rgb), 0.06);
  border-radius: 9px;
  cursor: pointer;
  color: rgba(var(--neutral-rgb), 0.5);
  transition: all 0.25s ease;
  z-index: 50;
  --wails-draggable: no-drag;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.plugin-toggle-btn:hover {
  background: rgba(var(--primary-rgb), 0.14);
  border-color: rgba(var(--primary-rgb), 0.25);
  color: rgba(var(--primary-rgb), 0.85);
  transform: scale(1.12);
  box-shadow: 0 2px 12px rgba(var(--primary-rgb), 0.15);
}

.plugin-toggle-btn:active {
  transform: scale(0.94);
}
</style>
