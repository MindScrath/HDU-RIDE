// components/assignments/AssignmentViewer.tsx
'use client'

import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { useRouter } from 'next/navigation'
import dayjs from 'dayjs'
import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { MarkdownRenderer } from '@/components/markdown/MarkdownRenderer'
import { api } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import type { Assignment, ClassItem, Submission, SubmissionRow } from '@/lib/types'
import { Maximize2, Minimize2 } from 'lucide-react'

interface AssignmentEntry {
  assignment: Assignment
  classId: string
  className: string
  courseId: string
}

interface Props {
  classId?: string
  assignmentId?: string
}

function sleep(ms: number) { return new Promise((r) => setTimeout(r, ms)) }

export function AssignmentViewer({ classId, assignmentId }: Props) {
  const router = useRouter()
  const user = useSession((s) => s.user)
  const isStudent = user?.role === 'student'
  const canManage = ['root', 'admin', 'teacher'].includes(user?.role ?? '')
  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState({ title: '', dueAt: '', rstudioImage: '', starter: '', submitPath: '' })

  const [classes, setClasses] = useState<ClassItem[]>([])
  const [entries, setEntries] = useState<AssignmentEntry[]>([])
  const [raw, setRaw] = useState('')
  const [status, setStatus] = useState<Record<string, unknown>>({})
  const [rows, setRows] = useState<SubmissionRow[]>([])
  const [workspaceURL, setWorkspaceURL] = useState('')
  const [workspaceLoading, setWorkspaceLoading] = useState(false)
  const [workspaceFullscreen, setWorkspaceFullscreen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [promptOpen, setPromptOpen] = useState(true)
  const [classFilter, setClassFilter] = useState(classId ?? '')
  const [gradeDialog, setGradeDialog] = useState(false)
  const [currentSubmission, setCurrentSubmission] = useState<Submission | null>(null)
  const [selectedSubmission, setSelectedSubmission] = useState<SubmissionRow | null>(null)
  const [gradeForm, setGradeForm] = useState({ score: 0, comment: '' })
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyStudent, setHistoryStudent] = useState('')
  const [listWidth, setListWidth] = useState(205)
  const [ideWidth, setIdeWidth] = useState(360)
  const gridRef = useRef<HTMLDivElement>(null)

  // Load saved widths from localStorage on mount
  useEffect(() => {
    const savedList = localStorage.getItem('hdu.assignment.listWidth')
    const savedIde = localStorage.getItem('hdu.assignment.ideWidth')
    if (savedList) setListWidth(Number(savedList))
    if (savedIde) setIdeWidth(Number(savedIde))
  }, [])

  const filteredEntries = useMemo(
    () => entries.filter((e) => !classFilter || e.classId === classFilter),
    [entries, classFilter]
  )

  const activeClassId = classId || filteredEntries[0]?.classId || ''
  const selected = assignmentId ?? filteredEntries[0]?.assignment.id ?? ''
  const selectedEntry = filteredEntries.find(
    (e) => e.assignment.id === selected && (!classFilter || e.classId === classFilter)
  ) ?? filteredEntries[0]
  const selectedAssignment = selectedEntry?.assignment

  const latestRows = useMemo(() => {
    const byUser = new Map<string, SubmissionRow>()
    for (const row of rows) {
      const prev = byUser.get(row.submission.userId)
      if (!prev || row.submission.attempt > prev.submission.attempt) {
        byUser.set(row.submission.userId, row)
      }
    }
    return Array.from(byUser.values()).sort((a, b) =>
      a.studentName.localeCompare(b.studentName, 'zh-CN')
    )
  }, [rows])

  const historyRows = useMemo(
    () =>
      rows
        .filter((r) => r.submission.userId === historyStudent)
        .sort((a, b) => b.submission.attempt - a.submission.attempt),
    [rows, historyStudent]
  )

  const reviewedCount = latestRows.filter((r) => r.grade.score !== null).length
  const pendingCount = latestRows.filter((r) => r.grade.score === null).length
  const averageScore = useMemo(() => {
    const scores = latestRows.map((r) => r.grade.score).filter((s): s is number => s !== null)
    return scores.length ? (scores.reduce((a, b) => a + b, 0) / scores.length).toFixed(1) : '-'
  }, [latestRows])

  // Load assignments
  useEffect(() => {
    async function load() {
      const data = await api.get<{ classes: ClassItem[] }>('/api/classes')
      setClasses(data.classes)
      const visibleClasses = classId ? data.classes.filter((c) => c.id === classId) : data.classes
      const loaded = await Promise.all(
        visibleClasses.map(async (klass) => {
          const d = await api.get<{ assignments: Assignment[] }>(`/api/classes/${klass.id}/assignments`)
          return d.assignments.map((a) => ({
            assignment: a,
            classId: klass.id,
            className: klass.name,
            courseId: klass.courseId,
          }))
        })
      )
      setEntries(loaded.flat())
      if (!classId) setClassFilter(new URLSearchParams(window.location.search).get('classId') ?? '')
    }
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Load assignment detail
  const loadAssignment = useCallback(async () => {
    if (!selected || !activeClassId) return
    const data = await api.get<{ assignment: Assignment; markdown: string; status: Record<string, unknown> }>(
      `/api/classes/${activeClassId}/assignments/${selected}`
    )
    setRaw(data.markdown)
    setStatus(data.status)
    setPromptOpen(isStudent)
    setSelectedSubmission(null)
    setWorkspaceURL('')
    setWorkspaceFullscreen(false)
    if (!isStudent) {
      const subData = await api.get<{ submissions: SubmissionRow[] }>(
        `/api/classes/${activeClassId}/assignments/${selected}/submissions`
      )
      setRows(subData.submissions)
    } else {
      setRows([])
    }
  }, [selected, activeClassId, isStudent])

  useEffect(() => { loadAssignment() }, [loadAssignment])

  async function handleCreateAssignment() {
    try {
      await api.post(`/api/classes/${activeClassId}/assignments`, createForm)
      toast.success('作业已创建')
      setCreateOpen(false)
      setCreateForm({ title: '', dueAt: '', rstudioImage: '', starter: '', submitPath: '' })
      // Reload assignments list
      const data = await api.get<{ assignments: Assignment[] }>(`/api/classes/${activeClassId}/assignments`)
      setEntries(data.assignments.map(a => ({ assignment: a, classId: activeClassId, className: '', courseId: '' })))
    } catch (err: any) {
      toast.error(err.message ?? '创建失败')
    }
  }

  // Workspace
  async function waitForGateway(url: string) {
    for (let i = 0; i < 30; i++) {
      try {
        const resp = await fetch(url, { credentials: 'include', cache: 'no-store' })
        if (resp.ok) return
      } catch { /* retry */ }
      await sleep(700)
    }
    throw new Error('RStudio 尚未就绪')
  }

  async function startWorkspace() {
    setWorkspaceLoading(true)
    try {
      const data = await api.post<{ workspace: { id: string; ideURL: string } }>(
        `/api/classes/${activeClassId}/assignments/${selected}/workspace`
      )
      await waitForGateway(data.workspace.ideURL)
      setWorkspaceURL(data.workspace.ideURL)
    } finally {
      setWorkspaceLoading(false)
    }
  }

  async function handleSubmit() {
    setSubmitting(true)
    try {
      await api.post(`/api/classes/${activeClassId}/assignments/${selected}/submit`)
      toast.success('工作区已提交')
      await loadAssignment()
    } finally {
      setSubmitting(false)
    }
  }

  async function selectSubmissionRow(row: SubmissionRow) {
    setSelectedSubmission(row)
    setWorkspaceURL('')
    setWorkspaceLoading(true)
    try {
      const data = await api.post<{ workspace: { id: string; ideURL: string } }>(
        `/api/submissions/${row.submission.id}/workspace`
      )
      await waitForGateway(data.workspace.ideURL)
      setWorkspaceURL(data.workspace.ideURL)
    } finally {
      setWorkspaceLoading(false)
    }
  }

  function openGrade(row: SubmissionRow) {
    selectSubmissionRow(row)
    setCurrentSubmission(row.submission)
    setGradeForm({ score: row.grade.score ?? 0, comment: row.grade.comment ?? '' })
    setGradeDialog(true)
  }

  async function saveGrade() {
    if (!currentSubmission) return
    const data = await api.post<{ id: string }>(`/api/submissions/${currentSubmission.id}/grade`, gradeForm)
    await api.post(`/api/grades/${data.id}/publish`)
    toast.success('成绩已发布')
    setGradeDialog(false)
    await loadAssignment()
  }

  function showHistory(row: SubmissionRow) {
    setHistoryStudent(row.submission.userId)
    setHistoryOpen(true)
  }

  async function exportGrades() {
    const blob = await api.download(
      `/api/classes/${activeClassId}/assignments/${selected}/grades/export?format=csv`
    )
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${selectedEntry?.className ?? 'class'}-${selected}-grades.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  // Resize
  function clamp(v: number, min: number, max: number) {
    return Math.min(Math.max(v, min), Math.max(min, max))
  }

  function startResize(target: 'list' | 'ide', e: React.PointerEvent) {
    const rect = gridRef.current?.getBoundingClientRect()
    if (!rect) return
    e.preventDefault()
    const move = (ev: PointerEvent) => {
      if (target === 'list') {
        setListWidth(clamp(ev.clientX - rect.left, 170, 280))
      } else {
        setIdeWidth(clamp(rect.right - ev.clientX, 280, rect.width - listWidth - 430))
      }
    }
    const stop = () => {
      localStorage.setItem('hdu.assignment.listWidth', String(listWidth))
      localStorage.setItem('hdu.assignment.ideWidth', String(ideWidth))
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop, { once: true })
  }

  const gridStyle = {
    '--assignment-list-width': `${listWidth}px`,
    '--assignment-ide-width': `${ideWidth}px`,
  } as React.CSSProperties

  function assignmentPath(entry: AssignmentEntry) {
    if (classId) return `/classes/${entry.classId}/assignments/${entry.assignment.id}`
    return `/assignments/${entry.assignment.id}?classId=${entry.classId}`
  }

  return (
    <>
      <div ref={gridRef} className="assignment-grid" style={gridStyle}>
        {/* Left: Assignment List */}
        <aside className="panel scroll">
          <div className="panel-head">
            <h3>作业</h3>
            {canManage && activeClassId && (
              <button className="text-blue-600 text-xs hover:underline" onClick={() => setCreateOpen(true)}>
                + 新建
              </button>
            )}
          </div>
          {!classId && (
            <div className="p-2 border-b">
              <Select value={classFilter} onValueChange={(v) => { setClassFilter(v ?? ''); router.push(v ? `?classId=${v}` : '/assignments') }}>
                <SelectTrigger><SelectValue placeholder="选择班级" /></SelectTrigger>
                <SelectContent>
                  {classes.map((klass) => (
                    <SelectItem key={klass.id} value={klass.id}>{klass.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          {filteredEntries.length === 0 && (
            <div className="p-6 text-center text-[#94a3b8] text-sm">
              {classes.length === 0
                ? '暂无班级，请先联系管理员创建班级并导入课程内容'
                : '暂无作业'}
            </div>
          )}
          {filteredEntries.map((entry) => {
            const isActive = entry.assignment.id === selected && entry.classId === activeClassId
            return (
              <Link
                key={`${entry.classId}-${entry.assignment.id}`}
                href={assignmentPath(entry)}
                className={cn('list-item', isActive && 'active')}
              >
                <strong>{entry.assignment.title}</strong>
                <div className="muted">{entry.className}</div>
                <div className="muted">截止 {dayjs(entry.assignment.dueAt).format('MM-DD HH:mm')}</div>
              </Link>
            )
          })}
        </aside>

        <div className="splitter" onPointerDown={(e) => startResize('list', e)} />

        {/* Center: Assignment Detail */}
        <section className="panel scroll">
          <div className="panel-head">
            <div>
              <h2>{selectedAssignment?.title ?? '作业'}</h2>
              <span className="muted">{selectedEntry?.className}</span>
            </div>
          </div>

          <div className={cn('prompt-block', !promptOpen && 'collapsed')}>
            <button className="prompt-toggle" onClick={() => setPromptOpen(!promptOpen)}>
              <strong>作业题面</strong>
              <span>{promptOpen ? '收起' : '展开'}</span>
            </button>
            {promptOpen && (
              <div className="markdown">
                <MarkdownRenderer content={raw} />
              </div>
            )}
          </div>

          {isStudent ? (
            <div className="student-submit">
              <div className="grid grid-cols-2 border rounded-md overflow-hidden mb-4">
                <div className="p-2 border-r"><span className="text-xs text-gray-500">提交次数</span><div className="font-bold">{String(status.attempts ?? 0)}</div></div>
                <div className="p-2"><span className="text-xs text-gray-500">最近提交</span><div className="font-bold">{String(status.latestSubmittedAt ?? '未提交')}</div></div>
              </div>
              <div className="submit-actions">
                <Button onClick={handleSubmit} disabled={submitting}>提交 RStudio 工作区</Button>
              </div>
            </div>
          ) : (
            <div className="grading-overview">
              <div className="grading-toolbar">
                <Button variant="outline" onClick={exportGrades}>导出成绩 CSV</Button>
              </div>
              <div className="metric-row">
                <div><strong>{latestRows.length}</strong><span>提交数</span></div>
                <div><strong>{pendingCount}</strong><span>待批改</span></div>
                <div><strong>{reviewedCount}</strong><span>已批改</span></div>
                <div><strong>{averageScore}</strong><span>平均分</span></div>
              </div>
              <div className="mt-3">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[110px]">学生</TableHead>
                      <TableHead>提交</TableHead>
                      <TableHead className="w-[82px]">成绩</TableHead>
                      <TableHead className="w-[112px]">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {latestRows.map((row) => (
                      <TableRow
                        key={row.submission.id}
                        className={cn(row.submission.id === selectedSubmission?.submission.id && 'bg-blue-50')}
                        onClick={() => selectSubmissionRow(row)}
                      >
                        <TableCell>{row.studentName}</TableCell>
                        <TableCell>
                          <Button variant="link" size="sm" onClick={(e) => { e.stopPropagation(); showHistory(row) }}>
                            第 {row.submission.attempt} 次
                          </Button>
                          {row.submission.late && <Badge variant="outline" className="ml-1 text-orange-600 border-orange-300">补交</Badge>}
                        </TableCell>
                        <TableCell>{row.grade.score ?? '未评分'}</TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button variant="link" size="sm" onClick={(e) => { e.stopPropagation(); selectSubmissionRow(row) }}>查看</Button>
                            <Button variant="link" size="sm" onClick={(e) => { e.stopPropagation(); openGrade(row) }}>批改</Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          )}
        </section>

        <div className="splitter" onPointerDown={(e) => startResize('ide', e)} />

        {/* Right: RStudio Workspace */}
        <section className={cn('panel workspace-panel', workspaceFullscreen && 'fullscreen')}>
          <div className="panel-head">
            <h3>{selectedSubmission ? '批改工作区' : 'RStudio 工作区'}</h3>
            <div className="flex items-center gap-2">
              {workspaceURL && (
                <Button variant="ghost" size="sm" onClick={() => setWorkspaceFullscreen(!workspaceFullscreen)}>
                  {workspaceFullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
                  <span className="ml-1">{workspaceFullscreen ? '退出全屏' : '全屏'}</span>
                </Button>
              )}
              {isStudent ? (
                <Button onClick={startWorkspace} disabled={workspaceLoading}>打开 RStudio</Button>
              ) : selectedSubmission ? (
                <Button onClick={() => selectSubmissionRow(selectedSubmission)} disabled={workspaceLoading}>重新打开</Button>
              ) : null}
            </div>
          </div>
          {workspaceURL ? (
            <iframe src={workspaceURL} className="ide-frame" />
          ) : !isStudent && selectedSubmission ? (
            <div className="submission-preview">
              <div className="preview-meta">
                <strong>{selectedSubmission.studentName}</strong>
                <span>第 {selectedSubmission.submission.attempt} 次提交</span>
                {selectedSubmission.grade.score !== null ? (
                  <Badge variant="outline" className="text-green-600 border-green-300">{selectedSubmission.grade.score}</Badge>
                ) : (
                  <Badge variant="outline" className="text-orange-600 border-orange-300">未评分</Badge>
                )}
              </div>
              {workspaceLoading && <Skeleton className="h-20 w-full" />}
            </div>
          ) : (
            <div className="flex items-center justify-center h-[calc(100%-52px)] text-gray-400">
              {isStudent ? '点击打开后创建独立 RStudio Pod' : '选择提交后在这里查看内容'}
            </div>
          )}
        </section>
      </div>

      {/* History Dialog */}
      <Dialog open={historyOpen} onOpenChange={setHistoryOpen}>
        <DialogContent className="sm:max-w-[520px]">
          <DialogHeader><DialogTitle>提交历史</DialogTitle></DialogHeader>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[100px]">版本</TableHead>
                <TableHead>提交时间</TableHead>
                <TableHead className="w-[90px]">成绩</TableHead>
                <TableHead className="w-[90px]">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {historyRows.map((row) => (
                <TableRow key={row.submission.id}>
                  <TableCell>第 {row.submission.attempt} 次</TableCell>
                  <TableCell>{dayjs(row.submission.createdAt).format('MM-DD HH:mm')}</TableCell>
                  <TableCell>{row.grade.score ?? '未评分'}</TableCell>
                  <TableCell>
                    <Button variant="link" size="sm" onClick={() => { setHistoryOpen(false); selectSubmissionRow(row) }}>
                      查看
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DialogContent>
      </Dialog>

      {/* Grade Dialog */}
      <Dialog open={gradeDialog} onOpenChange={setGradeDialog}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader><DialogTitle>评分</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>分数</Label>
              <Input
                type="number" min={0} max={100}
                value={gradeForm.score}
                onChange={(e) => setGradeForm({ ...gradeForm, score: Number(e.target.value) })}
              />
            </div>
            <div>
              <Label>评语</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm"
                rows={5}
                value={gradeForm.comment}
                onChange={(e) => setGradeForm({ ...gradeForm, comment: e.target.value })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setGradeDialog(false)}>取消</Button>
            <Button onClick={saveGrade}>发布成绩</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Assignment Dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader><DialogTitle>新建作业</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>标题</Label><Input value={createForm.title} onChange={e => setCreateForm({ ...createForm, title: e.target.value })} /></div>
            <div><Label>截止时间</Label><Input type="datetime-local" value={createForm.dueAt} onChange={e => setCreateForm({ ...createForm, dueAt: e.target.value })} /></div>
            <div><Label>RStudio 镜像</Label><Input value={createForm.rstudioImage} onChange={e => setCreateForm({ ...createForm, rstudioImage: e.target.value })} placeholder="rocker/rstudio:4.6.0" /></div>
            <div><Label>Starter 代码</Label><Input value={createForm.starter} onChange={e => setCreateForm({ ...createForm, starter: e.target.value })} /></div>
            <div><Label>提交路径</Label><Input value={createForm.submitPath} onChange={e => setCreateForm({ ...createForm, submitPath: e.target.value })} placeholder="submit/" /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button onClick={handleCreateAssignment} disabled={!createForm.title}>创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
