# code-detector v0.8

## 🎉 核心变更

### AST 解析器重写
- 用 **tree-sitter** AST 替换 6 个正则行解析器（Go/Python/Java/JavaScript/C++/Rust/Ruby/C#）
- 函数检测准确率提升 **53%**（Go: 217 → 333 函数）
- 新增 `treesitter_go.go` 和 `treesitter_all.go`（多语言通用框架）
- 保留 Kotlin/TypeScript 旧解析器（无 tree-sitter grammar）

### 数据库增强
- Functions 表新增 **10 个 AST 字段**：

  | 字段 | 说明 |
  |------|------|
  | `parameters` | 参数列表 `(a int, b string)` |
  | `return_types` | 返回类型 `(int, error)` |
  | `receiver` | 方法接收器 `(s *Server)` |
  | `is_method` | 是否为方法 |
  | `visibility` | public / private |
  | `cyclomatic` | 圈复杂度 |
  | `parameter_count` | 参数个数 |
  | `return_count` | return 语句数 |
  | `statement_count` | 语句数 |
  | `anonymous_funcs` | 匿名函数数 |

- 新增 `file_metrics` 表（15 维文件级指标）
- 新增 `type_defs` 表（结构化类型定义）

### 新查询命令

| 命令 | 说明 | 示例 |
|------|------|------|
| `-query complexity=N` | 圈复杂度 TOP N | `-query complexity=5` |
| `-query params=N` | 参数数量 ≥ N | `-query params=5` |
| `-query anon` | 含匿名函数的函数 | `-query anon` |
| `-query files` | 文件级统计 | `-query files` |
| `-query types` | 类型定义列表 | `-query types` |

## 🔧 其他改进

- **CGO 构建支持** — 添加 MinGW-w64 自动检测
- **跳过目录黑名单** — 扩展至 32 个通用依赖/缓存目录
- **全局变量修复** — 增加 `parent == source_file` 校验，排除函数内部局部变量
- **build.bat 更新** — 自动检测 MinGW-w64 路径
- **CI/CD** — 添加 GitHub Actions 自动构建工作流
- **版本号** — v0.8

## 📦 下载说明

| 文件 | 平台 | 说明 |
|------|------|------|
| `code-detector` | Linux | Ubuntu 编译，需 GCC（通常预装） |
| `code-detector.exe` | Windows | 运行时无需 MinGW，仅构建时需要 |

### 从源码构建

```bash
# Linux / macOS
git clone https://github.com/jiransj/code-detector.git
cd code-detector
make build

# Windows (需 MinGW-w64)
git clone https://github.com/jiransj/code-detector.git
cd code-detector
build.bat
```

---

**完整文档**: [README.md](https://github.com/jiransj/code-detector/blob/main/README.md)
