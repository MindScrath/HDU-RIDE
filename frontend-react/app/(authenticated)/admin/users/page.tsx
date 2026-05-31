'use client'

import { useEffect, useState, useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { api } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import type { Role, User } from '@/lib/types'

export default function AdminUsersPage() {
  const currentUser = useSession((s) => s.user)
  const [users, setUsers] = useState<User[]>([])
  const [selectedUsers, setSelectedUsers] = useState<User[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [createForm, setCreateForm] = useState({ username: '', displayName: '', password: '', role: 'student' as Role })
  const [editForm, setEditForm] = useState({ id: '', displayName: '', role: 'student' as Role, status: 'active' })
  const [passwordForm, setPasswordForm] = useState({ id: '', displayName: '', password: '' })

  const roleOptions = useMemo(() => {
    const roles: Array<{ label: string; value: Role }> = [
      { label: 'Root', value: 'root' },
      { label: 'Admin', value: 'admin' },
      { label: 'Teacher', value: 'teacher' },
      { label: 'Assistant', value: 'assistant' },
      { label: 'Student', value: 'student' },
    ]
    return currentUser?.role === 'root' ? roles : roles.filter((r) => r.value !== 'root')
  }, [currentUser])

  function canManage(user: User) {
    if (currentUser?.role === 'root') return true
    return currentUser?.role === 'admin' && user.role !== 'root'
  }

  function canMutateIdentity(user: User) {
    return canManage(user) && user.id !== currentUser?.id
  }

  const manageableSelection = selectedUsers.filter(canManage)

  async function load() {
    const data = await api.get<{ users: User[] }>('/api/admin/users')
    setUsers(data.users)
  }

  useEffect(() => { load() }, [])

  async function handleCreate() {
    await api.post('/api/admin/users', createForm)
    toast.success('用户已创建')
    setCreateOpen(false)
    setCreateForm({ username: '', displayName: '', password: '', role: 'student' })
    await load()
  }

  function openEdit(user: User) {
    setEditForm({ id: user.id, displayName: user.displayName, role: user.role, status: user.status })
    setEditOpen(true)
  }

  async function handleSaveEdit() {
    await api.patch(`/api/admin/users/${editForm.id}`, {
      displayName: editForm.displayName,
      role: editForm.role,
      status: editForm.status,
    })
    toast.success('用户已更新')
    setEditOpen(false)
    await load()
  }

  function openPassword(user: User) {
    setPasswordForm({ id: user.id, displayName: user.displayName, password: '' })
    setPasswordOpen(true)
  }

  async function handleSavePassword() {
    await api.post(`/api/admin/users/${passwordForm.id}/password`, { password: passwordForm.password })
    toast.success('密码已重置')
    setPasswordOpen(false)
  }

  async function handleDisable(user: User) {
    if (!confirm(`确定禁用账号 ${user.username}？`)) return
    await api.delete(`/api/admin/users/${user.id}`)
    toast.success('账号已禁用')
    await load()
  }

  async function handleBulk(action: 'disable' | 'activate' | 'setRole', role?: Role) {
    const rows = manageableSelection.filter((u) => action === 'activate' || canMutateIdentity(u))
    if (!rows.length) return
    if (action === 'disable') {
      if (!confirm(`确定禁用 ${rows.length} 个账号？`)) return
    }
    await api.post('/api/admin/users/bulk', { action, ids: rows.map((u) => u.id), role })
    toast.success('批量操作已完成')
    setSelectedUsers([])
    await load()
  }

  function toggleSelect(user: User) {
    setSelectedUsers((prev) =>
      prev.some((u) => u.id === user.id) ? prev.filter((u) => u.id !== user.id) : [...prev, user]
    )
  }

  return (
    <>
      <section className="panel single-panel">
        <div className="panel-head">
          <h2>用户管理</h2>
          <div className="toolbar-actions">
            {manageableSelection.length > 0 && (
              <>
                <Button variant="outline" onClick={() => handleBulk('activate')}>启用选中</Button>
                <Button variant="destructive" onClick={() => handleBulk('disable')}>禁用选中</Button>
                <DropdownMenu>
                  <DropdownMenuTrigger>
                    <Button variant="outline">批量角色</Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent>
                    {roleOptions.map((role) => (
                      <DropdownMenuItem key={role.value} onClick={() => handleBulk('setRole', role.value)}>
                        {role.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </>
            )}
            <Button onClick={() => setCreateOpen(true)}>新建用户</Button>
          </div>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[44px]" />
              <TableHead className="w-[180px]">账号</TableHead>
              <TableHead className="w-[180px]">姓名</TableHead>
              <TableHead className="w-[130px]">角色</TableHead>
              <TableHead className="w-[120px]">状态</TableHead>
              <TableHead>创建时间</TableHead>
              <TableHead className="w-[260px]">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((u) => (
              <TableRow key={u.id}>
                <TableCell>
                  <Checkbox
                    checked={selectedUsers.some((su) => su.id === u.id)}
                    disabled={!canManage(u)}
                    onCheckedChange={() => toggleSelect(u)}
                  />
                </TableCell>
                <TableCell>{u.username}</TableCell>
                <TableCell>{u.displayName}</TableCell>
                <TableCell>{u.role}</TableCell>
                <TableCell>{u.status}</TableCell>
                <TableCell>{u.createdAt}</TableCell>
                <TableCell>
                  <div className="flex gap-1">
                    <Button variant="link" size="sm" disabled={!canManage(u)} onClick={() => openEdit(u)}>编辑</Button>
                    <Button variant="link" size="sm" disabled={!canManage(u)} onClick={() => openPassword(u)}>重置密码</Button>
                    <Button variant="link" size="sm" className="text-red-500" disabled={!canMutateIdentity(u)} onClick={() => handleDisable(u)}>禁用</Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      {/* Create Dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>新建用户</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>账号</Label><Input value={createForm.username} onChange={(e) => setCreateForm({ ...createForm, username: e.target.value })} /></div>
            <div><Label>姓名</Label><Input value={createForm.displayName} onChange={(e) => setCreateForm({ ...createForm, displayName: e.target.value })} /></div>
            <div><Label>密码</Label><Input type="password" value={createForm.password} onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })} /></div>
            <div>
              <Label>角色</Label>
              <Select value={createForm.role} onValueChange={(v) => setCreateForm({ ...createForm, role: v as Role })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {roleOptions.map((r) => <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button onClick={handleCreate}>创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>编辑用户</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>姓名</Label><Input value={editForm.displayName} onChange={(e) => setEditForm({ ...editForm, displayName: e.target.value })} /></div>
            <div>
              <Label>角色</Label>
              <Select value={editForm.role} onValueChange={(v) => setEditForm({ ...editForm, role: v as Role })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {roleOptions.map((r) => <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>状态</Label>
              <Select value={editForm.status} onValueChange={(v) => v && setEditForm({ ...editForm, status: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="disabled">Disabled</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>取消</Button>
            <Button onClick={handleSaveEdit}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Password Dialog */}
      <Dialog open={passwordOpen} onOpenChange={setPasswordOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>重置密码</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>用户</Label><Input value={passwordForm.displayName} disabled /></div>
            <div><Label>新密码</Label><Input type="password" value={passwordForm.password} onChange={(e) => setPasswordForm({ ...passwordForm, password: e.target.value })} /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasswordOpen(false)}>取消</Button>
            <Button onClick={handleSavePassword}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
