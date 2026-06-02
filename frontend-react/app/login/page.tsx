'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import { GraduationCap } from 'lucide-react'

export default function LoginPage() {
  const login = useSession((s) => s.login)
  const router = useRouter()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const canSubmit = username.trim() && password.trim() && !loading

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!canSubmit) return
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
      <div className="login-box">
        {/* Brand */}
        <div className="flex items-center gap-3 mb-8">
          <div className="brand-mark" style={{ width: 44, height: 44, fontSize: 22 }}>
            R
          </div>
          <div>
            <h1 className="text-2xl font-bold text-[#0f172a] tracking-tight" style={{ fontFamily: 'Georgia, serif' }}>
              HDU RIDE
            </h1>
            <p className="text-[13px] text-[#64748b] mt-0.5">
              金融计量分析教学平台
            </p>
          </div>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="grid gap-4">
          <div>
            <Label>账号</Label>
            <Input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="请输入账号"
              autoComplete="username"
              className="h-11"
            />
          </div>
          <div>
            <Label>密码</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="请输入密码"
              autoComplete="current-password"
              className="h-11"
            />
          </div>
          <Button
            type="submit"
            className="w-full h-11 text-[15px] mt-2"
            disabled={!canSubmit}
          >
            {loading ? '登录中…' : '登录'}
          </Button>
        </form>

        <p className="text-center text-[12px] text-[#94a3b8] mt-6">
          <GraduationCap className="inline size-3.5 mr-1 -mt-0.5" />
          杭州电子科技大学 · 金融计量分析教学平台
        </p>
      </div>
    </div>
  )
}
