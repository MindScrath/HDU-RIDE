'use client'

import { CopilotKit } from '@copilotkit/react-core'
import { CopilotChat } from '@copilotkit/react-ui'
import '@copilotkit/react-ui/styles.css'

export default function AguiPage() {
  return (
    <div className="h-[calc(100vh-94px)] bg-[#f7f9fc] rounded-lg overflow-hidden shadow-sm">
      <CopilotKit runtimeUrl="/api/copilotkit">
        <CopilotChat
          labels={{
            title: 'AI 助手',
            initial: '你好！我是 HDU-RIDE AI 助手，基于通义千问。有什么可以帮你？',
            placeholder: '输入消息…（Enter 发送）',
          }}
          className="h-full"
        />
      </CopilotKit>
    </div>
  )
}
