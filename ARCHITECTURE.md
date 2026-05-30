# code-detector 程序架构与运作流程图谱

> 基于源码分析构建，描绘 `code-detector v0.9` 多编程语言函数扫描工具的完整运作流程。
> 以下数据均来自 `-query` 系列指令对 `scan_result.db` 的实际提取（211 函数 / 50 全局变量 / 544 依赖关系）。

---

## 一、整体架构总览

```
┌───────────────────────────────────────────────────────────────────────────┐
│                            code-detector v0.9                            │
│              多编程语言函数扫描 + 查询工具 (Go 语言实现)                     │
└───────────────────────────────────────────────────────────────────────────┘
                                    │
          ┌─────────────────────────┼──────────────────────────┐
          │                         │                          │
          ▼                         ▼                          ▼
   ┌──────────────┐          ┌──────────────┐          ┌──────────────┐
   │ 扫描入口层    │          │ 核心扫描层    │          │ 数据持久层    │
   │ cmd/         │──┬──────▶│ internal/    │──┬──────▶│ internal/db  │
   │ main.go      │  │       │ fscanner/    │  │       │ /store.go    │
   └──────────────┘  │       └──────────────┘  │       │ /schema.go   │
                     │                         │       │ /querier.go  │◀──┐
                     │       ┌──────────────┐  │       └──────────────┘   │
                     ├──────▶│ 解析器层      │  │                          │
                     │       │ internal/    │  │       ┌──────────────┐   │
                     │       │ parser/      │  │       │ 查询模式      │   │
                     │       │ (含Tree-sitter│  │       │ -query mode  │───┘
                     │       │  加速解析器)  │  │       │ cmd/main.go  │
                     │       └──────────────┘  │       │ runQueryMode │
                     │                         │       └──────────────┘
                     │       ┌──────────────┐  │
                     ├──────▶│ 分析器层      │  │
                     │       │ internal/    │  │
                     │       │ analyzer/    │  │
                     │       └──────────────┘  │
                     │                         │
                     │       ┌──────────────┐  │          ┌──────────────┐
                     │       │ MCP 协议模式  │  │          │ 配置层       │
                     └──────▶│ -mcp 标志    │──┘──────────│ internal/   │
                             │ stdio JSON   │             │ config/     │
                             │ 服务器       │             └──────────────┘
                             └──────────────┘
```

---

## 二、分层模块调用流程图

### 2.1 完整调用链路

