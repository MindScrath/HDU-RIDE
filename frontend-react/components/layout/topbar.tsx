// components/layout/topbar.tsx
'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { User } from 'lucide-react'
import { useSession } from '@/stores/session'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toast } from 'sonner'

export function Topbar() {
  const user = useSession((s) => s.user)
  const logout = useSession((s) => s.logout)
  const changePassword = useSession((s) => s.changePassword)
  const router = useRouter()
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  async function handleLogout() {
    await logout()
    router.push('/login')
  }

  async function handleSavePassword() {
    if (newPassword !== confirmPassword) {
      toast.error('两次输入的新密码不一致')
      return
    }
    setPasswordSaving(true)
    try {
      await changePassword(oldPassword, newPassword)
      toast.success('密码已修改')
      setPasswordOpen(false)
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } finally {
      setPasswordSaving(false)
    }
  }

  return (
    <>
      <header className="global-topbar">
        <div className="topbar-brand">
          <span className="brand-mark">R</span>
          <div className="brand-copy">
            <strong>HDU RIDE</strong>
          </div>
        </div>
        <div className="topbar-actions">
          {user && (
            <DropdownMenu>
              <DropdownMenuTrigger>
                <button className="user-button">
                  <User size={16} />
                  {user.displayName}
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem disabled>{user.role}</DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => setPasswordOpen(true)}>
                  修改密码
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={handleLogout}>退出</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </header>

      <Dialog open={passwordOpen} onOpenChange={setPasswordOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>修改密码</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>当前密码</Label>
              <Input type="password" autoComplete="current-password"
                value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} />
            </div>
            <div>
              <Label>新密码</Label>
              <Input type="password" autoComplete="new-password"
                value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
            </div>
            <div>
              <Label>确认新密码</Label>
              <Input type="password" autoComplete="new-password"
                value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasswordOpen(false)}>取消</Button>
            <Button onClick={handleSavePassword} disabled={passwordSaving}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
