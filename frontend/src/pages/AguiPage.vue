<script setup lang="ts">
import { ref, reactive, nextTick, computed } from 'vue'
import { ChatDotRound, Delete, Plus, CopyDocument, Promotion } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useSession } from '../composables/useSession'

const session = useSession()

// ── 数据类型 ──────────────────────────────────────────────
interface Message {
  role: 'user' | 'assistant'
  content: string
  ts: number
}
interface Conversation {
  id: string
  title: string
  messages: Message[]
  createdAt: number
}

// ── 会话状态（localStorage 持久化）────────────────────────
const STORAGE_KEY = 'hdu_ride_agui_conversations'

function loadConversations(): Conversation[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
  } catch {
    return []
  }
}
function saveConversations() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(conversations))
}

const conversations = reactive<Conversation[]>(loadConversations())
const activeId = ref<string | null>(conversations[0]?.id ?? null)

const activeConv = computed(() => conversations.find(c => c.id === activeId.value) ?? null)

function newConversation() {
  const id = crypto.randomUUID()
  conversations.unshift({ id, title: '新对话', messages: [], createdAt: Date.now() })
  activeId.value = id
  saveConversations()
}

function deleteConversation(id: string) {
  const idx = conversations.findIndex(c => c.id === id)
  if (idx !== -1) conversations.splice(idx, 1)
  if (activeId.value === id) activeId.value = conversations[0]?.id ?? null
  saveConversations()
}

function selectConversation(id: string) {
  activeId.value = id
}

// ── 输入与发送 ─────────────────────────────────────────────
const inputText = ref('')
const isStreaming = ref(false)
const messagesEl = ref<HTMLElement | null>(null)

function scrollToBottom() {
  nextTick(() => {
    if (messagesEl.value) messagesEl.value.scrollTop = messagesEl.value.scrollHeight
  })
}