```
 main()
   │
   ├─ parseFlags()           ← 解析 CLI 参数 (-lang, -db, -verbose, -graph, -query, -mcp 等)
   │
   ├─ [ -query 模式分支 ]                       ← 如果 -query 存在，跳过扫描直接查询
   │   └─ runQueryMode(flag.Arg(0))
   │       ├─ db.NewStore()                     ← 打开已有数据库
   │       ├─ switch queryMode:                 ← 根据查询子命令分发
   │       │   ├─ summary    → printSummary()
   │       │   ├─ functions  → printFunctions()
   │       │   ├─ func=NAME  → printQueryFuncDetail()
   │       │   ├─ vars       → printGlobals()
   │       │   ├─ deps       → printDepStats()
   │       │   ├─ calls=NAME → printCallers()
   │       │   ├─ dead       → analyzer.FindOrphanFunctions()
   │       │   ├─ missing    → findMissingDeps()
   │       │   ├─ top=N      → printTopFunctions(N)
   │       │   └─ deep=N     → printDeepNested(N)
   │       └─ store.Close()
   │
   ├─ [ -mcp 模式分支 ]                         ← MCP 协议服务模式
   │   └─ mcp.Serve(store)
   │       ├─ 通过 stdin/stdout 接收 JSON-RPC 请求
   │       ├─ handler: 函数查询 / 文件查询 / 依赖查询
   │       └─ 实时响应查询结果
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
   │   │   └── 14 种语言的 TS 树结构 → initQueries() 缓存
   │   ├─ parser.DefaultParsers()                     ← 批量注册全部 14 种语言\n   │   │   ├── NewTreeSitterGoParser()    → 注册 .go        (Go 专用 TS 增强)\n   │   │   ├── NewTreeSitterParser(".py") → 注册 .py        (Python)\n   │   │   ├── NewTreeSitterParser(".java") → 注册 .java    (Java)\n   │   │   ├── NewTreeSitterParser(".js") → 注册 .js/.jsx/.mjs (JavaScript)\n   │   │   ├── NewTreeSitterParser(".cs") → 注册 .cs        (C#)\n   │   │   ├── NewTreeSitterParser(".cpp") → 注册 .cpp/.c/.h 等 (C/C++)\n   │   │   ├── NewTreeSitterParser(".rs") → 注册 .rs        (Rust)\n   │   │   ├── NewTreeSitterParser(".rb") → 注册 .rb        (Ruby)\n   │   │   ├── NewTreeSitterParser(".ts") → 注册 .ts/.tsx   (TypeScript)\n   │   │   ├── NewTreeSitterParser(".swift") → 注册 .swift  (Swift)\n   │   │   ├── NewTreeSitterParser(".kt") → 注册 .kt/.kts   (Kotlin)\n   │   │   ├── NewTreeSitterParser(".php") → 注册 .php      (PHP)\n   │   │   ├── NewTreeSitterParser(".lua") → 注册 .lua      (Lua)\n   │   │   └── NewTreeSitterParser(".scala") → 注册 .scala  (Scala)\n   │   ├─ 对配置中扩展名尚无解析器的语言注册 GenericParser  ← 数据驱动正则兜底
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
   │   │           │   ├─ [Go 专用 Tree-sitter 路径]
   │   │           │   │   ├─ treesitter_go.go:
   │   │           │   │   │   ├─ tsParseRoot()           ← 调用 tree-sitter C API 解析 AST
   │   │           │   │   │   ├─ tsEachTopLevel()        ← 遍历顶层节点
   │   │           │   │   │   ├─ tsMakeFunc()            ← 从 AST 节点构建函数信息
   │   │           │   │   │   │   ├─ tsFindBodyNode()    ← 定位函数体
   │   │           │   │   │   │   ├─ tsExtractFuncSignature() ← 提取函数签名
   │   │           │   │   │   │   └─ tsCountCyclomatic() ← 计算圈复杂度
   │   │           │   │   │   ├─ tsAnalyzeCalls()        ← AST 分析函数调用
   │   │           │   │   │   ├─ tsCountStatements()     ← 统计语句数
   │   │           │   │   │   ├─ tsCountParams()         ← 统计参数个数
   │   │           │   │   │   ├─ tsGoNestingDepth()      ← 计算嵌套深度
   │   │           │   │   │   └─ tsFindLine()            ← 行号映射
   │   │           │   │   └─ treesitter_all.go:
   │   │           │   │       ├─ NewTreeSitterParser()   ← 通用 TS 解析器工厂
   │   │           │   │       ├─ initQueries()           ← 初始化语言查询
   │   │           │   │       ├─ getCachedQuery()        ← 缓存 TS 查询对象
   │   │           │   │       └─ Parse() / Globals()     ← 基于 TS 查询解析
   │   │           │   │
   │   │           │   ├─ [非 Go 专用解析器路径]
   │   │           │   │   ├─ 创建 FileLines (行偏移表)
   │   │           │   │   ├─ makeCommentMask()        ← 标记注释行
   │   │           │   │   ├─ makeStringMask()         ← 标记字符串行
   │   │           │   │   ├─ 行迭代 + 正则匹配函数定义
   │   │           │   │   ├─ matchBrace()             ← 花括号匹配找函数体
   │   │           │   │   └─ extractCallStats()       ← 提取函数内部调用
   │   │           │   │
   │   │           │   └─ parser.Globals(path, content) ← 提取全局变量
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
├── cmd/code-detector/
│   └── main.go              ← 入口：CLI解析、初始化、调度 + 查询模式
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
│   │   ├── treesitter_all.go ← Tree-sitter 通用框架 (新)
│   │   │   ├── NewTreeSitterParser()    ← 创建给定语言的 TS 解析器
│   │   │   ├── initQueries()            ← 初始化语言查询
│   │   │   ├── getCachedQuery()         ← 缓存 TS 查询对象
│   │   │   ├── Parse() / Globals()      ← 基于 TS 查询解析
│   │   │   └── Language()               ← 返回语言名
│   │   │
│   │   ├── treesitter_go.go  ← Go Tree-sitter 加速解析器 (新)
│   │   │   ├── NewTreeSitterGoParser()  ← 构造函数
│   │   │   ├── tsParseRoot()            ← 调用 C API 构建 AST
│   │   │   ├── tsEachTopLevel()         ← 遍历顶层节点
│   │   │   ├── tsMakeFunc()             ← AST→函数信息
│   │   │   │   ├── tsFindBodyNode()     ← 定位函数体
│   │   │   │   ├── tsExtractFuncSignature() ← 提取签名
│   │   │   │   └── tsCountCyclomatic()  ← 圈复杂度
│   │   │   ├── tsAnalyzeCalls()         ← AST 调用分析
│   │   │   ├── tsCountStatements()      ← 语句数
│   │   │   ├── tsCountParams()          ← 参数个数
│   │   │   ├── tsGoNestingDepth()       ← 嵌套深度
│   │   │   └── tsFindLine()             ← 行号映射
│   │   │
│   │   ├── go_parser.go      ← Go 语言正则解析器（旧方案，TS 优先时仅作回退）
│   │   │   ├── ...（同上）
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
│   │   ├── shared.go         ← 共享工具函数（注释/字符串掩码等）
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
│       ├── querier.go        ← 数据库查询 (新增)
│       │   ├── QueryFunctions()         ← 获取全部函数
│       │   ├── QueryFuncDetail()        ← 获取函数详情
│       │   ├── QueryDependencies()      ← 查询依赖关系
│       │   ├── QueryGlobalVars()        ← 获取全局变量
│       │   ├── QuerySessionByID()       ← 获取单个会话
│       │   ├── QueryFuncBody()          ← 获取函数体源码
│       │   └── QueryCallerFuncs()       ← 查询调用者
│
├── config.yaml              ← 用户自定义语言配置（可选）
├── Makefile / build.bat     ← 构建脚本
└── scaned_db/
    └── scan_result.db       ← SQLite 数据库（运行时生成）
```

