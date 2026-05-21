import { useColorMode } from '@vueuse/core'

/**
 * 主题 composable，自动持久化到 localStorage 的 `tes-theme` 键
 * 使用 .dark 类切换 shadcn-vue 暗色主题变量
 */
export function useTheme() {
  return useColorMode({
    attribute: 'class',
    modes: { light: '', dark: 'dark' },
    storageKey: 'tes-theme',
    selector: 'html',
  })
}
