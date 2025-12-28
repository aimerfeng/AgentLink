# 开发日志 006: 用户注册功能

**日期**: 2024-12-28  
**任务**: Task 5.1 - 实现用户注册功能  
**状态**: ✅ 已完成

## 任务描述

实现用户注册 API，支持创作者和开发者两种用户类型，使用 Argon2id 进行密码哈希。

## 实现内容

### 1. API 接口

```
POST /api/v1/auth/register

Request:
{
    "email": "user@example.com",
    "password": "SecurePass123!",
    "user_type": "creator"  // or "developer"
}

Response (201 Created):
{
    "user": {
        "id": "uuid",
        "email": "user@example.com",
        "user_type": "creator",
        "email_verified": false,
        "created_at": "2024-12-28T00:00:00Z"
    },
    "message": "Registration successful. Please verify your email."
}
```

### 2. 密码哈希实现

```go
import "github.com/alexedwards/argon2id"

func (s *AuthService) hashPassword(password string) (string, error) {
    params := &argon2id.Params{
        Memory:      64 * 1024,  // 64 MB
        Iterations:  3,
        Parallelism: 2,
        SaltLength:  16,
        KeyLength:   32,
    }
    return argon2id.CreateHash(password, params)
}

func (s *AuthService) verifyPassword(password, hash string) (bool, error) {
    return argon2id.ComparePasswordAndHash(password, hash)
}
```

### 3. 注册流程

```go
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*User, error) {
    // 1. 验证邮箱格式
    if !isValidEmail(req.Email) {
        return nil, ErrInvalidEmail
    }
    
    // 2. 检查邮箱是否已存在
    exists, err := s.emailExists(ctx, req.Email)
    if err != nil {
        return nil, err
    }
    if exists {
        return nil, ErrEmailAlreadyExists
    }
    
    // 3. 验证密码强度
    if err := validatePassword(req.Password); err != nil {
        return nil, err
    }
    
    // 4. 哈希密码
    hash, err := s.hashPassword(req.Password)
    if err != nil {
        return nil, err
    }
    
    // 5. 创建用户
    user := &User{
        Email:        req.Email,
        PasswordHash: hash,
        UserType:     req.UserType,
    }
    
    // 6. 开始事务
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)
    
    // 7. 插入用户
    err = tx.QueryRow(ctx, insertUserSQL, ...).Scan(&user.ID, ...)
    if err != nil {
        return nil, err
    }
    
    // 8. 创建初始配额（开发者）
    if req.UserType == "developer" {
        _, err = tx.Exec(ctx, insertQuotaSQL, user.ID, 100, 0)
        if err != nil {
            return nil, err
        }
    }
    
    // 9. 创建创作者 profile
    if req.UserType == "creator" {
        _, err = tx.Exec(ctx, insertProfileSQL, user.ID)
        if err != nil {
            return nil, err
        }
    }
    
    // 10. 提交事务
    if err := tx.Commit(ctx); err != nil {
        return nil, err
    }
    
    return user, nil
}
```

## 遇到的问题

### 问题 1: Argon2id 内存参数过高

**描述**: 在低内存环境下注册请求超时

**错误信息**:
```
context deadline exceeded
```

**解决方案**:
调整 Argon2id 参数，平衡安全性和性能

```go
// 生产环境推荐参数
params := &argon2id.Params{
    Memory:      64 * 1024,  // 64 MB
    Iterations:  3,
    Parallelism: 2,
    SaltLength:  16,
    KeyLength:   32,
}

// 开发环境可降低
params := &argon2id.Params{
    Memory:      32 * 1024,  // 32 MB
    Iterations:  1,
    Parallelism: 2,
    SaltLength:  16,
    KeyLength:   32,
}
```

### 问题 2: 邮箱唯一约束冲突

**描述**: 并发注册相同邮箱时出现竞态条件

**错误信息**:
```
ERROR: duplicate key value violates unique constraint "users_email_key"
```

**解决方案**:
捕获数据库唯一约束错误并返回友好提示

```go
import "github.com/jackc/pgx/v5/pgconn"

func (s *AuthService) Register(...) (*User, error) {
    // ...
    err = tx.QueryRow(ctx, insertUserSQL, ...).Scan(...)
    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" {
            return nil, ErrEmailAlreadyExists
        }
        return nil, err
    }
    // ...
}
```

### 问题 3: 事务回滚不完整

**描述**: 创建配额失败后用户记录未回滚

**解决方案**:
使用 defer 确保事务回滚

```go
tx, err := s.db.Begin(ctx)
if err != nil {
    return nil, err
}
defer tx.Rollback(ctx)  // 如果已 Commit，Rollback 是 no-op

// ... 操作 ...

if err := tx.Commit(ctx); err != nil {
    return nil, err
}
```

## 验证结果

- [x] 创作者注册成功
- [x] 开发者注册成功
- [x] 密码正确哈希存储
- [x] 初始配额正确创建
- [x] 重复邮箱被拒绝
- [x] 无效邮箱被拒绝

## 属性测试

**Property 35: Free Quota Initialization**
- 验证新开发者获得 100 次免费配额
- 100 次测试全部通过

## 相关文件

- `backend/internal/auth/auth.go`
- `backend/internal/auth/errors.go`
- `backend/internal/auth/auth_property_test.go`
- `backend/internal/server/api.go`
