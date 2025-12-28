# 错误修复 008: 并发 Map 写入崩溃

**日期**: 2024-12-28  
**严重程度**: 严重  
**状态**: ✅ 已修复

## 问题描述

高并发场景下应用崩溃，错误指向 map 并发写入。

## 错误信息

```
fatal error: concurrent map writes

goroutine 123 [running]:
runtime.throw(0x...)
    /usr/local/go/src/runtime/panic.go:1198 +0x71
runtime.mapassign_faststr(0x...)
    /usr/local/go/src/runtime/map_faststr.go:203 +0x3f1
```

## 根本原因

Go 的 map 不是并发安全的，多个 goroutine 同时读写会导致崩溃。

## 问题代码

```go
// 错误: 并发不安全
type Cache struct {
    data map[string]interface{}
}

func (c *Cache) Set(key string, value interface{}) {
    c.data[key] = value // 并发写入崩溃
}

func (c *Cache) Get(key string) interface{} {
    return c.data[key] // 并发读取可能崩溃
}
```

## 解决方案

### 方案 1: sync.RWMutex

```go
type Cache struct {
    mu   sync.RWMutex
    data map[string]interface{}
}

func (c *Cache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = value
}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    v, ok := c.data[key]
    return v, ok
}

func (c *Cache) Delete(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.data, key)
}
```

### 方案 2: sync.Map

```go
type Cache struct {
    data sync.Map
}

func (c *Cache) Set(key string, value interface{}) {
    c.data.Store(key, value)
}

func (c *Cache) Get(key string) (interface{}, bool) {
    return c.data.Load(key)
}

func (c *Cache) Delete(key string) {
    c.data.Delete(key)
}

func (c *Cache) Range(f func(key, value interface{}) bool) {
    c.data.Range(f)
}
```

### 方案 3: 分片锁（高并发场景）

```go
const shardCount = 32

type ShardedCache struct {
    shards [shardCount]*CacheShard
}

type CacheShard struct {
    mu   sync.RWMutex
    data map[string]interface{}
}

func NewShardedCache() *ShardedCache {
    c := &ShardedCache{}
    for i := 0; i < shardCount; i++ {
        c.shards[i] = &CacheShard{
            data: make(map[string]interface{}),
        }
    }
    return c
}

func (c *ShardedCache) getShard(key string) *CacheShard {
    hash := fnv32(key)
    return c.shards[hash%shardCount]
}

func (c *ShardedCache) Set(key string, value interface{}) {
    shard := c.getShard(key)
    shard.mu.Lock()
    defer shard.mu.Unlock()
    shard.data[key] = value
}

func (c *ShardedCache) Get(key string) (interface{}, bool) {
    shard := c.getShard(key)
    shard.mu.RLock()
    defer shard.mu.RUnlock()
    v, ok := shard.data[key]
    return v, ok
}

func fnv32(key string) uint32 {
    hash := uint32(2166136261)
    for i := 0; i < len(key); i++ {
        hash *= 16777619
        hash ^= uint32(key[i])
    }
    return hash
}
```

### 方案选择指南

| 场景 | 推荐方案 |
|------|---------|
| 读多写少 | sync.RWMutex |
| 读写均衡 | sync.Map |
| 高并发大量 key | 分片锁 |
| 简单场景 | sync.Mutex |

## 验证

```go
func TestConcurrentAccess(t *testing.T) {
    cache := NewCache()
    
    var wg sync.WaitGroup
    
    // 并发写入
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            cache.Set(fmt.Sprintf("key%d", i), i)
        }(i)
    }
    
    // 并发读取
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            cache.Get(fmt.Sprintf("key%d", i))
        }(i)
    }
    
    wg.Wait()
}
```

## 检测工具

```bash
# 使用 race detector
go test -race ./...

# 运行时检测
go run -race main.go
```

## 预防措施

1. 始终使用 `-race` 标志测试
2. 共享 map 必须加锁
3. 考虑使用 sync.Map
4. 高并发场景使用分片锁
