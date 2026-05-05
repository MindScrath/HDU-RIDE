<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import type { ClassItem, User } from '../types'

interface MemberRow {
  user: User
  memberRole: 'student' | 'assistant'
  joinedAt: string
}

const route = useRoute()
const router = useRouter()
const classId = computed(() => String(route.params.classId))
const klass = ref<ClassItem | null>(null)
const members = ref<MemberRow[]>([])
const importText = ref('username,displayName,password\nstudent001,学生一,student123')

async function load() {
  klass.value = (await api.get<{ class: ClassItem }>(`/api/classes/${classId.value}`)).class
  members.value = (await api.get<{ members: MemberRow[] }>(`/api/classes/${classId.value}/members`)).members
}

async function importMembers() {
  const students = importText.value
    .split(/\r?\n/)
    .slice(1)
    .map((line) => line.split(',').map((item) => item.trim()))
    .filter((row) => row[0] && row[1] && row[2])
    .map(([username, displayName, password]) => ({ username, displayName, password }))
  await api.post(`/api/classes/${classId.value}/members/import`, { students })
  ElMessage.success('成员已导入')
  await load()
}

onMounted(load)
</script>

<template>
  <section class="panel single-panel">
    <div class="panel-head">
      <div>
        <h2>{{ klass?.name ?? '成员' }}</h2>
        <span class="muted">学生与助教绑定在当前班级</span>
      </div>
      <el-button @click="router.push('/classes')">返回班级</el-button>
    </div>

    <div class="member-layout">
      <div class="member-import">
        <h3>导入学生</h3>
        <el-input v-model="importText" type="textarea" :rows="8" />
        <el-button type="primary" @click="importMembers">导入</el-button>
      </div>

      <el-table :data="members" style="width: 100%">
        <el-table-column prop="user.username" label="账号" width="160" />
        <el-table-column prop="user.displayName" label="姓名" width="160" />
        <el-table-column prop="memberRole" label="班级角色" width="130" />
        <el-table-column prop="user.status" label="状态" width="110" />
        <el-table-column prop="joinedAt" label="加入时间" min-width="180" />
      </el-table>
    </div>
  </section>
</template>
