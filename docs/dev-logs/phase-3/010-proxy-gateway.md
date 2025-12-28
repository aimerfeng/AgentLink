# 开发日志 010: Proxy Gateway 核心实现

**日期**: 2024-12-28  
**任务**: Task 12 - 实现 Proxy Gateway 核心  
**状态**: ✅ 已完成

## 任务描述

实现 AI 代理网关，处理 API 调用、配额管理、System Prompt 注入和流式响应。

## 实现内容

### 1. 请求处理流程

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│   Gateway   │────▶│  AI Provider│
└─────────────┘     └─────────────┘     └─────────────┘
                          │
                    ┌─────┴─────┐
                    │           │
              ┌─────▼─────┐ ┌───▼───┐
              │ Validate  │ │ Quota │
              │ API Key   │ │ Check │
              └───────────┘ └───────┘
```

### 2. API 接口

```
POST /proxy/v1/agents/:agentId/chat

Headers:
  X-AgentLink-Key: ak_xxxxxxxx...
  Content-Type: application/json

Request:
{
    "messages": [
        {"role": "user", "content": "Hello"}
    ],
    "stream": false
}

Response (non-streaming):
{
    "id": "chatcmpl-xxx",
    "choices": [
        {
            "message": {
                "role": "assistant",
                "content": "Hello! How can I help you?"
            }
        }
    ],
    "usage": {
        "prompt_tokens": 10,
        "completion_tokens": 8,
        "total_tokens": 18
    }
}
```

### 3. 核心处理逻辑

```go
func (p *ProxyService) ProcessChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 验证 API Key
    apiKey, err := p.validateAPIKey(ctx, req.APIKey)
    if err != nil {
        return nil, err
    }
    
    // 2. 获取 Agent
    agent, err := p.getAgent(ctx, req.AgentID)
    if err != nil {
        return nil, err
    }
    
    // 3. 检查 Agent 状态
    if agent.Status != "active" {
        return nil, ErrAgentNotActive
    }
    
    // 4. 检查配额
    if err := p.checkQuota(ctx, apiKey.UserID); err != nil {
        return nil, err
    }
    
    // 5. 检查限速
    if err := p.checkRateLimit(ctx, apiKey.UserID); err != nil {
        return nil, err
    }
    
    // 6. 解密 Agent 配置
    config, err := p.decryptConfig(agent)
    if err != nil {
        return nil, err
    }
    
    // 7. 注入 System Prompt
    messages := p.injectSystemPrompt(config.SystemPrompt, req.Messages)
    
    // 8. 调用上游 AI
    resp, err := p.callUpstream(ctx, config, messages)
    if err != nil {
        return nil, err
    }
    
    // 9. 扣减配额
    if err := p.deductQuota(ctx, apiKey.UserID); err != nil {
        // 记录但不影响响应
        log.Error().Err(err).Msg("failed to deduct quota")
    }
    
    // 10. 记录调用日志
    p.logCall(ctx, agent, apiKey, req, resp)
    
    return resp, nil
}
```

### 4. System Prompt 注入

```go
func (p *ProxyService) injectSystemPrompt(systemPrompt string, messages []Message) []Message {
    result := make([]Message, 0, len(messages)+1)
    
    // 添加 System Prompt 到首位
    result = append(result, Message{
        Role:    "system",
        Content: systemPrompt,
    })
    
    // 过滤用户提交的 system 消息
    for _, msg := range messages {
        if msg.Role != "system" {
            result = append(result, msg)
        }
    }
    
    return result
}
```

### 5. 流式响应 (SSE)

```go
func (p *ProxyService) ProcessChatStream(ctx context.Context, req *ChatRequest, w http.ResponseWriter) error {
    // 设置 SSE 头
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    flusher, ok := w.(http.Flusher)
    if !ok {
        return ErrStreamingNotSupported
    }
    
    // 调用上游（流式）
    stream, err := p.callUpstreamStream(ctx, config, messages)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    // 转发流式响应
    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        
        // 发送 SSE 事件
        data, _ := json.Marshal(chunk)
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush()
    }
    
    // 发送结束标记
    fmt.Fprintf(w, "data: [DONE]\n\n")
    flusher.Flush()
    
    return nil
}
```

## 遇到的问题

### 问题 1: 配额扣减竞态条件

**描述**: 并发请求可能导致配额超扣

**解决方案**:
使用 Redis Lua 脚本保证原子性

```lua
-- quota_deduct.lua
local key = KEYS[1]
local current = tonumber(redis.call('GET', key) or '0')
local limit = tonumber(ARGV[1])

if current >= limit then
    return -1  -- 配额不足
end

return redis.call('INCR', key)
```

### 问题 2: 上游超时处理

**描述**: AI 提供商响应慢导致请求堆积

**解决方案**:
设置请求超时和熔断器

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

resp, err := p.callUpstream(ctx, config, messages)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return nil, ErrUpstreamTimeout
    }
    return nil, err
}
```

### 问题 3: System Prompt 泄露

**描述**: 响应中可能包含 System Prompt

**解决方案**:
过滤响应中的敏感内容

```go
func (p *ProxyService) sanitizeResponse(resp *ChatResponse, systemPrompt string) {
    for i := range resp.Choices {
        content := resp.Choices[i].Message.Content
        // 检测并移除 System Prompt
        if strings.Contains(content, systemPrompt) {
            resp.Choices[i].Message.Content = strings.ReplaceAll(
                content, systemPrompt, "[REDACTED]",
            )
        }
    }
}
```

### 问题 4: 流式响应中断

**描述**: 客户端断开连接后继续处理

**解决方案**:
监听 context 取消

```go
select {
case <-ctx.Done():
    return ctx.Err()
case chunk := <-stream:
    // 处理 chunk
}
```

## 验证结果

- [x] API Key 验证正确
- [x] 配额检查和扣减正确
- [x] System Prompt 正确注入
- [x] 流式响应正常工作
- [x] 错误处理完善

## 属性测试

**Property 1: Prompt Security**
- System Prompt 注入到请求首位 ✅
- 用户 system 消息被过滤 ✅
- 响应中 System Prompt 被脱敏 ✅

**Property 3: Quota Consistency**
- 成功调用精确扣减一次 ✅
- 并发访问下配额一致 ✅

## 相关文件

- `backend/internal/proxy/proxy.go`
- `backend/internal/proxy/streaming.go`
- `backend/internal/proxy/prompt.go`
- `backend/internal/proxy/quota.go`
- `backend/internal/proxy/proxy_property_test.go`
