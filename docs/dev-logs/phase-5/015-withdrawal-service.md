# 开发日志 015: 创作者提现服务

**日期**: 2024-12-28  
**任务**: Task 22 - 实现创作者提现  
**状态**: ✅ 已完成

## 任务描述

实现创作者收益提现功能，包括阈值验证、费用计算和失败恢复。

## 实现内容

### 1. 提现配置

```go
type WithdrawalConfig struct {
    MinAmount       decimal.Decimal // 最低提现金额
    MaxAmount       decimal.Decimal // 最高提现金额
    FeePercentage   decimal.Decimal // 手续费百分比
    MinFee          decimal.Decimal // 最低手续费
    ProcessingDays  int             // 处理天数
}

var defaultWithdrawalConfig = WithdrawalConfig{
    MinAmount:      decimal.NewFromFloat(10.00),   // $10
    MaxAmount:      decimal.NewFromFloat(10000.00), // $10,000
    FeePercentage:  decimal.NewFromFloat(0.025),   // 2.5%
    MinFee:         decimal.NewFromFloat(0.50),    // $0.50
    ProcessingDays: 3,
}
```

### 2. 提现服务

```go
type WithdrawalService struct {
    db     *pgxpool.Pool
    config WithdrawalConfig
}

func (s *WithdrawalService) CreateWithdrawal(ctx context.Context, userID uuid.UUID, amount decimal.Decimal) (*Withdrawal, error) {
    // 1. 验证金额范围
    if amount.LessThan(s.config.MinAmount) {
        return nil, ErrAmountBelowMinimum
    }
    if amount.GreaterThan(s.config.MaxAmount) {
        return nil, ErrAmountAboveMaximum
    }
    
    // 2. 获取用户余额
    var balance decimal.Decimal
    err := s.db.QueryRow(ctx, `
        SELECT balance FROM creator_profiles WHERE user_id = $1
    `, userID).Scan(&balance)
    if err != nil {
        return nil, err
    }
    
    // 3. 验证余额充足
    if balance.LessThan(amount) {
        return nil, ErrInsufficientBalance
    }
    
    // 4. 计算手续费
    fee := s.calculateFee(amount)
    netAmount := amount.Sub(fee)
    
    // 5. 开始事务
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)
    
    // 6. 扣减余额
    result, err := tx.Exec(ctx, `
        UPDATE creator_profiles 
        SET balance = balance - $1, updated_at = NOW()
        WHERE user_id = $2 AND balance >= $1
    `, amount, userID)
    if err != nil {
        return nil, err
    }
    if result.RowsAffected() == 0 {
        return nil, ErrInsufficientBalance
    }
    
    // 7. 创建提现记录
    withdrawal := &Withdrawal{
        UserID:    userID,
        Amount:    amount,
        Fee:       fee,
        NetAmount: netAmount,
        Status:    "pending",
    }
    
    err = tx.QueryRow(ctx, `
        INSERT INTO withdrawals (user_id, amount, fee, net_amount, status)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at
    `, withdrawal.UserID, withdrawal.Amount, withdrawal.Fee, 
       withdrawal.NetAmount, withdrawal.Status,
    ).Scan(&withdrawal.ID, &withdrawal.CreatedAt)
    
    if err != nil {
        return nil, err
    }
    
    // 8. 提交事务
    if err := tx.Commit(ctx); err != nil {
        return nil, err
    }
    
    return withdrawal, nil
}

func (s *WithdrawalService) calculateFee(amount decimal.Decimal) decimal.Decimal {
    fee := amount.Mul(s.config.FeePercentage)
    if fee.LessThan(s.config.MinFee) {
        return s.config.MinFee
    }
    return fee.Round(2)
}
```

### 3. 失败恢复

