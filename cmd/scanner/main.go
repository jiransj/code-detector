package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"code-detector/internal/analyzer"
	"code-detector/internal/config"
	"code-detector/internal/db"
	"code-detector/internal/fscanner"
	"code-detector/internal/model"
	"code-detector/internal/parser"
)

const version = "0.5"

// cleanup 退出前需执行的清理（关闭 DB 确保 WAL 回归），main 中初始化，fatal 中调用
var cleanup func()

// waitForExit 在程序退出前等待用户按键（无论是命令行还是双击启动）
func waitForExit() {
	fmt.Print("\n按 Enter 键退出...")
	// 逐字节读取直到换行，兼容双击启动时的 stdin 状态
	var buf [1]byte
	for {
		if _, err := os.Stdin.Read(buf[:]); err != nil {
			break
		}
		if buf[0] == '\n' {
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
	reg.Register(parser.NewGoParser(), ".go")
	reg.Register(parser.NewPythonParser(), ".py")
	reg.Register(parser.NewJavaParser(), ".java")
	reg.Register(parser.NewKotlinParser(), ".kt", ".kts")
	reg.Register(parser.NewJavascriptParser(), ".js", ".jsx", ".mjs")
	reg.Register(parser.NewTypescriptParser(), ".ts", ".tsx")
	reg.Register(parser.NewCSharpParser(), ".cs")
	reg.Register(parser.NewCPPParser(), ".cpp", ".cxx", ".cc", ".c", ".h", ".hpp")
	reg.Register(parser.NewRustParser(), ".rs")
	reg.Register(parser.NewRubyParser(), ".rb")

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

	dbPath, cfgPath, langs, skipDirs, verbose, showVer, workers, buildGraph, incremental, maxSize := parseFlags()
	if showVer {
		fmt.Printf("code-detector v%s\n", version)
		return
	}

	projectRoot := resolveProjectRoot()
	_ = initProjectRoot(projectRoot)       // 验证路径
	cfg := initConfig(cfgPath, verbose)
	store := initDB(dbPath, verbose)
	cleanup = func() { store.Close() }
	defer cleanup()
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
func parseFlags() (dbPath string, cfgPath string, langs string, skipDirs string, verbose bool, showVer bool, workers int, buildGraph bool, incremental bool, maxSize int64) {
	flag.StringVar(&langs, "lang", "", "扫描语言，逗号分隔 (如 go,py,java)")
	flag.StringVar(&dbPath, "db", "scaned_db/scan_result.db", "SQLite 数据库路径（默认 scaned_db/ 目录下）")
	flag.StringVar(&cfgPath, "config", "config.yaml", "配置文件路径")
	flag.Int64Var(&maxSize, "max-size", 1048576, "单文件最大字节数 (默认 1MB)")
	flag.StringVar(&skipDirs, "skip-dirs", "", "额外跳过目录，逗号分隔")
	flag.BoolVar(&verbose, "verbose", false, "输出详细日志")
	flag.BoolVar(&showVer, "v", false, "显示版本号")
	flag.IntVar(&workers, "workers", 0, "并发工作数 (默认 CPU 核数)")
	flag.BoolVar(&buildGraph, "graph", false, "扫描完成后构建调用图并输出统计")
	flag.BoolVar(&incremental, "incremental", false, "增量扫描：仅重新解析 mtime 变更的文件")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `code-detector v%s — 多编程语言函数扫描工具

用法:
  code-detector [选项] <项目根目录>

选项:
  -lang <列表>     扫描语言，逗号分隔 (如 go,py,java)
  -db <路径>       SQLite 数据库路径（默认 scaned_db/scan_result.db）
  -config <路径>   配置文件路径（默认 config.yaml）
  -max-size <N>    单文件最大字节数 (默认 1MB)
  -skip-dirs <列表> 额外跳过目录，逗号分隔
  -workers <N>     并发工作数 (默认 CPU 核数)
  -verbose         输出详细日志
  -graph           扫描完成后构建调用图并输出统计
  -incremental     增量扫描：仅重新解析 mtime 变更的文件
  -v               显示版本号

示例:
  code-detector -lang go,java -db myproject.db ./myproject
  code-detector -lang go,python,js -verbose ./src
  code-detector -verbose -skip-dirs .git,bin,obj ./src
  code-detector -graph ./myproject
`, version)
	}
	flag.Parse()
	return
}
