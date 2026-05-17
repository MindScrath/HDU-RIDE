package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const bailianEndpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
const bailianModel = "qwen-plus"
const maxContextMessages = 20 // 最多携带的历史消息数，避免 token 超限

// aiMessage 代表一条对话消息（前端传入 / 转发给百炼 API）
type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// aiBailianRequest 是发往百炼 OpenAI 兼容端点的请求体
type aiBailianRequest struct {
	Model    string      `json:"model"`
	Messages []aiMessage `json:"messages"`
	Stream   bool        `json:"stream"`
}

// chatAI 是 POST /api/ai/chat 的处理函数，以 SSE 流式代理阿里云百炼大模型
func (a *App) chatAI(c *gin.Context) {
	if a.cfg.BailianAPIKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI 服务未配置，请联系管理员"})
		return
	}

	// 解析前端传来的消息列表
	var req struct {
		Messages []aiMessage `json:"messages" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: messages required"})
		return
	}

	// 截取最近 maxContextMessages 条，防止超限
	messages := req.Messages
	if len(messages) > maxContextMessages {
		messages = messages[len(messages)-maxContextMessages:]
	}

	// 构造发往百炼的请求体
	upstream := aiBailianRequest{
		Model:    bailianModel,
		Messages: messages,
		Stream:   true,
	}
	body, err := json.Marshal(upstream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "request marshal failed"})
		return
	}

	// 构造 HTTP 请求
	upReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, bailianEndpoint, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upstream request build failed"})
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+a.cfg.BailianAPIKey)
	if a.cfg.BailianAppID != "" {
		upReq.Header.Set("X-DashScope-AppId", a.cfg.BailianAppID)
	}

	// 发起请求
	resp, err := http.DefaultClient.Do(upReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("upstream error %d: %s", resp.StatusCode, string(errBody))})
		return
	}

	// 以 SSE 格式透传流给客户端
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writer := c.Writer
	flusher, canFlush := writer.(http.Flusher)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fmt.Fprintf(writer, "%s\n\n", line)
		if canFlush {
			flusher.Flush()
		}
	}
}
