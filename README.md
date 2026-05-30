<p align="center">
  <strong>🌐 语言 / Language</strong>
  &nbsp;&nbsp;|&nbsp;&nbsp;
  <strong>🇨🇳 中文</strong>
  &nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="README_EN.md">🇬🇧 English</a>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/version-v0.8-brightgreen.svg" alt="Version v0.8">
  <img src="https://img.shields.io/badge/go-1.26-blue.svg" alt="Go 1.26">
  <img src="https://img.shields.io/badge/platform-windows%20%7C%20linux-lightgrey.svg" alt="Platform">
</p>

---

# code-detector

**code-detector** 是一个以go语言编写，跨平台、多编程语言的函数扫描工具。它可以递归扫描指定项目目录，自动识别源文件中的函数/方法定义，记录其行号范围、函数体、调用依赖等信息，并将结果存入 SQLite 数据库以供后续分析。
## **功能：** 
探测所有代码中的函数 全局变量 自动注册进数据库
## **优势：** 
从函数角度审查项目的健壮性，函数的合理程度，是否重复造轮子
排除无关上下文干扰，对code agent具有良好支持辅助作用

当前版本：**v0.8**

---

## 支持的语言与文件扩展名

| 语言 | 内部名称 | 文件扩展名 | 解析器 |
|------|----------|-----------|--------|
| **Go** | `go` | `.go` | tree-sitter AST |
| **Python** | `python` | `.py` | tree-sitter AST |
| **Java** | `java` | `.java` | tree-sitter AST |
| **Kotlin** | `kotlin` | `.kt`, `.kts` | tree-sitter AST 🆕 |
| **Swift** | `swift` | `.swift` | tree-sitter AST 🆕 |
| **JavaScript** | `javascript` | `.js`, `.jsx`, `.mjs` | tree-sitter AST |
| **TypeScript** | `typescript` | `.ts`, `.tsx` | tree-sitter AST |
| **PHP** | `php` | `.php` | tree-sitter AST 🆕 |
| **Lua** | `lua` | `.lua` | tree-sitter AST 🆕 |
| **Scala** | `scala` | `.scala` | tree-sitter AST 🆕 |
| **C#** | `csharp` | `.cs` | tree-sitter AST |
| **C/C++** | `cpp` | `.cpp`, `.cxx`, `.cc`, `.c`, `.h`, `.hpp` | tree-sitter AST |
| **Rust** | `rust` | `.rs` | tree-sitter AST |
| **Ruby** | `ruby` | `.rb` | tree-sitter AST |
| **自定义语言** | — | 通过 `config.yaml` 扩展 | 通用正则解析器 |

> 通过 `config.yaml` 配置文件可以为任意扩展名注册自定义正则解析规则，支持单行注释和块注释过滤。

---

## 使用方法

### 基本用法

```cmd
code-detector [选项] <项目根目录>
```

不指定 `<项目根目录>` 时，默认扫描程序所在目录。

### 常用示例

**扫描所有支持的语言：**

```cmd
code-detector -verbose ./myproject
```

**仅扫描特定语言（逗号分隔）：**

```cmd
code-detector -lang go,java,python ./myproject
```

**指定语言名称列表：**

```cmd
code-detector -lang go,py,js,ts,rs -verbose ./src
```

`-lang` 参数接受语言内部名称或文件扩展名，例如：`go` / `py` / `java` / `js` / `ts` / `cs` / `cpp` / `rs` / `rb` / `kt`。

**排除目录并指定并发数：**

```cmd
code-detector -skip-dirs .git,bin,obj,node_modules -workers 8 ./myproject
```

**构建调用图并输出统计：**

```cmd
code-detector -graph ./myproject
```

**增量扫描（仅重新解析变更的文件）：**

```cmd
code-detector -incremental ./myproject
```

**一键扫描（跳过所有测试文件夹）：**

```cmd
scan.bat
```

项目根目录下提供了 `scan.bat`，双击即可执行带 `-skip-dirs testdata,testdata_extreme,tests,test,__tests__,node_modules,mock,mocks` 的扫描，自动跳过常见的测试/临时目录，适合日常快速扫描。

**指定输出数据库路径：**

