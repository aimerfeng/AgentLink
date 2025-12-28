# 错误修复 001: 数据库连接池耗尽

**日期**: 2024-12-28  
**严重程度**: 高  
**状态**: ✅ 已修复

## 问题描述

高并发请求时数据库连接池耗尽，导致请求失败。

## 错误信息

```
error: failed to acquire connection from pool: pool exhausted
context deadline exceeded (Client.Timeout exceeded while awaiting headers)
```

## 根本原因

1. 默认连接池大小过小（10 个连接）
2. 长事务占用连接时间过长
3. 连接泄漏（未正确关闭）

## 解决方案

### 1. 调整连接池配置

```go
// backend/internal/database/database.go
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, err
    }
    
    // 调整连接池参数
    config.MaxConns = 50                      // 最大连接数
    config.MinConns = 10                      // 最小连接数
    config.MaxConnLifetime = 1 * time.Hour    // 连接最大生命周期
    config.MaxConnIdleTime = 30 * time.Minute // 空闲连接超时
    config.HealthCheckPeriod = 1 * time.Minute // 健康检查间隔
    
    return pgxpool.NewWithConfig(ctx, config)
}
```

### 2. 确保连接正确释放

```go
// 使用 defer 确保释放
func (s *Service) DoSomething(ctx context.Context) error {
    conn, err := s.pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release() // 确保释放
    
    // ... 使用连接 ...
}

// 事务使用 defer 回滚
func (s *Service) DoTransaction(ctx context.Context) error {
    tx, err := s.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) // 如果已 Commit，这是 no-op
    
    // ... 事务操作 ...
    
    return tx.Commit(ctx)
}
```

### 3. 添加连接池监控

```go
// 定期记录连接池状态
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        stat := pool.Stat()
        log.Info().
            Int32("total", stat.TotalConns()).
            Int32("idle", stat.IdleConns()).
            Int32("acquired", stat.AcquiredConns()).
            Int64("acquire_count", stat.AcquireCount()).
            Msg("database pool stats")
    }
}()
```

## 验证

```bash
# 压力测试
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/agents

# 监控连接池
watch -n 1 'psql -c "SELECT count(*) FROM pg_stat_activity WHERE datname = '\''agentlink'\'';"'
```

## 预防措施

1. 设置合理的连接池大小
2. 使用 defer 确保连接释放
3. 监控连接池使用情况
4. 设置连接超时
