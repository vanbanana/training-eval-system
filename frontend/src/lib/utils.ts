import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * 类名合并工具函数（shadcn-vue 标配）
 * - clsx 处理条件类名
 * - tailwind-merge 合并 Tailwind 冲突类（如 px-2 + px-4 → px-4）
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
