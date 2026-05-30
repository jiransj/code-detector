package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"code-detector/internal/analyzer"
	"code-detector/internal/config"
	"code-detector/internal/db"
	"code-detector/internal/fscanner"
	mcp "code-detector/internal/mcp"
	"code-detector/internal/model"
	"code-detector/internal/parser"
)

const version = "1.1"

// cleanup 退出前需执行的清理（关闭 DB 确保 WAL 回归），main 中初始化，fatal 中调用
var cleanup func()

// noWait 为 true 时 waitForExit 跳过 stdin 等待（查询模式/管道模式）
var noWait bool

// outputFormat 输出格式: "text" 或 "json"
var outputFormat = "text"

// jsonOut 以 JSON 格式输出任意值到标准输出
func jsonOut(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON 序列化错误: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// waitForExit 在程序退出前等待用户按键（兼容命令行、双击启动、管道输入）
func waitForExit() {
	if noWait {
		return
	}

	fmt.Print("\n按 Enter 键退出...")

	// 尝试读取 stdin（终端模式下会阻塞等待 Enter）
	var buf [1]byte
	n, err := os.Stdin.Read(buf[:])
	if n == 0 || err != nil {
		// stdin 不可用（双击启动 / 管道 EOF）
		// 等 3 秒让用户在窗口关闭前看到最终输出
		time.Sleep(3 * time.Second)
		return
	}
	if buf[0] == '\n' || buf[0] == '\r' {
		return
	}
	// 读到其他字符，继续等到换行
	for {
		os.Stdin.Read(buf[:])
		if buf[0] == '\n' || buf[0] == '\r' {
			break
		}
	}
}

// dangerousRoots 拒绝扫描的系统关键目录
var dangerousRoots = []string{
	`C:\`, `D:\`, `E:\`, `F:\`, `G:\`, `H:\`, `I:\`, `J:\`,
	`C:\Windows`, `C:\WINNT`, `C:\WINNT\System32`,
	`C:\Program Files`, `C:\Program Files (x86)`,
	`C:\ProgramData`, `C:\System Volume Information`,
	`C:\Users\Default`, `C:\Users\All Users`,
	`C:\Recovery`, `C:\$Recycle.Bin`,
	`/`, `/etc`, `/bin`, `/boot`, `/dev`, `/lib`, `/lib64`,
	`/proc`, `/sbin`, `/sys`, `/usr`, `/var`,
	`/System`, `/Library`, `/Applications`,
}

// validateScanRoot 检查扫描根目录是否安全
func validateScanRoot(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("无法解析绝对路径: %w", err)
	}
	if len(abs) <= 3 && (abs[len(abs)-1] == '\\' || abs[len(abs)-1] == ':') {
		return fmt.Errorf("禁止扫描磁盘根目录: %s（可能导致系统卡死或数据损坏）", abs)
	}
	if abs == "/" {
		return fmt.Errorf("禁止扫描系统根目录 /")
	}
	absLower := strings.ToLower(abs)
	for _, dangerous := range dangerousRoots {
		dangerLower := strings.ToLower(dangerous)
		if absLower == dangerLower || strings.HasPrefix(absLower, dangerLower+"\\") || strings.HasPrefix(absLower, dangerLower+"/") {
			return fmt.Errorf("禁止扫描系统目录: %s（这可能包含敏感系统文件）", dangerous)
		}
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("目录不存在: %s", abs)
		}
		return fmt.Errorf("无法访问目录 %s: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("路径不是目录: %s", abs)
	}
	return nil
}

// fatal 打印错误信息，执行清理后退出
func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\n错误: "+format+"\n", args...)
	// 先执行清理（关闭 DB 确保 WAL 回归），再等待按键
	if cleanup != nil {
		cleanup()
	}
	waitForExit()
	os.Exit(1)
}

// ========== 拆分子函数 ==========

// resolveProjectRoot 确定项目根目录
func resolveProjectRoot() string {
	if flag.NArg() < 1 {
		exePath, err := os.Executable()
		if err != nil {
			fatal("无法获取程序路径: %v", err)
		}
		return filepath.Dir(exePath)
	}
	return flag.Arg(0)
}

// initProjectRoot 验证并返回项目绝对路径
func initProjectRoot(projectRoot string) string {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		fatal("无法解析路径: %v", err)
	}
	if err := validateScanRoot(absRoot); err != nil {
		fatal("路径安全检查未通过: %v", err)
	}
	return absRoot
}

// initConfig 加载配置
func initConfig(cfgPath string, verbose bool) *config.Config {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		fatal("加载配置失败: %v", err)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "info: 已加载 %d 种语言配置\n", len(cfg.Languages))
	}
	return cfg
}

// initDB 初始化数据库
func initDB(dbPath string, verbose bool) *db.Store {
	dbFile, err := filepath.Abs(dbPath)
	if err != nil {
		fatal("数据库路径错误: %v", err)
	}
	dbDir := filepath.Dir(dbFile)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fatal("创建数据库目录失败 %s: %v", dbDir, err)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "info: 数据库路径: %s\n", dbFile)
	}
	sqlDB, err := db.InitDB(dbFile)
	if err != nil {
		fatal("初始化数据库失败: %v", err)
	}
	return db.NewStore(sqlDB)
}

// initScanner 创建并配置扫描器
func initScanner(cfg *config.Config, store *db.Store, verbose bool, incremental bool, maxSize int64, workers int, langs string, skipDirs string) *fscanner.Scanner {
	reg := parser.NewRegistry()
	for _, pr := range parser.DefaultParsers() {
		reg.Register(pr.Parser, pr.Extensions...)
	}

	for _, langCfg := range cfg.Languages {
		for _, ext := range langCfg.Extensions {
			if reg.GetByExt(ext) == nil && langCfg.FunctionRegex != "" {
				gp := parser.NewGenericParser(langCfg)
				reg.Register(gp, ext)
				if verbose {
					fmt.Fprintf(os.Stderr, "info: 为 %s (%s) 注册通用解析器 (regex: %s)\n",
						langCfg.Name, ext, langCfg.FunctionRegex)
				}
				break
			}
		}
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "info: 已注册 %d 种语言的解析器\n", len(reg.SupportedExts()))
	}

	scan := fscanner.New(cfg, reg, store)
	scan.Verbose = verbose
	scan.Incremental = incremental
	if maxSize > 0 {
		scan.MaxFileSize = maxSize
	}
	if workers > 0 {
		scan.Workers = workers
	}

	langAliases := map[string]string{
		"py": "python", "js": "javascript", "ts": "typescript",
		"kt": "kotlin", "cpp": "cpp", "cs": "csharp",
		"rs": "rust", "rb": "ruby",
	}
	if langs != "" {
		for _, lang := range strings.Split(langs, ",") {
			lang = strings.TrimSpace(lang)
			if lang == "" {
				continue
			}
			if fullName, ok := langAliases[lang]; ok {
				lang = fullName
			}
			if cfg.GetLanguageByName(lang) != nil {
				scan.LangFilter[lang] = true
			} else if verbose {
				fmt.Fprintf(os.Stderr, "warn: 未知语言 '%s'，跳过\n", lang)
			}
		}
	}

	if skipDirs != "" {
		for _, dir := range strings.Split(skipDirs, ",") {
			dir = strings.TrimSpace(dir)
			if dir != "" {
				scan.SkipDirs[dir] = true
			}
		}
	}
	return scan
}

// printBanner 显示启动横幅
func printBanner(projectRoot, dbPath string, scan *fscanner.Scanner, cfg *config.Config, verbose bool) {
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║        code-detector v" + version + " — 多编程语言函数扫描工具      ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf(" ▶ 项目:     %s\n", projectRoot)
	fmt.Printf(" ▶ 数据库:   %s\n", dbPath)
	fmt.Printf(" ▶ 并发数:   %d\n", scan.Workers)
	if len(scan.LangFilter) > 0 {
		keys := make([]string, 0, len(scan.LangFilter))
		for k := range scan.LangFilter {
			keys = append(keys, k)
		}
		fmt.Printf(" ▶ 语言过滤: %s\n", strings.Join(keys, ", "))
	} else {
		fmt.Printf(" ▶ 语言过滤: 全部 (%d 种)\n", len(cfg.Languages))
	}
	fmt.Println()
}

// printResults 输出扫描结果摘要
func printResults(result *model.ScanResult, dbPath string) {
	fmt.Println("┌───────────────── 扫描结果 ─────────────────┐")
	fmt.Printf(" │ 项目路径     %s\n", result.Session.ProjectRoot)
	fmt.Printf(" │ 扫描用时     %v\n", result.Duration)
	fmt.Printf(" │ 扫描文件     %d 个\n", result.FileCount)
	fmt.Printf(" │ 发现函数     %d 个\n", len(result.Functions))
	fmt.Printf(" │ 全局变量     %d 个\n", len(result.GlobalVars))
	fmt.Printf(" │ 跳过文件     %d 个\n", result.SkipCount)
	if len(result.Errors) > 0 {
		fmt.Printf(" │ 解析错误     %d 个\n", len(result.Errors))
	}
	fmt.Printf(" │ 数据库       %s\n", dbPath)
	fmt.Println(" └─────────────────────────────────────────────┘")

	if len(result.Functions) > 0 {
		langStats := make(map[string]int)
		for _, f := range result.Functions {
			langStats[f.Language]++
		}
		fmt.Println("按语言统计 (函数):")
		for lang, cnt := range langStats {
			fmt.Printf("  %-12s %d 个\n", lang+":", cnt)
		}
		fmt.Println()
	}
	if len(result.GlobalVars) > 0 {
		varStats := make(map[string]int)
		for _, v := range result.GlobalVars {
			varStats[v.Language]++
		}
		fmt.Println("按语言统计 (全局变量):")
		for lang, cnt := range varStats {
			fmt.Printf("  %-12s %d 个\n", lang+":", cnt)
		}
		fmt.Println()
	}
}

// printCallGraph 输出调用图分析（仅 -graph 模式）
func printCallGraph(buildGraph bool, store *db.Store, result *model.ScanResult) {
	if !buildGraph || len(result.Functions) == 0 {
		return
	}
	an := analyzer.New(store)
	graph, err := an.BuildCallGraph(result.Session.ID)
	if err != nil {
		fatal("构建调用图失败: %v", err)
	}
	fmt.Println("======= 调用图统计 =======")
	fmt.Printf("  总节点数:  %d\n", len(graph.Nodes))

	// 统计调用总次数和平均嵌套深度
	totalCallCount := 0
	totalNestingDepth := 0
	maxNestingDepth := 0
	topCallCount := make([]*model.Function, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		totalCallCount += node.Function.CallCount
		totalNestingDepth += node.Function.NestingDepth
		if node.Function.NestingDepth > maxNestingDepth {
			maxNestingDepth = node.Function.NestingDepth
		}
		topCallCount = append(topCallCount, node.Function)
	}
	avgNesting := 0.0
	if len(graph.Nodes) > 0 {
		avgNesting = float64(totalNestingDepth) / float64(len(graph.Nodes))
	}
	fmt.Printf("  调用总次数: %d\n", totalCallCount)
	fmt.Printf("  平均嵌套深度: %.1f (最大 %d)\n", avgNesting, maxNestingDepth)

	// 按调用次数排序（自身调用的总次数）
	sort.Slice(topCallCount, func(i, j int) bool {
		return topCallCount[i].CallCount > topCallCount[j].CallCount
	})
	if len(topCallCount) > 10 {
		topCallCount = topCallCount[:10]
	}
	if len(topCallCount) > 0 && topCallCount[0].CallCount > 0 {
		fmt.Println("\n自身调用最多的函数 (Top 10):")
		for i, f := range topCallCount {
			fmt.Printf("  %2d. %s (%s) — 调用 %d 次, 嵌套深度 %d\n",
				i+1, f.Name, f.Language, f.CallCount, f.NestingDepth)
		}
	}

	hotFuncs := graph.FindHotFunctions(10)
	if len(hotFuncs) > 0 {
		fmt.Println("\n被调用最多的函数 (Top 10):")
		for i, node := range hotFuncs {
			callerCnt := len(node.Callers)
			calleeCnt := len(node.Callees)
			fmt.Printf("  %2d. %s (%s) — 被调 %d 次, 调用 %d 个函数\n",
				i+1, node.Function.Name, node.Function.Language, callerCnt, calleeCnt)
		}
	}
	orphans := graph.FindOrphanFunctions()
	if len(orphans) > 0 {
		fmt.Printf("\n未被调用的函数 (可能的死代码): %d 个\n", len(orphans))
		for i, node := range orphans {
			if i >= 20 {
				fmt.Printf("  ... 还有 %d 个\n", len(orphans)-i)
				break
			}
			fmt.Printf("  - %s (%s, %s)\n", node.Function.Name, node.Function.Language, node.Function.FilePath)
		}
	}
	fmt.Println("==========================")
}

// ========== 入口 ==========

func main() {
	defer waitForExit()

	dbPath, cfgPath, langs, skipDirs, verbose, showVer, workers, buildGraph, incremental, maxSize, debug, queryMode, mcpMode := parseFlags()
	if showVer {
		noWait = true
		fmt.Printf("code-detector v%s\n", version)
		return
	}

	// MCP 服务器模式：以 MCP 协议提供数据服务
	if mcpMode {
		noWait = true
		store := initDB(dbPath, verbose)
		cleanup = func() { store.Close() }
		defer cleanup()
		if err := mcp.RunStdioServer(store); err != nil {
			fatal("MCP 服务器错误: %v", err)
		}
		return
	}

	// 查询模式：不扫描，直接读取已有数据库
	if queryMode != "" {
		noWait = true // 查询模式/管道模式跳过 stdin 等待
		runQueryMode(dbPath, queryMode)
		return
	}

	projectRoot := resolveProjectRoot()
	_ = initProjectRoot(projectRoot)       // 验证路径
	cfg := initConfig(cfgPath, verbose)
	store := initDB(dbPath, verbose)
	cleanup = func() { store.Close() }
	defer cleanup()

	if debug {
		parser.DebugMode = true
		if !verbose {
			verbose = true
		}
	}
	scan := initScanner(cfg, store, verbose, incremental, maxSize, workers, langs, skipDirs)

	printBanner(projectRoot, dbPath, scan, cfg, verbose)

	fmt.Print(" ▶ 扫描中")
	if !verbose {
		fmt.Print(" (使用 -verbose 查看详情)")
	}
	fmt.Println()
	fmt.Println()

	result, err := scan.Scan(projectRoot)
	if err != nil {
		fatal("扫描失败: %v", err)
	}

	printResults(result, dbPath)
	printCallGraph(buildGraph, store, result)
	fmt.Println("扫描完成!")
}

// parseFlags 解析命令行参数
func parseFlags() (dbPath string, cfgPath string, langs string, skipDirs string, verbose bool, showVer bool, workers int, buildGraph bool, incremental bool, maxSize int64, debug bool, queryMode string, mcpMode bool) {
	flag.StringVar(&langs, "lang", "", "扫描语言，逗号分隔 (如 go,py,java)")
	flag.StringVar(&dbPath, "db", "scaned_db/scan_result.db", "SQLite 数据库路径（默认 scaned_db/ 目录下）")
	flag.StringVar(&cfgPath, "config", "config.yaml", "配置文件路径")
	flag.Int64Var(&maxSize, "max-size", 1048576, "单文件最大字节数 (默认 1MB)")
	flag.StringVar(&skipDirs, "skip-dirs", "", "额外跳过目录，逗号分隔")
	flag.BoolVar(&verbose, "verbose", false, "输出详细日志")
	flag.BoolVar(&debug, "debug", false, "调试模式（含解析器内部调试信息）")
	flag.BoolVar(&showVer, "v", false, "显示版本号")
	flag.IntVar(&workers, "workers", 0, "并发工作数 (默认 CPU 核数)")
	flag.BoolVar(&buildGraph, "graph", false, "扫描完成后构建调用图并输出统计")
	flag.BoolVar(&incremental, "incremental", false, "增量扫描：仅重新解析 mtime 变更的文件")
	flag.BoolVar(&mcpMode, "mcp", false, "以 MCP 协议模式启动服务器（通过 stdio 提供函数查询服务）")
	flag.StringVar(&queryMode, "query", "", "查询模式：读取已有数据库而不扫描")
	flag.StringVar(&outputFormat, "format", "text", "输出格式: text (默认) 或 json")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `code-detector v%s — 多编程语言函数扫描工具

用法:
  code-detector [选项] <项目根目录>
  code-detector -query <模式> [-db <路径>]

选项:
  -lang <列表>     扫描语言，逗号分隔 (如 go,py,java)
  -db <路径>       SQLite 数据库路径（默认 scaned_db/scan_result.db）
  -config <路径>   配置文件路径（默认 config.yaml）
  -max-size <N>    单文件最大字节数 (默认 1MB)
  -skip-dirs <列表> 额外跳过目录，逗号分隔
  -workers <N>     并发工作数 (默认 CPU 核数)
  -verbose         输出详细日志
  -mcp             以 MCP 协议模式启动服务器（通过 stdio 提供函数查询服务）
  -graph           扫描完成后构建调用图并输出统计
  -incremental     增量扫描：仅重新解析 mtime 变更的文件
  -format <格式>   输出格式: text (默认) 或 json
  -query <模式>    查询模式：读取已有数据库不扫描
                    summary     显示数据库概要
                    functions   列出所有函数
                    func=NAME   查看指定函数详情
                    vars        列出全局变量
                    deps        显示调用统计
                    calls=NAME  查看谁调用了指定函数
                    dead        列出未被调用的函数
                    missing     列出缺失的依赖
                    top=N       列出最大的 N 个函数
                    deep=N      列出深层嵌套函数
                    tree=NAME   递归提取函数及其所有传递依赖（含函数体）
  -v               显示版本号

示例:
  code-detector -lang go,java -db myproject.db ./myproject
  code-detector -lang go,python,js -verbose ./src
  code-detector -verbose -skip-dirs .git,bin,obj ./src
  code-detector -graph ./myproject
  code-detector -query summary
  code-detector -query functions
  code-detector -query func=main
  code-detector -query dead
  code-detector -query top=10
  code-detector -query tree=main
  code-detector -query tree=Parse
  code-detector -mcp
`, version)
	}
	flag.Parse()
	return
}

// ──────────────────────────────────────────────
// 查询模式实现
// ──────────────────────────────────────────────

// runQueryMode 根据 query 参数执行对应的查询并输出
func runQueryMode(dbPath string, queryMode string) {
	// 只读打开数据库，不创建文件
	sqlDB, err := db.InitDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 打开数据库失败: %v\n", err)
		return
	}
	store := db.NewStore(sqlDB)
	defer store.Close()
	// 注册清理函数，使子函数中的 fatal() 能正确关闭 DB
	cleanup = func() { store.Close() }

	// 解析 query 模式
	mode := queryMode
	param := ""
	if idx := strings.IndexByte(mode, '='); idx >= 0 {
		param = mode[idx+1:]
		mode = mode[:idx]
	}

	switch mode {
	case "summary":
		printQuerySummary(store)
	case "functions":
		printQueryFunctions(store)
	case "func":
		if param == "" {
			fmt.Println("用法: -query func=函数名")
			return
		}
		printQueryFuncDetail(store, param)
	case "vars":
		printQueryVars(store)
	case "deps":
		printQueryDeps(store)
	case "calls":
		if param == "" {
			fmt.Println("用法: -query calls=函数名")
			return
		}
		printQueryCallers(store, param)
	case "dead":
		printQueryDead(store)
	case "missing":
		printQueryMissing(store)
	case "top":
		n := 10
		if param != "" {
			if v, err := strconv.Atoi(param); err == nil && v > 0 {
				n = v
			}
		}
		printQueryTop(store, n)
	case "deep":
		n := 3
		if param != "" {
			if v, err := strconv.Atoi(param); err == nil && v > 0 {
				n = v
			}
		}
		printQueryDeep(store, n)
	// 🆕 AST 增强查询
	case "complexity":
		n := 10
		if param != "" {
			if v, err := strconv.Atoi(param); err == nil && v > 0 {
				n = v
			}
		}
		printQueryComplexity(store, n)
	case "params":
		n := 5
		if param != "" {
			if v, err := strconv.Atoi(param); err == nil && v > 0 {
				n = v
			}
		}
		printQueryByParams(store, n)
	case "anon":
		printQueryAnon(store)
	case "files":
		printQueryFiles(store)
	case "types":
		printQueryTypes(store)
	case "tree":
		if param == "" {
			fmt.Println("用法: -query tree=函数名")
			return
		}
		printQueryTree(store, param)
	default:
		fmt.Fprintf(os.Stderr, "未知的查询模式: %s\n可用模式: summary, functions, func=NAME, vars, deps, calls=NAME, dead, missing, top=N, deep=N, complexity=N, params=N, anon, files, types, tree=NAME\n", queryMode)
	}
}

// printQuerySummary 打印数据库概要
func printQuerySummary(store *db.Store) {
	summary, err := store.QuerySummary()
	if outputFormat == "json" {
		jsonOut(summary)
		return
	}
	if err != nil {
		fatal("查询摘要失败: %v", err)
	}

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║      code-detector 数据库概要            ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("  扫描会话数:  %d\n", summary["session_count"])
	fmt.Printf("  函数总数:    %d\n", summary["func_count"])
	fmt.Printf("  全局变量数:  %d\n", summary["var_count"])
	fmt.Printf("  依赖关系数:  %d\n", summary["dep_count"])
	fmt.Printf("  函数体总行数: %d\n", summary["body_lines"])
	fmt.Println()

	if latest := summary["latest_session"]; latest != nil {
		ls := latest.(*model.ScanSession)
		fmt.Println("  ── 最近一次扫描 ──")
		fmt.Printf("    项目:    %s\n", ls.ProjectRoot)
		fmt.Printf("    时间:    %s\n", ls.ScanTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("    耗时:    %d ms\n", ls.Duration)
		fmt.Printf("    文件数:  %d\n", ls.FileCount)
		fmt.Printf("    函数数:  %d\n", ls.FuncCount)
		fmt.Printf("    变量数:  %d\n", ls.VarCount)
		fmt.Println()
	}

	if langDist, ok := summary["lang_dist"].(map[string]int); ok && len(langDist) > 0 {
		fmt.Println("  按语言分布 (函数):")
		for lang, cnt := range langDist {
			fmt.Printf("    %-15s %d\n", lang+":", cnt)
		}
		fmt.Println()
	}

	if varLangDist, ok := summary["var_lang_dist"].(map[string]int); ok && len(varLangDist) > 0 {
		fmt.Println("  按语言分布 (全局变量):")
		for lang, cnt := range varLangDist {
			fmt.Printf("    %-15s %d\n", lang+":", cnt)
		}
		fmt.Println()
	}
}

// printQueryFunctions 列出所有函数（不含 body）
func printQueryFunctions(store *db.Store) {
	funcs, err := store.QueryAllFunctions()
	if err != nil {
		fatal("查询函数列表失败: %v", err)
	}

	if outputFormat == "json" {
		jsonOut(funcs)
		return
	}

	fmt.Printf("共 %d 个函数:\n\n", len(funcs))
	// 按文件分组
	byFile := make(map[string][]*db.FuncBrief)
	for _, f := range funcs {
		byFile[f.FilePath] = append(byFile[f.FilePath], f)
	}

	for file, flist := range byFile {
		fmt.Printf("  %s:\n", file)
		for _, f := range flist {
			depth := ""
			if f.NestingDepth > 0 {
				depth = fmt.Sprintf(" depth=%d", f.NestingDepth)
			}
			fmt.Printf("    L%04d-%04d %-30s [%s] calls=%d%s\n",
				f.LineStart, f.LineEnd, f.Name, f.Language, f.CallCount, depth)
		}
		fmt.Println()
	}
}

// printQueryFuncDetail 查看指定函数的详细信息（支持逗号分隔批量查询）
func printQueryFuncDetail(store *db.Store, param string) {
	// 支持批量: 逗号分隔多个函数名
	names := strings.Split(param, ",")
	var allFuncs []*db.FuncBrief
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		funcs, err := store.QueryFuncDetail(n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "查询函数 '%s' 失败: %v\n", n, err)
			continue
		}
		if len(funcs) == 0 {
			fmt.Fprintf(os.Stderr, "未找到名称包含 '%s' 的函数\n", n)
			continue
		}
		allFuncs = append(allFuncs, funcs...)
	}
	if len(allFuncs) == 0 {
		return
	}

	if outputFormat == "json" {
		type FuncJSON struct {
			ID           int64  `json:"id"`
			Name         string `json:"name"`
			PackageName  string `json:"package_name,omitempty"`
			Language     string `json:"language"`
			FilePath     string `json:"file_path"`
			LineStart    int    `json:"line_start"`
			LineEnd      int    `json:"line_end"`
			LineCount    int    `json:"line_count"`
			CallCount    int    `json:"call_count"`
			NestingDepth int   `json:"nesting_depth"`
			Parameters   string `json:"parameters,omitempty"`
			ReturnTypes  string `json:"return_types,omitempty"`
			Receiver     string `json:"receiver,omitempty"`
			IsMethod     bool   `json:"is_method"`
			Visibility   string `json:"visibility,omitempty"`
			Cyclomatic   int    `json:"cyclomatic"`
			ParamCount   int    `json:"param_count"`
			ReturnCount  int    `json:"return_count"`
			StmtCount    int    `json:"stmt_count"`
			AnonFuncs    int    `json:"anon_funcs"`
		}
		var result []FuncJSON
		for _, f := range allFuncs {
			result = append(result, FuncJSON{
				ID: f.ID, Name: f.Name, PackageName: f.PackageName,
				Language: f.Language, FilePath: f.FilePath,
				LineStart: f.LineStart, LineEnd: f.LineEnd,
				LineCount: f.LineCount, CallCount: f.CallCount,
				NestingDepth: f.NestingDepth,
				Parameters: f.Parameters, ReturnTypes: f.ReturnTypes,
				Receiver: f.Receiver, IsMethod: f.IsMethod,
				Visibility: f.Visibility, Cyclomatic: f.Cyclomatic,
				ParamCount: f.ParamCount, ReturnCount: f.ReturnCount,
				StmtCount: f.StmtCount, AnonFuncs: f.AnonFuncs,
			})
		}
		jsonOut(result)
		return
	}

	for _, f := range allFuncs {
		body, _ := store.QueryFuncBody(f.ID)
		deps, _ := store.QueryDependencies(f.ID)

		fmt.Println("═══════════════════════════════════════════")
		fmt.Printf("  函数名:     %s\n", f.Name)
		fmt.Printf("  包:         %s\n", f.PackageName)
		fmt.Printf("  语言:       %s\n", f.Language)
		fmt.Printf("  文件:       %s\n", f.FilePath)
		fmt.Printf("  行号:       %d - %d (%d 行)\n", f.LineStart, f.LineEnd, f.LineCount)
		fmt.Printf("  调用次数:   %d\n", f.CallCount)
		fmt.Printf("  嵌套深度:   %d\n", f.NestingDepth)
		fmt.Printf("  参数:       %s\n", f.Parameters)
		fmt.Printf("  返回类型:   %s\n", f.ReturnTypes)
		if f.IsMethod {
			fmt.Printf("  接收器:     %s\n", f.Receiver)
		}
		fmt.Printf("  可见性:     %s\n", f.Visibility)
		fmt.Printf("  圈复杂度:   %d\n", f.Cyclomatic)
		fmt.Printf("  参数个数:   %d\n", f.ParamCount)
		fmt.Printf("  return数:   %d\n", f.ReturnCount)
		fmt.Printf("  语句数:     %d\n", f.StmtCount)
		fmt.Printf("  匿名函数:   %d\n", f.AnonFuncs)
		fmt.Println()

		// 被调用的依赖 (already fetched above)
		if len(deps) > 0 {
			fmt.Printf("  调用了 (%d):\n", len(deps))
			for _, d := range deps {
				fmt.Printf("    - %s\n", d)
			}
		} else {
			fmt.Println("  调用了: (无)")
		}
		fmt.Println()

		// 调用该函数的函数
		callers, _ := store.QueryCallers(f.Name)
		if len(callers) > 0 {
			fmt.Printf("  被谁调用 (%d):\n", len(callers))
			for _, c := range callers {
				fmt.Printf("    - %s (%s L%d)\n", c.Name, c.FilePath, c.LineStart)
			}
		} else {
			fmt.Println("  被谁调用: (无)")
		}
		fmt.Println()

		// 函数体（截取前 20 行）
		if body != "" {
			lines := strings.Split(body, "\n")
			showLines := len(lines)
			if showLines > 20 {
				showLines = 20
			}
			fmt.Printf("  函数体 (前 %d / %d 行):\n", showLines, len(lines))
			for i := 0; i < showLines; i++ {
				fmt.Printf("    %s\n", lines[i])
			}
			if len(lines) > 20 {
				fmt.Printf("    ... (剩余 %d 行)\n", len(lines)-20)
			}
		}
		fmt.Println()
	}
}

// printQueryTree 递归展示函数及其所有传递依赖的完整信息（含函数体）
func printQueryTree(store *db.Store, param string) {
	items, err := store.QueryFuncTree(param)
	if err != nil {
		fatal("查询依赖树失败: %v", err)
	}
	if len(items) == 0 {
		fmt.Printf("未找到函数: %s\n", param)
		return
	}

	if outputFormat == "json" {
		jsonOut(items)
		return
	}

	// 按深度排序输出，同深度按名称排序
	sort.Slice(items, func(i, j int) bool {
		if items[i].Depth != items[j].Depth {
			return items[i].Depth < items[j].Depth
		}
		return items[i].Func.Name < items[j].Func.Name
	})

	fmt.Printf("╔══════════════════════════════════════════╗\n")
	fmt.Printf("║  依赖树: %-30s ║\n", param)
	fmt.Printf("╚══════════════════════════════════════════╝\n")
	fmt.Printf("  共 %d 个函数（含递归依赖）\n\n", len(items))

	for i, item := range items {
		f := item.Func
		indent := ""
		for j := 0; j < item.Depth; j++ {
			indent += "  "
		}
		prefix := ""
		if item.Depth == 0 {
			prefix = "◎ "
		} else if item.Depth == 1 {
			prefix = "├─ "
		} else {
			prefix = "└  "
		}

		fmt.Printf("─── [%d/%d] %s%s (%s) ─────────────────\n", i+1, len(items), indent+prefix, f.Name, f.Language)
		if f.FilePath != "" {
			fileLoc := f.FilePath
			fmt.Printf("  位置:     %s L%d-L%d (%d 行)\n", fileLoc, f.LineStart, f.LineEnd, f.LineCount)
		}
		if f.PackageName != "" {
			fmt.Printf("  包:       %s\n", f.PackageName)
		}
		if f.Parameters != "" {
			fmt.Printf("  参数:     %s\n", f.Parameters)
		}
		if f.ReturnTypes != "" {
			fmt.Printf("  返回:     %s\n", f.ReturnTypes)
		}
		if f.IsMethod {
			fmt.Printf("  接收器:   %s\n", f.Receiver)
		}
		if f.Visibility != "" {
			fmt.Printf("  可见性:   %s\n", f.Visibility)
		}
		if f.Cyclomatic > 0 {
			fmt.Printf("  圈复杂度: %d\n", f.Cyclomatic)
		}
		if len(item.Callees) > 0 {
			fmt.Printf("  调用了 (%d): %s\n", len(item.Callees), strings.Join(item.Callees, ", "))
		}
		fmt.Printf("  深度:     %d\n", item.Depth)
		fmt.Println()

		// 函数体（完整输出，不超过 50 行）
		if item.Body != "" {
			lines := strings.Split(item.Body, "\n")
			showLines := len(lines)
			if showLines > 50 {
				showLines = 50
			}
			fmt.Printf("  函数体 (前 %d / %d 行):\n", showLines, len(lines))
			for _, line := range lines[:showLines] {
				fmt.Printf("    %s\n", line)
			}
			if len(lines) > 50 {
				fmt.Printf("    ... (剩余 %d 行)\n", len(lines)-50)
			}
		} else {
			fmt.Printf("  函数体: (外部函数或未收录)\n")
		}
		fmt.Println()
	}

	fmt.Printf("─── 共计 %d 个函数 ───\n", len(items))
}

// printQueryVars 列出所有全局变量
func printQueryVars(store *db.Store) {
	vars, err := store.QueryVars()
	if err != nil {
		fatal("查询全局变量失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(vars)
		return
	}
	fmt.Printf("共 %d 个全局变量:\n\n", len(vars))
	for _, v := range vars {
		constStr := "var"
		if v.IsConst {
			constStr = "const"
		}
		vis := v.Visibility
		if vis != "" {
			vis = " " + vis
		}
		fmt.Printf("  %s%s %-25s type=%-10s [%s] %s L%d\n",
			constStr, vis, v.Name, v.VarType, v.Language, v.FilePath, v.LineNum)
	}
}

// printQueryDeps 显示调用统计
func printQueryDeps(store *db.Store) {
	stats, err := store.QueryDepStats()
	if err != nil {
		fatal("查询调用统计失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(stats)
		return
	}

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║      函数调用统计                        ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// 被调用最多的（热点）
	fmt.Println("  ── 被调用最多的函数 (TOP 10) ──")
	count := 0
	for _, s := range stats {
		if s.CallerCount > 0 {
			count++
			if count > 10 {
				break
			}
			fmt.Printf("    %-30s 被 %2d 个函数调用, 内部调用 %3d 次\n",
				s.FuncName, s.CallerCount, s.TotalCallCount)
		}
	}
	fmt.Println()

	// 从未被调用的
	fmt.Println("  ── 从未被调用的函数 (死代码候选) ──")
	deadCount := 0
	for _, s := range stats {
		if s.CallerCount == 0 {
			deadCount++
		}
	}
	fmt.Printf("    共 %d 个函数 (使用 -query dead 查看详情)\n", deadCount)
	fmt.Println()

	// 调用分支最广的
	fmt.Println("  ── 调用分支最广的函数 (TOP 5) ──")
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].CalleeCount > stats[j].CalleeCount
	})
	for i, s := range stats {
		if i >= 5 || s.CalleeCount == 0 {
			break
		}
		fmt.Printf("    %-30s 调用了 %2d 个不同函数\n", s.FuncName, s.CalleeCount)
	}
}

// printQueryCallers 查看谁调用了指定函数
func printQueryCallers(store *db.Store, funcName string) {
	callers, err := store.QueryCallers(funcName)
	if err != nil {
		fatal("查询调用方失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(callers)
		return
	}

	if len(callers) == 0 {
		fmt.Printf("没有函数调用 '%s'\n", funcName)
		return
	}

	fmt.Printf("调用 '%s' 的函数 (共 %d 个):\n\n", funcName, len(callers))
	for _, c := range callers {
		fmt.Printf("  %-30s [%s] %s L%d\n", c.Name, c.Language, c.FilePath, c.LineStart)
	}
}

// printQueryDead 列出未被调用的函数（通过调用图分析）
func printQueryDead(store *db.Store) {
	// 获取最新扫描会话 ID
	var sessionID int64
	err := store.DB.QueryRow(`SELECT id FROM scan_sessions ORDER BY id DESC LIMIT 1`).Scan(&sessionID)
	if err != nil {
		fatal("没有扫描数据，请先运行扫描: %v", err)
	}

	an := analyzer.New(store)
	graph, err := an.BuildCallGraph(sessionID)
	if err != nil {
		fatal("构建调用图失败: %v", err)
	}

	orphans := graph.FindOrphanFunctions()
	if outputFormat == "json" {
		type orphanBrief struct {
			Name      string `json:"name"`
			Language  string `json:"language"`
			FilePath  string `json:"file_path"`
			LineStart int    `json:"line_start"`
			LineEnd   int    `json:"line_end"`
			LineCount int    `json:"line_count"`
		}
		items := make([]orphanBrief, 0, len(orphans))
		for _, node := range orphans {
			items = append(items, orphanBrief{
				Name:      node.Function.Name,
				Language:  node.Function.Language,
				FilePath:  node.Function.FilePath,
				LineStart: node.Function.LineStart,
				LineEnd:   node.Function.LineEnd,
				LineCount: node.Function.LineEnd - node.Function.LineStart + 1,
			})
		}
		jsonOut(items)
		return
	}

	// 分类统计
	type tagCount struct {
		tag   string
		count int
	}
	tagTotal := make(map[string]int)
	classify := func(f *model.Function) string {
		// 测试函数
		if strings.HasSuffix(f.FilePath, "_test.go") {
			return "[test]"
		}
		// testdata 夹具
		if strings.Contains(f.FilePath, "\\testdata") {
			return "[fixture]"
		}
		// init — Go 运行时自动调用
		if f.Name == "init" {
			return "[runtime]"
		}
		// main — Go 程序入口
		if f.Name == "main" && f.PackageName == "main" {
			return "[entry]"
		}
		// MCP handler — 通过 handler 表注册调用
		if strings.Contains(f.FilePath, "internal\\mcp\\") && (strings.HasPrefix(f.Name, "handle") || strings.HasPrefix(f.Name, "resource")) {
			return "[mcp]"
		}
		// GetLang 访问器 — 通过 map/lang 表调度
		if strings.HasSuffix(f.Name, "GetLang") {
			return "[lang]"
		}
		// Parser 接口实现（Globals/Language）
		if f.Name == "Globals" || f.Name == "Language" {
			return "[iface]"
		}
		// 函数引用作为参数传递
		if f.Name == "isKeyword" || f.Name == "isAllUpper" || f.Name == "inStringLike" {
			return "[func-ref]"
		}
		// 闭包内的调用（AST 不穿透）
		if f.Name == "Close" && f.PackageName == "db" {
			return "[closure]"
		}
		// 默认：进一步通过 SQL 交叉验证
		callers, err := store.QueryCallers(f.Name)
		if err == nil && len(callers) > 0 {
			return "[cgap]" // 调用图遗漏（SQL 能找到调用者但图匹配没连上）
		}
		return "[dead]"
	}

	fmt.Printf("未被调用的函数 (共 %d 个):\n\n", len(orphans))
	for _, node := range orphans {
		tag := classify(node.Function)
		tagTotal[tag]++
		fmt.Printf("  %-9s %-30s [%s] %s L%d-L%d (%d 行)\n",
			tag, node.Function.Name, node.Function.Language, node.Function.FilePath,
			node.Function.LineStart, node.Function.LineEnd,
			node.Function.LineEnd-node.Function.LineStart+1)
	}
	if len(orphans) > 0 {
		fmt.Println()
		fmt.Println("  ── 分类统计 ──")
		for _, tc := range []struct{ tag, desc string }{
			{"[test]",     "测试函数 (go test 调用)"},
			{"[fixture]",  "测试数据文件中的函数"},
			{"[runtime]",  "Go 运行时入口 (init/main)"},
			{"[mcp]",      "MCP 服务注册处理函数"},
			{"[lang]",     "语言访问器 (通过 map 调度)"},
			{"[iface]",    "Parser 接口实现"},
			{"[func-ref]","作为函数参数传递"},
			{"[closure]",  "闭包体内调用 (AST 不穿透)"},
			{"[cgap]",     "调用图遗漏（SQL 查到调用者但图没连上）"},
			{"[dead]",     "⚠️ 真死代码，建议检查"},
		} {
			if cnt := tagTotal[tc.tag]; cnt > 0 {
				fmt.Printf("    %-9s %3d 个 — %s\n", tc.tag, cnt, tc.desc)
			}
		}
	}
	if len(orphans) == 0 {
		fmt.Println("  (无)")
	}
}

// printQueryMissing 列出被调用但未定义的函数
func printQueryMissing(store *db.Store) {
	missing, err := store.QueryMissing()
	if err != nil {
		fatal("查询缺失依赖失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(missing)
		return
	}

	if len(missing) == 0 {
		fmt.Println("未发现缺失的依赖引用。所有被调用的函数都有定义。")
		return
	}

	fmt.Printf("被调用但找不到定义的函数 (共 %d 个):\n\n", len(missing))
	for _, m := range missing {
		fmt.Printf("  - %s\n", m)
	}
}

// printQueryTop 列出最大的 N 个函数
func printQueryTop(store *db.Store, n int) {
	funcs, err := store.QueryTop(n)
	if err != nil {
		fatal("查询最大函数失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(funcs)
		return
	}

	fmt.Printf("行数最多的 %d 个函数:\n\n", len(funcs))
	for i, f := range funcs {
		fmt.Printf("  #%2d  %-30s [%s] %s L%d-L%d (%d 行, calls=%d, depth=%d)\n",
			i+1, f.Name, f.Language, f.FilePath, f.LineStart, f.LineEnd, f.LineCount, f.CallCount, f.NestingDepth)
	}
}

// printQueryDeep 列出嵌套深度 >= N 的函数
func printQueryDeep(store *db.Store, threshold int) {
	funcs, err := store.QueryDeepNesting(threshold)
	if err != nil {
		fatal("查询深度嵌套函数失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(funcs)
		return
	}

	if len(funcs) == 0 {
		fmt.Printf("没有嵌套深度 >= %d 的函数\n", threshold)
		return
	}

	fmt.Printf("嵌套深度 >= %d 的函数 (共 %d 个):\n\n", threshold, len(funcs))
	for _, f := range funcs {
		fmt.Printf("  %-30s depth=%d [%s] %s L%d-L%d (%d 行, calls=%d)\n",
			f.Name, f.NestingDepth, f.Language, f.FilePath, f.LineStart, f.LineEnd, f.LineCount, f.CallCount)
	}
}

// 🆕 printQueryComplexity 按圈复杂度列出
func printQueryComplexity(store *db.Store, limit int) {
	funcs, err := store.QueryByComplexity(limit)
	if err != nil {
		fatal("查询复杂度失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(funcs)
		return
	}
	fmt.Printf("圈复杂度最高的 %d 个函数:\n\n", len(funcs))
	for i, f := range funcs {
		fmt.Printf("  #%-2d %-30s cyclomatic=%d [%s] %s L%d-L%d (%d 行, params=%d)\n",
			i+1, f.Name, f.Cyclomatic, f.Language, f.FilePath, f.LineStart, f.LineEnd, f.LineCount, f.ParamCount)
	}
}

// 🆕 printQueryByParams 按参数数量列出
func printQueryByParams(store *db.Store, threshold int) {
	funcs, err := store.QueryByParams(threshold)
	if err != nil {
		fatal("查询参数失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(funcs)
		return
	}
	if len(funcs) == 0 {
		fmt.Printf("没有参数数量 >= %d 的函数\n", threshold)
		return
	}
	fmt.Printf("参数数量 >= %d 的函数 (共 %d 个):\n\n", threshold, len(funcs))
	for _, f := range funcs {
		fmt.Printf("  %-30s params=%d [%s] %s L%d-L%d (%s)\n",
			f.Name, f.ParamCount, f.Language, f.FilePath, f.LineStart, f.LineEnd, f.Parameters)
	}
}

// 🆕 printQueryAnon 列出含匿名函数的函数
func printQueryAnon(store *db.Store) {
	funcs, err := store.QueryAnonFuncs()
	if err != nil {
		fatal("查询匿名函数失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(funcs)
		return
	}
	if len(funcs) == 0 {
		fmt.Println("没有包含匿名函数的函数")
		return
	}
	fmt.Printf("包含匿名函数的函数 (共 %d 个):\n\n", len(funcs))
	for _, f := range funcs {
		fmt.Printf("  %-30s anon=%d [%s] %s L%d-L%d (%d 行)\n",
			f.Name, f.AnonFuncs, f.Language, f.FilePath, f.LineStart, f.LineEnd, f.LineCount)
	}
}

// 🆕 printQueryFiles 文件级统计
func printQueryFiles(store *db.Store) {
	metrics, err := store.QueryFileMetrics()
	if err != nil {
		fatal("查询文件统计失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(metrics)
		return
	}
	if len(metrics) == 0 {
		fmt.Println("没有文件统计数据 (需要先扫描)")
		return
	}
	fmt.Printf("文件统计 (共 %d 个文件):\n\n", len(metrics))
	for _, m := range metrics {
		fmt.Printf("  %s [%s]\n", m.FilePath, m.Language)
		fmt.Printf("    行: %d (代码 %d / 注释 %d / 空白 %d)  函数: %d  类型: %d\n",
			m.TotalLines, m.CodeLines, m.CommentLines, m.BlankLines, m.FuncCount, m.TypeCount)
		fmt.Printf("    平均圈复杂度: %.1f  最高圈复杂度: %d   公开/私有: %d/%d  方法: %d\n",
			m.AvgCyclomatic, m.MaxCyclomatic, m.PublicFuncs, m.PrivateFuncs, m.MethodsCount)
		fmt.Printf("    总参数: %d  最大参数: %d  总return: %d  总匿名: %d\n\n",
			m.TotalParameters, m.MaxParameters, m.TotalReturns, m.TotalAnonFuncs)
	}
}

// 🆕 printQueryTypes 类型定义列表
func printQueryTypes(store *db.Store) {
	defs, err := store.QueryTypeDefs()
	if err != nil {
		fatal("查询类型定义失败: %v", err)
	}
	if outputFormat == "json" {
		jsonOut(defs)
		return
	}
	if len(defs) == 0 {
		fmt.Println("没有类型定义数据 (需要先扫描)")
		return
	}
	fmt.Printf("类型定义 (共 %d 个):\n\n", len(defs))
	for _, d := range defs {
		fmt.Printf("  %-25s kind=%s [%s] %s L%d-L%d\n",
			d.Name, d.Kind, d.Language, d.FilePath, d.LineStart, d.LineEnd)
	}
}
