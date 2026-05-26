<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { useMarkdown } from '../composables/useMarkdown'
import type { ClassItem, LectureChapter } from '../types'

const route = useRoute()
const router = useRouter()
const { render } = useMarkdown()

const classes = ref<ClassItem[]>([])
const chapters = ref<LectureChapter[]>([])
const raw = ref('')
const loading = ref(false)
const error = ref('')

const classId = computed(() => String(route.params.classId ?? ''))
const basePath = computed(() => (classId.value ? `/classes/${classId.value}/lectures` : '/lectures'))
const sections = computed(() => chapters.value.flatMap((chapter) => chapter.sections))
const selected = computed(() => String(route.params.lectureId ?? sections.value[0]?.id ?? ''))
const selectedSection = computed(() => sections.value.find((section) => section.id === selected.value))

const renderedHtml = computed(() => {
  if (!raw.value) return ''
  try {
    return render(raw.value)
  } catch (e) {
    console.error('markdown render failed', e)
    return '<p style="color:red">Markdown 渲染失败</p>'
  }
})

let abortController: AbortController | null = null

async function loadClasses() {
  const data = await api.get<{ classes: ClassItem[] }>('/api/classes')
  classes.value = data.classes
  if (!classId.value && classes.value[0]) {
    router.replace(`/classes/${classes.value[0].id}/lectures`)
  }
}

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

  abortController?.abort()
  abortController = new AbortController()

  loading.value = true
  error.value = ''
  raw.value = ''

  try {
    const url = `${classId.value ? `/api/classes/${classId.value}` : '/api'}/lectures/${selected.value}`
    const data = await api.get<{ markdown: string }>(url)
    raw.value = data.markdown
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') return
    error.value = err instanceof Error ? err.message : '讲义加载失败'
  } finally {
    loading.value = false
  }
}

function switchClass(nextClassId: string) {
  if (nextClassId) router.push(`/classes/${nextClassId}/lectures`)
}

watch(() => route.params.classId, loadClasses, { immediate: true })
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
        <div>
          <h2>{{ selectedSection?.title ?? '讲义' }}</h2>
        </div>
        <el-select v-if="classes.length" :model-value="classId" class="context-select" placeholder="选择班级" @change="switchClass">
          <el-option v-for="klass in classes" :key="klass.id" :label="klass.name" :value="klass.id" />
        </el-select>
      </div>
      <div v-if="loading" class="lecture-placeholder">
        <el-skeleton :rows="10" animated />
      </div>
      <div v-else-if="error" class="lecture-placeholder">
        <el-result icon="error" :title="error" />
      </div>
      <div v-else class="markdown" v-html="renderedHtml" />
    </article>
  </div>
</template>