function formatTime(ts: number) {
  return new Date(ts).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function autoTitle(msg: string) {
  return msg.slice(0, 18).replace(/\n/g, ' ') + (msg.length > 18 ? '…' : '')
}

async function send() {
  const text = inputText.value.trim()
  if (!text || isStreaming.value) return

  // 如果没有激活会话，先建一个
  if (!activeConv.value) newConversation()
  const conv = activeConv.value!

  // 更新标题（第一条消息）
  if (conv.messages.length === 0) conv.title = autoTitle(text)

  // 添加用户消息
  conv.messages.push({ role: 'user', content: text, ts: Date.now() })
  inputText.value = ''
  scrollToBottom()
  saveConversations()

  // 占位 assistant 消息
  const assistantMsg: Message = { role: 'assistant', content: '', ts: Date.now() }
  conv.messages.push(assistantMsg)
  isStreaming.value = true

  try {
    const resp = await fetch('/api/ai/chat', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        messages: conv.messages
          .slice(0, -1) // 不含占位 assistant
          .map(m => ({ role: m.role, content: m.content }))
      })
    })

    if (!resp.ok) {
      const err = await resp.json().catch(() => ({ error: '请求失败' }))
      throw new Error(err.error ?? '请求失败')
    }

    const reader = resp.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() ?? ''
      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed || !trimmed.startsWith('data:')) continue
        const payload = trimmed.slice(5).trim()
        if (payload === '[DONE]') continue
        try {
          const chunk = JSON.parse(payload)
          const delta = chunk.choices?.[0]?.delta?.content ?? ''
          if (delta) {
            assistantMsg.content += delta
            scrollToBottom()
          }
        } catch { /* 跳过非 JSON 行 */ }
      }
    }
  } catch (e: any) {
    assistantMsg.content = `⚠️ ${e.message ?? '网络错误，请重试'}`
  } finally {
    isStreaming.value = false
    saveConversations()
    scrollToBottom()
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

async function copyMessage(content: string) {
  await navigator.clipboard.writeText(content)
  ElMessage.success('已复制')
}

// 推荐问题（欢迎屏）
const suggestions = [
  '帮我分析这段 R 代码的问题',
  '什么是线性回归？请用简单语言解释',
  '如何在 RStudio 中安装并加载 ggplot2？',
  '请帮我写一个数据清洗的 R 脚本模板',
]

function useSuggestion(s: string) {
  if (!activeConv.value) newConversation()
  inputText.value = s
  send()
}

// Markdown：简单处理代码块 + 换行（避免引入重依赖）
function renderMarkdown(text: string): string {
  return text
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) =>
      `<pre class="agui-code-block"><code class="language-${lang || 'text'}">${code.trimEnd()}</code></pre>`)
    .replace(/`([^`]+)`/g, '<code class="agui-inline-code">$1</code>')
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    .replace(/\n/g, '<br>')
}
</script>

<template>
  <div class="agui-shell">
    <!-- 左侧会话列表 -->
    <aside class="agui-sidebar">
      <div class="agui-sidebar-head">
        <span class="agui-sidebar-title">
          <el-icon><ChatDotRound /></el-icon>AI 助手
        </span>
        <button class="agui-new-btn" title="新建对话" @click="newConversation">
          <el-icon><Plus /></el-icon>
        </button>
      </div>

      <div class="agui-conv-list">
        <div
          v-for="conv in conversations"
          :key="conv.id"
          class="agui-conv-item"
          :class="{ active: conv.id === activeId }"
          @click="selectConversation(conv.id)"
        >
          <span class="agui-conv-title">{{ conv.title }}</span>
          <button
            class="agui-conv-del"
            title="删除"
            @click.stop="deleteConversation(conv.id)"
          >
            <el-icon><Delete /></el-icon>
          </button>
        </div>
        <div v-if="conversations.length === 0" class="agui-conv-empty">
          暂无对话，点击 + 新建
        </div>
      </div>
    </aside>

    <!-- 右侧聊天区 -->
    <main class="agui-main">
      <!-- 欢迎屏 -->
      <div v-if="!activeConv || activeConv.messages.length === 0" class="agui-welcome">
        <div class="agui-welcome-icon">✦</div>
        <h2 class="agui-welcome-title">你好，{{ session.state.user?.displayName ?? '同学' }}</h2>
        <p class="agui-welcome-sub">我是 HDU-RIDE AI 助手，基于通义千问。有什么可以帮你？</p>
        <div class="agui-suggestions">
          <button
            v-for="s in suggestions"
            :key="s"
            class="agui-suggestion-btn"
            @click="useSuggestion(s)"
          >
            {{ s }}
          </button>
        </div>
      </div>

      <!-- 消息区 -->
      <div v-else ref="messagesEl" class="agui-messages">
        <div
          v-for="(msg, idx) in activeConv.messages"
          :key="idx"
          class="agui-msg-row"
          :class="msg.role"
        >
          <!-- 头像 -->
          <div class="agui-avatar" :class="msg.role">
            <span v-if="msg.role === 'user'">
              {{ session.state.user?.displayName?.[0]?.toUpperCase() ?? 'U' }}
            </span>
            <span v-else>✦</span>
          </div>

          <div class="agui-bubble-wrap">
            <!-- 气泡 -->
            <div class="agui-bubble" :class="msg.role">
              <!-- 流式打字光标 -->
              <span
                v-if="msg.role === 'assistant' && idx === activeConv.messages.length - 1 && isStreaming && !msg.content"
                class="agui-typing"
              >
                <span /><span /><span />
              </span>
              <span
                v-else-if="msg.role === 'assistant'"
                v-html="renderMarkdown(msg.content)"
              />
              <span v-else>{{ msg.content }}</span>
            </div>
            <!-- 工具栏 -->
            <div class="agui-msg-meta">
              <span class="agui-msg-time">{{ formatTime(msg.ts) }}</span>
              <button
                v-if="msg.role === 'assistant' && msg.content"
                class="agui-copy-btn"
                title="复制"
                @click="copyMessage(msg.content)"
              >
                <el-icon><CopyDocument /></el-icon>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- 输入框 -->
      <div class="agui-input-area">
        <textarea
          v-model="inputText"
          class="agui-textarea"
          placeholder="输入消息…（Enter 发送，Shift+Enter 换行）"
          :disabled="isStreaming"
          rows="1"
          @keydown="onKeydown"
          @input="(e: Event) => {
            const el = e.target as HTMLTextAreaElement
            el.style.height = 'auto'
            el.style.height = Math.min(el.scrollHeight, 180) + 'px'
          }"
        />
        <button
          class="agui-send-btn"
          :class="{ active: inputText.trim() && !isStreaming }"
          :disabled="!inputText.trim() || isStreaming"
          @click="send"
        >
          <el-icon><Promotion /></el-icon>
        </button>
      </div>
      <p class="agui-disclaimer">AI 生成内容仅供参考，请自行核实关键信息</p>
    </main>
  </div>
</template>

<style scoped>
/* ── 整体布局 ─────────────────────────────────────────────── */
.agui-shell {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  height: calc(100vh - 94px);
  background: #f7f9fc;
  border-radius: 10px;
  overflow: hidden;
  box-shadow: 0 2px 16px rgba(30,41,59,.07);
}

/* ── 左侧会话列表 ─────────────────────────────────────────── */
.agui-sidebar {
  display: flex;
  flex-direction: column;
  background: #fff;
  border-right: 1px solid #e8edf5;
}

.agui-sidebar-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 12px 10px;
  border-bottom: 1px solid #edf1f7;
}

.agui-sidebar-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 650;
  font-size: 14px;
  color: #1a2236;
}

.agui-new-btn {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: 1px solid #d4dcea;
  border-radius: 6px;
  background: transparent;
  color: #5a6782;
  cursor: pointer;
  transition: background .15s, color .15s;
}
.agui-new-btn:hover { background: #eaf2ff; color: #0b5ed7; border-color: #b0caed; }

.agui-conv-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 8px;
}

.agui-conv-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  border-radius: 7px;
  cursor: pointer;
  transition: background .12s;
  margin-bottom: 2px;
}
.agui-conv-item:hover { background: #f0f4fb; }
.agui-conv-item.active { background: #eaf2ff; }

.agui-conv-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: #2d3a50;
}
.agui-conv-item.active .agui-conv-title { color: #0b5ed7; font-weight: 560; }

.agui-conv-del {
  opacity: 0;
  border: 0;
  background: transparent;
  color: #9aacbf;
  cursor: pointer;
  padding: 2px;
  border-radius: 4px;
  transition: opacity .12s, color .12s;
  display: grid;
  place-items: center;
}
.agui-conv-item:hover .agui-conv-del { opacity: 1; }
.agui-conv-del:hover { color: #ef4444; }

.agui-conv-empty {
  color: #9aacbf;
  font-size: 12px;
  text-align: center;
  margin-top: 32px;
}

/* ── 右侧聊天区 ───────────────────────────────────────────── */
.agui-main {
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: #f7f9fc;
}

/* 欢迎屏 */
.agui-welcome {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 32px;
  text-align: center;
}

.agui-welcome-icon {
  font-size: 48px;
  line-height: 1;
  background: linear-gradient(135deg, #0b5ed7 0%, #7c3aed 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin-bottom: 20px;
}

.agui-welcome-title {
  margin: 0 0 8px;
  font-size: 26px;
  font-weight: 700;
  color: #111827;
}

.agui-welcome-sub {
  margin: 0 0 32px;
  color: #6b7280;
  font-size: 15px;
}

.agui-suggestions {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
  max-width: 540px;
  width: 100%;
}

.agui-suggestion-btn {
  padding: 12px 16px;
  border: 1px solid #d4dcea;
  border-radius: 10px;
  background: #fff;
  color: #374151;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: border-color .15s, box-shadow .15s, background .15s;
  line-height: 1.5;
}
.agui-suggestion-btn:hover {
  border-color: #0b5ed7;
  background: #f0f6ff;
  box-shadow: 0 2px 8px rgba(11,94,215,.1);
}

/* 消息列表 */
.agui-messages {
  flex: 1;
  overflow-y: auto;
  padding: 24px 20px 12px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.agui-msg-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.agui-msg-row.user { flex-direction: row-reverse; }

/* 头像 */
.agui-avatar {
  flex: 0 0 36px;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 14px;
  font-weight: 700;
  user-select: none;
}
.agui-avatar.user {
  background: linear-gradient(135deg, #0b5ed7, #2563eb);
  color: #fff;
}
.agui-avatar.assistant {
  background: linear-gradient(135deg, #7c3aed, #0b5ed7);
  color: #fff;
  font-size: 18px;
}

/* 气泡 */
.agui-bubble-wrap { display: flex; flex-direction: column; max-width: 72%; }
.agui-msg-row.user .agui-bubble-wrap { align-items: flex-end; }

.agui-bubble {
  padding: 12px 16px;
  border-radius: 14px;
  font-size: 14px;
  line-height: 1.7;
  word-break: break-word;
}
.agui-bubble.user {
  background: linear-gradient(135deg, #0b5ed7, #2563eb);
  color: #fff;
  border-bottom-right-radius: 4px;
}
.agui-bubble.assistant {
  background: #fff;
  color: #1f2937;
  border: 1px solid #e8edf5;
  box-shadow: 0 1px 6px rgba(30,41,59,.06);
  border-bottom-left-radius: 4px;
}

.agui-msg-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  padding: 0 2px;
}
.agui-msg-row.user .agui-msg-meta { flex-direction: row-reverse; }

.agui-msg-time { font-size: 11px; color: #9ca3af; }
.agui-copy-btn {
  border: 0;
  background: transparent;
  color: #9ca3af;
  cursor: pointer;
  padding: 1px 3px;
  border-radius: 4px;
  display: grid;
  place-items: center;
  font-size: 13px;
  transition: color .12s;
}
.agui-copy-btn:hover { color: #0b5ed7; }

/* 打字动画 */
.agui-typing {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  height: 20px;
}
.agui-typing span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #0b5ed7;
  animation: agui-bounce 1.2s infinite ease-in-out;
}
.agui-typing span:nth-child(2) { animation-delay: .2s; }
.agui-typing span:nth-child(3) { animation-delay: .4s; }
@keyframes agui-bounce {
  0%, 80%, 100% { transform: scale(.7); opacity: .5; }
  40% { transform: scale(1); opacity: 1; }
}

/* 代码块 */
:deep(.agui-code-block) {
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  padding: 14px 16px;
  overflow-x: auto;
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 13px;
  line-height: 1.6;
  margin: 8px 0;
}
:deep(.agui-inline-code) {
  background: #f1f5f9;
  color: #e11d48;
  border-radius: 4px;
  padding: 1px 5px;
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 0.9em;
}

/* 输入区 */
.agui-input-area {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  margin: 0 16px 4px;
  padding: 12px 14px;
  background: #fff;
  border: 1px solid #dde4ee;
  border-radius: 14px;
  box-shadow: 0 2px 12px rgba(30,41,59,.07);
}

.agui-textarea {
  flex: 1;
  border: 0;
  outline: none;
  resize: none;
  font: inherit;
  font-size: 14px;
  color: #1f2937;
  background: transparent;
  min-height: 24px;
  max-height: 180px;
  line-height: 1.6;
}
.agui-textarea::placeholder { color: #b0bcc8; }
.agui-textarea:disabled { opacity: .6; }

.agui-send-btn {
  flex: 0 0 36px;
  width: 36px;
  height: 36px;
  border-radius: 10px;
  border: 0;
  background: #d4dcea;
  color: #8a97a8;
  cursor: not-allowed;
  display: grid;
  place-items: center;
  font-size: 18px;
  transition: background .15s, color .15s, transform .1s;
}
.agui-send-btn.active {
  background: linear-gradient(135deg, #0b5ed7, #2563eb);
  color: #fff;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(11,94,215,.3);
}
.agui-send-btn.active:hover { transform: scale(1.05); }
.agui-send-btn.active:active { transform: scale(.96); }

.agui-disclaimer {
  text-align: center;
  font-size: 11px;
  color: #b0bcc8;
  margin: 4px 0 10px;
}
</style>
