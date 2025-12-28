# 开发日志 013: Stripe 支付集成

**日期**: 2024-12-28  
**任务**: Task 19 - 实现 Stripe 支付集成  
**状态**: ✅ 已完成

## 任务描述

集成 Stripe 支付，支持用户购买 API 调用配额。

## 实现内容

### 1. Stripe Checkout Session

```go
import "github.com/stripe/stripe-go/v76"

func (s *PaymentService) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, plan string) (*CheckoutSession, error) {
    // 获取价格配置
    priceConfig := s.getPriceConfig(plan)
    if priceConfig == nil {
        return nil, ErrInvalidPlan
    }
    
    // 创建 Stripe Checkout Session
    params := &stripe.CheckoutSessionParams{
        Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
        LineItems: []*stripe.CheckoutSessionLineItemParams{
            {
                PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
                    Currency: stripe.String("usd"),
                    ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
                        Name:        stripe.String(priceConfig.Name),
                        Description: stripe.String(priceConfig.Description),
                    },
                    UnitAmount: stripe.Int64(priceConfig.Amount), // 分为单位
                },
                Quantity: stripe.Int64(1),
            },
        },
        SuccessURL: stripe.String(s.config.SuccessURL + "?session_id={CHECKOUT_SESSION_ID}"),
        CancelURL:  stripe.String(s.config.CancelURL),
        Metadata: map[string]string{
            "user_id": userID.String(),
            "plan":    plan,
            "quota":   fmt.Sprintf("%d", priceConfig.Quota),
        },
    }
    
    session, err := session.New(params)
    if err != nil {
        return nil, err
    }
    
    // 保存支付记录
    payment := &Payment{
        UserID:          userID,
        StripeSessionID: session.ID,
        Amount:          decimal.NewFromInt(priceConfig.Amount).Div(decimal.NewFromInt(100)),
        Currency:        "USD",
        Status:          "pending",
        Plan:            plan,
        Quota:           priceConfig.Quota,
    }
    
    if err := s.savePayment(ctx, payment); err != nil {
        return nil, err
    }
    
    return &CheckoutSession{
        SessionID: session.ID,
        URL:       session.URL,
    }, nil
}
```

### 2. Webhook 处理

```go
func (s *PaymentService) HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
    // 验证签名
    event, err := webhook.ConstructEvent(payload, signature, s.config.WebhookSecret)
    if err != nil {
        return ErrInvalidWebhookSignature
    }
    
    switch event.Type {
    case "checkout.session.completed":
        return s.handleCheckoutCompleted(ctx, event)
    case "checkout.session.expired":
        return s.handleCheckoutExpired(ctx, event)
    case "payment_intent.payment_failed":
        return s.handlePaymentFailed(ctx, event)
    default:
        log.Debug().Str("type", event.Type).Msg("unhandled webhook event")
    }
    
    return nil
}

func (s *PaymentService) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
    var session stripe.CheckoutSession
    if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
        return err
    }
    
    // 获取元数据
    userID, _ := uuid.Parse(session.Metadata["user_id"])
    quota, _ := strconv.Atoi(session.Metadata["quota"])
    
    // 开始事务
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)
    
    // 更新支付状态
    _, err = tx.Exec(ctx, `
        UPDATE payments 
        SET status = 'completed', completed_at = NOW() 
        WHERE stripe_session_id = $1
    `, session.ID)
    if err != nil {
        return err
    }
    
    // 增加用户配额
    _, err = tx.Exec(ctx, `
        UPDATE quotas 
        SET total_quota = total_quota + $1, updated_at = NOW() 
        WHERE user_id = $2
    `, quota, userID)
    if err != nil {
        return err
    }
    
    return tx.Commit(ctx)
}
```

### 3. 价格配置

```go
var pricePlans = map[string]*PricePlan{
    "starter": {
        Name:        "Starter Pack",
        Description: "100 API calls",
        Amount:      999,  // $9.99
        Quota:       100,
    },
    "pro": {
        Name:        "Pro Pack",
        Description: "1000 API calls",
        Amount:      4999, // $49.99
        Quota:       1000,
    },
    "enterprise": {
        Name:        "Enterprise Pack",
        Description: "10000 API calls",
        Amount:      29999, // $299.99
        Quota:       10000,
    },
}
```

## 遇到的问题

### 问题 1: Webhook 签名验证失败

**描述**: 本地开发时 Webhook 签名验证失败

**错误信息**:
```
webhook signature verification failed
```

**解决方案**:
使用 Stripe CLI 转发 Webhook

```bash
# 安装 Stripe CLI
# Windows
scoop install stripe

# 登录
stripe login

# 转发 Webhook
stripe listen --forward-to localhost:8080/api/v1/payments/webhook

# 获取 Webhook Secret
# 输出: whsec_xxxxx
```

### 问题 2: 重复处理 Webhook

**描述**: 同一事件被处理多次

**解决方案**:
使用幂等键防止重复处理

```go
func (s *PaymentService) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
    // 检查是否已处理
    var exists bool
    err := s.db.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM webhook_events WHERE event_id = $1)
    `, event.ID).Scan(&exists)
    
    if err != nil {
        return err
    }
    if exists {
        log.Debug().Str("event_id", event.ID).Msg("webhook already processed")
        return nil
    }
    
    // 记录事件
    _, err = s.db.Exec(ctx, `
        INSERT INTO webhook_events (event_id, event_type, processed_at)
        VALUES ($1, $2, NOW())
    `, event.ID, event.Type)
    if err != nil {
        return err
    }
    
    // 处理事件...
}
```

### 问题 3: 配额增加失败

**描述**: 支付成功但配额未增加

**解决方案**:
使用事务确保原子性，失败时记录待处理

```go
// 如果事务失败，记录待处理
if err := tx.Commit(ctx); err != nil {
    s.recordPendingQuotaIncrease(ctx, userID, quota, session.ID)
    return err
}
```

### 问题 4: 金额精度问题

**描述**: 浮点数金额计算不精确

**解决方案**:
使用整数分为单位，存储时使用 Decimal

```go
// Stripe 使用分为单位
UnitAmount: stripe.Int64(999), // $9.99

// 存储时转换
amount := decimal.NewFromInt(999).Div(decimal.NewFromInt(100)) // 9.99
```

## 验证结果

- [x] Checkout Session 创建成功
- [x] Webhook 签名验证正确
- [x] 支付成功后配额增加
- [x] 支付失败正确处理
- [x] 重复 Webhook 幂等处理

## 属性测试

**Payment Quota Consistency**
- 支付成功后配额精确增加 ✅
- 支付失败配额不变 ✅
- 并发支付配额正确累加 ✅

## 相关文件

- `backend/internal/payment/payment.go`
- `backend/internal/payment/payment_property_test.go`
- `backend/internal/models/payment.go`
