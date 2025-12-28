# 错误修复 007: Goroutine 泄漏

**日期**: 2024-12-28  
**严重程度**: 高  
**状态**: ✅ 已修复

## 问题描述

长时间运行后内存持续增长，发现大量 goroutine 未正确退出。

## 错误信息

```
runtime: goroutine stack exceeds 1000000000-byte limit
fatal error: stack overflow
```

## 诊断方法

```go
// 添加 pprof 端点
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// 查看 goroutine 数量
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

## 根本原因

1. HTTP 请求未设置超时
2. Channel 未正确关闭
3. Context 取消未处理
4. 定时器未停止

## 解决方案

### 1. HTTP 客户端超时

```go
// 错误: 无超时
client := &http.Client{}

// 正确: 设置超时
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

### 2. Context 取消处理

```go
func processRequest(ctx context.Context) error {
    resultCh := make(chan Result)
    errCh := make(chan error)
    
    go func() {
        result, err := doWork()
        if err != nil {
            errCh <- err
            return
        }
        resultCh <- result
    }()
    
    select {
    case <-ctx.Done():
        return ctx.Err() // 正确处理取消
    case err := <-errCh:
        return err
    case result := <-resultCh:
        return processResult(result)
    }
}
```

### 3. Channel 正确关闭

```go
func producer(ctx context.Context) <-chan int {
    ch := make(chan int)
    
    go func() {
        defer close(ch) // 确保关闭
        
        for i := 0; ; i++ {
            select {
            case <-ctx.Done():
                return // 响应取消
            case ch <- i:
            }
        }
    }()
    
    return ch
}
```

### 4. 定时器清理

```go
func periodicTask(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop() // 确保停止
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            doTask()
        }
    }
}

// 一次性定时器
func delayedTask() {
    timer := time.NewTimer(5 * time.Second)
    defer timer.Stop() // 即使提前返回也要停止
    
    select {
    case <-timer.C:
        doTask()
    case <-cancel:
        return
    }
}
```

### 5. 流式响应处理

```go
func handleStream(ctx context.Context, stream io.ReadCloser) error {
    defer stream.Close() // 确保关闭
    
    reader := bufio.NewReader(stream)
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        line, err := reader.ReadBytes('\n')
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        processLine(line)
    }
}
```

### 6. Worker Pool 模式

```go
type WorkerPool struct {
    workers int
    jobs    chan Job
    done    chan struct{}
}

func (p *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < p.workers; i++ {
        go func() {
            for {
                select {
                case <-ctx.Done():
                    return
                case <-p.done:
                    return
                case job := <-p.jobs:
                    job.Process()
                }
            }
        }()
    }
}

func (p *WorkerPool) Stop() {
    close(p.done)
}
```

## 验证

```go
func TestNoGoroutineLeak(t *testing.T) {
    before := runtime.NumGoroutine()
    
    // 执行测试操作
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    doSomething(ctx)
    
    // 等待 goroutine 退出
    time.Sleep(100 * time.Millisecond)
    
    after := runtime.NumGoroutine()
    
    if after > before+1 {
        t.Errorf("goroutine leak: before=%d, after=%d", before, after)
    }
}
```

## 预防措施

1. 所有 HTTP 请求设置超时
2. 使用 context 控制 goroutine 生命周期
3. defer 关闭 channel 和定时器
4. 定期检查 goroutine 数量
5. 使用 goleak 检测泄漏
