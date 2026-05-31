'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { api } from '@/lib/api'
import { toast } from 'sonner'

export default function AdminCoursesPage() {
  const [courseId, setCourseId] = useState('intro-r')
  const [file, setFile] = useState<File | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleImport() {
    if (!file) {
      toast.error('请选择课程包')
      return
    }
    setLoading(true)
    try {
      const payload = new FormData()
      payload.append('courseId', courseId)
      payload.append('file', file)
      await api.post('/api/admin/courses/import', payload)
      toast.success('课程已导入')
    } finally {
      setLoading(false)
    }
  }

  async function handleReload() {
    await api.post('/api/admin/courses/reload')
    toast.success('课程已重新加载')
  }

  return (
    <section className="panel single-panel !max-w-[720px]">
      <div className="panel-head">
        <h2>课程内容</h2>
        <span className="muted">上传 course.yml + chapters + assignments 课程包</span>
      </div>
      <div className="p-5">
        <div className="grid gap-3">
          <div>
            <Label>课程 ID</Label>
            <Input value={courseId} onChange={(e) => setCourseId(e.target.value)} />
          </div>
          <div>
            <Label>课程包 zip</Label>
            <Input type="file" accept=".zip" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
          </div>
          <div className="flex gap-2">
            <Button onClick={handleImport} disabled={loading}>导入课程</Button>
            <Button variant="outline" onClick={handleReload}>重新加载</Button>
          </div>
        </div>
      </div>
    </section>
  )
}
