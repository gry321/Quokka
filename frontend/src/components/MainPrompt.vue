<script setup lang="ts">
import { ref } from 'vue'
import { ElInput } from 'element-plus'   // 导入组件类型

const promptText = ref<string>('')

</script>

<template>
  <el-input
      v-model="promptText"
      placeholder="Hi, Quokka!"
      class="mainPrompt"
      clearable
  />
</template>

<style scoped>
.mainPrompt {
  width: 560px;
}

.mainPrompt :deep(.el-input__wrapper) {
  height: 64px;
  padding: 0 24px;
  background: var(--input-bg);
  border: 1.5px solid var(--input-border);
  border-radius: 16px;
  backdrop-filter: blur(12px);
  box-shadow: var(--input-shadow) !important;
  background-color: transparent;
  transition: all 0.25s ease;
}

.mainPrompt :deep(.el-input__inner) {
  font-size: 22px;
  font-weight: 500;
  color: var(--input-text);
  letter-spacing: 0.5px;
  background: transparent !important;
  height: 100%;
  padding: 0;
}

.mainPrompt :deep(.el-input__inner::placeholder) {
  color: var(--input-placeholder);
  font-weight: 400;
  font-size: 20px;
  letter-spacing: 0.3px;
}

.mainPrompt :deep(.el-input__wrapper.is-focus) {
  border-color: #7c6bff;
  background: var(--input-bg);
  box-shadow: var(--input-focus-shadow) !important;
}

.mainPrompt :deep(.el-input__suffix) {
  color: var(--suffix-color);
}
.mainPrompt :deep(.el-input__suffix:hover) {
  color: var(--suffix-hover);
}
</style>