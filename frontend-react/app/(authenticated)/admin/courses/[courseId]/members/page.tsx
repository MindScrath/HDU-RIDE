'use client'

import { useEffect, useState, use } from 'react'
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
import { useConfirm } from '@/components/ui/confirm-dialog'
import { toast } from 'sonner'
import { ArrowLeft, Plus, Trash2 } from 'lucide-react'
import type { CourseMember } from '@/lib/types'

export default function CourseMembersPage({ params }: { params: Promise<{ courseId: string }> }) {
  const { courseId } = use(params)
  const confirm = useConfirm()
  const router = useRouter()
  const [members, setMembers] = useState<CourseMember[]>([])
  const [addOpen, setAddOpen] = useState(false)
  const [addForm, setAddForm] = useState({ userId: '', role: 'teacher' as 'admin' | 'teacher' })

  async function load() {
    try {
      const data = await api.get<{ members: CourseMember[] }>(`/api/admin/courses/${courseId}/members`)
      setMembers(data.members)
    } catch {
      toast.error('加载成员失败')
    }
  }

  useEffect(() => { load() }, [courseId])

  async function handleAdd() {
    try {
      await api.post(`/api/admin/courses/${courseId}/members`, addForm)
      toast.success('成员已添加')
      setAddOpen(false)
      setAddForm({ userId: '', role: 'teacher' })
      await load()
    } catch (err: any) {
      toast.error(err.message ?? '添加失败')
    }
  }

  async function handleRemove(userId: string, name: string) {
    confirm({ title: "移除成员", message: `确定移除「${name}」？`, onConfirm: async () => { await api.delete(`/api/admin/courses/${courseId}/members/${userId}`); toast.success("成员已移除"); await load(); } })

      await api.delete(`/api/admin/courses/${courseId}/members/${userId}`)
      toast.success('成员已移除')
      await load()
    } catch (err: any) {
      toast.error(err.message ?? '移除失败')
    }
  }

  return (
    <>
      <section className="panel single-panel !max-w-[800px]">
        <div className="panel-head">
          <div className="flex items-center gap-3">
            <Button variant="ghost" size="sm" onClick={() => router.back()}>
              <ArrowLeft className="size-4" />
            </Button>
            <h2>课程成员</h2>
            <span className="muted">管理课程管理员和授课教师</span>
          </div>
          <div className="toolbar-actions">
            <Button onClick={() => setAddOpen(true)}>
              <Plus className="size-4 mr-1" />
              添加成员
            </Button>
          </div>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>用户名</TableHead>
              <TableHead>显示名</TableHead>
              <TableHead className="w-[100px]">全局角色</TableHead>
              <TableHead className="w-[100px]">课程角色</TableHead>
              <TableHead className="w-[100px]">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-[#94a3b8] py-8">
                  暂无成员，点击"添加成员"邀请教师或管理员
                </TableCell>
              </TableRow>
            )}
            {members.map((m) => (
              <TableRow key={m.userId}>
                <TableCell className="font-mono text-sm">{m.username}</TableCell>
                <TableCell>{m.displayName}</TableCell>
                <TableCell>
                  <Badge variant="outline">{m.globalRole}</Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={m.memberRole === 'admin' ? 'default' : 'secondary'}>
                    {m.memberRole === 'admin' ? '管理员' : '教师'}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Button variant="destructive" size="sm"
                    onClick={() => handleRemove(m.userId, m.displayName)}>
                    <Trash2 className="size-3.5 mr-1" />
                    移除
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      {/* Add Member Dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>添加课程成员</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>用户 ID</Label>
              <Input value={addForm.userId} onChange={e => setAddForm({ ...addForm, userId: e.target.value })}
                placeholder="输入用户的 username 或 ID" />
            </div>
            <div>
              <Label>角色</Label>
              <select className="flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm"
                value={addForm.role}
                onChange={e => setAddForm({ ...addForm, role: e.target.value as 'admin' | 'teacher' })}>
                <option value="teacher">教师</option>
                <option value="admin">管理员</option>
              </select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAddOpen(false)}>取消</Button>
            <Button onClick={handleAdd} disabled={!addForm.userId}>添加</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
