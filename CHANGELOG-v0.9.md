# code-detector v0.9

## 🎉 核心变更

### MCP 协议支持（新功能）
- 新增 **MCP (Model Context Protocol) 服务器模式**，通过 `-mcp` 标志启动
- 基于 `github.com/mark3labs/mcp-go v0.54.1` 实现标准 MCP/stdio 传输
- AI 客户端（如 Claude Desktop）可直接与 code-detector 交互查询数据

### 16 个 MCP Tool 端点

| 工具名 | 功能 | 对应 CLI 查询 |
|--------|------|-------------|
| `get_summary` | 数据库概要统计 | summary |
| `list_functions` | 函数列表（可按语言筛选） | functions |
| `get_function` | 函数详情（签名/复杂度/调用） | func=NAME |
| `get_function_body` | 函数体源码 | （扩展） |
| `list_variables` | 全局变量列表（可按语言筛选） | vars |
| `analyze_deps` | 调用关系统计 | deps |
| `find_callers` | 查询谁调用了指定函数 | calls=NAME |
| `find_dead_code` | 死代码检测 | dead |
| `find_missing_deps` | 缺失依赖检测 | missing |
| `top_functions` | 按行数排序 TOP N | top=N |
| `deep_nesting` | 深层嵌套函数检测 | deep=N |
| `high_complexity` | 高圈复杂度函数 TOP N | complexity=N |
| `many_params` | 参数数量超标检测 | params=N |
| `find_anonymous` | 含匿名函数的函数检测 | anon |
| `file_metrics` | 文件级统计信息 | files |
| `list_types` | 类型定义列表（可按 kind 筛选） | types |

### 6 个 MCP Resource URI

| URI | 内容 | 类型 |
|-----|------|------|
| `db://summary` | 数据库概要 | JSON |
| `db://functions` | 全量函数列表 | JSON |
| `db://variables` | 全局变量列表 | JSON |
| `db://files` | 文件级统计 | JSON |
| `db://types` | 类型定义 | JSON |
| `db://sessions/latest` | 最近扫描会话 | JSON |

## 🔧 其他改进

- **版本号** — v0.9
- **零侵入设计** — MCP 层作为独立 `internal/mcp/` 包，不修改现有 Store/Query 代码
- **单二进制发布** — 同一 `code-detector.exe` 同时支持 CLI 扫描和 MCP 服务两种模式

## 📦 使用方式

```bash
# 原有 CLI 扫描模式
code-detector -lang go -verbose ./myproject

# 新增 MCP 服务器模式（供 Claude Desktop 等 AI 客户端使用）
code-detector -mcp -db myproject.db
```

## ⚡ 文件变更

```
新增:
  internal/mcp/server.go      — MCP Server 核心
  internal/mcp/tools_def.go   — 16 个 Tool 定义 + handlers
  internal/mcp/resources.go   — 6 个 Resource URI

修改:
  cmd/code-detector/main.go   — 新增 -mcp 标志
  go.mod / go.sum              — 新增 mcp-go 依赖
  README.md / README_EN.md     — 版本更新
```
