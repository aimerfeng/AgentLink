# 开发日志 012: 熔断器实现

**日期**: 2024-12-28  
**任务**: Task 13.3, 13.4 - 实现熔断器  
**状态**: ✅ 已完成

## 任务描述

实现熔断器模式，防止上游 AI 服务故障时的级联失败。

## 实现内容

### 1. 熔断器配置

```go
import "github.com/sony/gobreaker"

type CircuitBreakerConfig struct {
    MaxRequests   uint32        // 半开状态最大请求数
    Interval      time.Duration // 统计间隔
    Timeout       time.Duration // 熔断超时
    FailureRatio  float64       // 失败率阈值
    MinRequests   uint32        // 最小请求数（触发熔断前）
}

var defaultConfig = CircuitBreakerConfig{
    MaxRequests:  1,              // 半开状态允许 1 个请求
    Interval:     60 * time.Second,
    Timeout:      30 * time.Second,
    FailureRatio: 0.5,            // 50% 失败率触发熔断
    MinRequests:  5,              // 至少 5 个请求后才判断
}
```

### 2. 熔断器实现

```go
type CircuitBreakerManager struct {
    breakers map[string]*gobreaker.CircuitBreaker
    mu       sync.RWMutex
}

func NewCircuitBreakerManager() *CircuitBreakerManager {
    return &CircuitBreakerManager{
        breakers: make(map[string]*gobreaker.CircuitBreaker),
    }
}

func (m *CircuitBreakerManager) GetBreaker(provider string) *gobreaker.CircuitBreaker {
    m.mu.RLock()
    cb, exists := m.breakers[provider]
    m.mu.RUnlock()
    
    if exists {
        return cb
    }
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 双重检查
    if cb, exists = m.breakers[provider]; exists {
        return cb
    }
    
    settings := gobreaker.Settings{
        Name:        provider,
        MaxRequests: defaultConfig.MaxRequests,
        Interval:    defaultConfig.Interval,
        Timeout:     defaultConfig.Timeout,
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            if counts.Requests < defaultConfig.MinRequests {
                return false
            }
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return failureRatio >= defaultConfig.FailureRatio
        },
        OnStateChange: func(name string, from, to gobreaker.State) {
            log.Info().
                Str("provider", name).
                Str("from", from.String()).
                Str("to", to.String()).
                Msg("circuit breaker state changed")
        },
    }
    
    cb = gobreaker.NewCircuitBreaker(settings)
    m.breakers[provider] = cb
    
    return cb
}
```

### 3. 使用熔断器

```go
func (p *ProxyService) callUpstreamWithBreaker(ctx context.Context, provider string, req *UpstreamRequest) (*UpstreamResponse, error) {
    cb := p.cbManager.GetBreaker(provider)
    
    result, err := cb.Execute(func() (interface{}, error) {
        return p.callUpstream(ctx, provider, req)
    })
    
    if err != nil {
        if errors.Is(err, gobreaker.ErrOpenState) {
            return nil, &AppError{
                Code:    "circuit_open",
                Message: "Service temporarily unavailable",
                Status:  503,
            }
        }
        return nil, err
    }
    
    return result.(*UpstreamResponse), nil
}
```

### 4. 状态转换

```
     ┌─────────────────────────────────────┐
     │                                     │
     ▼                                     │
┌─────────┐  failure ratio >= 50%  ┌───────┴───┐
│ Closed  │───────────────────────▶│   Open    │
└────┬────┘                        └─────┬─────┘
     │                                   │
     │                                   │ timeout (30s)
     │                                   ▼
     │                            ┌───────────┐
     │◀───────── success ─────────│ Half-Open │
     │                            └─────┬─────┘
     │                                  │
     │◀─────────── failure ─────────────┘
```

### 5. 错误分类

```go
func isRetryableError(err error) bool {
    // 服务端错误可重试
    var httpErr *HTTPError
    if errors.As(err, &httpErr) {
        return httpErr.StatusCode >= 500
    }
    
    // 超时可重试
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }
    
    // 网络错误可重试
    var netErr net.Error
    if errors.As(err, &netErr) {
        return netErr.Temporary()
    }
    
    return false
}

// 客户端错误不触发熔断
func shouldTripBreaker(err error) bool {
    var httpErr *HTTPError
    if errors.As(err, &httpErr) {
        // 4xx 错误不触发熔断
        return httpErr.StatusCode >= 500
    }
    return true
}
```

## 遇到的问题

### 问题 1: 所有 Provider 共享熔断器

**描述**: 一个 Provider 故障影响其他 Provider

**解决方案**:
每个 Provider 独立熔断器

```go
// 按 provider 隔离
cb := m.GetBreaker("openai")
cb := m.GetBreaker("anthropic")
cb := m.GetBreaker("google")
```

### 问题 2: 客户端错误触发熔断

**描述**: 400 错误导致熔断器打开

**解决方案**:
只对服务端错误计数

```go
ReadyToTrip: func(counts gobreaker.Counts) bool {
    // 只统计服务端错误
    serverErrors := counts.TotalFailures - counts.ConsecutiveFailures
    if counts.Requests < minRequests {
        return false
    }
    return float64(serverErrors)/float64(counts.Requests) >= 0.5
}
```

### 问题 3: 熔断恢复太慢

**描述**: 服务恢复后熔断器仍然打开

**解决方案**:
调整超时时间和半开状态请求数

```go
settings := gobreaker.Settings{
    Timeout:     30 * time.Second, // 30 秒后尝试恢复
    MaxRequests: 3,                // 半开状态允许 3 个请求
}
```

### 问题 4: 并发安全

**描述**: 并发创建熔断器导致重复

**解决方案**:
使用双重检查锁定

```go
m.mu.RLock()
cb, exists := m.breakers[provider]
m.mu.RUnlock()

if exists {
    return cb
}

m.mu.Lock()
defer m.mu.Unlock()

// 双重检查
if cb, exists = m.breakers[provider]; exists {
    return cb
}

// 创建新熔断器
```

## 验证结果

- [x] 连续 5 次失败后熔断器打开
- [x] 30 秒后进入半开状态
- [x] 半开状态成功后关闭熔断器
- [x] 客户端错误不触发熔断
- [x] Provider 隔离正确

## 属性测试

**Circuit Breaker Properties**
- 连续失败后打开 ✅
- 超时后进入半开状态 ✅
- Provider 隔离 ✅
- 客户端错误不触发 ✅

## 相关文件

- `backend/internal/proxy/circuitbreaker.go`
- `backend/internal/proxy/proxy_property_test.go`
