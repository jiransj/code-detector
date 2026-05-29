package fscanner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"code-detector/internal/config"
	"code-detector/internal/db"
	"code-detector/internal/model"
	"code-detector/internal/parser"
)

// Scanner 文件系统扫描器
type Scanner struct {
	Config      *config.Config
	Registry    *parser.Registry
	Store       *db.Store
	SkipDirs    map[string]bool
	MaxFileSize int64 // 最大文件字节数，超过则跳过
	Workers     int   // 并发工作数
	Verbose     bool
	LangFilter  map[string]bool // 为空则扫描所有
	Incremental bool  // 增量扫描：仅重新解析变更文件
}

// New 创建扫描器
func New(cfg *config.Config, reg *parser.Registry, store *db.Store) *Scanner {
	return &Scanner{
		Config:      cfg,
		Registry:    reg,
		Store:       store,
		SkipDirs:    defaultSkipDirs(),
		MaxFileSize: 1 * 1024 * 1024, // 默认 1MB
		Workers:     runtime.NumCPU(),
		LangFilter:  make(map[string]bool),
	}
}

func defaultSkipDirs() map[string]bool {
	return map[string]bool{
		".git":            true,
		"node_modules":    true,
		"vendor":          true,
		"__pycache__":     true,
		".svn":            true,
		".hg":             true,
		".gradle":         true,
		"target":          true, // Maven/Gradle output
		"bin":             true,
		"obj":             true,
		"dist":            true,
		"build":           true,
		".idea":           true,
		".vscode":         true,
		"venv":            true,
		".venv":           true,
		"env":             true,
		".env":            true,
		"third_party":     true,
		"third-party":     true,
		"packages":        true,
		"bower_components": true,
	}
}

// Scan 遍历 root 目录，解析所有支持的文件，返回结果
// 拆分为: collectFiles → parseConcurrently → writeResults 三个子步骤
func (s *Scanner) Scan(root string) (*model.ScanResult, error) {
	start := time.Now()

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	// 拒绝扫描系统临时目录
	tempDir := os.TempDir()
	var tempPrefix string
	if tempDir != "" {
		if tp, err := filepath.Abs(tempDir); err == nil {
			tempPrefix = tp
			if strings.HasPrefix(absRoot, tempPrefix+string(filepath.Separator)) {
				return nil, fmt.Errorf("refusing to scan system temp directory: %s", absRoot)
			}
		}
	}

	// 检查根目录是否存在
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absRoot)
	}

	// 创建扫描会话
	sessionID, err := s.Store.CreateSession(absRoot)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	result := &model.ScanResult{
		Session: model.ScanSession{
			ProjectRoot: absRoot,
		},
	}

	// 步骤1: 收集文件
	files, fileCount, err := s.collectFiles(absRoot, tempPrefix, result)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		if err := s.Store.UpdateSession(sessionID, time.Since(start), 0, 0, 0); err != nil && s.Verbose {
			fmt.Fprintf(os.Stderr, "warn: UpdateSession: %v\n", err)
		}
		result.Duration = time.Since(start)
		result.FileCount = 0
		return result, nil
	}
	result.FileCount = fileCount

	// 步骤2+3: 并发解析文件 + 流水线写入数据库
	resultsCh := s.parseConcurrently(files, absRoot, sessionID)
	if err := s.writeResults(resultsCh, sessionID, start, fileCount, result); err != nil {
		return nil, err
	}

	return result, nil
}

