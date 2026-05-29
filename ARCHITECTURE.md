# code-detector 程序架构与运作流程图谱

> 基于源码分析构建，描绘 `code-detector v0.5` 多编程语言函数扫描工具的完整运作流程。

---

## 一、整体架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                        code-detector                               │
│              多编程语言函数扫描工具 (Go 语言实现)                     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
          ┌─────────────────────────┼─────────────────────────┐
          │                         │                         │
          ▼                         ▼                         ▼
   ┌─────────────┐          ┌──────────────┐          ┌──────────────┐
   │ 入口层       │          │ 核心扫描层    │          │ 数据持久层    │
   │ cmd/scanner │──┬──────▶│ internal/    │──┬──────▶│ internal/db  │
   │ /main.go    │  │       │ fscanner/    │  │       │ /store.go    │
   └─────────────┘  │       └──────────────┘  │       │ /schema.go   │
                    │                         │       └──────────────┘
                    │                         │
                    │       ┌──────────────┐  │
                    ├──────▶│ 解析器层      │  │
                    │       │ internal/    │  │
                    │       │ parser/      │  │
                    │       └──────────────┘  │
                    │                         │
                    │       ┌──────────────┐  │
                    └──────▶│ 分析器层      │  │
                            │ internal/    │  │
                            │ analyzer/    │  │
                            └──────────────┘  │
                                              │
                    ┌──────────────┐          │
                    │ 配置层       │──────────┘
                    │ internal/   │
                    │ config/     │
                    └──────────────┘
