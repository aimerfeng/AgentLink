# 错误修复 006: 金额精度丢失

**日期**: 2024-12-28  
**严重程度**: 高  
**状态**: ✅ 已修复

## 问题描述

使用 float64 处理金额时出现精度丢失，导致计费不准确。

## 错误示例

```go
// 问题代码
price := 0.1 + 0.2
fmt.Println(price) // 输出: 0.30000000000000004

// JSON 序列化问题
type Agent struct {
    PricePerCall float64 `json:"price_per_call"`
}
// 输入: {"price_per_call": 0.001}
// 实际存储: 0.0009999999999999998
```

## 根本原因

1. IEEE 754 浮点数无法精确表示某些十进制数
2. JSON 序列化/反序列化时精度丢失
3. 数据库 DECIMAL 与 Go float64 转换问题

## 解决方案

### 1. 使用 Decimal 库

```go
import "github.com/shopspring/decimal"

type Agent struct {
    PricePerCall decimal.Decimal `json:"price_per_call"`
}

// 创建 Decimal
price := decimal.NewFromFloat(0.001)
price := decimal.NewFromString("0.001")

// 运算
total := price.Mul(decimal.NewFromInt(100))
fee := total.Mul(decimal.NewFromFloat(0.025))

// 保留精度
fee = fee.Round(6) // 保留 6 位小数
```

### 2. 数据库存储

```sql
-- 使用 DECIMAL 类型
price_per_call DECIMAL(10, 6) NOT NULL,
fee DECIMAL(10, 6) NOT NULL,
```

### 3. JSON 序列化

```go
// shopspring/decimal 自动处理 JSON 序列化
type Payment struct {
    Amount decimal.Decimal `json:"amount"`
}

// 输出: {"amount": "9.99"}
// 注意: 序列化为字符串以保持精度
```

### 4. 数据库扫描

```go
import "github.com/jackc/pgx/v5/pgtype"

// 使用 pgtype.Numeric 扫描
var numeric pgtype.Numeric
err := row.Scan(&numeric)

// 转换为 decimal.Decimal
price := decimal.NewFromBigInt(numeric.Int, numeric.Exp)
```

### 5. 比较操作

```go
// 错误: 直接比较可能因精度问题失败
if price == 0.001 { ... }

// 正确: 使用 Decimal 比较
if price.Equal(decimal.NewFromFloat(0.001)) { ... }

// 或使用容差比较
if price.Sub(expected).Abs().LessThan(decimal.NewFromFloat(0.000001)) { ... }
```

## 验证

```go
func TestDecimalPrecision(t *testing.T) {
    // 测试加法精度
    a := decimal.NewFromFloat(0.1)
    b := decimal.NewFromFloat(0.2)
    sum := a.Add(b)
    
    expected := decimal.NewFromFloat(0.3)
    if !sum.Equal(expected) {
        t.Errorf("expected %s, got %s", expected, sum)
    }
    
    // 测试乘法精度
    price := decimal.NewFromFloat(0.001)
    quantity := decimal.NewFromInt(1000)
    total := price.Mul(quantity)
    
    expectedTotal := decimal.NewFromInt(1)
    if !total.Equal(expectedTotal) {
        t.Errorf("expected %s, got %s", expectedTotal, total)
    }
}
```

## 预防措施

1. 所有金额使用 `decimal.Decimal`
2. 数据库使用 `DECIMAL` 类型
3. API 返回金额为字符串
4. 避免 float64 与 Decimal 混用
