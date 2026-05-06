<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'
import { useSession } from '../composables/useSession'
import type { ClassItem, User } from '../types'

interface MemberRow {
  user: User
  memberRole: 'student' | 'assistant'
  joinedAt: string
}

const route = useRoute()
const router = useRouter()
const session = useSession()
const classId = computed(() => String(route.params.classId))
const klass = ref<ClassItem | null>(null)
const members = ref<MemberRow[]>([])
const selectedRows = ref<MemberRow[]>([])
const importText = ref('username,displayName,password\nstudent001,学生一,student123')
const passwordOpen = ref(false)
const passwordTarget = ref<MemberRow | null>(null)
const passwordForm = reactive({ password: '' })
const canManageMembers = computed(() => ['root', 'admin', 'teacher'].includes(session.state.user?.role ?? ''))

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

async function removeMembers(ids: string[]) {
  if (!ids.length) return
  await ElMessageBox.confirm(`确定移除 ${ids.length} 个班级成员？账号本身会保留。`, '移除成员', { type: 'warning' })
  await api.post(`/api/classes/${classId.value}/members/bulk`, { action: 'remove', userIds: ids })
  ElMessage.success('成员已移除')
  selectedRows.value = []
  await load()
}

async function setMemberRole(ids: string[], memberRole: 'student' | 'assistant') {
  if (!ids.length) return
  await api.post(`/api/classes/${classId.value}/members/bulk`, { action: 'setMemberRole', userIds: ids, memberRole })
  ElMessage.success(memberRole === 'assistant' ? '已设为助教' : '已设为学生')
  selectedRows.value = []
  await load()
}

function openPassword(row: MemberRow) {
  passwordTarget.value = row
  passwordForm.password = ''
  passwordOpen.value = true
}

async function savePassword() {
  if (!passwordTarget.value) return
  await api.post(`/api/classes/${classId.value}/members/${passwordTarget.value.user.id}/password`, { password: passwordForm.password })
  ElMessage.success('密码已重置')
  passwordOpen.value = false
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
      <div class="toolbar-actions">
        <el-button v-if="canManageMembers && selectedRows.length" plain @click="setMemberRole(selectedRows.map((row) => row.user.id), 'assistant')">设为助教</el-button>
        <el-button v-if="canManageMembers && selectedRows.length" plain @click="setMemberRole(selectedRows.map((row) => row.user.id), 'student')">设为学生</el-button>
        <el-button v-if="canManageMembers && selectedRows.length" type="danger" plain @click="removeMembers(selectedRows.map((row) => row.user.id))">移除选中</el-button>
        <el-button @click="router.push('/classes')">返回班级</el-button>
      </div>
    </div>

    <div class="member-layout">
      <div v-if="canManageMembers" class="member-import">
        <h3>导入学生</h3>
        <el-input v-model="importText" type="textarea" :rows="8" />
        <el-button type="primary" @click="importMembers">导入</el-button>
      </div>

      <el-table :data="members" style="width: 100%" @selection-change="selectedRows = $event">
        <el-table-column v-if="canManageMembers" type="selection" width="44" />
        <el-table-column prop="user.username" label="账号" width="160" />
        <el-table-column prop="user.displayName" label="姓名" width="160" />
        <el-table-column prop="memberRole" label="班级角色" width="130" />
        <el-table-column prop="user.status" label="状态" width="110" />
        <el-table-column prop="joinedAt" label="加入时间" min-width="180" />
        <el-table-column v-if="canManageMembers" label="操作" width="260">
          <template #default="{ row }">
            <el-button link type="primary" @click="setMemberRole([row.user.id], row.memberRole === 'assistant' ? 'student' : 'assistant')">
              {{ row.memberRole === 'assistant' ? '设为学生' : '设为助教' }}
            </el-button>
            <el-button v-if="row.memberRole === 'student'" link type="primary" @click="openPassword(row)">重置密码</el-button>
            <el-button link type="danger" @click="removeMembers([row.user.id])">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </section>

  <el-dialog v-model="passwordOpen" title="重置学生密码" width="420px">
    <el-form label-position="top">
      <el-form-item label="学生">
        <el-input :model-value="passwordTarget?.user.displayName" disabled />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="passwordForm.password" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="passwordOpen = false">取消</el-button>
      <el-button type="primary" @click="savePassword">保存</el-button>
    </template>
  </el-dialog>
</template>