```

---

## 二、分层模块调用流程图

### 2.1 完整调用链路

```
 main()
   │
   ├─ parseFlags()           ← 解析 CLI 参数 (-lang, -db, -verbose, -graph 等)
   │
   ├─ resolveProjectRoot()   ← 获取扫描目标路径（命令行参数或当前目录）
   │
   ├─ initProjectRoot()      ← 路径安全检查（拒绝扫描系统关键目录）
   │
   ├─ initConfig()           ← 加载 config.yaml（或内置默认10种语言配置）
   │   └─ config.LoadConfig() / DefaultConfig()
   │
   ├─ initDB()               ← 初始化 SQLite 数据库
   │   └─ db.InitDB() / db.NewStore()
   │
   ├─ initScanner()          ← 创建扫描器实例（核心装配过程）
   │   │
   │   ├─ parser.NewRegistry()           ← 创建解析器注册表
   │   ├─ reg.Register(NewGoParser(), ".go")         ← 注册 Go 解析器
   │   ├─ reg.Register(NewPythonParser(), ".py")     ← 注册 Python 解析器
   │   ├─ reg.Register(NewJavaParser(), ".java")     ← 注册 Java 解析器
   │   ├─ reg.Register(NewKotlinParser(), ".kt", ...)
   │   ├─ reg.Register(NewJavascriptParser(), ".js", ...)
   │   ├─ reg.Register(NewTypescriptParser(), ".ts", ...)
   │   ├─ reg.Register(NewCSharpParser(), ".cs")
   │   ├─ reg.Register(NewCPPParser(), ".cpp", ".c", ...)
   │   ├─ reg.Register(NewRustParser(), ".rs")
   │   ├─ reg.Register(NewRubyParser(), ".rb")
   │   ├─ 对配置中的额外语言注册 GenericParser  ← 数据驱动的通用正则解析器
   │   └─ fscanner.New(cfg, reg, store)              ← 组装 Scanner 对象
   │
   ├─ printBanner()          ← 打印启动 UI 横幅
   │
   ├─ scan.Scan(projectRoot) ← ★ 核心扫描入口 ★
   │   │
   │   ├─ Store.CreateSession()           ← 创建扫描会话（DB记录）
   │   │   └─ pruneOldSessionsLocked()    ← 清理旧会话（保留最近3个）
   │   │
   │   ├─ collectFiles()                 ← ★ 文件收集阶段 ★
   │   │   │
   │   │   ├─ filepath.Walk()            ← 递归遍历目录树
   │   │   ├─ 跳过: SkipDirs / 隐藏目录 / 临时目录 / %环境变量%目录
   │   │   ├─ 跳过: 超限大文件 (>1MB)
   │   │   ├─ 过滤: Registry.GetByExt()  ← 只保留有对应解析器的扩展名
   │   │   ├─ 过滤: LangFilter           ← 按 -lang 参数过滤
   │   │   ├─ 去重: os.SameFile()        ← 硬链接去重
   │   │   └─ 增量: Store.GetFileCache() ← 增量模式检查 mtime 缓存
   │   │
   │   ├─ parseConcurrently()            ← ★ 并发解析阶段 ★
   │   │   │
   │   │   ├─ 启动 N 个 worker goroutine (默认 CPU核数, ≤16)
   │   │   ├─ jobs channel ← 文件路径队列
   │   │   ├─ results channel ← 解析结果队列
   │   │   │
   │   │   └─ 每个 worker:
   │   │       └─ parseFile(path)
   │   │           │
   │   │           ├─ Registry.GetByExt()        ← 按扩展名获取解析器
   │   │           ├─ os.ReadFile()              ← 读取文件内容
   │   │           ├─ detectAndConvertEncoding() ← ★ 编码检测与转换 ★
   │   │           │   ├─ 检测 BOM (UTF-8/UTF-16 LE/BE)
   │   │           │   ├─ 检测 UTF-16 无 BOM（启发式）
   │   │           │   ├─ utf8.Valid() 检查
   │   │           │   └─ 非 UTF-8 回退（GBK等标记）
   │   │           │
   │   │           ├─ detectBinaryAfterEncoding() ← 二次检测是否为二进制
   │   │           │
   │   │           ├─ parser.Parse(path, content) ← ★ 语言解析器 ★
   │   │           │   │
   │   │           │   ├─ 创建 FileLines (行偏移表)
   │   │           │   ├─ makeCommentMask()        ← 标记注释行
   │   │           │   ├─ makeStringMask()         ← 标记字符串行
   │   │           │   ├─ 行迭代 + 正则匹配函数定义
   │   │           │   ├─ matchBrace()             ← 花括号匹配找函数体
   │   │           │   └─ extractCallStats()       ← 提取函数内部调用
   │   │           │
   │   │           └─ parser.Globals(path, content) ← 提取全局变量
   │   │
   │   ├─ result.Functions = allFunctions
   │   ├─ result.GlobalVars = allGlobals
   │   │
   │   └─ writeResults()                  ← ★ 批量写入数据库 ★
   │       │
   │       ├─ Store.BatchInsertFunctions()   ← 批量写入函数（含去重哈希）
   │       │   └─ Store.BatchInsertDeps()    ← 写入函数调用依赖关系
   │       │
   │       ├─ Store.BatchInsertGlobalVars()  ← 批量写入全局变量
   │       └─ Store.UpdateSession()          ← 更新会话统计
   │
   ├─ printResults(result)   ← 打印扫描结果 UI 表格
   │
   ├─ printCallGraph()       ← (仅 -graph 模式) 构建并输出调用图
   │   └─ analyzer.BuildCallGraph()
   │       ├─ 查询 DB 获取全部函数
   │       ├─ 构建 FuncNode 图（限定名索引）
   │       ├─ 填充 Callers / Callees 关系
   │       ├─ FindHotFunctions()    ← 被调用最多的函数
   │       └─ FindOrphanFunctions() ← 未被调用的函数（死代码检测）
   │
   └─ store.Close()          ← 关闭 DB（自动 checkpoint WAL）
```

### 2.2 编码检测流程（detectAndConvertEncoding）

```
                    ┌──────────────┐
                    │  原始 content │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        有 BOM?       UTF-16 无BOM?    UTF-8 有效?
         │              │              │
    ┌────┴────┐    ┌────┴────┐    ┌────┴────┐
    │UTF-8 BOM│    │UTF-16LE│    │ 是 → 直接返回  │
    │→ 去前3字 │    │→ 解码  │    └─────────┘
    │UTF-16LE │    │UTF-16BE│
    │UTF-16BE │    │→ 解码  │
    └─────────┘    └─────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ 非 UTF-8 编码 │
                    │ (GBK等启发式) │
                    └──────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ 二进制检测失败 │
                    │ → 返回 error  │
                    └──────────────┘
