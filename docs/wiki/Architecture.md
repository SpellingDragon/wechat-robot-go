# 架构设计

本文档描述 wechat-robot-go SDK 的架构设计、包结构和依赖关系。

## 包结构

```
wechat/
├── internal/
│   ├── crypto/       # AES-128-ECB 加解密（零内部依赖）
│   ├── model/        # 消息数据类型 + MessageHandler（零内部依赖）
│   ├── store/        # ContextTokenStore 接口 + 文件/内存实现（零内部依赖）
│   ├── middleware/   # 中间件框架（依赖 model/）
│   ├── text/         # 智能文本分片算法（零内部依赖）
│   └── media/        # CDN 上传下载 + 消息构建器（依赖 crypto/ + model/）
├── auth.go           # 登录认证
├── bot.go            # Bot 门面 API
├── client.go         # HTTP 客户端
├── config.go         # 配置加载
├── errors.go         # 错误类型
├── message_send.go   # 消息发送
├── message_send_media.go  # 媒体消息发送
├── options.go        # Functional Options
├── poller.go         # 长轮询
├── text_split.go     # SendLongText
├── typing.go         # Typing 状态
└── types.go          # Type aliases（向后兼容）
```

## 依赖关系图

SDK 采用严格的单向依赖设计，确保**零循环依赖**：

```
┌─────────────────────────────────────────────────────────────────┐
│                         wechat/ (主包)                          │
│  bot.go, auth.go, poller.go, client.go, message_send.go ...   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ 依赖
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                       internal/ 子包                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│   │ crypto/  │  │  model/  │  │  store/  │  │  text/   │       │
│   │ (零依赖) │  │ (零依赖) │  │ (零依赖) │  │ (零依赖) │       │
│   └────┬─────┘  └────┬─────┘  └──────────┘  └──────────┘       │
│        │             │                                          │
│        │             │                                          │
│        ▼             ▼                                          │
│   ┌─────────────────────────────┐   ┌──────────────────┐       │
│   │         media/              │   │   middleware/    │       │
│   │   (依赖 crypto/ + model/)   │   │  (依赖 model/)   │       │
│   └─────────────────────────────┘   └──────────────────┘       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 依赖方向总结

| 包 | 依赖 | 被依赖 |
|---|------|--------|
| `crypto/` | 无 | `media/` |
| `model/` | 无 | `media/`, `middleware/` |
| `store/` | 无 | `wechat/` |
| `text/` | 无 | `wechat/` |
| `middleware/` | `model/` | `wechat/` |
| `media/` | `crypto/`, `model/` | `wechat/` |

## 分层设计

SDK 采用三层架构：

### 低级层（Zero Dependencies）

位于 `internal/` 中的基础包，不依赖任何其他内部包：

| 包 | 职责 |
|---|------|
| `crypto/` | AES-128-ECB 加解密，处理媒体文件的安全传输 |
| `model/` | 定义核心数据类型：Message、MessageItem、MessageHandler |
| `store/` | ContextTokenStore 接口定义，提供文件存储和内存存储实现 |
| `text/` | 智能文本分片算法，在自然边界处切分长文本 |

### 中级层（Internal Dependencies Only）

依赖低级层的功能包：

| 包 | 职责 | 依赖 |
|---|------|------|
| `middleware/` | 中间件框架，实现 Chain、WithRecovery、WithLogging | `model/` |
| `media/` | CDN 上传下载、消息构建器 | `crypto/`, `model/` |

### 高级层（Public API）

`wechat/` 主包，作为对外的门面 API：

| 文件 | 职责 |
|------|------|
| `bot.go` | Bot 结构体，整合所有功能的入口点 |
| `auth.go` | QR 码登录流程和凭证管理 |
| `poller.go` | 长轮询实现，消息接收引擎 |
| `client.go` | HTTP 客户端封装，处理 API 调用 |
| `message_send.go` | 文本消息发送 |
| `message_send_media.go` | 媒体消息发送 |
| `typing.go` | Typing 状态管理 |
| `options.go` | Functional Options 配置模式 |
| `types.go` | Type aliases，提供向后兼容 |

## 设计原则

### 1. internal/ 隔离

所有实现细节放在 `internal/` 目录下，外部无法直接导入：

```go
// ❌ 外部代码无法这样导入
import "github.com/SpellingDragon/wechat-robot-go/wechat/internal/crypto"

