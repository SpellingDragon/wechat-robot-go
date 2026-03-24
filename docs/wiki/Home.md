# wechat-robot-go

基于腾讯官方 iLink Bot API 的微信机器人 Go SDK。

## 项目简介

**wechat-robot-go** 是一个简洁、高效的 WeChat Bot Go SDK，封装了腾讯 iLink Bot API 的所有核心功能。本 SDK 采用零外部依赖设计，仅使用 Go 标准库，提供开箱即用的机器人开发体验。

## 核心特性

- **扫码登录** - 凭证自动持久化，支持断点续用
- **消息收发** - 长轮询接收 + HTTP 发送，实时可靠
- **context_token 持久化** - 支持主动发送消息和进程重启恢复
- **Typing 状态** - 实现"对方正在输入中"效果
- **媒体文件** - AES-128-ECB 加密 + CDN 上传/下载
- **智能文本分片** - 自动分割长文本，优先自然边界切分
- **中间件支持** - 可组合的中间件链（日志、panic 恢复、自定义）
- **优雅关闭** - 等待进行中的 handler 完成
- **线程安全** - 支持并发消息处理

## 快速导航

| 页面 | 说明 |
|------|------|
| [[Home]] | 首页（当前页面） |
| [[Architecture]] | 架构设计与包结构 |
| [[Middleware-Guide]] | 中间件详细指南 |

## 安装

```bash
go get github.com/SpellingDragon/wechat-robot-go
```

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/SpellingDragon/wechat-robot-go/wechat"
)

func main() {
    bot := wechat.NewBot()

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // 扫码登录
    err := bot.Login(ctx, func(qrCode string) {
        fmt.Println("请使用微信扫描二维码:")
        fmt.Println(qrCode)
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "登录失败: %v\n", err)
        os.Exit(1)
    }

    // 注册消息处理器
    bot.OnMessage(func(ctx context.Context, msg *wechat.Message) error {
        text := msg.Text()
        if text == "" {
            return nil
        }
        return bot.Reply(ctx, msg, "Echo: "+text)
    })

    // 启动 Bot
    fmt.Println("Bot 已启动，按 Ctrl+C 停止")
    bot.Run(ctx)
}
```

## API 概览

### Bot 核心方法

| 方法 | 说明 |
|------|------|
| `NewBot(opts ...Option)` | 创建 Bot 实例 |
| `Login(ctx, onQRCode)` | 扫码登录 |
| `OnMessage(handler)` | 注册消息处理回调 |
| `Use(middlewares...)` | 注册中间件 |
| `Run(ctx)` | 启动长轮询循环 |
| `Stop()` | 停止消息轮询 |
| `Reply(ctx, msg, text)` | 回复消息 |
| `SendText(ctx, toUserID, text, contextToken)` | 发送文本 |
| `SendTextToUser(ctx, toUserID, text)` | 主动发送 |

### 配置选项

| Option | 说明 | 默认值 |
|--------|------|--------|
| `WithBaseURL(url)` | API 服务器地址 | `https://ilinkai.weixin.qq.com` |
| `WithTokenFile(path)` | 凭证文件路径 | `.weixin-token.json` |
| `WithContextTokenDir(dir)` | context_token 存储目录 | `.wechat-context-tokens` |
| `WithLogger(logger)` | 自定义日志 | `slog.Default()` |

## 关联资源

- **源码仓库**: [GitHub - SpellingDragon/wechat-robot-go](https://github.com/SpellingDragon/wechat-robot-go)
- **Go 文档**: [pkg.go.dev](https://pkg.go.dev/github.com/SpellingDragon/wechat-robot-go)
- **协议参考**: [iLink Bot API 协议分析](https://github.com/hao-ji-xing/openclaw-weixin/blob/main/weixin-bot-api.md)

## 许可证

MIT License
