import MarkdownIt from 'markdown-it'
import markdownItKatex from '@traptitech/markdown-it-katex'
import 'katex/dist/katex.min.css'

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })
md.use(markdownItKatex, { throwOnError: false, errorColor: '#cc0000' })

export function useMarkdown() {
  function render(raw: string): string {
    return md.render(raw)
  }

  return { render }
}