// ✅ 只能通过主包的公开 API 使用
import "github.com/SpellingDragon/wechat-robot-go/wechat"
```

### 2. Type Alias 向后兼容

`types.go` 通过类型别名暴露内部类型，保证 API 稳定性：

```go
// types.go
package wechat

import (
    "github.com/SpellingDragon/wechat-robot-go/wechat/internal/middleware"
    "github.com/SpellingDragon/wechat-robot-go/wechat/internal/model"
    "github.com/SpellingDragon/wechat-robot-go/wechat/internal/store"
)

// Type aliases for backward compatibility
type Message = model.Message
type MessageHandler = model.MessageHandler
type Middleware = middleware.Middleware
type ContextTokenStore = store.ContextTokenStore

// Function aliases
var Chain = middleware.Chain
var WithRecovery = middleware.WithRecovery
var WithLogging = middleware.WithLogging
```

好处：
- 内部重构不影响外部代码
- 保持简洁的导入路径
- 外部只需 `import "...../wechat"`

### 3. 零外部依赖

整个 SDK 仅使用 Go 标准库，无第三方依赖：

```go
// 使用标准库的 slog 替代 logrus/zap
import "log/slog"

// 使用标准库的 crypto/aes
import "crypto/aes"

// 使用标准库的 net/http
import "net/http"
```

优势：
- 减少依赖冲突
- 更小的二进制体积
- 更好的稳定性

### 4. Functional Options 模式

配置采用 Functional Options 模式，提供灵活且向后兼容的 API：

```go
// 可选配置
bot := wechat.NewBot(
    wechat.WithBaseURL("https://custom.example.com"),
    wechat.WithTokenFile("custom-token.json"),
    wechat.WithLogger(customLogger),
)

// 全部使用默认值也可以
bot := wechat.NewBot()
```

## 组件交互

```
                     ┌─────────────┐
                     │   User      │
                     └──────┬──────┘
                            │
                            ▼
┌───────────────────────────────────────────────────────────────────┐
│                           Bot                                     │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────────┐ │
│   │  Auth   │  │ Poller  │  │ Typing  │  │   MediaManager      │ │
│   └────┬────┘  └────┬────┘  └────┬────┘  └──────────┬──────────┘ │
└────────┼────────────┼───────────┼───────────────────┼────────────┘
         │            │           │                   │
         │            │           │                   │
         ▼            ▼           ▼                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Client                                   │
│                    (HTTP Infrastructure)                         │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │   iLink Bot API     │
                    │ (weixin.qq.com)     │
                    └─────────────────────┘
```

### 组件职责

| 组件 | 职责 |
|------|------|
| **Bot** | 顶层门面，整合所有功能模块 |
| **Auth** | 处理 QR 码登录、凭证持久化和刷新 |
| **Poller** | 长轮询 `/ilink/bot/getupdates`，接收消息 |
| **TypingManager** | 管理 "正在输入" 状态和 ticket 缓存 |
| **MediaManager** | 处理文件加解密、CDN 上传下载 |
| **Client** | 底层 HTTP 封装，处理认证头、错误解析 |

## 消息流程

### 接收消息

```
iLink API → Poller → Middlewares → MessageHandler → User Code
                        │
                        ├── WithRecovery (panic 恢复)
                        ├── WithLogging (日志记录)
                        └── Custom Middlewares
```

### 发送消息

```
User Code → Bot.Reply/SendText → message_send.go → Client → iLink API
```

### 媒体处理

```
Upload:  Data → AES Encrypt → CDN Upload → MediaItem
Download: URL → CDN Download → AES Decrypt → Data
```
