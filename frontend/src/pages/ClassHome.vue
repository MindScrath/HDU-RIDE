<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { useSession } from '../composables/useSession'
import type { ClassItem } from '../types'

const router = useRouter()
const session = useSession()
const classes = ref<ClassItem[]>([])
const createOpen = ref(false)
const form = reactive({ courseId: 'intro-r', name: '', term: '2026 春', note: '' })

async function load() {
  try {
    classes.value = (await api.get<{ classes: ClassItem[] }>('/api/classes')).classes
  } catch (err) {
    if ((err as { status?: number }).status === 401) router.push('/login')
  }
}

async function createClass() {
  await api.post('/api/classes', form)
  ElMessage.success('班级已创建')
  createOpen.value = false
  await load()
}

onMounted(load)
</script>

<template>
  <section class="panel single-panel">
    <div class="panel-head">
      <div>
        <h2>班级</h2>
        <span class="muted">班级成员从这里进入，讲义和作业也可从左侧直接打开</span>
      </div>
      <el-button v-if="session.canTeach.value" type="primary" @click="createOpen = true">新建班级</el-button>
    </div>
    <el-table :data="classes" style="width: 100%">
      <el-table-column prop="name" label="班级" min-width="180" />
      <el-table-column prop="courseId" label="课程 ID" width="150" />
      <el-table-column prop="term" label="学期" width="130" />
      <el-table-column prop="note" label="备注" min-width="180" />
      <el-table-column label="操作" width="320">
        <template #default="{ row }">
          <el-button @click="router.push(`/classes/${row.id}/lectures`)">讲义</el-button>
          <el-button type="primary" @click="router.push(`/classes/${row.id}/assignments`)">作业</el-button>
          <el-button v-if="session.canTeach.value" @click="router.push(`/classes/${row.id}/members`)">成员</el-button>
        </template>
      </el-table-column>
    </el-table>
  </section>

  <el-dialog v-model="createOpen" title="新建班级" width="420px">
    <el-form label-position="top">
      <el-form-item label="课程 ID"><el-input v-model="form.courseId" /></el-form-item>
      <el-form-item label="班级名称"><el-input v-model="form.name" /></el-form-item>
      <el-form-item label="学期"><el-input v-model="form.term" /></el-form-item>
      <el-form-item label="备注"><el-input v-model="form.note" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="createOpen = false">取消</el-button>
      <el-button type="primary" @click="createClass">创建</el-button>
    </template>
  </el-dialog>
</template>
