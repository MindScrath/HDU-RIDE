// 百炼 App API → OpenAI 兼容 SSE 代理
// 前端 CopilotChat 发送 OpenAI 格式请求，此路由转换为百炼 App API 格式

export async function POST(req: Request) {
  const apiKey = process.env.BAILIAN_API_KEY
  const appId = process.env.BAILIAN_APP_ID

  if (!apiKey || !appId) {
    return new Response(JSON.stringify({ error: 'AI 服务未配置' }), {
      status: 503,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  // 解析 CopilotKit 发来的 OpenAI 格式消息
  const body = await req.json().catch(() => null)
  const messages: { role: string; content: string }[] = body?.messages ?? []

  if (messages.length === 0) {
    return new Response(JSON.stringify({ error: 'messages required' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  // 取最后一条 user 消息作为 prompt
  let prompt = ''
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === 'user') {
      prompt = messages[i].content
      break
    }
  }
  if (!prompt) {
    return new Response(JSON.stringify({ error: 'no user message found' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  // 构建 history（user/bot 对）
  const history: { user: string; bot: string }[] = []
  for (let i = 0; i < messages.length - 1; i++) {
    if (messages[i].role === 'user') {
      const pair: { user: string; bot: string } = { user: messages[i].content, bot: '' }
      if (i + 1 < messages.length && messages[i + 1].role === 'assistant') {
        pair.bot = messages[i + 1].content
        i++
      }
      history.push(pair)
    }
  }

  // 调用百炼 App API
  const bailianBody = JSON.stringify({
    input: {
      prompt,
      ...(history.length > 0 ? { history } : {}),
    },
    parameters: {},
  })

  const upstream = await fetch(
    `https://dashscope.aliyuncs.com/api/v1/apps/${appId}/completion`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${apiKey}`,
        'X-DashScope-SSE': 'enable',
      },
      body: bailianBody,
    }
  )

  if (!upstream.ok || !upstream.body) {
    const errText = await upstream.text().catch(() => 'unknown')
    return new Response(JSON.stringify({ error: `百炼 API 错误 ${upstream.status}: ${errText}` }), {
      status: 502,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  // 流式转换：百炼 SSE → OpenAI 兼容 SSE
  const encoder = new TextEncoder()

  const stream = new ReadableStream({
    async start(controller) {
      const reader = upstream.body!.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let prevText = ''

      try {
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop() ?? ''

          for (const line of lines) {
            const trimmed = line.trim()
            if (!trimmed.startsWith('data:')) continue
            const payload = trimmed.slice(5).trim()

            let parsed: { output?: { text?: string; finish_reason?: string } }
            try {
              parsed = JSON.parse(payload)
            } catch {
              continue
            }

            const currentText = parsed.output?.text ?? ''
            const delta = currentText.slice(prevText.length)
            prevText = currentText

            if (!delta && parsed.output?.finish_reason !== 'stop') continue

            // 转为 OpenAI SSE 格式
            const chunk = {
              choices: [{ delta: { content: delta } }],
            }
            controller.enqueue(encoder.encode(`data: ${JSON.stringify(chunk)}\n\n`))

            if (parsed.output?.finish_reason === 'stop') {
              controller.enqueue(encoder.encode('data: [DONE]\n\n'))
              controller.close()
              return
            }
          }
        }
        // 流结束但没收到 stop
        controller.enqueue(encoder.encode('data: [DONE]\n\n'))
        controller.close()
      } catch {
        controller.close()
      }
    },
  })

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      Connection: 'keep-alive',
    },
  })
}
