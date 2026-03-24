# 中间件指南

本指南详细介绍 wechat-robot-go SDK 的中间件系统，包括内置中间件的使用和自定义中间件的编写方法。

## 中间件概念

中间件（Middleware）是一种包装 `MessageHandler` 的函数，用于在消息处理前后添加横切关注点（cross-cutting concerns），如日志、错误恢复、权限检查等。

### 核心类型定义

```go
// MessageHandler 是消息处理函数类型
type MessageHandler func(ctx context.Context, msg *Message) error

// Middleware 是中间件函数类型
type Middleware func(MessageHandler) MessageHandler
```

### 工作原理

中间件的本质是 **函数包装器**：

```go
func MyMiddleware() Middleware {
    return func(next MessageHandler) MessageHandler {
        return func(ctx context.Context, msg *Message) error {
            // 前置逻辑（消息到达时执行）
            fmt.Println("before handling")
            
            // 调用下一个处理器
            err := next(ctx, msg)
            
            // 后置逻辑（处理完成后执行）
            fmt.Println("after handling")
            
            return err
        }
    }
}
```

## 内置中间件

### WithRecovery

Panic 恢复中间件，防止 handler 中的 panic 崩溃整个程序：

```go
import "log/slog"

bot := wechat.NewBot()
bot.Use(wechat.WithRecovery(slog.Default()))
```

**功能**：
- 捕获 handler 中的 panic
- 记录 panic 信息和堆栈跟踪
- 返回错误而非中断程序
- 确保消息轮询继续运行

**日志输出示例**：

```
ERROR handler panic recovered from_user_id=user@im.wechat panic="runtime error: index out of range" stack="goroutine 1 [running]:..."
```

### WithLogging

请求日志中间件，记录每条消息的处理情况：

```go
bot := wechat.NewBot()
bot.Use(wechat.WithLogging(slog.Default()))
```

**功能**：
- 记录收到的消息信息
- 记录处理结果（成功或错误）
- 提供调试和监控能力

**日志输出示例**：

```
INFO handling message from_user_id=user@im.wechat item_count=1
WARN handler returned error from_user_id=user@im.wechat error="some error"
```

## Chain 函数

`Chain` 用于组合多个中间件：

```go
// 语法
combined := wechat.Chain(m1, m2, m3)

// 等价于
// m1(m2(m3(handler)))
```

**执行顺序**：

```
请求 → m1.before → m2.before → m3.before → handler → m3.after → m2.after → m1.after → 响应
```

**示例**：

```go
// 组合多个中间件
chain := wechat.Chain(
    wechat.WithRecovery(logger),  // 最外层：确保 panic 被捕获
    wechat.WithLogging(logger),   // 中间层：记录日志
    RateLimiter(10, time.Minute), // 内层：限流检查
)

// 应用到 handler
wrappedHandler := chain(myHandler)

// 或者直接使用 Use
bot.Use(
    wechat.WithRecovery(logger),
    wechat.WithLogging(logger),
    RateLimiter(10, time.Minute),
)
```

## 自定义中间件

### 基本模板

```go
func MyMiddleware() wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            // === 前置逻辑 ===
            // 在这里进行检查、修改 context、记录开始时间等
            
            // === 调用下一个处理器 ===
            err := next(ctx, msg)
            
            // === 后置逻辑 ===
            // 在这里进行清理、记录结果等
            
            return err
        }
    }
}
```

### 带参数的中间件

```go
func MyMiddleware(param1 string, param2 int) wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            // 使用 param1 和 param2
            fmt.Printf("param1=%s, param2=%d\n", param1, param2)
            return next(ctx, msg)
        }
    }
}

// 使用
bot.Use(MyMiddleware("hello", 42))
```

### 带状态的中间件

```go
func StatefulMiddleware() wechat.Middleware {
    // 状态变量（闭包捕获）
    var (
        mu      sync.Mutex
        counter int
    )
    
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            mu.Lock()
            counter++
            currentCount := counter
            mu.Unlock()
            
            fmt.Printf("Message #%d\n", currentCount)
            return next(ctx, msg)
        }
    }
}
```

## 高级示例

### Rate Limiter 中间件

限制每个用户的消息频率：

```go
func RateLimiter(limit int, window time.Duration) wechat.Middleware {
    var (
        mu       sync.Mutex
        requests = make(map[string][]time.Time)
    )
    
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            mu.Lock()
            defer mu.Unlock()
            
            userID := msg.FromUserID
            now := time.Now()
            
            // 清理过期请求记录
            var valid []time.Time
            for _, t := range requests[userID] {
                if now.Sub(t) < window {
                    valid = append(valid, t)
                }
            }
            
            // 检查是否超限
            if len(valid) >= limit {
                slog.Warn("rate limit exceeded",
                    "user_id", userID,
                    "limit", limit,
                    "window", window,
                )
                return nil // 静默丢弃，或返回 errors.New("rate limit exceeded")
            }
            
            // 记录本次请求
            requests[userID] = append(valid, now)
            
            return next(ctx, msg)
        }
    }
}

// 使用：每分钟最多 10 条消息
bot.Use(RateLimiter(10, time.Minute))
```

### Auth 中间件（白名单）

只允许特定用户使用 Bot：

```go
func AuthMiddleware(allowedUsers map[string]bool) wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            if !allowedUsers[msg.FromUserID] {
                slog.Warn("unauthorized user blocked",
                    "user_id", msg.FromUserID,
                )
                return nil // 静默忽略
            }
            return next(ctx, msg)
        }
    }
}

// 使用
allowed := map[string]bool{
    "user1@im.wechat": true,
    "user2@im.wechat": true,
    "admin@im.wechat": true,
}
bot.Use(AuthMiddleware(allowed))
```