```go
func (s *WithdrawalService) HandleWithdrawalFailure(ctx context.Context, withdrawalID uuid.UUID, reason string) error {
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)
    
    // 获取提现记录
    var withdrawal Withdrawal
    err = tx.QueryRow(ctx, `
        SELECT id, user_id, amount, status 
        FROM withdrawals 
        WHERE id = $1 
        FOR UPDATE
    `, withdrawalID).Scan(&withdrawal.ID, &withdrawal.UserID, &withdrawal.Amount, &withdrawal.Status)
    
    if err != nil {
        return err
    }
    
    // 只处理 pending 或 processing 状态
    if withdrawal.Status != "pending" && withdrawal.Status != "processing" {
        return ErrInvalidWithdrawalStatus
    }
    
    // 更新状态为失败
    _, err = tx.Exec(ctx, `
        UPDATE withdrawals 
        SET status = 'failed', failure_reason = $1, failed_at = NOW()
        WHERE id = $2
    `, reason, withdrawalID)
    if err != nil {
        return err
    }
    
    // 恢复用户余额
    _, err = tx.Exec(ctx, `
        UPDATE creator_profiles 
        SET balance = balance + $1, updated_at = NOW()
        WHERE user_id = $2
    `, withdrawal.Amount, withdrawal.UserID)
    if err != nil {
        return err
    }
    
    return tx.Commit(ctx)
}
```

### 4. 数据库表

```sql
-- 000006_withdrawals.up.sql
CREATE TABLE withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount DECIMAL(10, 2) NOT NULL,
    fee DECIMAL(10, 2) NOT NULL,
    net_amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    payout_method VARCHAR(50),
    payout_details JSONB,
    failure_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    failed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_withdrawals_user ON withdrawals(user_id);
CREATE INDEX idx_withdrawals_status ON withdrawals(status);
CREATE INDEX idx_withdrawals_created ON withdrawals(created_at);
```

## 遇到的问题

### 问题 1: 余额扣减竞态条件

**描述**: 并发提现可能导致余额为负

**解决方案**:
使用条件更新确保余额充足

```go
result, err := tx.Exec(ctx, `
    UPDATE creator_profiles 
    SET balance = balance - $1
    WHERE user_id = $2 AND balance >= $1  -- 条件检查
`, amount, userID)

if result.RowsAffected() == 0 {
    return nil, ErrInsufficientBalance
}
```

### 问题 2: 失败恢复重复执行

**描述**: 同一提现失败被多次恢复

**解决方案**:
使用状态检查和行锁

```go
// 使用 FOR UPDATE 锁定行
err = tx.QueryRow(ctx, `
    SELECT ... FROM withdrawals WHERE id = $1 FOR UPDATE
`, withdrawalID).Scan(...)

// 检查状态
if withdrawal.Status != "pending" && withdrawal.Status != "processing" {
    return ErrInvalidWithdrawalStatus
}
```

### 问题 3: 手续费计算精度

**描述**: 浮点数计算导致精度丢失

**解决方案**:
使用 Decimal 库进行精确计算

```go
import "github.com/shopspring/decimal"

fee := amount.Mul(s.config.FeePercentage)
fee = fee.Round(2) // 保留两位小数
```

### 问题 4: 提现状态不一致

**描述**: 外部支付系统状态与本地不同步

**解决方案**:
实现状态同步机制

```go
// 定时任务同步状态
func (s *WithdrawalService) SyncWithdrawalStatus(ctx context.Context) error {
    // 获取处理中的提现
    rows, err := s.db.Query(ctx, `
        SELECT id, payout_id FROM withdrawals 
        WHERE status = 'processing' AND processed_at < NOW() - INTERVAL '1 hour'
    `)
    // ... 查询外部系统状态并更新 ...
}
```

## 验证结果

- [x] 最低金额验证正确
- [x] 最高金额验证正确
- [x] 余额不足被拒绝
- [x] 手续费计算正确
- [x] 失败恢复余额正确

## 属性测试

**Withdrawal Threshold Properties**
- 低于最低金额被拒绝 ✅
- 高于最高金额被拒绝 ✅
- 余额不足被拒绝 ✅

**Withdrawal Fee Properties**
- 手续费 >= 最低手续费 ✅
- 手续费 = max(金额 * 2.5%, $0.50) ✅

**Withdrawal Failure Recovery Properties**
- 失败后余额恢复 ✅
- 重复恢复被拒绝 ✅

## 相关文件

- `backend/internal/withdrawal/withdrawal.go`
- `backend/internal/withdrawal/withdrawal_property_test.go`
- `backend/internal/models/withdrawal.go`
- `backend/migrations/000006_withdrawals.up.sql`
