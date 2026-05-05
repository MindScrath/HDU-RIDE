<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import type { LectureChapter } from '../types'

const route = useRoute()
const router = useRouter()
const chapters = ref<LectureChapter[]>([])
const html = ref('')
const classId = computed(() => String(route.params.classId ?? ''))
const basePath = computed(() => (classId.value ? `/classes/${classId.value}/lectures` : '/lectures'))
const sections = computed(() => chapters.value.flatMap((chapter) => chapter.sections))
const selected = computed(() => String(route.params.lectureId ?? sections.value[0]?.id ?? ''))
const selectedSection = computed(() => sections.value.find((section) => section.id === selected.value))

async function loadLectures() {
  const path = classId.value ? `/api/classes/${classId.value}/lectures` : '/api/lectures'
  const data = await api.get<{ lectures: LectureChapter[] }>(path)
  chapters.value = data.lectures
  if (!route.params.lectureId && sections.value[0]) {
    router.replace(`${basePath.value}/${sections.value[0].id}`)
  }
}

async function loadLecture() {
  if (!selected.value) return
  html.value = (await api.get<{ html: string }>(`${classId.value ? `/api/classes/${classId.value}` : '/api'}/lectures/${selected.value}`)).html
}

watch(classId, loadLectures, { immediate: true })
watch(selected, loadLecture, { immediate: true })
</script>

<template>
  <div class="page-grid">
    <aside class="panel scroll lecture-tree">
      <div class="panel-head"><h3>章节</h3></div>
      <div v-for="chapter in chapters" :key="chapter.id" class="chapter-block">
        <div class="chapter-title">{{ chapter.title }}</div>
        <RouterLink
          v-for="section in chapter.sections"
          :key="section.id"
          :to="`${basePath}/${section.id}`"
          class="section-item"
          :class="{ active: section.id === selected }"
        >
          <span>{{ section.title }}</span>
          <small>已发布</small>
        </RouterLink>
      </div>
    </aside>
    <article class="panel scroll">
      <div class="panel-head">
        <h2>{{ selectedSection?.title ?? '讲义' }}</h2>
      </div>
      <div class="markdown" v-html="html" />
    </article>
  </div>
</template>