---

## 四、主要函数树（按调用层级）

### 4.1 入口层 (cmd/code-detector/main.go)

```
main()
├── [ -query 模式分支 ]                  ← 查询模式（跳过扫描）
│   └── runQueryMode(queryArg)
│       ├── summary     → printSummary(store)      显示数据库概要
│       ├── functions   → printFunctions(store)    列出所有函数
│       ├── func=NAME   → printQueryFuncDetail()   查看函数详情
│       ├── vars        → printGlobals(store)      列出全局变量
│       ├── deps        → printDepStats(store)     调用统计
│       ├── calls=NAME  → printCallers(store)      查看调用者
│       ├── dead        → analyzer.FindOrphanFunctions() 死代码
│       ├── missing     → findMissingDeps(store)   缺失依赖
│       ├── top=N       → printTopFunctions(N)     最大N个函数
│       └── deep=N      → printDeepNested(N)       深层嵌套函数
│
├── [ -mcp 模式分支 ]                     ← MCP 服务模式
│   └── mcp.Serve(store)                 JSON-RPC over stdio
│
├── [ 默认扫描模式 ]
│   ├── parseFlags()                    解析命令行标志
│   ├── resolveProjectRoot()            解析项目根路径
│   ├── initProjectRoot()               路径安全检查
│   │   └── validateScanRoot()          验证是否系统目录
│   ├── initConfig()                    加载语言配置
│   │   └── config.LoadConfig()
│   ├── initDB()                        初始化数据库
│   │   ├── db.InitDB()
│   │   └── db.NewStore()
│   ├── initScanner()                   ★ 装配核心
│   │   ├── parser.NewRegistry()        创建解析器注册表
│   │   ├── parser.DefaultParsers()     批量注册14种Tree-sitter解析器
│   │   ├── parser.NewGenericParser()   加载配置中的额外语言（回退）
│   │   └── fscanner.New()              构造 Scanner
│   ├── printBanner()                   打印启动横幅
│   ├── scan.Scan()                     ★ 核心扫描
│   ├── printResults()                  打印结果表格
│   ├── printCallGraph()                调用图分析（可选）
│   │   └── analyzer.New() / BuildCallGraph()
│   └── store.Close()                   关闭数据库
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

### 4.4 Tree-sitter Go 加速解析器 (internal/parser/treesitter_go.go)

```
NewTreeSitterGoParser()                 构造函数
└── 创建 *TreeSitterGoParser 实例

TreeSitterGoParser.Parse(path, content)
├── tsParseRoot(content)                ★ 调用 tree-sitter C API 解析 AST
│   └── 返回 sitter.Node 根节点
├── tsEachTopLevel(root, fn)            遍历 AST 顶层声明
│   └── 对每个顶层节点回调 fn
├── tsMakeFunc(node, source)            ★ AST 节点 → Function 结构
│   ├── tsFindBodyNode(node)            在 AST 中定位函数体
│   │   └── 遍历子节点找到 block 节点
│   ├── tsExtractFuncSignature(node)    提取完整函数签名
│   │   └── 从 AST 拼接参数/返回值文本
│   ├── tsCountCyclomatic(node)         计算圈复杂度
│   │   └── AST 遍历统计 if/for/while/case/&&/||
│   ├── tsAnalyzeCalls(node)            ★ AST 分析函数调用
│   │   └── AST 遍历找 call_expression 节点
│   │       └── tsFindLine(node)        节点行号 → 源代码行
│   ├── tsCountStatements(node)         统计语句数
│   ├── tsCountParams(node)             统计参数个数
│   └── tsGoNestingDepth(node)          计算嵌套深度
│       └── AST 遍历统计 block 嵌套层数
├── 返回 []*Function

