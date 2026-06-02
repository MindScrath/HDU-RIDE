'use client'

import { createContext, useContext, useState, useCallback, type ReactNode } from 'react'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface ConfirmOptions {
  title?: string
  message: string
  onConfirm: () => void | Promise<void>
  variant?: 'default' | 'destructive'
}

interface ConfirmContextValue {
  confirm: (opts: ConfirmOptions) => void
}

const ConfirmContext = createContext<ConfirmContextValue | null>(null)

export function useConfirm() {
  const ctx = useContext(ConfirmContext)
  if (!ctx) throw new Error('useConfirm must be used within ConfirmProvider')
  return ctx.confirm
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [opts, setOpts] = useState<ConfirmOptions | null>(null)

  const confirm = useCallback((options: ConfirmOptions) => {
    setOpts(options)
    setOpen(true)
  }, [])

  async function handleConfirm() {
    if (opts?.onConfirm) {
      await opts.onConfirm()
    }
    setOpen(false)
    setOpts(null)
  }

  return (
    <ConfirmContext.Provider value={{ confirm }}>
      {children}
      <Dialog open={open} onOpenChange={(v) => { setOpen(v); if (!v) setOpts(null) }}>
        <DialogContent className="sm:max-w-[400px]">
          <DialogHeader>
            <DialogTitle>{opts?.title ?? '确认操作'}</DialogTitle>
            <DialogDescription>{opts?.message}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setOpen(false); setOpts(null) }}>
              取消
            </Button>
            <Button variant={opts?.variant ?? 'destructive'} onClick={handleConfirm}>
              确认
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ConfirmContext.Provider>
  )
}