```

---

## 三、项目包依赖树

```
code-detector/
│
├── cmd/scanner/
│   └── main.go              ← 入口：CLI解析、初始化、调度
│       ├── imports: flag, fmt, os, sort, path/filepath, strings
│       ├── imports: internal/analyzer      ← 调用图分析
│       ├── imports: internal/config        ← 语言配置
│       ├── imports: internal/db            ← 数据库操作
│       ├── imports: internal/fscanner      ← 核心扫描器
│       ├── imports: internal/model         ← 数据模型
│       └── imports: internal/parser        ← 语言解析器注册表
│
├── internal/
│   │
│   ├── config/
│   │   └── config.go         ← 语言配置加载（YAML / 内置默认）
│   │       ├── LoadConfig()              ← 加载YAML，不存在则用默认
│   │       ├── DefaultConfig()           ← 内置10种语言配置
│   │       ├── GetLanguageConfigsMap()   ← 扩展名→配置索引
│   │       └── GetLanguageByName()       ← 语言名→配置查找
│   │
│   ├── model/
│   │   └── model.go          ← 数据模型定义
│   │       ├── LanguageConfig            ← 语言配置结构
│   │       ├── Function                  ← 函数定义
│   │       │   ├── ID, SessionID, Name, PackageName
│   │       │   ├── Language, FilePath, LineStart, LineEnd
│   │       │   ├── Body, Dependencies, Hash
│   │       │   └── CallCount, NestingDepth
│   │       ├── GlobalVariable            ← 全局变量定义
│   │       ├── ScanSession               ← 扫描会话
│   │       ├── ScanResult                ← 扫描结果聚合
│   │       └── CallStats                 ← 调用统计
│   │
│   ├── fscanner/
│   │   └── scanner.go        ← ★ 核心扫描引擎 ★
│   │       ├── Scanner                  ← 扫描器结构体
│   │       ├── New()                    ← 构造函数
│   │       ├── Scan()                   ← 主入口：收集→解析→写入
│   │       ├── collectFiles()           ← 递归遍历收集文件
│   │       ├── parseConcurrently()      ← 并发解析
│   │       ├── parseFile()              ← 单文件解析（编码检测+解析器）
│   │       ├── writeResults()           ← 批量写入数据库
│   │       ├── detectAndConvertEncoding() ← 编码检测
│   │       ├── decodeUTF16LE/BE()       ← UTF-16解码
│   │       ├── detectUTF16()            ← UTF-16启发式检测
│   │       ├── detectBinaryAfterEncoding() ← 二进制检测
│   │       └── isLikelyTextEncoding()   ← 文本编码启发式判断
│   │
│   ├── parser/
│   │   ├── parser.go         ← Parser 接口 + Registry 注册表
│   │   │   ├── Parser interface          ← Parse() / Globals()
│   │   │   ├── Registry                  ← 扩展名→解析器映射
│   │   │   ├── NewRegistry()
│   │   │   ├── Register()               ← 注册解析器
│   │   │   ├── GetByExt()               ← 按扩展名查找
│   │   │   ├── GetByLang()              ← 按语言名查找
│   │   │   └── SupportedExts()
│   │   │
│   │   ├── go_parser.go      ← Go 语言解析器
│   │   │   ├── GoParser                 ← 结构体实现 Parser 接口
│   │   │   ├── Parse()                  ← 正则匹配 func 定义 + matchBrace
│   │   │   ├── Globals()                ← 提取 var/const 声明
│   │   │   ├── makeCommentMask()        ← 标记注释行
│   │   │   ├── makeStringMask()         ← 标记字符串行
│   │   │   ├── makeGoFuncBodyMask()     ← 标记函数体行（用于排除局部变量）
│   │   │   ├── extractGoPackageName()   ← 提取包名
│   │   │   └── goKeywords               ← Go 关键字集（过滤非调用）
│   │   │
│   │   ├── python_parser.go  ← Python 解析器
│   │   ├── java_parser.go    ← Java 解析器
│   │   ├── cpp_parser.go     ← C/C++ 解析器
│   │   ├── csharp_parser.go  ← C# 解析器
│   │   ├── javascript_parser.go ← JavaScript/TypeScript 解析器
│   │   ├── rust_parser.go    ← Rust 解析器
│   │   ├── ruby_parser.go    ← Ruby 解析器
│   │   ├── generic_parser.go ← 通用正则解析器（数据驱动）
│   │   │
│   │   ├── match_brace.go    ← 花括号匹配器
│   │   ├── filelines.go      ← 行偏移表（O(1)行定位）
│   │   └── callstats.go      ← 函数调用统计
│   │
│   ├── analyzer/
│   │   └── analyzer.go       ← 调用图分析器
│   │       ├── Analyzer                 ← 分析器
│   │       ├── CallGraph                ← 调用图结构
│   │       ├── FuncNode                 ← 函数节点（含 Callers/Callees）
│   │       ├── BuildCallGraph()         ← 从 DB 构建调用图
│   │       ├── FindHotFunctions()       ← 查找热点函数
│   │       └── FindOrphanFunctions()    ← 查找未调用函数（死代码）
│   │
│   └── db/
│       ├── schema.go         ← 数据库表结构定义 + 迁移
│       ├── store.go          ← CRUD 操作
│       │   ├── CreateSession()          ← 创建扫描会话
│       │   ├── BatchInsertFunctions()   ← 批量写入函数（含去重）
│       │   ├── BatchInsertGlobalVars()  ← 批量写入全局变量
│       │   ├── BatchInsertDeps()        ← 批量写入依赖关系
│       │   ├── QueryDependencies()      ← 查询函数依赖
│       │   ├── UpdateSession()          ← 更新会话统计
│       │   ├── GetFileCache()           ← 增量模式缓存查询
│       │   ├── UpsertFileCache()        ← 更新文件缓存
│       │   ├── Checkpoint()             ← WAL checkpoint
│       │   └── Close()                  ← 关闭数据库
│       └── schema.go
│           ├── InitDB()                 ← 初始化+迁移
│           └── 表结构:
│               ├── scan_sessions        ← 扫描会话
│               ├── functions            ← 函数定义
│               ├── function_deps        ← 函数调用依赖
│               ├── global_variables     ← 全局变量
│               └── file_cache           ← 增量缓存
│
├── config.yaml              ← 用户自定义语言配置（可选）
├── Makefile / build.bat     ← 构建脚本
└── scaned_db/
    └── scan_result.db       ← SQLite 数据库（运行时生成）
