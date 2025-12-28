# 开发日志 016: 结算系统实现

**日期**: 2024-12-28  
**任务**: Task 23 - 实现结算系统  
**状态**: ✅ 已完成

## 任务描述

实现创作者收益结算系统，包括结算计算、批量处理和定时任务。

## 实现内容

### 1. 结算配置

```go
type SettlementConfig struct {
    PlatformFeeRate decimal.Decimal // 平台费率
    MinSettlement   decimal.Decimal // 最低结算金额
    SettlementHour  int             // 结算时间（小时）
}

var defaultSettlementConfig = SettlementConfig{
    PlatformFeeRate: decimal.NewFromFloat(0.20), // 20%
    MinSettlement:   decimal.NewFromFloat(1.00), // $1
    SettlementHour:  2,                          // UTC 02:00
}
```

### 2. 结算服务

```go
type SettlementService struct {
    db     *pgxpool.Pool
    config SettlementConfig
}

func (s *SettlementService) CalculateSettlement(ctx context.Context, creatorID uuid.UUID, startTime, endTime time.Time) (*SettlementSummary, error) {
    // 查询时间段内的调用收入
    var totalRevenue decimal.Decimal
    var callCount int
    
    err := s.db.QueryRow(ctx, `
        SELECT 
            COALESCE(SUM(cl.cost), 0) as total_revenue,
            COUNT(*) as call_count
        FROM call_logs cl
        JOIN agents a ON cl.agent_id = a.id
        WHERE a.creator_id = $1 
          AND cl.status = 'success'
          AND cl.created_at >= $2 
          AND cl.created_at < $3
          AND cl.settled = false
    `, creatorID, startTime, endTime).Scan(&totalRevenue, &callCount)
    
    if err != nil {
        return nil, err
    }
    
    // 计算平台费用
    platformFee := totalRevenue.Mul(s.config.PlatformFeeRate)
    creatorEarnings := totalRevenue.Sub(platformFee)
    
    return &SettlementSummary{
        CreatorID:       creatorID,
        StartTime:       startTime,
        EndTime:         endTime,
        TotalRevenue:    totalRevenue,
        PlatformFee:     platformFee,
        CreatorEarnings: creatorEarnings,
        CallCount:       callCount,
    }, nil
}

func (s *SettlementService) ProcessSettlement(ctx context.Context, creatorID uuid.UUID, startTime, endTime time.Time) (*Settlement, error) {
    // 计算结算
    summary, err := s.CalculateSettlement(ctx, creatorID, startTime, endTime)
    if err != nil {
        return nil, err
    }
    
    // 检查最低结算金额
    if summary.CreatorEarnings.LessThan(s.config.MinSettlement) {
        return nil, ErrBelowMinSettlement
    }
    
    // 开始事务
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)
    
    // 创建结算记录
    settlement := &Settlement{
        CreatorID:       creatorID,
        StartTime:       startTime,
        EndTime:         endTime,
        TotalRevenue:    summary.TotalRevenue,
        PlatformFee:     summary.PlatformFee,
        CreatorEarnings: summary.CreatorEarnings,
        CallCount:       summary.CallCount,
        Status:          "completed",
    }
    
    err = tx.QueryRow(ctx, `
        INSERT INTO settlements (creator_id, start_time, end_time, total_revenue, platform_fee, creator_earnings, call_count, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, created_at
    `, settlement.CreatorID, settlement.StartTime, settlement.EndTime,
       settlement.TotalRevenue, settlement.PlatformFee, settlement.CreatorEarnings,
       settlement.CallCount, settlement.Status,
    ).Scan(&settlement.ID, &settlement.CreatedAt)
    
    if err != nil {
        return nil, err
    }
    
    // 标记调用日志为已结算
    _, err = tx.Exec(ctx, `
        UPDATE call_logs 
        SET settled = true, settlement_id = $1
        WHERE agent_id IN (SELECT id FROM agents WHERE creator_id = $2)
          AND status = 'success'
          AND created_at >= $3 
          AND created_at < $4
          AND settled = false
    `, settlement.ID, creatorID, startTime, endTime)
    
    if err != nil {
        return nil, err
    }
    
    // 更新创作者余额
    _, err = tx.Exec(ctx, `
        UPDATE creator_profiles 
        SET balance = balance + $1, updated_at = NOW()
        WHERE user_id = $2
    `, settlement.CreatorEarnings, creatorID)
    
    if err != nil {
        return nil, err
    }
    
    return settlement, tx.Commit(ctx)
}
```

### 3. 批量结算

```go
func (s *SettlementService) ProcessBatchSettlement(ctx context.Context, endTime time.Time) (*BatchSettlementResult, error) {
    startTime := endTime.Add(-24 * time.Hour) // 结算前一天
    
    // 获取有待结算收入的创作者
    rows, err := s.db.Query(ctx, `
        SELECT DISTINCT a.creator_id
        FROM call_logs cl
        JOIN agents a ON cl.agent_id = a.id
        WHERE cl.status = 'success'
          AND cl.created_at >= $1 
          AND cl.created_at < $2
          AND cl.settled = false
    `, startTime, endTime)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var creatorIDs []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        creatorIDs = append(creatorIDs, id)
    }
    
    result := &BatchSettlementResult{
        StartTime:  startTime,
        EndTime:    endTime,
        TotalCount: len(creatorIDs),
    }
    
    // 逐个处理结算
    for _, creatorID := range creatorIDs {
        settlement, err := s.ProcessSettlement(ctx, creatorID, startTime, endTime)
        if err != nil {
            if errors.Is(err, ErrBelowMinSettlement) {
                result.SkippedCount++
                continue
            }
            result.FailedCount++
            result.Errors = append(result.Errors, SettlementError{
                CreatorID: creatorID,
                Error:     err.Error(),
            })
            continue
        }
        result.SuccessCount++
        result.TotalSettled = result.TotalSettled.Add(settlement.CreatorEarnings)
    }
    
    return result, nil
}
```

### 4. 定时任务

```go
type SettlementScheduler struct {
    service *SettlementService
    ticker  *time.Ticker
    done    chan bool
}

func NewSettlementScheduler(service *SettlementService) *SettlementScheduler {
    return &SettlementScheduler{
        service: service,
        done:    make(chan bool),
    }
}

func (s *SettlementScheduler) Start() {
    // 计算到下一个结算时间的间隔
    now := time.Now().UTC()
    nextRun := time.Date(now.Year(), now.Month(), now.Day(), 
        s.service.config.SettlementHour, 0, 0, 0, time.UTC)
    
    if now.After(nextRun) {
        nextRun = nextRun.Add(24 * time.Hour)
    }
    
    initialDelay := nextRun.Sub(now)
    
    go func() {
        // 等待到第一次执行时间
        time.Sleep(initialDelay)
        
        // 执行第一次结算
        s.runSettlement()
        
        // 之后每 24 小时执行一次
        s.ticker = time.NewTicker(24 * time.Hour)
        
        for {
            select {
            case <-s.ticker.C:
                s.runSettlement()
            case <-s.done:
                return
            }
        }
    }()
}

func (s *SettlementScheduler) runSettlement() {
    ctx := context.Background()
    endTime := time.Now().UTC().Truncate(24 * time.Hour)
    
    result, err := s.service.ProcessBatchSettlement(ctx, endTime)
    if err != nil {
        log.Error().Err(err).Msg("batch settlement failed")
        return
    }
    
    log.Info().
        Int("total", result.TotalCount).
        Int("success", result.SuccessCount).
        Int("skipped", result.SkippedCount).
        Int("failed", result.FailedCount).
        Str("total_settled", result.TotalSettled.String()).
        Msg("batch settlement completed")
}

func (s *SettlementScheduler) Stop() {
    if s.ticker != nil {
        s.ticker.Stop()
    }
    s.done <- true
}
```

## 遇到的问题

### 问题 1: 重复结算

**描述**: 同一调用被多次结算

**解决方案**:
使用 settled 标志和 settlement_id 关联

```go
// 标记为已结算
UPDATE call_logs 
SET settled = true, settlement_id = $1
WHERE ... AND settled = false
```

### 问题 2: 结算时间窗口重叠

**描述**: 结算时间窗口可能重叠导致遗漏或重复

**解决方案**:
使用精确的时间范围（左闭右开）

```go
// [startTime, endTime)
WHERE created_at >= $1 AND created_at < $2
```

### 问题 3: 大量创作者结算超时

**描述**: 批量结算时数据库连接超时

**解决方案**:
分批处理，每批限制数量

```go
const batchSize = 100

for i := 0; i < len(creatorIDs); i += batchSize {
    end := i + batchSize
    if end > len(creatorIDs) {
        end = len(creatorIDs)
    }
    batch := creatorIDs[i:end]
    // 处理这一批
}
```

### 问题 4: 定时任务重复执行

**描述**: 多实例部署时定时任务重复执行

**解决方案**:
使用分布式锁

```go
func (s *SettlementScheduler) runSettlement() {
    // 获取分布式锁
    lock, err := s.redis.SetNX(ctx, "settlement:lock", "1", 1*time.Hour).Result()
    if err != nil || !lock {
        log.Debug().Msg("settlement already running on another instance")
        return
    }
    defer s.redis.Del(ctx, "settlement:lock")
    
    // 执行结算...
}
```

## 验证结果

- [x] 结算计算正确
- [x] 平台费用计算正确
- [x] 创作者收益计算正确
- [x] 批量结算正常工作
- [x] 定时任务正常执行

## 属性测试

**Settlement Calculation Properties**
- 平台费用 = 总收入 * 20% ✅
- 创作者收益 = 总收入 - 平台费用 ✅
- 总收入 = 平台费用 + 创作者收益 ✅

**Settlement Processing Properties**
- 结算后调用日志标记为已结算 ✅
- 结算后创作者余额增加 ✅
- 低于最低金额不结算 ✅

## 相关文件

- `backend/internal/settlement/settlement.go`
- `backend/internal/settlement/scheduler.go`
- `backend/internal/settlement/settlement_property_test.go`
