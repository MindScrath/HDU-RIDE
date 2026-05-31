'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { MarkdownRenderer } from '@/components/markdown/MarkdownRenderer'
import { api } from '@/lib/api'
import type { ClassItem, LectureChapter } from '@/lib/types'

interface Props {
  classId?: string
  lectureId?: string
}

export function LectureViewer({ classId, lectureId }: Props) {
  const router = useRouter()
  const [classes, setClasses] = useState<ClassItem[]>([])
  const [chapters, setChapters] = useState<LectureChapter[]>([])
  const [raw, setRaw] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const basePath = classId ? `/classes/${classId}/lectures` : '/lectures'
  const sections = chapters.flatMap((ch) => ch.sections)
  const selected = lectureId ?? sections[0]?.id ?? ''
  const selectedSection = sections.find((s) => s.id === selected)

  // Load classes
  useEffect(() => {
    api.get<{ classes: ClassItem[] }>('/api/classes').then((data) => {
      setClasses(data.classes)
      if (!classId && data.classes[0]) {
        router.replace(`/classes/${data.classes[0].id}/lectures`)
      }
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Load chapters
  const loadChapters = useCallback(async () => {
    const path = classId ? `/api/classes/${classId}/lectures` : '/api/lectures'
    const data = await api.get<{ lectures: LectureChapter[] }>(path)
    setChapters(data.lectures)
    if (!lectureId && data.lectures[0]?.sections[0]) {
      router.replace(`${basePath}/${data.lectures[0].sections[0].id}`)
    }
  }, [classId, basePath, lectureId, router])

  useEffect(() => { loadChapters() }, [loadChapters])

  // Load lecture content
  useEffect(() => {
    if (!selected) return
    setLoading(true)
    setError('')
    setRaw('')

    const url = `${classId ? `/api/classes/${classId}` : '/api'}/lectures/${selected}`
    api.get<{ markdown: string }>(url)
      .then((data) => setRaw(data.markdown))
      .catch((err) => {
        setError(err instanceof Error ? err.message : '讲义加载失败')
      })
      .finally(() => setLoading(false))
  }, [selected, classId])

  return (
    <div className="page-grid">
      <aside className="panel scroll lecture-tree">
        <div className="panel-head"><h3>章节</h3></div>
        {chapters.map((ch) => (
          <div key={ch.id} className="chapter-block">
            <div className="chapter-title">{ch.title}</div>
            {ch.sections.map((section) => (
              <Link
                key={section.id}
                href={`${basePath}/${section.id}`}
                className={`section-item ${section.id === selected ? 'active' : ''}`}
              >
                <span>{section.title}</span>
                <small className="text-green-600 text-[11px]">已发布</small>
              </Link>
            ))}
          </div>
        ))}
      </aside>
      <article className="panel scroll">
        <div className="panel-head">
          <h2>{selectedSection?.title ?? '讲义'}</h2>
          {classes.length > 0 && (
            <Select
              value={classId ?? ''}
              onValueChange={(v) => {
                const val = v ?? ''
                router.push(val ? `/classes/${val}/lectures` : '/lectures')
              }}
            >
              <SelectTrigger className="context-select">
                <SelectValue placeholder="选择班级" />
              </SelectTrigger>
              <SelectContent>
                {classes.map((klass) => (
                  <SelectItem key={klass.id} value={klass.id}>{klass.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
        {loading && (
          <div className="p-6">
            <Skeleton className="h-4 w-full mb-2" />
            <Skeleton className="h-4 w-3/4 mb-2" />
            <Skeleton className="h-4 w-5/6 mb-2" />
          </div>
        )}
        {error && (
          <div className="p-10 text-center text-red-500">{error}</div>
        )}
        {!loading && !error && (
          <div className="markdown">
            <MarkdownRenderer content={raw} />
          </div>
        )}
      </article>
    </div>
  )
}