```cmd
code-detector -db ./output/my_scan.db -verbose ./myproject
```

**调试模式（查看解析器跳过详情）：**

```cmd
code-detector -debug ./myproject
```

> `-debug` 模式会输出各语言解析器的内部调试信息（如括号匹配失败时跳过了哪个函数及其位置），便于排查解析器 bug。正常使用 `-verbose` 即可。

---

## 命令行选项

| 选项 | 说明 |
|------|------|
| `-lang <列表>` | 要扫描的语言，逗号分隔（如 `go,py,java`）。不指定则扫描所有支持语言 |
| `-db <路径>` | SQLite 数据库输出路径（默认 `scaned_db/scan_result.db`） |
| `-config <路径>` | 配置文件路径（默认 `config.yaml`） |
| `-max-size <N>` | 单文件最大字节数（默认 1MB），超过此大小的文件将被跳过 |
| `-skip-dirs <列表>` | 额外跳过的子目录名，逗号分隔（默认跳过 `.git`、`node_modules` 等） |
| `-workers <N>` | 并发工作协程数（默认等于 CPU 逻辑核心数） |
| `-verbose` | 输出详细日志，显示扫描进度、注册信息和解析器跳过记录 |
| `-debug` | 等同于 `-verbose`，并输出各语言解析器的调试信息（如括号匹配失败时的跳过详情），用于报告解析器 bug |
| `-graph` | 扫描完成后构建调用关系图并输出统计摘要 |
| `-incremental` | 增量扫描模式：仅重新解析 mtime（修改时间）发生变化的文件 |
| `-format` | 输出格式: `text`（默认）或 `json`（仅查询模式） | `-format json` |
| `-v` | 显示版本号 |

---

## 在 CMD 中使用

直接在终端中运行可执行文件：

```cmd
code-detector -lang go,python -verbose D:\projects\myapp
```

滚动输出扫描日志，完成后自动退出。如果双击运行（非交互式终端），程序不会暂停等待按键。

## 在 PowerShell 中使用

```powershell
.\code-detector.exe -lang go,js,ts -graph .\myproject
```

指定完整路径：

```powershell
& "D:\tools\code-detector.exe" -lang cpp,cs -workers 8 -verbose "D:\projects\myapp"
```

---

## 输出说明

扫描结果默认存储在 `scaned_db/scan_result.db`（SQLite 数据库），包含 **6 张表**，以下是完整的字段说明：

---

### `scan_sessions` — 扫描会话表

| DB 字段 | 中文说明 | 说明 |
|---------|---------|------|
| `id` | 会话 ID | 自增主键 |
| `project_root` | 项目根目录 | 被扫描的项目根路径 |
| `scan_time` | 扫描时间 | 扫描开始时间 |
| `duration_ms` | 扫描耗时(毫秒) | 扫描总耗时 |
| `file_count` | 扫描文件数 | 扫描的文件总数 |
| `func_count` | 函数总数 | 发现的函数/方法总数 |
| `var_count` | 全局变量总数 | 发现的全局变量/常量总数 |

---

### `functions` — 函数表

| DB 字段 | 中文说明 | 说明 |
|---------|---------|------|
| `id` | 函数 ID | 自增主键 |
| `session_id` | 所属会话 ID | 关联 `scan_sessions.id` |
| `name` | 函数名 | 函数/方法名称 |
| `package_name` | 包名/命名空间 | 所属包或命名空间（如 Go 的 package、Java 的 package、C# 的 namespace） |
| `language` | 编程语言 | 语言内部名称（如 `go`、`python`、`java`） |
| `file_path` | 文件路径 | 相对于项目根目录的路径 |
| `line_start` | 起始行号 | 函数定义起始行 |
| `line_end` | 结束行号 | 函数体结束行 |
| `body` | 函数体源码 | 函数的完整源代码 |
| `hash` | 内容哈希 | 函数内容 SHA256 哈希（前 16 字节，用于去重判断） |
| `call_count` | 调用总次数 | 函数内部调用次数（含重复调用同一函数） |
| `nesting_depth` | 嵌套深度 | 最大括号嵌套层级 |
| `parameters` | 参数列表 | 函数参数定义字符串，如 `(a int, b string)` |
| `return_types` | 返回类型 | 返回值类型，如 `(int, error)` |
| `receiver` | 接收器 | 方法接收器，如 `(s *Server)` |
| `is_method` | 是否为方法 | `1` 为方法，`0` 为普通函数 |
| `visibility` | 可见性 | `public` 或 `private`（基于首字母大小写） |
| `cyclomatic` | 圈复杂度 | McCabe 圈复杂度（if/for/switch/case/&&/|| 计数） |
| `parameter_count` | 参数个数 | 函数参数数量 |
| `return_count` | return 语句数 | 函数体中 return 语句的数量 |
| `statement_count` | 语句数 | 函数体中的语句总数 |
| `anonymous_funcs` | 匿名函数数 | 函数内部包含的匿名函数/闭包数量 |

