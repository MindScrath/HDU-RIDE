'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'
import { useSession } from '@/stores/session'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { toast } from 'sonner'
import { Plus, Pencil, Users, Upload } from 'lucide-react'
import type { Course } from '@/lib/types'

export default function AdminCoursesPage() {
  const user = useSession((s) => s.user)
  const router = useRouter()
  const confirm = useConfirm()
  const isRoot = user?.role === 'root'

  const [courses, setCourses] = useState<Course[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Course | null>(null)
  const [form, setForm] = useState({ name: '', code: '', description: '' })
  const [importOpen, setImportOpen] = useState(false)
  const [importCourseId, setImportCourseId] = useState('')
  const [importFile, setImportFile] = useState<File | null>(null)

  async function load() {
    try {
      const data = await api.get<{ courses: Course[] }>('/api/admin/courses')
      setCourses(data.courses)
    } catch {
      toast.error('加载课程列表失败')
    }
  }

  useEffect(() => { load() }, [])

  async function handleCreate() {
    try {
      await api.post('/api/admin/courses', form)
      toast.success('课程已创建')
      setCreateOpen(false)
      setForm({ name: '', code: '', description: '' })
      await load()
    } catch (err: any) {
      toast.error(err.message ?? '创建失败')
    }
  }

  function openEdit(c: Course) {
    setEditTarget(c)
    setEditOpen(true)
  }

  async function handleEdit() {
    if (!editTarget) return
    try {
      await api.patch(`/api/admin/courses/${editTarget.id}`, {
        name: editTarget.name,
        description: editTarget.description,
        status: editTarget.status,
      })
      toast.success('课程已更新')
      setEditOpen(false)
      await load()
    } catch (err: any) {
      toast.error(err.message ?? '更新失败')
    }
  }

  async function handleArchive(c: Course) {
    confirm({ title: "归档课程", message: `确定归档课程「${c.name}」？归档后仍可恢复。`, onConfirm: async () => { await api.patch(`/api/admin/courses/${c.id}`, { status: "archived" }); toast.success("课程已归档"); await load(); } })

      await api.patch(`/api/admin/courses/${c.id}`, { status: 'archived' })
      toast.success('课程已归档')
      await load()
    } catch (err: any) {
      toast.error(err.message ?? '归档失败')
    }
  }

  async function handleImport() {
    if (!importFile || !importCourseId) {
      toast.error('请选择课程包并填写课程 ID')
      return
    }
    const payload = new FormData()
    payload.append('courseId', importCourseId)
    payload.append('file', importFile)
    try {
      await api.post('/api/admin/courses/import', payload)
      toast.success('课程已导入')
      setImportOpen(false)
      setImportFile(null)
    } catch (err: any) {
      toast.error(err.message ?? '导入失败')
    }
  }

  async function handleReload() {
    try {
      await api.post('/api/admin/courses/reload')
      toast.success('课程已重新加载')
    } catch (err: any) {
      toast.error(err.message ?? '重新加载失败')
    }
  }

  return (
    <>
      <section className="panel single-panel">
        <div className="panel-head">
          <div>
            <h2>课程管理</h2>
            <span className="muted">管理课程、成员和内容</span>
          </div>
          <div className="toolbar-actions">
            <Button variant="outline" onClick={() => setImportOpen(true)}>
              <Upload className="size-4 mr-1" />
              导入课程包
            </Button>
            <Button variant="outline" onClick={handleReload}>
              重新加载
            </Button>
            {isRoot && (
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="size-4 mr-1" />
                新建课程
              </Button>
            )}
          </div>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>课程名称</TableHead>
              <TableHead className="w-[120px]">代码</TableHead>
              <TableHead className="w-[90px]">状态</TableHead>
              <TableHead className="w-[80px]">成员</TableHead>
              <TableHead className="w-[80px]">班级</TableHead>
              <TableHead className="w-[200px]">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {courses.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-[#94a3b8] py-8">
                  暂无课程，点击"新建课程"或"导入课程包"开始
                </TableCell>
              </TableRow>
            )}
            {courses.map((c) => (
              <TableRow key={c.id}>
                <TableCell className="font-medium">{c.name}</TableCell>
                <TableCell className="font-mono text-sm">{c.code}</TableCell>
                <TableCell>
                  <Badge variant={c.status === 'active' ? 'default' : 'secondary'}>
                    {c.status === 'active' ? '启用' : '已归档'}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Button variant="link" size="sm" className="px-0"
                    onClick={() => router.push(`/admin/courses/${c.id}/members`)}>
                    <Users className="size-3.5 mr-1" />
                    {c.memberCount}
                  </Button>
                </TableCell>
                <TableCell>{c.classCount}</TableCell>
                <TableCell>
                  <div className="flex gap-1">
                    <Button variant="outline" size="sm"
                      onClick={() => {
                        setEditTarget({ ...c })
                        setEditOpen(true)
                      }}>
                      <Pencil className="size-3.5 mr-1" />
                      编辑
                    </Button>
                    <Button variant="outline" size="sm"
                      onClick={() => router.push(`/admin/courses/${c.id}/lectures`)}>
                      讲义
                    </Button>
                    <Button variant="outline" size="sm"
                      onClick={() => router.push(`/admin/courses/${c.id}/members`)}>
                      成员
                    </Button>
                    {isRoot && c.status === 'active' && (
                      <Button variant="destructive" size="sm" onClick={() => handleArchive(c)}>
                        归档
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      {/* Create Dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader><DialogTitle>新建课程</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>课程代码</Label><Input value={form.code} onChange={e => setForm({ ...form, code: e.target.value })} placeholder="econ-301" /></div>
            <div><Label>课程名称</Label><Input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="计量经济学" /></div>
            <div><Label>描述</Label><Input value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} placeholder="本科计量经济学课程" /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button onClick={handleCreate} disabled={!form.code || !form.name}>创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader><DialogTitle>编辑课程</DialogTitle></DialogHeader>
          {editTarget && (
            <div className="grid gap-3">
              <div><Label>课程名称</Label><Input value={editTarget.name} onChange={e => setEditTarget({ ...editTarget, name: e.target.value })} /></div>
              <div><Label>描述</Label><Input value={editTarget.description} onChange={e => setEditTarget({ ...editTarget, description: e.target.value })} /></div>
              <div>
                <Label>状态</Label>
                <select className="flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm"
                  value={editTarget.status}
                  onChange={e => setEditTarget({ ...editTarget, status: e.target.value as 'active' | 'archived' })}>
                  <option value="active">启用</option>
                  <option value="archived">已归档</option>
                </select>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>取消</Button>
            <Button onClick={handleEdit}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Import Dialog */}
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader><DialogTitle>导入课程包</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>课程 ID</Label><Input value={importCourseId} onChange={e => setImportCourseId(e.target.value)} placeholder="intro-r" /></div>
            <div><Label>课程包 zip</Label><Input type="file" accept=".zip" onChange={e => setImportFile(e.target.files?.[0] ?? null)} /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>取消</Button>
            <Button onClick={handleImport} disabled={!importFile || !importCourseId}>导入</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
