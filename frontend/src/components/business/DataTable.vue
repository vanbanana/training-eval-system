<script setup lang="ts" generic="TData">
import {
  type ColumnDef,
  FlexRender,
  type SortingState,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useVueTable,
} from '@tanstack/vue-table'
import { ref } from 'vue'
import {
  Table,
  TableBody,
  TableCell,
  TableEmpty,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { ArrowUpDown, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Skeleton } from '@/components/ui/skeleton'

const props = withDefaults(
  defineProps<{
    columns: ColumnDef<TData, unknown>[]
    data: TData[]
    pageSize?: number
    enableSorting?: boolean
    enablePagination?: boolean
    loading?: boolean
    emptyText?: string
  }>(),
  {
    pageSize: 20,
    enableSorting: true,
    enablePagination: true,
    loading: false,
  },
)

const emits = defineEmits<{
  (e: 'rowClick', row: TData): void
}>()

const sorting = ref<SortingState>([])

const table = useVueTable({
  get data() {
    return props.data
  },
  get columns() {
    return props.columns
  },
  state: {
    get sorting() {
      return sorting.value
    },
  },
  onSortingChange: (updater) => {
    sorting.value = typeof updater === 'function' ? updater(sorting.value) : updater
  },
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getPaginationRowModel: getPaginationRowModel(),
  initialState: {
    pagination: {
      pageSize: props.pageSize,
    },
  },
})
</script>

<template>
  <div class="rounded-lg border border-border bg-card overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow v-for="hg in table.getHeaderGroups()" :key="hg.id">
          <TableHead v-for="header in hg.headers" :key="header.id" :class="header.column.getCanSort() ? 'cursor-pointer select-none' : ''" @click="header.column.getCanSort() && header.column.toggleSorting()">
            <div class="flex items-center gap-1.5">
              <FlexRender v-if="!header.isPlaceholder" :render="header.column.columnDef.header" :props="header.getContext()" />
              <ArrowUpDown v-if="header.column.getCanSort()" class="h-3 w-3 text-subtle-foreground" />
            </div>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <template v-if="loading">
          <TableRow v-for="n in 5" :key="n">
            <TableCell v-for="col in columns" :key="(col as any).id ?? Math.random()">
              <Skeleton class="h-4 w-full" />
            </TableCell>
          </TableRow>
        </template>
        <template v-else-if="table.getRowModel().rows.length">
          <TableRow
            v-for="row in table.getRowModel().rows"
            :key="row.id"
            class="cursor-pointer"
            @click="emits('rowClick', row.original)"
          >
            <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
              <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
            </TableCell>
          </TableRow>
        </template>
        <TableEmpty v-else :colspan="columns.length">{{ emptyText ?? '暂无数据' }}</TableEmpty>
      </TableBody>
    </Table>

    <div v-if="enablePagination && table.getRowModel().rows.length > 0" class="flex items-center justify-between border-t border-border px-4 py-2.5">
      <div class="text-xs text-muted-foreground">
        共 {{ table.getFilteredRowModel().rows.length }} 条 · 第 {{ table.getState().pagination.pageIndex + 1 }} / {{ Math.max(1, table.getPageCount()) }} 页
      </div>
      <div class="flex items-center gap-1.5">
        <Button variant="outline" size="icon-sm" :disabled="!table.getCanPreviousPage()" @click="table.previousPage()">
          <ChevronLeft class="h-4 w-4" />
        </Button>
        <Button variant="outline" size="icon-sm" :disabled="!table.getCanNextPage()" @click="table.nextPage()">
          <ChevronRight class="h-4 w-4" />
        </Button>
      </div>
    </div>
  </div>
</template>
