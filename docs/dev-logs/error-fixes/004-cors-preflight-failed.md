# 错误修复 004: CORS 预检请求失败

**日期**: 2024-12-28  
**严重程度**: 中  
**状态**: ✅ 已修复

## 问题描述

前端调用 API 时，浏览器 CORS 预检请求失败，导致请求被阻止。

## 错误信息

```
Access to XMLHttpRequest at 'http://localhost:8080/api/v1/agents' from origin 
'http://localhost:3000' has been blocked by CORS policy: Response to preflight 
request doesn't pass access control check: No 'Access-Control-Allow-Origin' 
header is present on the requested resource.
```

## 根本原因

1. 后端未配置 CORS 中间件
2. OPTIONS 请求未正确处理
3. 允许的 Headers 配置不完整

## 解决方案

### 1. 配置 CORS 中间件

```go
// backend/internal/middleware/middleware.go
func CORS(allowedOrigins []string) gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        
        // 检查是否是允许的来源
        allowed := false
        for _, o := range allowedOrigins {
            if o == "*" || o == origin {
                allowed = true
                break
            }
        }
        
        if allowed {
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Access-Control-Allow-Credentials", "true")
            c.Header("Access-Control-Allow-Headers", 
                "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, "+
                "Authorization, X-AgentLink-Key, X-Request-ID, accept, origin, "+
                "Cache-Control, X-Requested-With")
            c.Header("Access-Control-Allow-Methods", 
                "POST, OPTIONS, GET, PUT, DELETE, PATCH")
            c.Header("Access-Control-Max-Age", "86400") // 24 小时
        }
        
        // 处理预检请求
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        
        c.Next()
    }
}
```

### 2. 应用中间件

```go
// backend/internal/server/api.go
func NewAPIServer(config *config.Config) *gin.Engine {
    router := gin.New()
    
    // CORS 必须在其他中间件之前
    router.Use(middleware.CORS([]string{
        "http://localhost:3000",
        "https://agentlink.io",
    }))
    
    router.Use(gin.Recovery())
    router.Use(middleware.RequestID())
    router.Use(middleware.Logger())
    
    // ... 路由配置 ...
}
```

### 3. 开发环境配置

```go
// 开发环境允许所有来源
if config.Environment == "development" {
    router.Use(middleware.CORS([]string{"*"}))
} else {
    router.Use(middleware.CORS(config.AllowedOrigins))
}
```

### 4. 前端配置

```typescript
// frontend/src/lib/api.ts
const api = axios.create({
    baseURL: process.env.NEXT_PUBLIC_API_URL,
    withCredentials: true, // 发送凭证
    headers: {
        'Content-Type': 'application/json',
    },
});
```

## 验证

```bash
# 测试预检请求
curl -X OPTIONS http://localhost:8080/api/v1/agents \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type, Authorization" \
  -v

# 预期响应头
< Access-Control-Allow-Origin: http://localhost:3000
< Access-Control-Allow-Methods: POST, OPTIONS, GET, PUT, DELETE, PATCH
< Access-Control-Allow-Headers: Content-Type, Authorization, ...
< Access-Control-Max-Age: 86400
```

## 预防措施

1. CORS 中间件放在最前面
2. 正确处理 OPTIONS 请求
3. 配置完整的允许 Headers
4. 生产环境限制允许的来源
