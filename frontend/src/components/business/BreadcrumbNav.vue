<script setup lang="ts">
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { RouterLink } from 'vue-router'

interface BreadcrumbItemDef {
  label: string
  to?: string
}

defineProps<{
  items: BreadcrumbItemDef[]
}>()
</script>

<template>
  <Breadcrumb>
    <BreadcrumbList>
      <template v-for="(item, idx) in items" :key="idx">
        <BreadcrumbItem>
          <BreadcrumbLink v-if="item.to" as-child class="text-muted-foreground">
            <RouterLink :to="item.to">{{ item.label }}</RouterLink>
          </BreadcrumbLink>
          <BreadcrumbPage v-else>{{ item.label }}</BreadcrumbPage>
        </BreadcrumbItem>
        <BreadcrumbSeparator v-if="idx < items.length - 1" />
      </template>
    </BreadcrumbList>
  </Breadcrumb>
</template>
