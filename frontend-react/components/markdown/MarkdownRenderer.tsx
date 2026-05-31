// components/markdown/MarkdownRenderer.tsx
'use client'

import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import remarkGfm from 'remark-gfm'
import rehypeKatex from 'rehype-katex'
import type { Components } from 'react-markdown'

function CodeBlock({ className, children, ...props }: React.HTMLAttributes<HTMLElement>) {
  const match = /language-(\w+)/.exec(className || '')
  const language = match ? match[1] : ''
  return (
    <pre className="overflow-auto p-4 rounded-lg bg-slate-900 text-slate-100">
      {language && (
        <div className="text-xs text-slate-400 mb-1">{language}</div>
      )}
      <code className={className} {...props}>
        {children}
      </code>
    </pre>
  )
}

const components: Components = {
  code({ className, children, ...props }) {
    const isInline = !className?.includes('language-')
    if (isInline) {
      return (
        <code
          className="bg-slate-100 text-rose-600 rounded px-1 py-0.5 font-mono text-[0.9em]"
          {...props}
        >
          {children}
        </code>
      )
    }
    return <CodeBlock className={className} {...props}>{children}</CodeBlock>
  },
}

export function MarkdownRenderer({ content }: { content: string }) {
  if (!content) return null
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkMath]}
      rehypePlugins={[rehypeKatex]}
      components={components}
    >
      {content}
    </ReactMarkdown>
  )
}
