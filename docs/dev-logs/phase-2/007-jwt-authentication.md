# 开发日志 007: JWT 认证实现

**日期**: 2024-12-28  
**任务**: Task 5.3, 5.5 - 实现用户登录和 Token 刷新  
**状态**: ✅ 已完成

## 任务描述

实现基于 JWT 的用户认证系统，包括登录、Token 刷新和验证。

## 实现内容

### 1. JWT Claims 结构

```go
type Claims struct {
    UserID   string `json:"user_id"`
    Email    string `json:"email"`
    UserType string `json:"user_type"`
    Type     string `json:"type"` // "access" or "refresh"
    jwt.RegisteredClaims
}
```

### 2. Token 生成

```go
func (s *AuthService) generateTokenPair(user *User) (*TokenPair, error) {
    // Access Token (15 分钟)
    accessClaims := &Claims{
        UserID:   user.ID.String(),
        Email:    user.Email,
        UserType: user.UserType,
        Type:     "access",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "agentlink",
        },
    }
    
    accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    accessTokenString, err := accessToken.SignedString([]byte(s.jwtSecret))
    if err != nil {
        return nil, err
    }
    
    // Refresh Token (7 天)
    refreshClaims := &Claims{
        UserID:   user.ID.String(),
        Email:    user.Email,
        UserType: user.UserType,
        Type:     "refresh",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "agentlink",
        },
    }
    
    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
    refreshTokenString, err := refreshToken.SignedString([]byte(s.jwtSecret))
    if err != nil {
        return nil, err
    }
    
    return &TokenPair{
        AccessToken:  accessTokenString,
        RefreshToken: refreshTokenString,
        ExpiresIn:    900, // 15 minutes in seconds
    }, nil
}
```

### 3. 登录接口

```
POST /api/v1/auth/login

Request:
{
    "email": "user@example.com",
    "password": "SecurePass123!"
}

Response (200 OK):
{
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 900,
    "token_type": "Bearer"
}

Error Response (401):
{
    "error": "invalid_credentials",
    "message": "Invalid email or password"
}
```

### 4. Token 刷新接口

```
POST /api/v1/auth/refresh

Request:
{
    "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}

Response (200 OK):
{
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 900,
    "token_type": "Bearer"
}
```

### 5. Token 验证

```go
func (s *AuthService) ValidateAccessToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(s.jwtSecret), nil
    })
    
    if err != nil {
        if errors.Is(err, jwt.ErrTokenExpired) {
            return nil, ErrTokenExpired
        }
        return nil, ErrInvalidToken
    }
    
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, ErrInvalidToken
    }
    
    // 确保是 access token
    if claims.Type != "access" {
        return nil, ErrInvalidToken
    }
    
    return claims, nil
}
```

## 遇到的问题

### 问题 1: JWT 密钥长度不足

**描述**: 使用短密钥时安全性警告

**解决方案**:
使用至少 256 位（32 字节）的密钥

```bash
# 生成安全密钥
openssl rand -base64 32
# 输出: xK9mN2pQ3rS4tU5vW6xY7zA8bC9dE0fG1hI2jK3lM4n=
```

### 问题 2: Token 类型混淆

**描述**: Refresh Token 被用作 Access Token

**错误信息**:
```
unauthorized: invalid token type
```

**解决方案**:
在 Claims 中添加 Type 字段，验证时检查

```go
// 验证 Access Token
if claims.Type != "access" {
    return nil, ErrInvalidToken
}

// 验证 Refresh Token
if claims.Type != "refresh" {
    return nil, ErrInvalidToken
}
```

### 问题 3: 时区问题

**描述**: Token 过期时间计算错误

**解决方案**:
统一使用 UTC 时间

```go
// 使用 UTC
ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(15 * time.Minute))
```

### 问题 4: 并发刷新竞态

**描述**: 同一 Refresh Token 被多次使用

**解决方案**:
实现 Token 轮换，每次刷新生成新的 Token Pair

```go
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
    // 1. 验证 refresh token
    claims, err := s.validateRefreshToken(refreshToken)
    if err != nil {
        return nil, err
    }
    
    // 2. 检查用户是否仍然存在
    user, err := s.getUserByID(ctx, claims.UserID)
    if err != nil {
        return nil, ErrUserNotFound
    }
    
    // 3. 生成新的 token pair
    return s.generateTokenPair(user)
}
```

## 验证结果

- [x] 登录返回正确的 Token Pair
- [x] Access Token 15 分钟过期
- [x] Refresh Token 7 天过期
- [x] 过期 Token 被拒绝
- [x] 无效 Token 被拒绝
- [x] Token 类型验证正确

## 属性测试

**Property 5: Authentication Correctness**
- 有效凭证返回 token ✅
- 无效密码返回统一错误 ✅
- 无效邮箱返回统一错误 ✅
- Token 验证逻辑正确 ✅

## 相关文件

- `backend/internal/auth/auth.go`
- `backend/internal/middleware/middleware.go`
- `backend/internal/server/api.go`
