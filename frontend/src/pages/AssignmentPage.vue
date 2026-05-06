<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { ElMessage } from 'element-plus'
import { Close, FullScreen } from '@element-plus/icons-vue'
import { api } from '../api'
import { useSession } from '../composables/useSession'
import type { Assignment, ClassItem, Submission } from '../types'

interface AssignmentEntry {
  assignment: Assignment
  classId: string
  className: string
  courseId: string
}

interface SubmissionRow {
  submission: Submission
  studentName: string
  grade: { id: string | null; score: number | null; comment: string; publishedAt?: string | null }
}

const route = useRoute()
const router = useRouter()
const session = useSession()
const classes = ref<ClassItem[]>([])
const entries = ref<AssignmentEntry[]>([])
const html = ref('')
const status = ref<Record<string, unknown>>({})
const rows = ref<SubmissionRow[]>([])
const workspaceURL = ref('')
const workspaceLoading = ref(false)
const workspaceFullscreen = ref(false)
const submitting = ref(false)
const gradeDialog = ref(false)
const current = ref<Submission | null>(null)
const selectedSubmission = ref<SubmissionRow | null>(null)
const historyStudent = ref('')
const historyOpen = ref(false)
const promptOpen = ref(true)
const classFilter = ref('')
const gradeForm = reactive({ score: 0, comment: '' })
const grid = ref<HTMLElement | null>(null)
const layout = reactive({
  list: Number(localStorage.getItem('hdu.assignment.listWidth')) || 205,
  ide: Number(localStorage.getItem('hdu.assignment.ideWidth')) || 360
})

const classIdParam = computed(() => String(route.params.classId ?? ''))
const globalMode = computed(() => !classIdParam.value)
const isStudent = computed(() => session.state.user?.role === 'student')
const selected = computed(() => String(route.params.assignmentId ?? filteredEntries.value[0]?.assignment.id ?? ''))
const selectedEntry = computed(() => {
  const queryClass = String(route.query.classId ?? '')
  return (
    entries.value.find((entry) => entry.assignment.id === selected.value && (!queryClass || entry.classId === queryClass)) ??
    filteredEntries.value.find((entry) => entry.assignment.id === selected.value) ??
    filteredEntries.value[0]
  )
})
const activeClassId = computed(() => classIdParam.value || selectedEntry.value?.classId || '')
const selectedAssignment = computed(() => selectedEntry.value?.assignment)
const latestRows = computed(() => {
  const byUser = new Map<string, SubmissionRow>()
  for (const row of rows.value) {
    const prev = byUser.get(row.submission.userId)
    if (!prev || row.submission.attempt > prev.submission.attempt) byUser.set(row.submission.userId, row)
  }
  return Array.from(byUser.values()).sort((a, b) => a.studentName.localeCompare(b.studentName, 'zh-CN'))
})
const historyRows = computed(() =>
  rows.value
    .filter((row) => row.submission.userId === historyStudent.value)
    .sort((a, b) => b.submission.attempt - a.submission.attempt)
)
const gridStyle = computed(() => ({
  '--assignment-list-width': `${layout.list}px`,
  '--assignment-ide-width': `${layout.ide}px`
}))
const filteredEntries = computed(() => {
  return entries.value.filter((entry) => !classFilter.value || entry.classId === classFilter.value)
})
const reviewedCount = computed(() => latestRows.value.filter((row) => row.grade.score !== null).length)
const pendingCount = computed(() => latestRows.value.filter((row) => row.grade.score === null).length)
const averageScore = computed(() => {
  const scores = latestRows.value.map((row) => row.grade.score).filter((score): score is number => score !== null)
  return scores.length ? (scores.reduce((sum, score) => sum + score, 0) / scores.length).toFixed(1) : '-'
})

function assignmentPath(entry: AssignmentEntry) {
  if (!globalMode.value) return `/classes/${entry.classId}/assignments/${entry.assignment.id}`
  return { path: `/assignments/${entry.assignment.id}`, query: { classId: entry.classId } }
}

