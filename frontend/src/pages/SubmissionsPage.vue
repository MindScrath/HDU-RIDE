<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import type { Submission } from '../types'

const route = useRoute()
const classId = computed(() => String(route.params.classId))
const assignmentId = computed(() => String(route.params.assignmentId))
const rows = ref<Array<{ submission: Submission; studentName: string; grade: { id: string; score: number | null; comment: string } }>>([])
const gradeDialog = ref(false)
const current = ref<Submission | null>(null)
const gradeForm = reactive({ score: 0, comment: '' })

async function load() {
  if (assignmentId.value === '_') return
  rows.value = (
    await api.get<{ submissions: typeof rows.value }>(
      `/api/classes/${classId.value}/assignments/${assignmentId.value}/submissions`
    )
  ).submissions
}

function openGrade(row: (typeof rows.value)[number]) {
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
  await load()
}

onMounted(load)
</script>

<template>
  <section class="panel">
    <div class="panel-head">
      <h2>批改</h2>
      <span class="muted">按作业查看学生提交、评分和发布评语</span>
    </div>
    <el-empty v-if="assignmentId === '_'" description="请先从作业页进入批改" />
    <el-table v-else :data="rows" style="width: 100%">
      <el-table-column prop="studentName" label="学生" width="150" />
      <el-table-column label="提交" min-width="260">
        <template #default="{ row }">
          第 {{ row.submission.attempt }} 次
          <el-tag v-if="row.submission.late" type="warning">补交</el-tag>
          <span class="muted">{{ row.submission.createdAt }}</span>
        </template>
      </el-table-column>
      <el-table-column label="对象存储" min-width="260">
        <template #default="{ row }">
          <div v-if="row.submission.textObject">{{ row.submission.textObject }}</div>
          <div v-if="row.submission.fileObject">{{ row.submission.fileObject }}</div>
        </template>
      </el-table-column>
      <el-table-column label="成绩" width="120">
        <template #default="{ row }">{{ row.grade.score ?? '未评分' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="130">
        <template #default="{ row }">
          <el-button type="primary" @click="openGrade(row)">评分</el-button>
        </template>
      </el-table-column>
    </el-table>
  </section>

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