// collectFiles 遍历 root 目录收集需要扫描的文件列表
func (s *Scanner) collectFiles(absRoot string, tempPrefix string, result *model.ScanResult) ([]string, int, error) {
	var files []string
	var seenFileInfos []os.FileInfo

	err := filepath.Walk(absRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			if s.Verbose {
				fmt.Fprintf(os.Stderr, "warn: error accessing %s: %v\n", path, err)
			}
			return nil
		}

		if fi.IsDir() {
			dirName := fi.Name()
			if s.SkipDirs[dirName] || strings.HasPrefix(dirName, ".") && dirName != "." {
				return filepath.SkipDir
			}
			if tempPrefix != "" && strings.HasPrefix(path, tempPrefix+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			// 跳过名称含 % 的目录（疑似未展开的环境变量引用，如字面 %TEMP%）
			if strings.Contains(dirName, "%") {
				if s.Verbose {
					fmt.Fprintf(os.Stderr, "skip env-var dir: %s\n", path)
				}
				return filepath.SkipDir
			}
			return nil
		}

		if fi.Size() > s.MaxFileSize {
			if s.Verbose {
				fmt.Fprintf(os.Stderr, "skip large file: %s (%d bytes)\n", path, fi.Size())
			}
			result.SkipCount++
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if s.Registry.GetByExt(ext) == nil {
			return nil
		}

		langCfg := s.Config.GetLanguageConfigsMap()[ext]
		if langCfg != nil && len(s.LangFilter) > 0 && !s.LangFilter[langCfg.Name] {
			return nil
		}

		for _, seen := range seenFileInfos {
			if os.SameFile(fi, seen) {
				if s.Verbose {
					fmt.Fprintf(os.Stderr, "skip duplicate (hardlink): %s\n", path)
				}
				return nil
			}
		}
		seenFileInfos = append(seenFileInfos, fi)

		if s.Incremental {
			cachedMtime, _, found, err := s.Store.GetFileCache(path)
			if err != nil && s.Verbose {
				fmt.Fprintf(os.Stderr, "warn: cache lookup failed for %s: %v\n", path, err)
			}
			if found && cachedMtime == fi.ModTime().Unix() {
				if s.Verbose {
					fmt.Fprintf(os.Stderr, "skip (cached): %s\n", path)
				}
				return nil
			}
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("walk root: %w", err)
	}
	return files, len(files), nil
}

// parseResult 单个文件的解析结果
type parseResult struct {
	path      string
	functions []*model.Function
	globals   []*model.GlobalVariable
	err       error
}

// parseConcurrently 并发解析所有收集到的文件，通过 channel 流式返回结果
func (s *Scanner) parseConcurrently(files []string, absRoot string, sessionID int64) <-chan parseResult {
	type parseJob struct {
		path string
	}

	fileCount := len(files)
	jobs := make(chan parseJob, fileCount)
	results := make(chan parseResult, fileCount)
	var wg sync.WaitGroup

	workerCount := s.Workers
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > 16 {
		workerCount = 16
	}

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				funcs, globals, err := s.parseFile(job.path)
				if err == nil && s.Incremental {
					if fi, statErr := os.Stat(job.path); statErr == nil {
						_ = s.Store.UpsertFileCache(job.path, fi.ModTime().Unix(), "", sessionID)
					}
				}
				results <- parseResult{path: job.path, functions: funcs, globals: globals, err: err}
			}
		}()
	}

	go func() {
		for _, f := range files {
			jobs <- parseJob{path: f}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

const batchFlushThreshold = 5000

// writeResults 从 channel 消费解析结果，分批流水线写入数据库（与解析重叠执行）
func (s *Scanner) writeResults(resultsCh <-chan parseResult, sessionID int64, start time.Time, fileCount int, result *model.ScanResult) error {
	var funcBatch []*model.Function
	var varBatch []*model.GlobalVariable
	var parseErrors []string

	// 复用 result 的切片来累积全量数据（用于最终输出），funcBatch/varBatch 为当前批次指针
	result.Functions = result.Functions[:0]
	result.GlobalVars = result.GlobalVars[:0]

	// flushBatch 将当前批次写入数据库
	flushBatch := func() error {
		if len(funcBatch) == 0 && len(varBatch) == 0 {
			return nil
		}
		if s.Verbose {
			fmt.Fprintf(os.Stderr, "info: flushing batch: %d functions, %d global vars...\n",
				len(funcBatch), len(varBatch))
		}

		if len(funcBatch) > 0 {
			ids, skipped, changed, err := s.Store.BatchInsertFunctions(funcBatch, sessionID)
			if err != nil {
				return fmt.Errorf("batch insert: %w", err)
			}
			if s.Verbose {
				fmt.Fprintf(os.Stderr, "info: functions: %d new, %d skipped (unchanged), %d changed\n",
					len(ids)-skipped-changed, skipped, changed)
			}

			if skipped < len(funcBatch) {
				depsMap := make(map[int64][]string)
				for i, id := range ids {
					if len(funcBatch[i].Dependencies) > 0 {
						depsMap[id] = funcBatch[i].Dependencies
					}
				}
				if len(depsMap) > 0 {
					if err := s.Store.BatchInsertDeps(depsMap); err != nil {
						return fmt.Errorf("batch insert deps: %w", err)
					}
				}
			} else if s.Verbose {
				fmt.Fprintf(os.Stderr, "info: deps skipped (no function changes)\n")
			}
		}

		if len(varBatch) > 0 {
			skipped, changed, err := s.Store.BatchInsertGlobalVars(varBatch, sessionID)
			if err != nil {
				return fmt.Errorf("batch insert globals: %w", err)
			}
			if s.Verbose {
				fmt.Fprintf(os.Stderr, "info: global vars: %d new, %d skipped (unchanged), %d changed\n",
					len(varBatch)-skipped-changed, skipped, changed)
			}
		}

		funcBatch = funcBatch[:0]
		varBatch = varBatch[:0]
		return nil
	}

	for res := range resultsCh {
		if res.err != nil {
			errMsg := fmt.Sprintf("parse %s: %v", res.path, res.err)
			parseErrors = append(parseErrors, errMsg)
			if s.Verbose {
				fmt.Fprintf(os.Stderr, "error: %s\n", errMsg)
			}
			continue
		}
		relPath, _ := filepath.Rel(result.Session.ProjectRoot, res.path)
		for _, f := range res.functions {
			f.FilePath = relPath
			funcBatch = append(funcBatch, f)
		}
		for _, g := range res.globals {
			g.FilePath = relPath
			varBatch = append(varBatch, g)
		}

		result.Functions = append(result.Functions, res.functions...)
		result.GlobalVars = append(result.GlobalVars, res.globals...)

		// 累积达到阈值时提前 flush，与剩余解析任务重叠执行
		if len(funcBatch) >= batchFlushThreshold || len(varBatch) >= batchFlushThreshold {
			if err := flushBatch(); err != nil {
				return err
			}
		}
	}

	// 刷新最后一批
	if err := flushBatch(); err != nil {
		return err
	}

	result.Errors = parseErrors

	// 更新会话统计
	if err := s.Store.UpdateSession(sessionID, time.Since(start), fileCount, len(result.Functions), len(result.GlobalVars)); err != nil && s.Verbose {
		fmt.Fprintf(os.Stderr, "warn: UpdateSession: %v\n", err)
	}

	result.Duration = time.Since(start)
	result.Session.ID = sessionID
	result.Session.FileCount = fileCount
	result.Session.FuncCount = len(result.Functions)
	result.Session.VarCount = len(result.GlobalVars)
	result.Session.Duration = result.Duration.Milliseconds()

	// 批量写入完成后 checkpoint WAL，避免 WAL 无限膨胀
	s.Store.Checkpoint()

	return nil
}

// parseFile 解析单个文件，自动处理编码，返回 (函数列表, 全局变量列表, 错误)
func (s *Scanner) parseFile(path string) ([]*model.Function, []*model.GlobalVariable, error) {
	ext := strings.ToLower(filepath.Ext(path))
	p := s.Registry.GetByExt(ext)
	if p == nil {
		return nil, nil, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read file: %w", err)
	}

	// 编码检测与转换
	content, encoding, err := detectAndConvertEncoding(content)
	if err != nil {
		if s.Verbose {
			fmt.Fprintf(os.Stderr, "warn: 编码转换失败 %s (%s): %v\n", path, encoding, err)
		}
		return nil, nil, nil
	}
	if encoding != "utf-8" && s.Verbose {
		fmt.Fprintf(os.Stderr, "info: %s 编码为 %s，已转换为 UTF-8\n", path, encoding)
	}

	// 检查是否为二进制文件（针对已转换的内容重新检测）
	if len(content) > 0 && detectBinaryAfterEncoding(content) {
		if s.Verbose {
			fmt.Fprintf(os.Stderr, "warn: 跳过二进制文件: %s\n", path)
		}
		return nil, nil, nil
	}

	// 解析函数
	funcs, err := p.Parse(path, content)
	if err != nil {
		return nil, nil, err
	}
	// 解析全局变量
	globals, err := p.Globals(path, content)
	if err != nil {
		// 全局变量解析失败不中断，记录警告
		if s.Verbose {
			fmt.Fprintf(os.Stderr, "warn: 全局变量解析失败 %s: %v\n", path, err)
		}
		globals = nil
	}

	return funcs, globals, nil
}

// detectAndConvertEncoding 检测并转换文件编码为 UTF-8
// 返回 (转换后的内容, 原编码名称, 错误)
func detectAndConvertEncoding(content []byte) ([]byte, string, error) {
	if len(content) == 0 {
		return content, "utf-8", nil
	}

	// 1. 检测 BOM
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		// UTF-8 with BOM: 去掉前 3 字节
		return content[3:], "utf-8-bom", nil
	}

	if len(content) >= 2 {
		if content[0] == 0xFF && content[1] == 0xFE {
			// UTF-16 LE with BOM
			decoded := decodeUTF16LE(content[2:])
			return decoded, "utf-16-le", nil
		}
		if content[0] == 0xFE && content[1] == 0xFF {
			// UTF-16 BE with BOM
			decoded := decodeUTF16BE(content[2:])
			return decoded, "utf-16-be", nil
		}
	}

	// 2. 尝试 UTF-16 无 BOM 检测（前 512 字节中奇数位为0，偶数位非0）
	if len(content) >= 4 && isLikelyUTF16LE(content) {
		decoded := decodeUTF16LE(content)
		return decoded, "utf-16-le", nil
	}
	if len(content) >= 4 && isLikelyUTF16BE(content) {
		decoded := decodeUTF16BE(content)
		return decoded, "utf-16-be", nil
	}

	// 3. 检查是否为有效 UTF-8
	if utf8.Valid(content) {
		return content, "utf-8", nil
	}

	// 4. 非 UTF-8 编码（可能是 GBK 等）
	// 对于非 UTF-8，尝试启发式：如果大部分字节是高字节且不是 UTF-8，
	// 说明是 GBK/Shift-JIS 等编码，无法在无依赖库的情况下转换
	// 返回原始内容并标记 warning，让解析器尽力处理
	if isLikelyTextEncoding(content) {
		return content, "unknown-non-utf8", nil
	}

	// 看起来像二进制
	return nil, "binary", fmt.Errorf("content appears to be binary")
}

// s 需要在调用 isLikelyTextEncoding 时传入... 改为包级函数
func isLikelyTextEncoding(content []byte) bool {
	if len(content) == 0 {
		return true
	}
	highByteCount := 0
	nullCount := 0
	total := len(content)
	if total > 1024 {
		total = 1024
	}
	for i := 0; i < total; i++ {
		if content[i] == 0 {
			nullCount++
		} else if content[i] > 0x7F {
			highByteCount++
		}
	}
	// 如果空字节占比 > 20%，不像文本
	if nullCount*5 > total {
		return false
	}
	// 如果高字节占比 > 60%，可能是 GBK/Shift-JIS 等
	if highByteCount*2 > total {
		return true
	}
	return true
}

// decodeUTF16LE 将 UTF-16 LE 字节序列解码为 UTF-8
func decodeUTF16LE(data []byte) []byte {
	// 删除尾部空字节，但确保结果长度为偶数
	// 注意：不能直接 TrimRight("\x00")，因为 UTF-16LE 中每个 ASCII 字符
	// 的高字节就是 0x00，TrimRight 会破坏最后一个字符（如 "AB"→[0x41,0x00,0x42,0x00]→[0x41,0x00,0x42] 奇数长）
	trimmed := bytes.TrimRight(data, "\x00")
	if len(trimmed)%2 == 0 {
		data = trimmed
	}

	var buf bytes.Buffer
	for i := 0; i+1 < len(data); i += 2 {
		// 简单处理：只处理 BMP 范围内的字符（基本平面）
		code := uint16(data[i]) | uint16(data[i+1])<<8
		if code < 0x80 {
			buf.WriteByte(byte(code))
		} else if code < 0x800 {
			buf.WriteByte(0xC0 | byte(code>>6))
			buf.WriteByte(0x80 | byte(code&0x3F))
		} else {
			buf.WriteByte(0xE0 | byte(code>>12))
			buf.WriteByte(0x80 | byte((code>>6)&0x3F))
			buf.WriteByte(0x80 | byte(code&0x3F))
		}
	}
	return buf.Bytes()
}

// decodeUTF16BE 将 UTF-16 BE 字节序列解码为 UTF-8
func decodeUTF16BE(data []byte) []byte {
	// 删除尾部空字节，但确保结果长度为偶数
	trimmed := bytes.TrimRight(data, "\x00")
	if len(trimmed)%2 == 0 {
		data = trimmed
	}

	var buf bytes.Buffer
	for i := 0; i+1 < len(data); i += 2 {
		code := uint16(data[i])<<8 | uint16(data[i+1])
		if code < 0x80 {
			buf.WriteByte(byte(code))
		} else if code < 0x800 {
			buf.WriteByte(0xC0 | byte(code>>6))
			buf.WriteByte(0x80 | byte(code&0x3F))
		} else {
			buf.WriteByte(0xE0 | byte(code>>12))
			buf.WriteByte(0x80 | byte((code>>6)&0x3F))
			buf.WriteByte(0x80 | byte(code&0x3F))
		}
	}
	return buf.Bytes()
}

// detectUTF16 检测是否为无 BOM 的 UTF-16（LE 或 BE）
// bigEndian=true 检测 BE（偶数位=0，奇数位≠0），false 检测 LE（偶数位≠0，奇数位=0）
func detectUTF16(content []byte, bigEndian bool) bool {
	checkLen := len(content)
	if checkLen > 512 {
		checkLen = 512
	}
	if checkLen < 4 || checkLen%2 != 0 {
		return false
	}

	c1, c2 := 0, 0
	checks := 0
	for i := 0; i+1 < checkLen; i += 2 {
		checks++
		a, b := content[i], content[i+1]
		if bigEndian {
			// BE: 偶数位置=0 && 奇数位置≠0
			if a == 0 && b != 0 {
				c1++
			}
			if b != 0 {
				c2++
			}
		} else {
			// LE: 奇数位置=0 && 偶数位置≠0
			if b == 0 && a != 0 {
				c1++
			}
			if a != 0 {
				c2++
			}
		}
	}
	// >70% 的位置符合模式则判断为 UTF-16
	return checks > 4 && c1*10 > checks*7 && c2*10 > checks*7
}

// isLikelyUTF16LE 保留兼容旧调用的别名
func isLikelyUTF16LE(content []byte) bool { return detectUTF16(content, false) }

// isLikelyUTF16BE 保留兼容旧调用的别名
func isLikelyUTF16BE(content []byte) bool { return detectUTF16(content, true) }

// detectBinaryAfterEncoding 检测编码转换后是否仍为二进制
func detectBinaryAfterEncoding(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	checkLen := 512
	if len(content) < checkLen {
		checkLen = len(content)
	}
	// 检查不可打印字符比例（允许常见控制字符 \t \n \r）
	controlCount := 0
	for i := 0; i < checkLen; i++ {
		b := content[i]
		if b == 0 || (b < 0x20 && b != '\t' && b != '\n' && b != '\r') {
			controlCount++
		}
	}
	return controlCount*10 > checkLen*3 // > 30% 不可打印字符则视为二进制
}
