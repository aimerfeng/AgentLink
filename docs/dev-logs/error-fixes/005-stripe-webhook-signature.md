# 错误修复 005: Stripe Webhook 签名验证失败

**日期**: 2024-12-28  
**严重程度**: 高  
**状态**: ✅ 已修复

## 问题描述

Stripe Webhook 请求签名验证失败，导致支付回调无法处理。

## 错误信息

```
stripe webhook signature verification failed: 
signature is invalid or timestamp is too old
```

## 根本原因

1. Webhook Secret 配置错误
2. 请求体被中间件修改
3. 时间戳验证失败（时钟偏移）
4. 本地开发未使用 Stripe CLI

## 解决方案

### 1. 正确读取原始请求体

```go
// backend/internal/server/api.go
func (s *APIServer) handleStripeWebhook(c *gin.Context) {
    // 读取原始请求体（不能使用 c.Bind）
    payload, err := io.ReadAll(c.Request.Body)
    if err != nil {
        c.JSON(400, gin.H{"error": "failed to read request body"})
        return
    }
    
    // 获取签名头
    signature := c.GetHeader("Stripe-Signature")
    if signature == "" {
        c.JSON(400, gin.H{"error": "missing stripe signature"})
        return
    }
    
    // 验证签名
    err = s.paymentService.HandleStripeWebhook(c.Request.Context(), payload, signature)
    if err != nil {
        log.Error().Err(err).Msg("webhook processing failed")
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"received": true})
}
```

### 2. 禁用请求体解析中间件

```go
// Webhook 路由不使用 JSON 解析中间件
webhookGroup := router.Group("/webhooks")
{
    // 不使用 gin.BindJSON
    webhookGroup.POST("/stripe", s.handleStripeWebhook)
    webhookGroup.POST("/coinbase", s.handleCoinbaseWebhook)
}
```

### 3. 本地开发使用 Stripe CLI

```bash
# 安装 Stripe CLI
# Windows
scoop install stripe

# macOS
brew install stripe/stripe-cli/stripe

# 登录
stripe login

# 转发 Webhook 到本地
stripe listen --forward-to localhost:8080/webhooks/stripe

# 输出 Webhook Secret
> Ready! Your webhook signing secret is whsec_xxxxx

# 设置环境变量
export STRIPE_WEBHOOK_SECRET=whsec_xxxxx
```

### 4. 增加时间戳容差

```go
// backend/internal/payment/payment.go
func (s *PaymentService) HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
    // 增加时间戳容差（默认 300 秒）
    event, err := webhook.ConstructEventWithOptions(
        payload, 
        signature, 
        s.config.WebhookSecret,
        webhook.ConstructEventOptions{
            Tolerance: 600, // 10 分钟容差
        },
    )
    if err != nil {
        return fmt.Errorf("webhook signature verification failed: %w", err)
    }
    
    // ... 处理事件 ...
}
```

### 5. 配置检查

```go
// 启动时验证配置
func (s *PaymentService) validateConfig() error {
    if s.config.WebhookSecret == "" {
        return errors.New("STRIPE_WEBHOOK_SECRET is required")
    }
    
    if !strings.HasPrefix(s.config.WebhookSecret, "whsec_") {
        return errors.New("invalid STRIPE_WEBHOOK_SECRET format")
    }
    
    return nil
}
```

## 验证

```bash
# 使用 Stripe CLI 触发测试事件
stripe trigger checkout.session.completed

# 检查日志
docker-compose logs -f api | grep webhook
```

## 预防措施

1. 使用 Stripe CLI 进行本地开发
2. 正确读取原始请求体
3. 配置合理的时间戳容差
4. 验证 Webhook Secret 格式