```

---

## 四、主要函数树（按调用层级）

### 4.1 入口层 (cmd/scanner/main.go)

```
main()
├── parseFlags()                        解析命令行标志
├── resolveProjectRoot()                解析项目根路径
├── initProjectRoot()                   路径安全检查
│   └── validateScanRoot()              验证是否系统目录
├── initConfig()                        加载语言配置
│   └── config.LoadConfig()
├── initDB()                            初始化数据库
│   ├── db.InitDB()
│   └── db.NewStore()
├── initScanner()                       ★ 装配核心
│   ├── parser.NewRegistry()            创建解析器注册表
│   ├── parser.NewGoParser() → Register 10种语言解析器
│   ├── parser.NewGenericParser()       加载配置中的额外语言
│   └── fscanner.New()                  构造 Scanner
├── printBanner()                       打印启动横幅
├── scan.Scan()                         ★ 核心扫描
├── printResults()                      打印结果表格
├── printCallGraph()                    调用图分析（可选）
│   └── analyzer.New() / BuildCallGraph()
└── store.Close()                       关闭数据库
```

### 4.2 扫描引擎层 (internal/fscanner/scanner.go)

```
Scanner.Scan(root)                      ★ 扫描主流程
├── Scanner.collectFiles()              文件收集
│   ├── filepath.Walk()                 目录递归遍历
│   ├── 目录过滤：SkipDirs / 隐藏目录 / %TEMP%
│   ├── 大小过滤：MaxFileSize
│   ├── 扩展名过滤：Registry.GetByExt()
│   ├── 语言过滤：LangFilter
│   ├── 硬链接去重：os.SameFile()
│   └── 增量缓存：Store.GetFileCache()
│
├── Scanner.parseConcurrently()         并发解析
│   ├── goroutine workers (N = CPU核数)
│   └── per worker: Scanner.parseFile(path)
│       ├── Registry.GetByExt()         获取解析器
│       ├── os.ReadFile()               读取文件
│       ├── detectAndConvertEncoding()  编码检测/转换
│       │   ├── detectUTF16()
│       │   ├── decodeUTF16LE/BE()
│       │   └── isLikelyTextEncoding()
│       ├── detectBinaryAfterEncoding() 二进制检测
│       ├── Parser.Parse()              ★ 解析函数
│       └── Parser.Globals()            提取全局变量
│
└── Scanner.writeResults()              写入结果
    ├── Store.BatchInsertFunctions()    批量写函数
    ├── Store.BatchInsertDeps()         写依赖关系
    ├── Store.BatchInsertGlobalVars()   写全局变量
    └── Store.UpdateSession()           更新会话
