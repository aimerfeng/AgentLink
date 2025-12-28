# 错误修复 002: JWT Token 过期处理

**日期**: 2024-12-28  
**严重程度**: 中  
**状态**: ✅ 已修复

## 问题描述

用户 Token 过期后，前端没有正确处理刷新逻辑，导致用户被强制登出。

## 错误信息

```json
{
    "error": "token_expired",
    "message": "Token has expired"
}
```

## 根本原因

1. 前端没有实现 Token 自动刷新
2. 后端没有区分 Token 过期和 Token 无效
3. Refresh Token 过期时间设置不合理

## 解决方案

### 1. 后端区分错误类型

```go
// backend/internal/middleware/middleware.go
func (m *JWTAuthenticator) JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := extractBearerToken(c)
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{
                "error":   "missing_token",
                "message": "Authorization header is required",
            })
            return
        }
        
        claims, err := m.authService.ValidateAccessToken(token)
        if err != nil {
            if errors.Is(err, auth.ErrTokenExpired) {
                c.AbortWithStatusJSON(401, gin.H{
                    "error":   "token_expired",
                    "message": "Token has expired, please refresh",
                })
                return
            }
            c.AbortWithStatusJSON(401, gin.H{
                "error":   "invalid_token",
                "message": "Invalid token",
            })
            return
        }
        
        // ... 设置 context ...
    }
}
```

### 2. 前端实现 Token 刷新

```typescript
// frontend/src/lib/api.ts
class APIClient {
    private refreshPromise: Promise<void> | null = null;
    
    async request<T>(config: RequestConfig): Promise<T> {
        try {
            return await this.doRequest<T>(config);
        } catch (error) {
            if (error.code === 'token_expired') {
                // 避免并发刷新
                if (!this.refreshPromise) {
                    this.refreshPromise = this.refreshToken();
                }
                
                await this.refreshPromise;
                this.refreshPromise = null;
                
                // 重试原请求
                return await this.doRequest<T>(config);
            }
            throw error;
        }
    }
    
    private async refreshToken(): Promise<void> {
        const refreshToken = localStorage.getItem('refresh_token');
        if (!refreshToken) {
            throw new Error('No refresh token');
        }
        
        const response = await fetch('/api/v1/auth/refresh', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ refresh_token: refreshToken }),
        });
        
        if (!response.ok) {
            // Refresh Token 也过期，需要重新登录
            localStorage.removeItem('access_token');
            localStorage.removeItem('refresh_token');
            window.location.href = '/login';
            throw new Error('Session expired');
        }
        
        const data = await response.json();
        localStorage.setItem('access_token', data.access_token);
        localStorage.setItem('refresh_token', data.refresh_token);
    }
}
```

### 3. 调整 Token 过期时间

```go
// Access Token: 15 分钟
accessClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))

// Refresh Token: 7 天
refreshClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour))
```

## 验证

```bash
# 测试 Token 过期
curl -X GET http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <expired_token>"

# 预期响应
{
    "error": "token_expired",
    "message": "Token has expired, please refresh"
}
```

## 预防措施

1. 前端实现自动 Token 刷新
2. 后端返回明确的错误类型
3. 设置合理的 Token 过期时间
4. 监控 Token 刷新失败率
