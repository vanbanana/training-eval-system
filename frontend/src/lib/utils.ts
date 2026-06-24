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

/**
 * 生成文字头像的缩写。
 * - 中文/CJK 名称：取后两字作为字号（如「系统管理员」→「理员」、「张伟」→「张伟」），
 *   避免与紧邻的全名首字重复造成「系系统管理员」式的视觉重影。
 * - 拉丁名称：取前两个单词首字母（如「John Doe」→「JD」），否则取前两位字母。
 */
export function avatarInitial(name?: string | null): string {
  const n = (name ?? '').trim()
  if (!n) return '?'
  if (/[\u4e00-\u9fff]/.test(n)) {
    return n.length <= 2 ? n : n.slice(-2)
  }
  const parts = n.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return n.slice(0, 2).toUpperCase()
}
