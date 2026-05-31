'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'

export default function LoginPage() {
  const login = useSession((s) => s.login)
  const router = useRouter()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await login(username, password)
      router.push('/classes')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <section className="panel login-box">
        <div className="flex items-center gap-3 mb-5" style={{ color: '#172033' }}>
          <span className="brand-mark">R</span>
          <div>
            <strong className="block text-[#101827] font-serif text-lg leading-tight">
              HDU RIDE
            </strong>
            <small className="text-[#6c7787] text-[11px]">
              金融计量分析教学平台
            </small>
          </div>
        </div>
        <form onSubmit={handleSubmit} className="grid gap-3">
          <div>
            <Label>账号</Label>
            <Input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
            />
          </div>
          <div>
            <Label>密码</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            登录
          </Button>
        </form>
      </section>
    </div>
  )
}
