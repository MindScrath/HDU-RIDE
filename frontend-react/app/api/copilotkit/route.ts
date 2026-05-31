import {
  CopilotRuntime,
  OpenAIAdapter,
  copilotRuntimeNextJSAppRouterEndpoint,
} from '@copilotkit/runtime'
import OpenAI from 'openai'

const openai = new OpenAI({
  apiKey: process.env.BAILIAN_API_KEY!,
  baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  defaultHeaders: {
    'X-DashScope-AppId': process.env.BAILIAN_APP_ID!,
  },
})

const serviceAdapter = new OpenAIAdapter({
  openai,
  model: 'qwen-plus',
})

const runtime = new CopilotRuntime()

const { handleRequest } = copilotRuntimeNextJSAppRouterEndpoint({
  runtime,
  serviceAdapter,
  endpoint: '/api/copilotkit',
})

export const POST = handleRequest