---

### `function_deps` — 函数依赖关系表

| DB 字段 | 中文说明 | 说明 |
|---------|---------|------|
| `id` | 依赖 ID | 自增主键 |
| `caller_id` | 调用方函数 ID | 调用者函数 ID，关联 `functions.id` |
| `callee_name` | 被调用函数名 | 被调用的函数名称 |

---

### `global_vars` — 全局变量表

| DB 字段 | 中文说明 | 说明 |
|---------|---------|------|
| `id` | 变量 ID | 自增主键 |
| `session_id` | 所属会话 ID | 关联 `scan_sessions.id` |
| `name` | 变量名 | 变量/常量名称 |
| `var_type` | 变量类型 | 数据类型（如 `int`、`string`、`[]byte`） |
| `language` | 编程语言 | 语言内部名称 |
| `package_name` | 包名/命名空间 | 所属包或命名空间 |
| `visibility` | 可见性 | `public` 或 `private` |
| `file_path` | 文件路径 | 相对于项目根目录的路径 |
| `line_num` | 所在行号 | 变量定义的行号 |
| `is_const` | 是否为常量 | `1` 表示常量，`0` 表示变量 |
| `hash` | 内容哈希 | 变量内容哈希值（用于去重） |

---

### `file_cache` — 文件缓存表（增量扫描用）

| DB 字段 | 中文说明 | 说明 |
|---------|---------|------|
| `file_path` | 文件路径 | 主键，文件完整路径 |
| `mtime` | 修改时间戳 | 文件最后修改时间的 Unix 时间戳 |
| `hash` | 文件哈希 | 文件内容的 SHA256 哈希值 |
| `session_id` | 所属会话 ID | 关联 `scan_sessions.id` |

### `file_metrics` — 文件统计表（AST 增强）

| DB 字段 | 中文说明 | 说明 |
|---------|---------|------|
| `file_path` | 文件路径 | 相对于项目根目录的路径 |
| `language` | 编程语言 | 语言内部名称 |
| `total_lines` | 文件总行数 | 文件最大函数结束行 |
| `func_count` | 函数数量 | 文件中的函数/方法总数 |
| `type_count` | 类型定义数 | 文件中的结构体/接口等类型定义数 |
| `avg_cyclomatic` | 平均圈复杂度 | 文件内函数的平均圈复杂度 |
| `max_cyclomatic` | 最高圈复杂度 | 文件内函数的最大圈复杂度 |
| `total_parameters` | 参数总数 | 文件内所有函数的参数数量之和 |
| `max_parameters` | 最大参数数 | 文件内单函数的最大参数数量 |
| `total_returns` | return 总数 | 文件内所有函数的 return 语句数之和 |
| `total_statements` | 语句总数 | 文件内所有函数的语句数之和 |
| `total_anon_funcs` | 匿名函数总数 | 文件内所有函数包含的匿名函数数之和 |
| `public_funcs` | 公开函数数 | 公开（public）函数数量 |
| `private_funcs` | 私有函数数 | 私有（private）函数数量 |
| `methods_count` | 方法数 | 文件中方法（method）的数量 |

### `type_defs` — 类型定义表（AST 增强）

