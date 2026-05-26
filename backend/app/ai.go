package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxContextMessages = 20 // 最多携带的历史消息数，避免 token 超限
const maxUploadSize = 50 << 20 // 50 MB

// aiMessage 代表一条对话消息（前端传入 / 转发给百炼 API）
type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ── 文件上传：申请租约 + 上传 ────────────────────────────────

// uploadAIFile 是 POST /api/ai/upload 的处理函数
// 前端通过 multipart/form-data 上传文件，后端代理到百炼 apply_upload_lease → PUT 上传
// 返回 { fileId, fileName } 给前端
func (a *App) uploadAIFile(c *gin.Context) {
	if a.cfg.BailianAPIKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI 服务未配置"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择一个文件"})
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件不能超过 50 MB"})
		return
	}

	// ① 申请上传租约
	leaseReq := map[string]string{"file_name": header.Filename}
	leaseBody, _ := json.Marshal(leaseReq)

	leaseHTTP, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		"https://dashscope.aliyuncs.com/api/v2/apps/zhiwen-file/apply_upload_lease",
		bytes.NewReader(leaseBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "构建租约请求失败"})
		return
	}
	leaseHTTP.Header.Set("Content-Type", "application/json")
	leaseHTTP.Header.Set("Authorization", "Bearer "+a.cfg.BailianAPIKey)

	leaseResp, err := http.DefaultClient.Do(leaseHTTP)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "申请上传租约失败: " + err.Error()})
		return
	}
	defer leaseResp.Body.Close()

	var lease struct {
		Data struct {
			FileUploadURL string            `json:"file_upload_url"`
			FileID        string            `json:"file_id"`
			Headers       map[string]string `json:"headers"`
		} `json:"data"`
		Success   bool   `json:"success"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(leaseResp.Body).Decode(&lease); err != nil || !lease.Success {
		msg := "申请上传租约失败"
		if lease.Message != "" {
			msg = lease.Message
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": msg})
		return
	}

	// ② 读取文件内容
	fileData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// ③ PUT 上传到预签名 URL
	putReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPut,
		lease.Data.FileUploadURL, bytes.NewReader(fileData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "构建上传请求失败"})
		return
	}
	// 设置租约返回的 headers
	for k, v := range lease.Data.Headers {
		putReq.Header.Set(k, v)
	}
	if putReq.Header.Get("Content-Type") == "" {
		putReq.Header.Set("Content-Type", "application/octet-stream")
	}

	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "上传文件失败: " + err.Error()})
		return
	}
	defer putResp.Body.Close()

	if putResp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(putResp.Body)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("上传文件返回 %d: %s", putResp.StatusCode, string(errBody))})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fileId":   lease.Data.FileID,
		"fileName": header.Filename,
	})
}

// ── AI 对话 ─────────────────────────────────────────────────

// chatAI 是 POST /api/ai/chat 的处理函数，以 SSE 流式代理阿里云百炼应用
func (a *App) chatAI(c *gin.Context) {
	if a.cfg.BailianAPIKey == "" || a.cfg.BailianAppID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI 服务未配置，请联系管理员"})
		return
	}

	// 解析前端传来的消息列表 + 可选的文件 ID
	var req struct {
		Messages []aiMessage `json:"messages" binding:"required"`
		FileIDs  []string    `json:"fileIds"`
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

	// 取最后一条用户消息作为 prompt
	prompt := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			prompt = messages[i].Content
			break
		}
	}
	if prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no user message found"})
		return
	}

	// 构建百炼应用 API 请求体
	bailianInput := map[string]interface{}{
		"prompt": prompt,
	}

	// 如果有历史消息，构建 history
	if len(messages) > 1 {
		var history []map[string]string
		for i := 0; i < len(messages)-1; i++ {
			m := messages[i]
			if m.Role == "user" {
				pair := map[string]string{"user": m.Content, "bot": ""}
				if i+1 < len(messages)-1 && messages[i+1].Role == "assistant" {
					pair["bot"] = messages[i+1].Content
					i++
				}
				history = append(history, pair)
			}
		}
		if len(history) > 0 {
			bailianInput["history"] = history
		}
	}

	bailianReq := map[string]interface{}{
		"input":      bailianInput,
		"parameters": map[string]interface{}{},
	}

	// 如果有附件，通过 session_files 传入
	if len(req.FileIDs) > 0 {
		var sessionFiles []map[string]string
		for _, fid := range req.FileIDs {
			sessionFiles = append(sessionFiles, map[string]string{
				"file_id": fid,
			})
		}
		params := bailianReq["parameters"].(map[string]interface{})
		params["agent_options"] = map[string]interface{}{
			"session_files": sessionFiles,
		}
	}

	body, err := json.Marshal(bailianReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "request marshal failed"})
		return
	}

	// 构造 HTTP 请求 - 使用百炼应用端点
	endpoint := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/apps/%s/completion", a.cfg.BailianAppID)
	upReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upstream request build failed"})
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+a.cfg.BailianAPIKey)
	upReq.Header.Set("X-DashScope-SSE", "enable")

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

	// 以 SSE 格式透传流给客户端，转换百炼格式为 OpenAI 兼容格式
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writer := c.Writer
	flusher, canFlush := writer.(http.Flusher)

	prevText := "" // 跟踪上一次的累积文本，用于计算增量
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// 百炼 SSE 格式: "data:{ ... }" 或 "event:..." 或 "id:..."
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimPrefix(line, "data:")

		// 解析百炼响应
		var bailianResp struct {
			Output struct {
				Text         string `json:"text"`
				FinishReason string `json:"finish_reason"`
			} `json:"output"`
		}
		if err := json.Unmarshal([]byte(payload), &bailianResp); err != nil {
			continue
		}

		// 计算增量文本（百炼返回累积文本）
		currentText := bailianResp.Output.Text
		delta := ""
		if len(currentText) > len(prevText) {
			delta = currentText[len(prevText):]
		}
		prevText = currentText

		if delta == "" && bailianResp.Output.FinishReason != "stop" {
			continue
		}

		// 转换为 OpenAI SSE 格式，前端解析 chunk.choices[0].delta.content
		if bailianResp.Output.FinishReason == "stop" {
			if delta != "" {
				chunk := map[string]interface{}{
					"choices": []map[string]interface{}{
						{"delta": map[string]string{"content": delta}},
					},
				}
				chunkJSON, _ := json.Marshal(chunk)
				fmt.Fprintf(writer, "data: %s\n\n", chunkJSON)
				if canFlush {
					flusher.Flush()
				}
			}
			fmt.Fprintf(writer, "data: [DONE]\n\n")
			if canFlush {
				flusher.Flush()
			}
			break
		}

		chunk := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"delta": map[string]string{"content": delta}},
			},
		}
		chunkJSON, _ := json.Marshal(chunk)
		fmt.Fprintf(writer, "data: %s\n\n", chunkJSON)
		if canFlush {
			flusher.Flush()
		}
	}
}
