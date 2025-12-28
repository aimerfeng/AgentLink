# 错误修复 003: Redis 连接拒绝

**日期**: 2024-12-28  
**严重程度**: 高  
**状态**: ✅ 已修复

## 问题描述

应用启动时无法连接 Redis，导致限速和缓存功能失效。

## 错误信息

```
dial tcp 127.0.0.1:6379: connect: connection refused
redis: connection pool timeout
```

## 根本原因

1. Redis 服务未启动
2. Redis 连接配置错误
3. Docker 网络配置问题
4. 防火墙阻止连接

## 解决方案

### 1. 检查 Redis 服务状态

```bash
# Docker 环境
docker-compose ps redis
docker-compose logs redis

# 本地安装
redis-cli ping
# 预期输出: PONG
```

### 2. 修复 Docker 网络配置

```yaml
# docker-compose.yml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - agentlink

  api:
    depends_on:
      redis:
        condition: service_healthy
    environment:
      REDIS_URL: redis://redis:6379  # 使用服务名而非 localhost
    networks:
      - agentlink

networks:
  agentlink:
    driver: bridge
```

### 3. 添加连接重试逻辑

```go
// backend/internal/cache/redis.go
func NewRedisClient(ctx context.Context, url string) (*redis.Client, error) {
    opt, err := redis.ParseURL(url)
    if err != nil {
        return nil, err
    }
    
    client := redis.NewClient(opt)
    
    // 重试连接
    var lastErr error
    for i := 0; i < 5; i++ {
        if err := client.Ping(ctx).Err(); err != nil {
            lastErr = err
            log.Warn().
                Err(err).
                Int("attempt", i+1).
                Msg("failed to connect to redis, retrying...")
            time.Sleep(time.Duration(i+1) * time.Second)
            continue
        }
        log.Info().Msg("connected to redis")
        return client, nil
    }
    
    return nil, fmt.Errorf("failed to connect to redis after 5 attempts: %w", lastErr)
}
```

### 4. 实现优雅降级

```go
// 当 Redis 不可用时，使用内存缓存作为后备
type CacheService struct {
    redis    *redis.Client
    fallback *sync.Map
}

func (c *CacheService) Get(ctx context.Context, key string) (string, error) {
    // 尝试 Redis
    val, err := c.redis.Get(ctx, key).Result()
    if err == nil {
        return val, nil
    }
    
    // Redis 不可用，使用内存缓存
    if err == redis.Nil || isConnectionError(err) {
        if v, ok := c.fallback.Load(key); ok {
            return v.(string), nil
        }
        return "", ErrCacheMiss
    }
    
    return "", err
}
```

## 验证

```bash
# 测试 Redis 连接
redis-cli -h localhost -p 6379 ping

# 测试应用连接
curl http://localhost:8080/health
# 预期响应包含 redis: "connected"
```

## 预防措施

1. 使用 Docker 健康检查
2. 实现连接重试逻辑
3. 添加优雅降级机制
4. 监控 Redis 连接状态
