import { ref, onMounted } from 'vue'

const isDark = ref(false)

let initialized = false

function initTheme() {
  if (initialized) return
  initialized = true

  const saved = localStorage.getItem('quokka-theme')
  if (saved === 'dark') {
    isDark.value = true
  } else if (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    isDark.value = true
  }
  applyTheme()
}

function applyTheme() {
  if (isDark.value) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function toggleTheme() {
  isDark.value = !isDark.value
  localStorage.setItem('quokka-theme', isDark.value ? 'dark' : 'light')
  applyTheme()
}

export function useTheme() {
  onMounted(initTheme)
  return { isDark, toggleTheme }
}
