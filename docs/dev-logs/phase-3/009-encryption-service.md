# 开发日志 009: System Prompt 加密服务

**日期**: 2024-12-28  
**任务**: Task 9.5, 9.6 - 实现 System Prompt 加密存储  
**状态**: ✅ 已完成

## 任务描述

实现 AES-256-GCM 加密服务，保护 Agent 的 System Prompt 不被泄露。

## 实现内容

### 1. 加密算法选择

| 算法 | 优点 | 缺点 |
|------|------|------|
| AES-CBC | 广泛支持 | 需要单独 MAC |
| AES-GCM | 认证加密，防篡改 | 需要唯一 nonce |
| ChaCha20-Poly1305 | 软件实现快 | 硬件支持少 |

**选择**: AES-256-GCM（认证加密，防篡改）

### 2. 加密实现

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
)

type EncryptionService struct {
    key []byte // 32 bytes for AES-256
}

func NewEncryptionService(key string) (*EncryptionService, error) {
    keyBytes, err := hex.DecodeString(key)
    if err != nil || len(keyBytes) != 32 {
        return nil, ErrInvalidEncryptionKey
    }
    return &EncryptionService{key: keyBytes}, nil
}

func (s *EncryptionService) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, nil, err
    }
    
    // 生成随机 nonce
    nonce = make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return nil, nil, err
    }
    
    // 加密（包含认证标签）
    ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
    
    return ciphertext, nonce, nil
}

func (s *EncryptionService) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    // 解密并验证认证标签
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, ErrDecryptionFailed // 可能是篡改
    }
    
    return plaintext, nil
}
```

### 3. 数据库存储

```sql
-- agents 表
config_encrypted BYTEA,  -- 加密后的配置
config_iv BYTEA          -- Nonce/IV
```

### 4. 使用示例

```go
// 加密存储
configJSON, _ := json.Marshal(config)
encrypted, iv, err := encService.Encrypt(configJSON)
if err != nil {
    return nil, ErrEncryptionFailed
}

// 存储到数据库
_, err = db.Exec(ctx, `
    UPDATE agents 
    SET config_encrypted = $1, config_iv = $2 
    WHERE id = $3
`, encrypted, iv, agentID)

// 解密读取
var encrypted, iv []byte
err = db.QueryRow(ctx, `
    SELECT config_encrypted, config_iv FROM agents WHERE id = $1
`, agentID).Scan(&encrypted, &iv)

configJSON, err := encService.Decrypt(encrypted, iv)
if err != nil {
    return nil, ErrDecryptionFailed
}

var config AgentConfig
json.Unmarshal(configJSON, &config)
```

## 遇到的问题

### 问题 1: Nonce 重用

**描述**: 相同 nonce 加密不同数据会泄露信息

**解决方案**:
每次加密生成新的随机 nonce

```go
nonce = make([]byte, gcm.NonceSize()) // 12 bytes for GCM
if _, err := rand.Read(nonce); err != nil {
    return nil, nil, err
}
```

### 问题 2: 密钥管理

**描述**: 密钥硬编码在代码中

**解决方案**:
从环境变量加载，生产环境使用密钥管理服务

```go
// 从环境变量加载
key := os.Getenv("ENCRYPTION_KEY")
if key == "" {
    log.Fatal("ENCRYPTION_KEY is required")
}

// 生产环境可使用 AWS KMS、HashiCorp Vault 等
```

### 问题 3: 篡改检测

**描述**: 需要检测密文是否被篡改

**解决方案**:
GCM 模式自带认证标签，篡改会导致解密失败

```go
plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
if err != nil {
    // 认证失败，可能是篡改
    return nil, ErrDecryptionFailed
}
```

### 问题 4: 密钥轮换

**描述**: 需要支持密钥轮换而不影响现有数据

**解决方案**:
存储密钥版本，支持多密钥解密

```go
type EncryptionService struct {
    keys map[int][]byte // version -> key
    currentVersion int
}

func (s *EncryptionService) Encrypt(plaintext []byte) ([]byte, []byte, int, error) {
    // 使用当前版本密钥加密
    return ciphertext, nonce, s.currentVersion, nil
}

func (s *EncryptionService) Decrypt(ciphertext, nonce []byte, version int) ([]byte, error) {
    key, ok := s.keys[version]
    if !ok {
        return nil, ErrKeyVersionNotFound
    }
    // 使用对应版本密钥解密
}
```

## 验证结果

- [x] 加密后解密得到原始数据
- [x] 不同加密产生不同密文
- [x] 篡改检测正常工作
- [x] 密钥长度验证正确

## 属性测试

**Property 19: Encryption Round-Trip**
- 加密后解密得到原始数据 ✅ (100 次测试)
- 不同加密产生不同密文 ✅ (100 次测试)
- 篡改检测：修改密文后解密失败 ✅ (100 次测试)

## 安全考虑

1. **密钥存储**: 使用环境变量或密钥管理服务
2. **Nonce 唯一性**: 每次加密生成新随机 nonce
3. **认证加密**: GCM 模式提供完整性保护
4. **密钥轮换**: 支持多版本密钥
5. **内存安全**: 使用后清零敏感数据

## 相关文件

- `backend/internal/agent/agent.go` (加密服务集成)
- `backend/internal/agent/agent_property_test.go`