| DB 字段 | 中文说明 | 说明 |
|---------|---------|------|
| `name` | 类型名 | 类型定义名称 |
| `kind` | 类型种类 | 类型类别：`struct` / `interface` / `alias` / `enum` |
| `language` | 编程语言 | 语言内部名称 |
| `package_name` | 包名/命名空间 | 所属包或命名空间 |
| `file_path` | 文件路径 | 相对于项目根目录的路径 |
| `line_start` | 起始行号 | 类型定义起始行 |
| `line_end` | 结束行号 | 类型定义结束行 |
| `body` | 类型体源码 | 类型定义的完整源代码 |
| `fields` | 字段描述 | 结构化字段描述（JSON 格式） |

## 查询模式（`-query`）

无需重新扫描，直接读取已有 SQLite 数据库进行分析，支持对扫描结果的离线审计。

```cmd
code-detector -query <模式> [-db <数据库路径>] [-format text|json]
```

| 模式 | 说明 | 示例 |
|------|------|------|
| `summary` | 显示数据库概要（会话数、函数/变量总数、语言分布等） | `-query summary` |
| `functions` | 列出所有函数（不含函数体，按文件分组显示行号、调用次数） | `-query functions` |
| `func=NAME` | 查看函数详情（支持逗号批量: `func=A,B,C`，含依赖、调用方、函数体预览） | `-query func=main` |
| `vars` | 列出所有全局变量 | `-query vars` |
| `deps` | 调用统计：最热函数、死代码候选、调用分支最广的函数 | `-query deps` |
| `calls=NAME` | 查看哪些函数调用了指定函数 | `-query calls=Parse` |
| `dead` | 列出 call_count = 0 的潜在死代码 | `-query dead` |
| `missing` | 列出被调用但找不到定义的函数名（用于发现依赖缺失） | `-query missing` |
| `top=N` | 列出行数最多的 N 个函数（超大函数风险分析） | `-query top=10` |
| `deep=N` | 列出嵌套深度 >= N 的函数（复杂度分析） | `-query deep=3` |
| `complexity=N` | 🆕 列出圈复杂度最高的 N 个函数 | `-query complexity=5` |
| `params=N` | 🆕 列出参数数量 >= N 的函数 | `-query params=5` |
| `anon` | 🆕 列出包含匿名函数/闭包的函数 | `-query anon` |
| `files` | 🆕 文件级统计：函数数/圈复杂度/参数/return/可见性分布 | `-query files` |
| `types` | 🆕 列出所有类型定义（struct/interface） | `-query types` |

**批量查询示例** — 同时查看多个函数详情：
```cmd
code-detector -query func=main,Scan,InitDB
```

**JSON 输出示例** — 所有 `-query` 模式均支持：
```cmd
code-detector -query summary -format json
code-detector -query func=main -format json
code-detector -query top=5 -format json
```

启用 `-graph` 选项时，终端会输出调用关系统计摘要。

---

## 安全保护

- 程序内置了**系统关键目录保护**，拒绝扫描 Windows 系统盘根目录、`C:\Windows`、`/etc`、`/proc` 等系统目录，避免磁盘卡死或数据损坏。
- 提供退出前的按键等待（仅在交互式终端中触发），方便双击运行时查看结果。

---

## 配置文件

默认 `config.yaml` 为空（使用内置解析器），您可以按以下格式自定义语言解析规则：

```yaml
languages:
  - name: "my_lang"
    extensions: [".mylang"]
    function_regex: "func\\s+(?P<name>\\w+)\\s*\\("
    single_comment: ["//"]
    block_comment: [["/*", "*/"]]
```

> 如果某个扩展名已被内置解析器占用，配置中的自定义规则不会覆盖内置解析器。内置支持以外的扩展名才会使用自定义正则解析。

---

## 构建

### 在 Windows 下构建

```cmd
build.bat
```

### 使用 Makefile

```cmd
make build
```

构建产物为 `code-detector.exe`。

---

## 许可证

本项目基于 [MIT 许可证](LICENSE) 开源。

## 致谢

本项目使用了 [tree-sitter](https://tree-sitter.github.io/) — 强大的增量解析框架，
其核心及各语言语法解析器均基于 MIT 许可证发布。

- tree-sitter © 2018 Max Brunsfeld — [MIT](https://github.com/tree-sitter/tree-sitter/blob/master/LICENSE)
- go-tree-sitter © 2019 Maxim Sukharev — [MIT](https://github.com/smacker/go-tree-sitter/blob/master/LICENSE)
