<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const form = reactive({ courseId: 'intro-r', file: null as File | null })
const loading = ref(false)

function onFileChange(file: { raw?: File }) {
  form.file = file.raw ?? null
}

async function importCourse() {
  if (!form.file) {
    ElMessage.error('请选择课程包')
    return
  }
  loading.value = true
  try {
    const payload = new FormData()
    payload.append('courseId', form.courseId)
    payload.append('file', form.file)
    await api.post('/api/admin/courses/import', payload)
    ElMessage.success('课程已导入')
  } finally {
    loading.value = false
  }
}

async function reloadCourses() {
  await api.post('/api/admin/courses/reload')
  ElMessage.success('课程已重新加载')
}
</script>

<template>
  <section class="panel single-panel" style="max-width: 720px">
    <div class="panel-head">
      <h2>课程内容</h2>
      <span class="muted">上传 course.yml + chapters + assignments 课程包</span>
    </div>
    <div style="padding: 18px 20px 22px">
      <el-form label-position="top">
        <el-form-item label="课程 ID">
          <el-input v-model="form.courseId" />
        </el-form-item>
        <el-form-item label="课程包 zip">
          <el-upload :auto-upload="false" :limit="1" :on-change="onFileChange">
            <el-button>选择 zip</el-button>
          </el-upload>
        </el-form-item>
        <el-button type="primary" :loading="loading" @click="importCourse">导入课程</el-button>
        <el-button @click="reloadCourses">重新加载</el-button>
      </el-form>
    </div>
  </section>
</template>