### 消息过滤中间件

根据消息内容进行过滤：

```go
func MessageFilter(keywords []string) wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            text := msg.Text()
            
            // 检查是否包含敏感词
            for _, kw := range keywords {
                if strings.Contains(text, kw) {
                    slog.Info("message filtered",
                        "user_id", msg.FromUserID,
                        "keyword", kw,
                    )
                    return nil // 丢弃消息
                }
            }
            
            return next(ctx, msg)
        }
    }
}

// 使用
bot.Use(MessageFilter([]string{"spam", "广告", "推销"}))
```

### 重试中间件

在 handler 失败时自动重试：

```go
func RetryMiddleware(maxRetries int, delay time.Duration) wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            var lastErr error
            
            for i := 0; i <= maxRetries; i++ {
                if i > 0 {
                    slog.Info("retrying handler",
                        "attempt", i+1,
                        "user_id", msg.FromUserID,
                    )
                    time.Sleep(delay)
                }
                
                lastErr = next(ctx, msg)
                if lastErr == nil {
                    return nil // 成功
                }
                
                // 检查是否是可重试的错误
                if !isRetryable(lastErr) {
                    return lastErr
                }
            }
            
            return fmt.Errorf("max retries exceeded: %w", lastErr)
        }
    }
}

func isRetryable(err error) bool {
    // 自定义判断逻辑，例如网络错误可重试
    return errors.Is(err, context.DeadlineExceeded) ||
           strings.Contains(err.Error(), "timeout")
}

// 使用：最多重试 3 次，每次间隔 1 秒
bot.Use(RetryMiddleware(3, time.Second))
```

### 耗时统计中间件

统计每条消息的处理时间：

```go
func TimingMiddleware(logger *slog.Logger) wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            start := time.Now()
            
            err := next(ctx, msg)
            
            duration := time.Since(start)
            logger.Info("message processed",
                "user_id", msg.FromUserID,
                "duration_ms", duration.Milliseconds(),
                "success", err == nil,
            )
            
            return err
        }
    }
}
```

### Context 注入中间件

向 context 中注入额外数据：

```go
type contextKey string

const UserInfoKey contextKey = "user_info"

type UserInfo struct {
    ID       string
    Nickname string
    Level    int
}

func UserInfoMiddleware(userDB map[string]UserInfo) wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            // 从数据库加载用户信息
            info, exists := userDB[msg.FromUserID]
            if !exists {
                info = UserInfo{ID: msg.FromUserID, Level: 0}
            }
            
            // 注入到 context
            ctx = context.WithValue(ctx, UserInfoKey, info)
            
            return next(ctx, msg)
        }
    }
}

// 在 handler 中使用
func myHandler(ctx context.Context, msg *wechat.Message) error {
    info := ctx.Value(UserInfoKey).(UserInfo)
    fmt.Printf("User %s (Level %d) said: %s\n", info.Nickname, info.Level, msg.Text())
    return nil
}
```

## 最佳实践

### 1. 中间件顺序

推荐的中间件注册顺序：

```go
bot.Use(
    // 1. 最外层：Panic 恢复（确保任何 panic 都被捕获）
    wechat.WithRecovery(logger),
    
    // 2. 日志记录
    wechat.WithLogging(logger),
    
    // 3. 认证/授权
    AuthMiddleware(allowedUsers),
    
    // 4. 限流
    RateLimiter(10, time.Minute),
    
    // 5. 业务相关（内容过滤等）
    MessageFilter(keywords),
)
```

### 2. 避免阻塞

中间件应尽量轻量，避免长时间阻塞：

```go
// ❌ 不好：在中间件中进行耗时操作
func BadMiddleware() wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            time.Sleep(5 * time.Second) // 阻塞所有消息
            return next(ctx, msg)
        }
    }
}

// ✅ 好：异步处理或使用 goroutine
func GoodMiddleware() wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            go asyncOperation(msg) // 异步执行
            return next(ctx, msg)
        }
    }
}
```

### 3. 正确处理 context

尊重 context 的取消信号：

```go
func ContextAwareMiddleware() wechat.Middleware {
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            // 检查 context 是否已取消
            select {
            case <-ctx.Done():
                return ctx.Err()
            default:
            }
            
            return next(ctx, msg)
        }
    }
}
```

### 4. 线程安全

如果中间件维护状态，必须确保线程安全：

```go
func ThreadSafeMiddleware() wechat.Middleware {
    var (
        mu    sync.Mutex  // 或使用 sync.RWMutex
        state map[string]int
    )
    state = make(map[string]int)
    
    return func(next wechat.MessageHandler) wechat.MessageHandler {
        return func(ctx context.Context, msg *wechat.Message) error {
            mu.Lock()
            state[msg.FromUserID]++
            mu.Unlock()
            
            return next(ctx, msg)
        }
    }
}
```

## 完整示例

综合使用多个中间件的完整示例：

```go
package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/SpellingDragon/wechat-robot-go/wechat"
)

func main() {
    logger := slog.Default()
    
    bot := wechat.NewBot(wechat.WithLogger(logger))

    // 注册中间件链
    bot.Use(
        wechat.WithRecovery(logger),
        wechat.WithLogging(logger),
        RateLimiter(10, time.Minute),
        AuthMiddleware(map[string]bool{
            "admin@im.wechat": true,
        }),
    )

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    if err := bot.Login(ctx, func(qr string) {
        fmt.Println(qr)
    }); err != nil {
        fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
        os.Exit(1)
    }

    bot.OnMessage(func(ctx context.Context, msg *wechat.Message) error {
        return bot.Reply(ctx, msg, "Hello! "+msg.Text())
    })

    bot.Run(ctx)
}
```