```

### 4.3 Go 解析器层 (internal/parser/go_parser.go)

```
GoParser.Parse(path, content)
├── NewFileLinesFromBytes()             构建行偏移表
├── makeCommentMask()                   标记注释行
├── makeStringMask()                    标记字符串行
├── extractGoPackageName()              提取包名
├── 行迭代 → goFuncRegex 匹配          正则匹配 func 定义
│   ├── 跳过注释/字符串/空行
│   └── 匹配失败 → 匿名函数/导入路径中的"func"
├── matchBrace(text, offset)            ★ 花括号匹配
│   └── 返回匹配的 } 位置
├── extractCallStats()                  提取内部调用
│   ├── 行内正则匹配函数调用
│   ├── 过滤 Go 关键字/isAllUpper
│   └── 统计 CallCount / NestingDepth
└── 返回 []*Function

GoParser.Globals(path, content)
├── makeCommentMask()                   标记注释
├── makeStringMask()                    标记字符串
├── makeGoFuncBodyMask()                标记函数体内行
├── varDeclRegex 匹配 var/const 声明
└── 返回 []*GlobalVariable
```

### 4.4 花括号匹配 (internal/parser/match_brace.go)

```
matchBrace(text, openPos)               ★ 从 { 找匹配的 }
│
│  输入: text(源码全文), openPos(指向'{'的字节偏移)
│  输出: 匹配到的'}'字节偏移, 或 error
│
├── 初始化 stack=0, 字符串/注释状态=false
├── 从 openPos 逐字节扫描:
│   ├── 跳过: 转义字符
│   ├── 跳过: 字符串内容 ("", '', ``)
│   ├── 跳过: 注释 (//, /* */)
│   ├── '{' → stack++
│   └── '}' → stack--; if stack==0 → return 位置
└── 未找到匹配 → 返回 error
```

### 4.5 调用图分析器 (internal/analyzer/analyzer.go)

```
Analyzer.BuildCallGraph(sessionID)
├── 查询 DB 获取本会话全部函数
├── 构建 Nodes map (限定名索引)
├── 查询每个函数的依赖 → 填充 Callers/Callees
│   ├── 优先尝试限定名匹配 (pkg.Name)
│   └── 回退普通名匹配
└── 返回 *CallGraph
    ├── FindHotFunctions(N)     ← Top N 被调用最多
    └── FindOrphanFunctions()   ← 未被调用的函数
```

---

## 五、数据流图

```
┌──────────┐    CLI参数     ┌───────────┐
│  用户输入 │──────────────▶│ main.go   │
│  cmd/目录 │               │ 入口调度   │
└──────────┘               └─────┬─────┘
                                 │
                    ┌────────────┴────────────┐
                    │                         │
                    ▼                         ▼
           ┌─────────────────┐     ┌──────────────────┐
           │ config.LoadConfig│     │  db.InitDB()     │
           │ → 语言设置       │     │ → SQLite 初始化   │
           └────────┬────────┘     └────────┬─────────┘
                    │                       │
                    ▼                       ▼
           ┌─────────────────────────────────────────┐
           │        fscanner.New(cfg, reg, store)     │
           │        创建 Scanner 实例                  │
           └────────────────┬────────────────────────┘
                            │
                            ▼
           ┌─────────────────────────────────────────┐
           │         Scanner.Scan(root)              │
           │                                         │
           │  1. collectFiles()                      │
           │     ┌─────────────────────────┐         │
           │     │  filepath.Walk()        │         │
           │     │  → 过滤扩展名/大小/目录  │         │
           │     │  → 硬链接去重           │         │
           │     │  → 增量缓存检查         │         │
           │     └──────────┬──────────────┘         │
           │                ▼                        │
           │  2. parseConcurrently()                 │
           │     ┌─────────────────────────┐         │
           │     │  Worker 1: parseFile()  │         │
           │     │  Worker 2: parseFile()  │  ...    │
           │     │  Worker N: parseFile()  │         │
           │     └──────────┬──────────────┘         │
           │                ▼                        │
           │  3. writeResults()                      │
           │     ┌─────────────────────────┐         │
           │     │ BatchInsertFunctions()  │         │
           │     │ BatchInsertDeps()       │         │
           │     │ BatchInsertGlobalVars() │         │
           │     │ UpdateSession()         │         │
           │     └──────────┬──────────────┘         │
           └────────────────┼────────────────────────┘
                            │
                            ▼
           ┌─────────────────────────────────────────┐
           │  printResults() / printCallGraph()      │
           │  → 终端 UI 输出                          │
           │  → (可选) 调用图构建 + 死代码检测          │
           └─────────────────────────────────────────┘
```

---

## 六、并发模型

```
                    ┌──────────────┐
                    │  主 goroutine │
                    │  Scan()      │
                    └──────┬───────┘
                           │
              ┌────────────┴────────────┐
              │                         │
              ▼                         ▼
     ┌────────────────┐       ┌──────────────────┐
     │ collectFiles() │       │ jobs channel     │
     │ → files []str  │──────▶│ (缓冲=文件数)     │
     └────────────────┘       └────────┬─────────┘
                                       │
                    ┌──────────────────┼──────────────────┐
                    │                  │                  │
                    ▼                  ▼                  ▼
            ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
            │  Worker 1    │  │  Worker 2    │  │  Worker N    │
            │ parseFile()  │  │ parseFile()  │  │ parseFile()  │
            └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
                   │                 │                 │
                   └─────────────────┼─────────────────┘
                                     │
                                     ▼
                            ┌────────────────┐
                            │ results channel│
                            │ (缓冲=文件数)   │
                            └───────┬────────┘
                                    │
                                    ▼
                            ┌────────────────┐
                            │ 聚合 allFuncs  │
                            │ allGlobals     │
                            └────────────────┘
```

---

## 七、支持的语言与对应解析器

| 语言 | 解析器 | 扩展名 | 策略 |
|------|--------|--------|------|
| Go | `GoParser` (专用) | `.go` | 花括号匹配 |
| Python | `PythonParser` (专用) | `.py` | 缩进匹配 |
| Java | `JavaParser` (专用) | `.java` | 花括号匹配 |
| Kotlin | `KotlinParser` (专用) | `.kt`, `.kts` | 花括号匹配 |
| JavaScript | `JavascriptParser` (专用) | `.js`, `.jsx`, `.mjs` | 花括号匹配 |
| TypeScript | `TypescriptParser` (专用) | `.ts`, `.tsx` | 花括号匹配 |
| C# | `CSharpParser` (专用) | `.cs` | 花括号匹配 |
| C/C++ | `CPPParser` (专用) | `.cpp`, `.c`, `.h`, `.hpp` 等 | 花括号匹配 |
| Rust | `RustParser` (专用) | `.rs` | 花括号匹配 |
| Ruby | `RubyParser` (专用) | `.rb` | `end` 关键字匹配 |
| 其他 | `GenericParser` (通用) | 由 `config.yaml` 定义 | 数据驱动正则 |

---

## 八、关键设计决策

1. **解析器接口化** — 每种语言实现 `Parser` 接口，注册到 `Registry`，便于扩展新语言
2. **数据驱动后备** — 对于没有专用解析器的语言，可用 `config.yaml` 配置正则表达式，`GenericParser` 兜底
3. **编码自适应** — 自动检测 UTF-8/UTF-16 BOM、无 BOM UTF-16 启发式检测、GBK 等非 UTF-8 回退
4. **并发模型** — 生产者-消费者模式：主 goroutine 收集文件后通过 channel 分发，N 个 worker 并发解析
5. **增量扫描** — 通过文件 mtime 缓存跳过未变更文件，适合大型项目的重复扫描
6. **去重机制** — 函数内容 SHA256 哈希去重 + 硬链接 `os.SameFile()` 去重
7. **会话管理** — 自动清理旧会话（保留最近3个），防止数据库无限膨胀
8. **WAL 模式** — SQLite WAL 模式提升并发写入性能，扫描完成时主动 checkpoint
