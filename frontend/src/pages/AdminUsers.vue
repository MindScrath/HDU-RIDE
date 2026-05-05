<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import type { Role, User } from '../types'

const users = ref<User[]>([])
const createOpen = ref(false)
const form = reactive({ username: '', displayName: '', password: '', role: 'student' as Role })

async function load() {
  users.value = (await api.get<{ users: User[] }>('/api/admin/users')).users
}

async function createUser() {
  await api.post('/api/admin/users', form)
  ElMessage.success('用户已创建')
  createOpen.value = false
  await load()
}

onMounted(load)
</script>

<template>
  <section class="panel single-panel">
    <div class="panel-head">
      <h2>用户管理</h2>
      <el-button type="primary" @click="createOpen = true">新建用户</el-button>
    </div>
    <el-table :data="users" style="width: 100%">
      <el-table-column prop="username" label="账号" width="180" />
      <el-table-column prop="displayName" label="姓名" width="180" />
      <el-table-column prop="role" label="角色" width="130" />
      <el-table-column prop="status" label="状态" width="120" />
      <el-table-column prop="createdAt" label="创建时间" min-width="180" />
    </el-table>
  </section>

  <el-dialog v-model="createOpen" title="新建用户" width="420px">
    <el-form label-position="top">
      <el-form-item label="账号"><el-input v-model="form.username" /></el-form-item>
      <el-form-item label="姓名"><el-input v-model="form.displayName" /></el-form-item>
      <el-form-item label="密码"><el-input v-model="form.password" type="password" /></el-form-item>
      <el-form-item label="角色">
        <el-select v-model="form.role" style="width: 100%">
          <el-option label="Admin" value="admin" />
          <el-option label="Teacher" value="teacher" />
          <el-option label="Assistant" value="assistant" />
          <el-option label="Student" value="student" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="createOpen = false">取消</el-button>
      <el-button type="primary" @click="createUser">创建</el-button>
    </template>
  </el-dialog>
</template>
