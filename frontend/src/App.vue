<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Document, Expand, Fold, Grid, Notebook, Setting, User, ChatDotRound } from '@element-plus/icons-vue'
import { useSession } from './composables/useSession'

const route = useRoute()
const router = useRouter()
const session = useSession()

const isLogin = computed(() => route.path === '/login')
const sidebarCollapsed = ref(window.innerWidth < 980)
const passwordOpen = ref(false)
const passwordSaving = ref(false)
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })
const nav = computed(() => [
  { key: 'classes', label: '班级', path: '/classes', icon: Grid },
  { key: 'lectures', label: '讲义', path: '/lectures', icon: Notebook },
  { key: 'assignments', label: '作业', path: '/assignments', icon: Document },
  { key: 'agui', label: 'AI 助手', path: '/agui', icon: ChatDotRound },
  { key: 'admin', label: '管理', path: '/admin/users', icon: Setting, adminOnly: true }
])
const activeNav = computed(() => {
  if (route.path.startsWith('/admin')) return 'admin'
  if (route.path.includes('/lectures')) return 'lectures'
  if (route.path.includes('/assignments')) return 'assignments'
  if (route.path.startsWith('/agui')) return 'agui'
  return 'classes'
})

async function logout() {
  await session.logout()
  router.push('/login')
}

async function savePassword() {
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    ElMessage.error('两次输入的新密码不一致')
    return
  }
  passwordSaving.value = true
  try {
    await session.changePassword(passwordForm.oldPassword, passwordForm.newPassword)
    ElMessage.success('密码已修改')
    passwordOpen.value = false
    passwordForm.oldPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
  } finally {
    passwordSaving.value = false
  }
}
</script>

<template>
  <RouterView v-if="isLogin" />
  <div v-else class="app-shell" :class="{ 'is-collapsed': sidebarCollapsed }">
    <header class="global-topbar">
      <div class="topbar-brand">
        <span class="brand-mark">R</span>
        <div class="brand-copy">
          <strong>HDU RIDE</strong>
        </div>
      </div>
      <div class="topbar-actions">
        <el-dropdown v-if="session.state.user">
          <button class="user-button">
            <el-icon><User /></el-icon>
            {{ session.state.user.displayName }}
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item>{{ session.state.user.role }}</el-dropdown-item>
              <el-dropdown-item divided @click="passwordOpen = true">修改密码</el-dropdown-item>
              <el-dropdown-item divided @click="logout">退出</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <div class="app-body">
      <aside class="sidebar">
      <nav>
        <RouterLink
          v-for="item in nav"
          v-show="(!item.adminOnly || session.isAdmin.value) && (!item.teachOnly || session.canTeach.value)"
          :key="item.label"
          :to="item.path"
          custom
          v-slot="{ href, navigate }"
        >
          <a
            :href="href"
          class="nav-item"
            :class="{ active: activeNav === item.key }"
            @click="navigate"
          >
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
          </a>
        </RouterLink>
      </nav>
      <button class="collapse-button" :title="sidebarCollapsed ? '展开侧栏' : '收起侧栏'" @click="sidebarCollapsed = !sidebarCollapsed">
        <el-icon><component :is="sidebarCollapsed ? Expand : Fold" /></el-icon>
        <span>{{ sidebarCollapsed ? '展开' : '收起' }}</span>
      </button>
    </aside>

    <main class="workspace">
      <RouterView />
    </main>
    </div>
  </div>

  <el-dialog v-model="passwordOpen" title="修改密码" width="420px">
    <el-form label-position="top">
      <el-form-item label="当前密码">
        <el-input v-model="passwordForm.oldPassword" type="password" autocomplete="current-password" show-password />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="passwordForm.newPassword" type="password" autocomplete="new-password" show-password />
      </el-form-item>
      <el-form-item label="确认新密码">
        <el-input v-model="passwordForm.confirmPassword" type="password" autocomplete="new-password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="passwordOpen = false">取消</el-button>
      <el-button type="primary" :loading="passwordSaving" @click="savePassword">保存</el-button>
    </template>
  </el-dialog>
</template>