async function loadAssignments() {
  classes.value = (await api.get<{ classes: ClassItem[] }>('/api/classes')).classes
  const visibleClasses = classIdParam.value ? classes.value.filter((item) => item.id === classIdParam.value) : classes.value
  const loaded = await Promise.all(
    visibleClasses.map(async (klass) => {
      const data = await api.get<{ assignments: Assignment[] }>(`/api/classes/${klass.id}/assignments`)
      return data.assignments.map((assignment) => ({
        assignment,
        classId: klass.id,
        className: klass.name,
        courseId: klass.courseId
      }))
    })
  )
  entries.value = loaded.flat()
  if (globalMode.value) classFilter.value = String(route.query.classId ?? '')
  const exists = entries.value.some(
    (entry) => entry.assignment.id === route.params.assignmentId && (!route.query.classId || entry.classId === route.query.classId)
  )
  if (!exists && filteredEntries.value[0]) router.replace(assignmentPath(filteredEntries.value[0]))
}

function chooseClass() {
  const next = entries.value.find((entry) => !classFilter.value || entry.classId === classFilter.value)
  if (next) router.replace(assignmentPath(next))
}

async function loadAssignment() {
  if (!selected.value || !activeClassId.value) return
  const data = await api.get<{ assignment: Assignment; html: string; status: Record<string, unknown> }>(
    `/api/classes/${activeClassId.value}/assignments/${selected.value}`
  )
  html.value = data.html
  status.value = data.status
  promptOpen.value = isStudent.value
  selectedSubmission.value = null
  workspaceURL.value = ''
  workspaceFullscreen.value = false
  if (isStudent.value) {
    rows.value = []
  } else {
    await loadSubmissions()
  }
}

async function loadSubmissions() {
  rows.value = (
    await api.get<{ submissions: SubmissionRow[] }>(
      `/api/classes/${activeClassId.value}/assignments/${selected.value}/submissions`
    )
  ).submissions
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

async function waitForGateway(url: string) {
  for (let i = 0; i < 30; i += 1) {
    try {
      const response = await fetch(url, { credentials: 'include', cache: 'no-store' })
      if (response.ok) return
    } catch {
      await sleep(700)
      continue
    }
    await sleep(700)
  }
  throw new Error('RStudio 尚未就绪')
}

async function startWorkspace() {
  workspaceLoading.value = true
  try {
    const data = await api.post<{ workspace: { id: string; ideURL: string } }>(
      `/api/classes/${activeClassId.value}/assignments/${selected.value}/workspace`
    )
    await waitForGateway(data.workspace.ideURL)
    workspaceURL.value = data.workspace.ideURL
  } finally {
    workspaceLoading.value = false
  }
}

async function submitAssignment() {
  submitting.value = true
  try {
    await api.post(`/api/classes/${activeClassId.value}/assignments/${selected.value}/submit`)
    ElMessage.success('工作区已提交')
    await loadAssignment()
  } finally {
    submitting.value = false
  }
}

async function selectSubmission(row: SubmissionRow) {
  selectedSubmission.value = row
  workspaceURL.value = ''
  workspaceLoading.value = true
  try {
    const data = await api.post<{ workspace: { id: string; ideURL: string } }>(
      `/api/submissions/${row.submission.id}/workspace`
    )
    await waitForGateway(data.workspace.ideURL)
    workspaceURL.value = data.workspace.ideURL
  } finally {
    workspaceLoading.value = false
  }
}

function openGrade(row: SubmissionRow) {
  void selectSubmission(row)
  current.value = row.submission
  gradeForm.score = row.grade.score ?? 0
  gradeForm.comment = row.grade.comment ?? ''
  gradeDialog.value = true
}

async function saveGrade() {
  if (!current.value) return
  const data = await api.post<{ id: string }>(`/api/submissions/${current.value.id}/grade`, gradeForm)
  await api.post(`/api/grades/${data.id}/publish`)
  ElMessage.success('成绩已发布')
  gradeDialog.value = false
  await loadSubmissions()
}

function showHistory(row: SubmissionRow) {
  historyStudent.value = row.submission.userId
  historyOpen.value = true
}

function switchAttempt(row: SubmissionRow) {
  historyOpen.value = false
  void selectSubmission(row)
}

function rowClassName({ row }: { row: SubmissionRow }) {
  return row.submission.id === selectedSubmission.value?.submission.id ? 'selected-submission-row' : ''
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), Math.max(min, max))
}

