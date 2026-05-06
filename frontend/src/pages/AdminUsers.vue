<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'
import { useSession } from '../composables/useSession'
import type { Role, User } from '../types'

const session = useSession()
const users = ref<User[]>([])
const selectedUsers = ref<User[]>([])
const createOpen = ref(false)
const editOpen = ref(false)
const passwordOpen = ref(false)
const createForm = reactive({ username: '', displayName: '', password: '', role: 'student' as Role })
const editForm = reactive({ id: '', displayName: '', role: 'student' as Role, status: 'active' })
const passwordForm = reactive({ id: '', displayName: '', password: '' })
const roleOptions = computed(() => {
  const roles: Array<{ label: string; value: Role }> = [
    { label: 'Root', value: 'root' },
    { label: 'Admin', value: 'admin' },
    { label: 'Teacher', value: 'teacher' },
    { label: 'Assistant', value: 'assistant' },
    { label: 'Student', value: 'student' }
  ]
  return session.state.user?.role === 'root' ? roles : roles.filter((item) => item.value !== 'root')
})
const manageableSelection = computed(() => selectedUsers.value.filter((user) => canManage(user)))

function canManage(user: User) {
  if (session.state.user?.role === 'root') return true
  return session.state.user?.role === 'admin' && user.role !== 'root'
}

function canMutateIdentity(user: User) {
  return canManage(user) && user.id !== session.state.user?.id
}

function onSelectionChange(rows: User[]) {
  selectedUsers.value = rows
}

async function load() {
  users.value = (await api.get<{ users: User[] }>('/api/admin/users')).users
}

async function createUser() {
  await api.post('/api/admin/users', createForm)
  ElMessage.success('用户已创建')
  createOpen.value = false
  Object.assign(createForm, { username: '', displayName: '', password: '', role: 'student' as Role })
  await load()
}

function openEdit(user: User) {
  Object.assign(editForm, {
    id: user.id,
    displayName: user.displayName,
    role: user.role,
    status: user.status
  })
  editOpen.value = true
}

async function saveEdit() {
  await api.patch(`/api/admin/users/${editForm.id}`, {
    displayName: editForm.displayName,
    role: editForm.role,
    status: editForm.status
  })
  ElMessage.success('用户已更新')
  editOpen.value = false
  await load()
}

function openPassword(user: User) {
  Object.assign(passwordForm, { id: user.id, displayName: user.displayName, password: '' })
  passwordOpen.value = true
}

async function savePassword() {
  await api.post(`/api/admin/users/${passwordForm.id}/password`, { password: passwordForm.password })
  ElMessage.success('密码已重置')
  passwordOpen.value = false
}

async function disableUser(user: User) {
  await ElMessageBox.confirm(`确定禁用账号 ${user.username}？`, '禁用账号', { type: 'warning' })
  await api.delete(`/api/admin/users/${user.id}`)
  ElMessage.success('账号已禁用')
  await load()
}

async function bulk(action: 'disable' | 'activate' | 'setRole', role?: Role) {
  const rows = manageableSelection.value.filter((user) => action === 'activate' || canMutateIdentity(user))
  if (!rows.length) return
  if (action === 'disable') {
    await ElMessageBox.confirm(`确定禁用 ${rows.length} 个账号？`, '批量禁用', { type: 'warning' })
  }
  await api.post('/api/admin/users/bulk', { action, ids: rows.map((user) => user.id), role })
  ElMessage.success('批量操作已完成')
  selectedUsers.value = []
  await load()
}

onMounted(load)
</script>

<template>
  <section class="panel single-panel">
    <div class="panel-head">
      <h2>用户管理</h2>
      <div class="toolbar-actions">
        <el-button v-if="manageableSelection.length" plain @click="bulk('activate')">启用选中</el-button>
        <el-button v-if="manageableSelection.length" type="danger" plain @click="bulk('disable')">禁用选中</el-button>
        <el-dropdown v-if="manageableSelection.length">
          <el-button plain>批量角色</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="role in roleOptions" :key="role.value" @click="bulk('setRole', role.value)">
                {{ role.label }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button type="primary" @click="createOpen = true">新建用户</el-button>
      </div>
    </div>
    <el-table :data="users" style="width: 100%" @selection-change="onSelectionChange">
      <el-table-column type="selection" width="44" :selectable="canManage" />
      <el-table-column prop="username" label="账号" width="180" />
      <el-table-column prop="displayName" label="姓名" width="180" />
      <el-table-column prop="role" label="角色" width="130" />
      <el-table-column prop="status" label="状态" width="120" />
      <el-table-column prop="createdAt" label="创建时间" min-width="180" />
      <el-table-column label="操作" width="260">
        <template #default="{ row }">
          <el-button link type="primary" :disabled="!canManage(row)" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" :disabled="!canManage(row)" @click="openPassword(row)">重置密码</el-button>
          <el-button link type="danger" :disabled="!canMutateIdentity(row)" @click="disableUser(row)">禁用</el-button>
        </template>
      </el-table-column>
    </el-table>
  </section>

  <el-dialog v-model="createOpen" title="新建用户" width="420px">
    <el-form label-position="top">
      <el-form-item label="账号"><el-input v-model="createForm.username" /></el-form-item>
      <el-form-item label="姓名"><el-input v-model="createForm.displayName" /></el-form-item>
      <el-form-item label="密码"><el-input v-model="createForm.password" type="password" show-password /></el-form-item>
      <el-form-item label="角色">
        <el-select v-model="createForm.role" style="width: 100%">
          <el-option v-for="role in roleOptions" :key="role.value" :label="role.label" :value="role.value" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="createOpen = false">取消</el-button>
      <el-button type="primary" @click="createUser">创建</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="editOpen" title="编辑用户" width="420px">
    <el-form label-position="top">
      <el-form-item label="姓名"><el-input v-model="editForm.displayName" /></el-form-item>
      <el-form-item label="角色">
        <el-select v-model="editForm.role" style="width: 100%">
          <el-option v-for="role in roleOptions" :key="role.value" :label="role.label" :value="role.value" />
        </el-select>
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="editForm.status" style="width: 100%">
          <el-option label="Active" value="active" />
          <el-option label="Disabled" value="disabled" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="editOpen = false">取消</el-button>
      <el-button type="primary" @click="saveEdit">保存</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="passwordOpen" title="重置密码" width="420px">
    <el-form label-position="top">
      <el-form-item label="用户"><el-input :model-value="passwordForm.displayName" disabled /></el-form-item>
      <el-form-item label="新密码"><el-input v-model="passwordForm.password" type="password" show-password /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="passwordOpen = false">取消</el-button>
      <el-button type="primary" @click="savePassword">保存</el-button>
    </template>
  </el-dialog>
</template>