TreeSitterGoParser.Globals(path, content)
├── tsParseRoot(content)                解析 AST
└── tsEachTopLevel → var_declaration / const_declaration
    └── 返回 []*GlobalVariable
```

### 4.5 Tree-sitter 通用框架 (internal/parser/treesitter_all.go)

```
NewTreeSitterParser(lang, path)         创建给定语言的 TS 解析器
├── initQueries()                       初始化语言查询
│   └── 加载预定义的 TS query 字符串
├── getCachedQuery(name)                缓存 TS Query 对象
│   └── 避免重复编译同一查询
├── Parse(path, content)                基于 TS query 解析函数
│   └── 执行查询 → 匹配函数节点
├── Globals(path, content)              基于 TS query 提取全局变量
└── Language()                          返回语言名
```

### 4.6 花括号匹配 (internal/parser/match_brace.go)

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

### 4.7 调用图分析器 (internal/analyzer/analyzer.go)

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
                    ┌────────────┴────────────────────────────┐
                    │                                         │
                    ▼                                         ▼
           ┌───────────────────┐                ┌──────────────────────┐
           │  [ -query 模式 ]  │                │  [ 默认扫描模式 ]     │
           │  runQueryMode()   │                │                      │
           │  ┌─────────────┐  │                │  config.LoadConfig   │
           │  │ summary     │  │                │  db.InitDB()         │
           │  │ functions   │  │                │  fscanner.New()      │
           │  │ func=NAME   │  │                │  Scanner.Scan()      │
           │  │ vars        │  │                │                      │
           │  │ deps        │  │                │  1. collectFiles()   │
           │  │ calls=NAME  │  │                │  2. parseConcurrent  │
           │  │ dead        │  │                │  3. writeResults()   │
           │  │ top=N       │  │                │                      │
           │  │ deep=N      │  │                │  printResults()      │
           │  │ missing     │  │                │  printCallGraph()    │
           │  └──────┬──────┘  │                └──────────┬───────────┘
           └─────────┼─────────┘                           │
                     │                                     │
                     └──────────┬──────────────────────────┘
                                ▼
                   ┌────────────────────────┐
                   │    scaned_db/           │
                   │  scan_result.db         │
                   │  (SQLite WAL 模式)       │
                   └────────────────────────┘
                                ▲
                                │
                   ┌────────────┴────────────┐
                   │                         │
                   ▼                         ▼
          ┌──────────────────┐    ┌──────────────────────┐
          │ [ -graph 模式 ]  │    │  [ -mcp 模式 ]       │
          │ BuildCallGraph() │    │  MCP JSON-RPC 服务   │
          │ FindHotFunctions │    │  stdin/stdout 协议    │
          │ FindOrphanFuncs  │    │  实时函数查询          │
          └──────────────────┘    └──────────────────────┘
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

| 语言 | 解析器实现 | 扩展名 | 策略 | 加速方式 |
|------|-----------|--------|------|---------|
| Go | `TreeSitterGoParser` (专用) + `GoParser` (回退) | `.go` | AST 分析 + 花括号匹配回退 | ✅ Tree-sitter C API |
| Python | `TreeSitterParser` | `.py` | AST 分析 | ✅ Tree-sitter C API |
| Java | `TreeSitterParser` | `.java` | AST 分析 | ✅ Tree-sitter C API |
| Kotlin | `TreeSitterParser` | `.kt`, `.kts` | AST 分析 | ✅ Tree-sitter C API |
| JavaScript | `TreeSitterParser` | `.js`, `.jsx`, `.mjs` | AST 分析 | ✅ Tree-sitter C API |
| TypeScript | `TreeSitterParser` | `.ts`, `.tsx` | AST 分析 | ✅ Tree-sitter C API |
| C# | `TreeSitterParser` | `.cs` | AST 分析 | ✅ Tree-sitter C API |
| C/C++ | `TreeSitterParser` | `.cpp`, `.cxx`, `.cc`, `.c`, `.h`, `.hpp` | AST 分析 | ✅ Tree-sitter C API |
| Rust | `TreeSitterParser` | `.rs` | AST 分析 | ✅ Tree-sitter C API |
| Ruby | `TreeSitterParser` | `.rb` | AST 分析 | ✅ Tree-sitter C API |
| Swift | `TreeSitterParser` | `.swift` | AST 分析 | ✅ Tree-sitter C API |
| PHP | `TreeSitterParser` | `.php` | AST 分析 | ✅ Tree-sitter C API |
| Lua | `TreeSitterParser` | `.lua` | AST 分析 | ✅ Tree-sitter C API |
| Scala | `TreeSitterParser` | `.scala` | AST 分析 | ✅ Tree-sitter C API |

---

## 附：项目自扫描统计（来自 `-query` 真实数据）

以下数据由 `code-detector -query` 系列指令从项目自身的 `scaned_db/scan_result.db` 中提取：

| 指标 | 数值 | 说明 |
|------|------|------|
| 函数总数 | **211** | `-query functions` |
| 全局变量数 | **50** | `-query vars` |
| 依赖关系数 | **544** | `-query deps` |
| 函数体总行数 | **5,195** | 所有函数体行合计 |
| 扫描会话数 | 1 | `-query summary` |
| 扫描文件数 | 19 | `.go` 源文件 |
| 扫描耗时 | 465 ms | 最近一次 |
| 死代码函数 | **50** | `-query dead`（未被其他函数调用） |
| 最大嵌套深度 | ≤9 | `-query deep=10` 无结果 |

### 热点函数 TOP 5（被调用最多）

| 函数 | 被调次数 | 位置 |
|------|---------|------|
| `fatal` | 20 | 通用错误处理 |
| `jsonOut` | 15 | JSON 输出 |
| `scanFuncBriefs` | 8 | 查询输出 |
| `getCachedQuery` | 6 | Tree-sitter 查询缓存 |
| `matchBrace` | 6 | 花括号匹配核心 |

### 最大函数 TOP 5（按行数）

| # | 函数 | 行数 | 位置 |
|---|------|------|------|
| 1 | `Parse` | **146** | `generic_parser.go` |
| 2 | `printQueryFuncDetail` | **132** | `main.go` |
| 3 | `BatchInsertFunctions` | **98** | `store.go` |
| 4 | `matchBrace` | **95** | `match_brace.go` |
| 5 | `tsMakeFunc` | **93** | `treesitter_go.go` |

### 调用分支最广的函数

| 函数 | 调用的不同函数数 |
|------|----------------|
| `Parse` | 25 |
| `runQueryMode` | 20 |
| `main` | 16 |
| `tsMakeFunc` | 15 |
| `collectFiles` | 13 |

---

## 八、关键设计决策

1. **解析器体系：Tree-sitter 为主，正则回退为辅** — 全部 14 种语言优先使用 Tree-sitter C API 进行 AST 解析，Go 拥有专用 `TreeSitterGoParser` 增强圈复杂度/嵌套深度等精确度量；旧的正则解析器保留作为回退方案
2. **数据驱动后备** — 对于没有专用解析器的语言，可用 `config.yaml` 配置正则表达式，`GenericParser` 兜底
3. **编码自适应** — 自动检测 UTF-8/UTF-16 BOM、无 BOM UTF-16 启发式检测、GBK 等非 UTF-8 回退
4. **并发模型** — 生产者-消费者模式：主 goroutine 收集文件后通过 channel 分发，N 个 worker 并发解析
5. **增量扫描** — 通过文件 mtime 缓存跳过未变更文件，适合大型项目的重复扫描
6. **去重机制** — 函数内容 SHA256 哈希去重 + 硬链接 `os.SameFile()` 去重
7. **会话管理** — 自动清理旧会话（保留最近3个），防止数据库无限膨胀
8. **WAL 模式** — SQLite WAL 模式提升并发写入性能，扫描完成时主动 checkpoint
9. **Tree-sitter 加速** — 对 Go 语言提供基于 tree-sitter C API 的 AST 解析路径，支持圈复杂度、语句数、参数数等精确度量和嵌套深度检测
10. **查询模式 (-query)** — 复用已有的分析器 (`internal/analyzer`) 和数据库查询层 (`internal/db/querier.go`)，无需重新扫描即可获取死代码、热点函数、调用链等分析结果
11. **MCP 协议集成** — 通过 `-mcp` 标志启动 JSON-RPC over stdio 服务器，使 AI 工具（如 Claude/Cline）能直接查询函数库，无需 CLI 交互
