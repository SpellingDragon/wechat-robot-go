# 贡献指南

感谢您对 wechat-robot-go 的关注！欢迎提交 Issue 和 Pull Request。

## 开发环境

- Go 1.21 或更高版本
- Git

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/SpellingDragon/wechat-robot-go.git
cd wechat-robot-go

# 安装依赖
go mod download

# 运行测试
go test ./...

# 运行示例
go run ./examples/echo
```

## 代码规范

### 格式化

使用 `gofmt` 格式化代码：

```bash
gofmt -w .
```

### Lint

使用 `golangci-lint` 进行代码检查：

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run
```

### 测试

- 所有新功能必须添加单元测试
- 测试覆盖率目标：80%+
- 使用 table-driven 测试模式

```bash
# 运行所有测试
go test ./... -v

# 查看覆盖率
go test ./... -cover

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 提交规范

### Commit Message

使用 Conventional Commits 格式：

```
<type>(<scope>): <description>

[optional body]
```

类型：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建/工具链相关

示例：
```
feat(bot): add typing indicator support
fix(client): handle session expiration correctly
docs(readme): add AI agent example
```

### Pull Request

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feature/my-feature`
3. 提交更改：`git commit -m "feat: add some feature"`
4. 推送分支：`git push origin feature/my-feature`
5. 创建 Pull Request

PR 检查清单：
- [ ] 代码通过 `gofmt` 格式化
- [ ] 代码通过 `golangci-lint` 检查
- [ ] 添加了必要的测试
- [ ] 所有测试通过
- [ ] 更新了相关文档

## 项目结构

```
wechat/
├── auth.go           # 登录认证
├── bot.go            # Bot 核心逻辑
├── client.go         # HTTP 客户端
├── crypto.go         # 加密解密
├── media.go          # 媒体文件处理
├── message.go        # 消息结构
├── message_send.go   # 消息发送
├── polling.go        # 长轮询
├── token_store.go    # Token 存储
├── types.go          # 类型定义
└── *_test.go         # 测试文件

examples/
├── echo/             # Echo 示例
└── ai-agent/         # AI Agent 示例
```

## 行为准则

- 尊重所有贡献者
- 保持专业和友好的交流
- 接受建设性批评

## 问题反馈

- 使用 GitHub Issues 报告 Bug
- 提供详细的复现步骤和环境信息
- 标注合适的标签（bug, enhancement, question 等）

## 许可证

本项目采用 MIT 许可证。贡献的代码将以相同许可证授权。
