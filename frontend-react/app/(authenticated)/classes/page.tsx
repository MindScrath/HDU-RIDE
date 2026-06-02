'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { api, ApiError } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import type { ClassItem } from '@/lib/types'

export default function ClassesPage() {
  const user = useSession((s) => s.user)
  const isAdmin = useSession((s) => s.isAdmin)
  const canTeach = useSession((s) => s.canTeach)
  const router = useRouter()
  const [classes, setClasses] = useState<ClassItem[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [createOpen, setCreateOpen] = useState(false)
  const [form, setForm] = useState({ courseId: 'intro-r', name: '', term: '2026 春', note: '', teacherIds: '' })

  const canManage = ['root', 'admin', 'teacher'].includes(user?.role ?? '')

  async function load() {
    try {
      const data = await api.get<{ classes: ClassItem[] }>('/api/classes')
      setClasses(data.classes)
    } catch (err) {
      if ((err as ApiError).status === 401) router.push('/login')
    }
  }

  useEffect(() => { load() }, [])

  async function handleCreate() {
    try {
      const teacherIds = form.teacherIds
        .split(',')
        .map(s => s.trim())
        .filter(Boolean)
      await api.post('/api/classes', { ...form, teacherIds })
      toast.success('班级已创建')
      setCreateOpen(false)
      setForm({ courseId: 'intro-r', name: '', term: '2026 春', note: '', teacherIds: '' })
      await load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '创建班级失败')
    }
  }

  async function handleDelete(ids: string[]) {
    if (!ids.length) return
    if (!confirm(`确定删除 ${ids.length} 个班级？关联成员、提交和成绩会一并删除。`)) return
    await api.post('/api/classes/bulk', { action: 'delete', ids })
    toast.success('班级已删除')
    setSelectedIds(new Set())
    await load()
  }

  function toggleSelect(id: string) {
    const next = new Set(selectedIds)
    next.has(id) ? next.delete(id) : next.add(id)
    setSelectedIds(next)
  }

  return (
    <>
      <section className="panel single-panel">
        <div className="panel-head">
          <div>
            <h2>班级</h2>
            <span className="muted">班级成员从这里进入，讲义和作业也可从左侧直接打开</span>
          </div>
          <div className="toolbar-actions">
            {canManage && selectedIds.size > 0 && (
              <Button variant="destructive" onClick={() => handleDelete(Array.from(selectedIds))}>
                删除选中
              </Button>
            )}
            {canManage && (
              <Button onClick={() => setCreateOpen(true)}>新建班级</Button>
            )}
          </div>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              {canManage && <TableHead className="w-[44px]" />}
              <TableHead>班级</TableHead>
              <TableHead className="w-[150px]">课程 ID</TableHead>
              <TableHead className="w-[130px]">学期</TableHead>
              <TableHead>备注</TableHead>
              <TableHead className="w-[320px]">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {classes.map((klass) => (
              <TableRow key={klass.id}>
                {canManage && (
                  <TableCell>
                    <Checkbox
                      checked={selectedIds.has(klass.id)}
                      onCheckedChange={() => toggleSelect(klass.id)}
                    />
                  </TableCell>
                )}
                <TableCell>{klass.name}</TableCell>
                <TableCell>{klass.courseId}</TableCell>
                <TableCell>{klass.term}</TableCell>
                <TableCell>{klass.note}</TableCell>
                <TableCell>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => router.push(`/classes/${klass.id}/lectures`)}>讲义</Button>
                    <Button size="sm" onClick={() => router.push(`/classes/${klass.id}/assignments`)}>作业</Button>
                    {(canManage || canTeach()) && (
                      <Button variant="outline" size="sm" onClick={() => router.push(`/classes/${klass.id}/members`)}>成员</Button>
                    )}
                    {canManage && (
                      <Button variant="destructive" size="sm" onClick={() => handleDelete([klass.id])}>删除</Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>新建班级</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>课程 ID</Label><Input value={form.courseId} onChange={(e) => setForm({ ...form, courseId: e.target.value })} /></div>
            <div><Label>班级名称</Label><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
            <div><Label>学期</Label><Input value={form.term} onChange={(e) => setForm({ ...form, term: e.target.value })} /></div>
            <div><Label>备注</Label><Input value={form.note} onChange={(e) => setForm({ ...form, note: e.target.value })} /></div>
            <div><Label>教师 ID（逗号分隔）</Label><Input value={form.teacherIds} onChange={(e) => setForm({ ...form, teacherIds: e.target.value })} placeholder="user1,user2" /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button onClick={handleCreate}>创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
