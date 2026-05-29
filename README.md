# code-detector

**code-detector** 是一个以go语言编写，跨平台、多编程语言的函数扫描工具。它可以递归扫描指定项目目录，自动识别源文件中的函数/方法定义，记录其行号范围、函数体、调用依赖等信息，并将结果存入 SQLite 数据库以供后续分析。
**功能：** 
探测所有代码中的函数 全局变量 自动注册进数据库
**优势：** 
从函数角度审查项目的健壮性，函数的合理程度，是否重复造轮子
排除无关上下文干扰，对code agent具有良好支持辅助作用

当前版本：**v0.5**

---

## 支持的语言与文件扩展名

| 语言 | 内部名称 | 文件扩展名 | 解析器 |
|------|----------|-----------|--------|
| **Go** | `go` | `.go` | 专用解析器 |
| **Python** | `python` | `.py` | 专用解析器 |
| **Java** | `java` | `.java` | 专用解析器 |
| **Kotlin** | `kotlin` | `.kt`, `.kts` | 专用解析器（复用 Java 解析器） |
| **JavaScript** | `javascript` | `.js`, `.jsx`, `.mjs` | 专用解析器 |
| **TypeScript** | `typescript` | `.ts`, `.tsx` | 专用解析器（复用 JS 解析器） |
| **C#** | `csharp` | `.cs` | 专用解析器 |
| **C/C++** | `cpp` | `.cpp`, `.cxx`, `.cc`, `.c`, `.h`, `.hpp` | 专用解析器 |
| **Rust** | `rust` | `.rs` | 专用解析器 |
| **Ruby** | `ruby` | `.rb` | 专用解析器 |
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

**指定输出数据库路径：**

```cmd
code-detector -db ./output/my_scan.db -verbose ./myproject
```

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
| `-verbose` | 输出详细日志，显示扫描进度和注册信息 |
| `-graph` | 扫描完成后构建调用关系图并输出统计摘要 |
| `-incremental` | 增量扫描模式：仅重新解析 mtime（修改时间）发生变化的文件 |
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

扫描结果默认存储在 `scaned_db/scan_result.db`（SQLite 数据库），包含以下核心信息：

- **函数名** 与方法签名
- **所属语言** 与源文件路径
- **起始行号** 与结束行号
- **函数体源码**
- **调用依赖**（调用了哪些其他函数）
- **调用次数** 与 **嵌套深度**

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
