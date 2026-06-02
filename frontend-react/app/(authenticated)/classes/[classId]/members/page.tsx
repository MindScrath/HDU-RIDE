'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { api } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import type { ClassItem, MemberRow } from '@/lib/types'

export default function ClassMembersPage() {
  const { classId } = useParams<{ classId: string }>()
  const user = useSession((s) => s.user)
  const router = useRouter()
  const [klass, setKlass] = useState<ClassItem | null>(null)
  const [members, setMembers] = useState<MemberRow[]>([])
  const [teachers, setTeachers] = useState<{ userId: string; username: string; displayName: string; globalRole: string }[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [importText, setImportText] = useState('username,displayName,password\nstudent001,学生一,student123')
  const [addTeacherOpen, setAddTeacherOpen] = useState(false)
  const [addTeacherId, setAddTeacherId] = useState('')
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordTarget, setPasswordTarget] = useState<MemberRow | null>(null)
  const [newPassword, setNewPassword] = useState('')

  const canManage = ['root', 'admin', 'teacher'].includes(user?.role ?? '')

  async function load() {
    const klassData = await api.get<{ class: ClassItem }>(`/api/classes/${classId}`)
    setKlass(klassData.class)
    const memData = await api.get<{ members: MemberRow[] }>(`/api/classes/${classId}/members`)
    setMembers(memData.members)
    try {
      const teacherData = await api.get<{ teachers: any[] }>(`/api/classes/${classId}/teachers`)
      setTeachers(teacherData.teachers)
    } catch { /* teachers endpoint may not exist yet */ }
  }

  useEffect(() => { load() }, [])

  async function handleAddTeacher() {
    try {
      await api.post(`/api/classes/${classId}/teachers`, { userId: addTeacherId })
      toast.success('教师已添加')
      setAddTeacherOpen(false)
      setAddTeacherId('')
      await load()
    } catch (err: any) {
      toast.error(err.message ?? '添加教师失败')
    }
  }

  async function handleRemoveTeacher(userId: string) {
    if (!confirm('确定移除该教师？')) return
    try {
      await api.delete(`/api/classes/${classId}/teachers/${userId}`)
      toast.success('教师已移除')
      await load()
    } catch (err: any) {
      toast.error(err.message ?? '移除教师失败')
    }
  }

  async function handleImport() {
    const students = importText
      .split(/\r?\n/)
      .slice(1)
      .map((line) => line.split(',').map((item) => item.trim()))
      .filter((row) => row[0] && row[1] && row[2])
      .map(([username, displayName, password]) => ({ username, displayName, password }))
    await api.post(`/api/classes/${classId}/members/import`, { students })
    toast.success('成员已导入')
    await load()
  }

  async function handleRemove(ids: string[]) {
    if (!ids.length) return
    if (!confirm(`确定移除 ${ids.length} 个班级成员？账号本身会保留。`)) return
    await api.post(`/api/classes/${classId}/members/bulk`, { action: 'remove', userIds: ids })
    toast.success('成员已移除')
    setSelectedIds(new Set())
    await load()
  }

  async function handleSetRole(ids: string[], memberRole: 'student' | 'assistant') {
    if (!ids.length) return
    await api.post(`/api/classes/${classId}/members/bulk`, { action: 'setMemberRole', userIds: ids, memberRole })
    toast.success(memberRole === 'assistant' ? '已设为助教' : '已设为学生')
    setSelectedIds(new Set())
    await load()
  }

  async function handleSavePassword() {
    if (!passwordTarget) return
    await api.post(`/api/classes/${classId}/members/${passwordTarget.user.id}/password`, { password: newPassword })
    toast.success('密码已重置')
    setPasswordOpen(false)
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
            <h2>{klass?.name ?? '成员'}</h2>
            <span className="muted">学生与助教绑定在当前班级</span>
          </div>
          <div className="toolbar-actions">
            {canManage && selectedIds.size > 0 && (
              <>
                <Button variant="outline" onClick={() => handleSetRole(Array.from(selectedIds), 'assistant')}>设为助教</Button>
                <Button variant="outline" onClick={() => handleSetRole(Array.from(selectedIds), 'student')}>设为学生</Button>
                <Button variant="destructive" onClick={() => handleRemove(Array.from(selectedIds))}>移除选中</Button>
              </>
            )}
            <Button variant="outline" onClick={() => router.push('/classes')}>返回班级</Button>
          </div>
        </div>
        {/* Teachers Section */}
        {canManage && teachers.length > 0 && (
          <div className="px-4 pt-3 pb-1 border-b">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-semibold">授课教师</h3>
              <Button variant="outline" size="sm" onClick={() => setAddTeacherOpen(true)}>添加教师</Button>
            </div>
            <div className="flex flex-wrap gap-2 mb-2">
              {teachers.map(t => (
                <span key={t.userId} className="inline-flex items-center gap-1 px-2 py-0.5 bg-blue-50 border border-blue-200 rounded text-sm">
                  {t.displayName} ({t.username})
                  <button className="text-red-400 hover:text-red-600 ml-1" onClick={() => handleRemoveTeacher(t.userId)}>×</button>
                </span>
              ))}
            </div>
          </div>
        )}
        {canManage && teachers.length === 0 && (
          <div className="px-4 pt-3 pb-1 border-b flex items-center justify-between">
            <span className="text-sm text-[#94a3b8]">暂无授课教师</span>
            <Button variant="outline" size="sm" onClick={() => setAddTeacherOpen(true)}>添加教师</Button>
          </div>
        )}
        <div className="member-layout">
          {canManage && (
            <div className="member-import">
              <h3>导入学生</h3>
              <Textarea value={importText} onChange={(e) => setImportText(e.target.value)} rows={8} />
              <Button onClick={handleImport}>导入</Button>
            </div>
          )}
          <Table>
            <TableHeader>
              <TableRow>
                {canManage && <TableHead className="w-[44px]" />}
                <TableHead className="w-[160px]">账号</TableHead>
                <TableHead className="w-[160px]">姓名</TableHead>
                <TableHead className="w-[130px]">班级角色</TableHead>
                <TableHead className="w-[110px]">状态</TableHead>
                <TableHead>加入时间</TableHead>
                {canManage && <TableHead className="w-[260px]">操作</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((row) => (
                <TableRow key={row.user.id}>
                  {canManage && (
                    <TableCell>
                      <Checkbox
                        checked={selectedIds.has(row.user.id)}
                        onCheckedChange={() => toggleSelect(row.user.id)}
                      />
                    </TableCell>
                  )}
                  <TableCell>{row.user.username}</TableCell>
                  <TableCell>{row.user.displayName}</TableCell>
                  <TableCell>{row.memberRole}</TableCell>
                  <TableCell>{row.user.status}</TableCell>
                  <TableCell>{row.joinedAt}</TableCell>
                  {canManage && (
                    <TableCell>
                      <div className="flex gap-1">
                        <Button
                          variant="link" size="sm"
                          onClick={() => handleSetRole([row.user.id], row.memberRole === 'assistant' ? 'student' : 'assistant')}
                        >
                          {row.memberRole === 'assistant' ? '设为学生' : '设为助教'}
                        </Button>
                        {row.memberRole === 'student' && (
                          <Button
                            variant="link" size="sm"
                            onClick={() => { setPasswordTarget(row); setNewPassword(''); setPasswordOpen(true) }}
                          >
                            重置密码
                          </Button>
                        )}
                        <Button variant="link" size="sm" className="text-red-500" onClick={() => handleRemove([row.user.id])}>移除</Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </section>

      <Dialog open={passwordOpen} onOpenChange={setPasswordOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>重置学生密码</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>学生</Label>
              <Input value={passwordTarget?.user.displayName ?? ''} disabled />
            </div>
            <div>
              <Label>新密码</Label>
              <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasswordOpen(false)}>取消</Button>
            <Button onClick={handleSavePassword}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add Teacher Dialog */}
      <Dialog open={addTeacherOpen} onOpenChange={setAddTeacherOpen}>
        <DialogContent className="sm:max-w-[400px]">
          <DialogHeader><DialogTitle>添加授课教师</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>教师用户 ID</Label>
              <Input value={addTeacherId} onChange={e => setAddTeacherId(e.target.value)} placeholder="输入教师的 username 或 ID" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAddTeacherOpen(false)}>取消</Button>
            <Button onClick={handleAddTeacher} disabled={!addTeacherId}>添加</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
