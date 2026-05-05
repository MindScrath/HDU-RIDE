<script setup lang="ts">
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useSession } from '../composables/useSession'

const router = useRouter()
const session = useSession()
const form = reactive({ username: '', password: '' })

async function submit() {
  try {
    await session.login(form.username, form.password)
    router.push('/classes')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '登录失败')
  }
}
</script>

<template>
  <div class="login-page">
    <section class="panel login-box">
      <div class="brand" style="color: #172033; margin-bottom: 20px">
        <span class="brand-mark">R</span>
        <div>
          <strong>HDU RIDE</strong>
          <small>计量金融 R 教学平台</small>
        </div>
      </div>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="form.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" autocomplete="current-password" show-password />
        </el-form-item>
        <el-button type="primary" native-type="submit" style="width: 100%">登录</el-button>
      </el-form>
    </section>
  </div>
</template>