function startResize(target: 'list' | 'ide', event: PointerEvent) {
  const rect = grid.value?.getBoundingClientRect()
  if (!rect) return
  event.preventDefault()

  const move = (next: PointerEvent) => {
    if (target === 'list') {
      layout.list = clamp(next.clientX - rect.left, 170, 280)
    } else {
      layout.ide = clamp(rect.right - next.clientX, 280, rect.width - layout.list - 430)
    }
  }
  const stop = () => {
    localStorage.setItem('hdu.assignment.listWidth', String(layout.list))
    localStorage.setItem('hdu.assignment.ideWidth', String(layout.ide))
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', stop)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stop, { once: true })
}

watch(classIdParam, loadAssignments, { immediate: true })
watch([selected, activeClassId, isStudent], loadAssignment, { immediate: true })
watch(
  () => route.query.classId,
  (value) => {
    if (globalMode.value) classFilter.value = String(value ?? '')
  }
)

async function exportGrades() {
  const blob = await api.download(`/api/classes/${activeClassId.value}/assignments/${selected.value}/grades/export?format=csv`)
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${selectedEntry.value?.className ?? 'class'}-${selected.value}-grades.csv`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div ref="grid" class="assignment-grid" :style="gridStyle">
    <aside class="panel scroll assignment-list-panel">
      <div class="panel-head"><h3>作业</h3></div>
      <div v-if="globalMode" class="assignment-filters">
        <el-select v-model="classFilter" clearable placeholder="选择班级" @change="chooseClass">
          <el-option v-for="klass in classes" :key="klass.id" :label="klass.name" :value="klass.id" />
        </el-select>
      </div>
      <RouterLink
        v-for="entry in filteredEntries"
        :key="`${entry.classId}-${entry.assignment.id}`"
        :to="assignmentPath(entry)"
        class="list-item"
        :class="{ active: entry.assignment.id === selected && entry.classId === activeClassId }"
      >
        <strong>{{ entry.assignment.title }}</strong>
        <div class="muted">{{ entry.className }}</div>
        <div class="muted">截止 {{ dayjs(entry.assignment.dueAt).format('MM-DD HH:mm') }}</div>
      </RouterLink>
    </aside>

    <div class="splitter" role="separator" aria-label="调整作业列表宽度" @pointerdown="startResize('list', $event)" />

    <section class="panel scroll">
      <div class="panel-head">
        <div>
          <h2>{{ selectedAssignment?.title ?? '作业' }}</h2>
          <span class="muted">{{ selectedEntry?.className }}</span>
        </div>
      </div>

      <div class="prompt-block" :class="{ collapsed: !promptOpen }">
        <button class="prompt-toggle" @click="promptOpen = !promptOpen">
          <strong>作业题面</strong>
          <span>{{ promptOpen ? '收起' : '展开' }}</span>
        </button>
        <div v-show="promptOpen" class="markdown" v-html="html" />
      </div>

      <div v-if="isStudent" class="student-submit">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="提交次数">{{ status.attempts ?? 0 }}</el-descriptions-item>
          <el-descriptions-item label="最近提交">{{ status.latestSubmittedAt ?? '未提交' }}</el-descriptions-item>
        </el-descriptions>
        <div class="submit-actions">
          <el-button type="primary" :loading="submitting" @click="submitAssignment">提交 RStudio 工作区</el-button>
        </div>
      </div>

      <div v-else class="grading-overview">
        <div class="grading-toolbar">
          <el-button type="primary" plain @click="exportGrades">导出成绩 CSV</el-button>
        </div>
        <div class="metric-row">
          <div><strong>{{ latestRows.length }}</strong><span>提交数</span></div>
          <div><strong>{{ pendingCount }}</strong><span>待批改</span></div>
          <div><strong>{{ reviewedCount }}</strong><span>已批改</span></div>
          <div><strong>{{ averageScore }}</strong><span>平均分</span></div>
        </div>
        <el-table
          :data="latestRows"
          :row-class-name="rowClassName"
          size="small"
          style="width: 100%; margin-top: 10px"
          @row-click="selectSubmission"
        >
          <el-table-column prop="studentName" label="学生" width="110" />
          <el-table-column label="提交" min-width="140">
            <template #default="{ row }">
              <el-button link type="primary" @click.stop="showHistory(row)">第 {{ row.submission.attempt }} 次</el-button>
              <el-tag v-if="row.submission.late" type="warning">补交</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="成绩" width="82">
            <template #default="{ row }">{{ row.grade.score ?? '未评分' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="112">
            <template #default="{ row }">
              <el-button link type="primary" @click.stop="selectSubmission(row)">查看</el-button>
              <el-button link type="primary" @click.stop="openGrade(row)">批改</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <div class="splitter" role="separator" aria-label="调整 RStudio 宽度" @pointerdown="startResize('ide', $event)" />

    <section class="panel workspace-panel" :class="{ fullscreen: workspaceFullscreen }">
      <div class="panel-head">
        <h3>{{ selectedSubmission ? '批改工作区' : 'RStudio 工作区' }}</h3>
        <div class="workspace-actions">
          <el-button v-if="workspaceURL" :icon="workspaceFullscreen ? Close : FullScreen" @click="workspaceFullscreen = !workspaceFullscreen">
            {{ workspaceFullscreen ? '退出全屏' : '全屏' }}
          </el-button>
          <el-button v-if="isStudent" type="primary" :loading="workspaceLoading" @click="startWorkspace">打开 RStudio</el-button>
          <el-button v-else-if="selectedSubmission" type="primary" :loading="workspaceLoading" @click="selectSubmission(selectedSubmission)">重新打开</el-button>
        </div>
      </div>
      <iframe v-if="workspaceURL" :src="workspaceURL" class="ide-frame" />
      <div v-else-if="!isStudent && selectedSubmission" class="submission-preview">
        <div class="preview-meta">
          <strong>{{ selectedSubmission.studentName }}</strong>
          <span>第 {{ selectedSubmission.submission.attempt }} 次提交</span>
          <el-tag v-if="selectedSubmission.grade.score !== null" type="success">{{ selectedSubmission.grade.score }}</el-tag>
          <el-tag v-else type="warning">未评分</el-tag>
        </div>
        <el-skeleton v-if="workspaceLoading" :rows="6" animated />
      </div>
      <el-empty
        v-else
        :description="isStudent ? '点击打开后创建独立 RStudio Pod' : '选择提交后在这里查看内容'"
        style="height: calc(100% - 52px)"
      />
    </section>
  </div>

  <el-dialog v-model="historyOpen" title="提交历史" width="520px">
    <el-table :data="historyRows" size="small" style="width: 100%">
      <el-table-column label="版本" width="100">
        <template #default="{ row }">第 {{ row.submission.attempt }} 次</template>
      </el-table-column>
      <el-table-column label="提交时间" min-width="170">
        <template #default="{ row }">{{ dayjs(row.submission.createdAt).format('MM-DD HH:mm') }}</template>
      </el-table-column>
      <el-table-column label="成绩" width="90">
        <template #default="{ row }">{{ row.grade.score ?? '未评分' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="90">
        <template #default="{ row }">
          <el-button link type="primary" @click="switchAttempt(row)">查看</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-dialog>

  <el-dialog v-model="gradeDialog" title="评分" width="440px">
    <el-form label-position="top">
      <el-form-item label="分数">
        <el-input-number v-model="gradeForm.score" :min="0" :max="100" />
      </el-form-item>
      <el-form-item label="评语">
        <el-input v-model="gradeForm.comment" type="textarea" :rows="5" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="gradeDialog = false">取消</el-button>
      <el-button type="primary" @click="saveGrade">发布成绩</el-button>
    </template>
  </el-dialog>
</template>
