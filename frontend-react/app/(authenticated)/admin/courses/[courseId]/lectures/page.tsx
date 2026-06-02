'use client'

import { useEffect, useState, use } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { toast } from 'sonner'
import { ArrowLeft, Save, Eye, BookOpen } from 'lucide-react'
import { MarkdownRenderer } from '@/components/markdown/MarkdownRenderer'

interface SectionMeta { id: string; title: string; order: number }
interface ChapterMeta { id: string; title: string; order: number; sections: SectionMeta[] }

export default function LectureEditorPage({ params }: { params: Promise<{ courseId: string }> }) {
  const { courseId } = use(params)
  const router = useRouter()
  const [chapters, setChapters] = useState<ChapterMeta[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [content, setContent] = useState('')
  const [title, setTitle] = useState('')
  const [saving, setSaving] = useState(false)
  const [previewing, setPreviewing] = useState(false)

  async function loadTree() {
    try {
      const data = await api.get<{ lectures: ChapterMeta[] }>(`/api/admin/courses/${courseId}/lectures`)
      setChapters(data.lectures)
    } catch {
      toast.error('加载讲义失败')
    }
  }

  useEffect(() => { loadTree() }, [courseId])

  async function selectLecture(id: string) {
    setSelectedId(id)
    try {
      const data = await api.get<{ id: string; content: string; title: string }>(`/api/admin/courses/${courseId}/lectures/${id}`)
      setContent(data.content)
      setTitle(data.title)
      setPreviewing(false)
    } catch {
      toast.error('加载内容失败')
    }
  }

  async function handleSave() {
    setSaving(true)
    try {
      await api.put(`/api/admin/courses/${courseId}/lectures/${selectedId}`, { content })
      toast.success('已保存')
    } catch (err: any) {
      toast.error(err.message ?? '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page-grid" style={{ height: 'calc(100vh - 100px)' }}>
      {/* Left: Chapter Tree */}
      <aside className="panel scroll lecture-tree">
        <div className="panel-head">
          <Button variant="ghost" size="sm" onClick={() => router.back()}>
            <ArrowLeft className="size-4 mr-1" />
          </Button>
          <h3>章节</h3>
        </div>
        {chapters.map((ch) => (
          <div key={ch.id} className="chapter-block">
            <div className="chapter-title flex items-center gap-1">
              <BookOpen className="size-3" />
              {ch.title}
            </div>
            {ch.sections.map((section) => (
              <div
                key={section.id}
                className={`section-item cursor-pointer ${section.id === selectedId ? 'active' : ''}`}
                onClick={() => selectLecture(section.id)}
              >
                <span>{section.title}</span>
              </div>
            ))}
          </div>
        ))}
        {chapters.length === 0 && (
          <div className="p-6 text-center text-[#94a3b8] text-sm">
            暂无讲义内容，请先导入课程包
          </div>
        )}
      </aside>

      {/* Right: Editor / Preview */}
      <section className="panel scroll" style={{ display: 'flex', flexDirection: 'column' }}>
        <div className="panel-head">
          <h2>{title || '讲义编辑器'}</h2>
          {selectedId && (
            <div className="toolbar-actions">
              <Button variant="outline" size="sm" onClick={() => setPreviewing(!previewing)}>
                <Eye className="size-4 mr-1" />
                {previewing ? '编辑' : '预览'}
              </Button>
              <Button size="sm" onClick={handleSave} disabled={saving}>
                <Save className="size-4 mr-1" />
                {saving ? '保存中…' : '保存'}
              </Button>
            </div>
          )}
        </div>
        {!selectedId ? (
          <div className="flex-1 flex items-center justify-center text-[#94a3b8]">
            请从左侧选择要编辑的讲义
          </div>
        ) : previewing ? (
          <div className="markdown flex-1">
            <MarkdownRenderer content={content} />
          </div>
        ) : (
          <div className="flex-1 p-4">
            <Label className="mb-2 block">Markdown 内容</Label>
            <Textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className="h-full min-h-[500px] font-mono text-sm leading-relaxed"
              placeholder="输入 Markdown 内容…"
            />
            <div className="flex justify-between items-center mt-3">
              <span className="text-xs text-[#94a3b8]">
                {content.length} 字符 | 保存后需在"课程管理"页点击"重新加载"才能生效
              </span>
            </div>
          </div>
        )}
      </section>
    </div>
  )
}
