<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Document, Expand, Fold, Grid, Notebook, Setting, User } from '@element-plus/icons-vue'
import { useSession } from './composables/useSession'

const route = useRoute()
const router = useRouter()
const session = useSession()

const isLogin = computed(() => route.path === '/login')
const classId = computed(() => String(route.params.classId ?? ''))
const sidebarCollapsed = ref(window.innerWidth < 980)
const nav = computed(() => [
  { key: 'classes', label: '班级', path: '/classes', icon: Grid },
  { key: 'lectures', label: '讲义', path: '/lectures', icon: Notebook },
  { key: 'assignments', label: '作业', path: '/assignments', icon: Document },
  { key: 'admin', label: '管理', path: '/admin/users', icon: Setting, adminOnly: true }
])
const activeNav = computed(() => {
  if (route.path.startsWith('/admin')) return 'admin'
  if (route.path.includes('/lectures')) return 'lectures'
  if (route.path.includes('/assignments')) return 'assignments'
  return 'classes'
})

async function logout() {
  await session.logout()
  router.push('/login')
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
      <el-select model-value="计量金融 2026 春" class="course-select" :show-arrow="true">
        <el-option label="计量金融 2026 春" value="计量金融 2026 春" />
      </el-select>
      <div class="topbar-actions">
        <el-dropdown v-if="session.state.user">
          <button class="user-button">
            <el-icon><User /></el-icon>
            {{ session.state.user.displayName }}
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item>{{ session.state.user.role }}</el-dropdown-item>
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
</template>
