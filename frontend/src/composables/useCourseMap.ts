  import { ref, shallowRef } from 'vue'
import axios from 'axios'

export interface Course {
  id: number
  name: string
  code: string
  is_archived: boolean
  class_count: number
}

// 全局共享 map（跨组件复用，避免重复拉取）
const courseMap = shallowRef<Map<number, Course>>(new Map())
const loading = ref(false)
const loaded = ref(false)
let inflight: Promise<void> | null = null

async function loadInternal() {
  loading.value = true
  try {
    const { data } = await axios.get<Course[]>('/api/courses')
    const m = new Map<number, Course>()
    for (const c of data) m.set(c.id, c)
    courseMap.value = m
    loaded.value = true
  } finally {
    loading.value = false
  }
}

/**
 * 课程 ID → 名称映射 composable.
 *
 * - 全局共享单例 map，多个组件可同时调用 `load()`，只会真正发一次请求
 * - 未命中时返回 `课程 #ID` 作为兜底，避免界面崩溃
 *
 * 用法:
 *   const { load, courseName, courseMap } = useCourseMap()
 *   onMounted(load)
 *   ...
 *   {{ courseName(task.course_id) }}
 */
export function useCourseMap() {
  async function load(force = false) {
    if (loaded.value && !force) return
    if (inflight) {
      await inflight
      return
    }
    inflight = loadInternal()
      .catch((e) => {
        // eslint-disable-next-line no-console
        console.warn('[useCourseMap] failed to load courses', e)
      })
      .finally(() => {
        inflight = null
      })
    await inflight
  }

  function courseName(id: number | null | undefined): string {
    if (id == null) return '——'
    return courseMap.value.get(id)?.name ?? `课程 #${id}`
  }

  function courseCode(id: number | null | undefined): string {
    if (id == null) return ''
    return courseMap.value.get(id)?.code ?? ''
  }

  return {
    load,
    loading,
    loaded,
    courseMap,
    courseName,
    courseCode,
  }
}
