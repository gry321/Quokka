<script lang="ts" setup>
import MainPrompt from "./components/MainPrompt.vue";
import { ElInput } from "element-plus";
import { nextTick, onMounted, ref, onUnmounted } from "vue";
import { ResizeWindow } from "../wailsjs/go/main/App";

const prompt = ref<InstanceType<typeof ElInput> | null>(null);

const doFocus = () => {
  prompt.value?.focus?.();
};

// 窗口自适应函数
const fitWindow = () => {
  const container = document.querySelector('.app-container') as HTMLElement;
  if (!container) return;
  // 使用 getBoundingClientRect 获取精确尺寸（包括 padding/border）
  const rect = container.getBoundingClientRect();
  const padding = 20; // 额外边距，避免内容贴边
  const width = Math.ceil(rect.width + padding);
  const height = Math.ceil(rect.height + padding);
  ResizeWindow(width, height);
};

let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  // 首次渲染后立即调整
  nextTick(fitWindow);

  // 监听容器尺寸变化（如输入框清空按钮显示/隐藏导致尺寸变化）
  const container = document.querySelector('.app-container');
  if (container) {
    resizeObserver = new ResizeObserver(() => {
      fitWindow();
    });
    resizeObserver.observe(container);
  }

  // 聚焦输入框
  doFocus();
});

onUnmounted(() => {
  resizeObserver?.disconnect();
});

// 页面完全加载后再调整一次（保险）
window.addEventListener('load', () => {
  setTimeout(fitWindow, 50);
});
</script>

<template>
  <!-- 去掉 position: absolute，让容器由内容撑开，并居中 -->
  <div class="app-container">
    <MainPrompt ref="prompt" />
  </div>
</template>

<style scoped>
/* 容器由内容撑开，无需固定宽高 */
.app-container {
  display: inline-block;   /* 宽度由内容决定 */
  padding: 16px 20px;      /* 内边距，避免输入框紧贴窗口边缘 */
  /* 若需要窗口可拖拽，可保留 --wails-draggable，但建议在父级 #app 设置 */
}
</style>