package db

import (
	"database/sql"
	"fmt"
	"strings"

	"code-detector/internal/model"
)

// ──────────────────────────────────────────────
// 查询/分析方法集 — 用于 -query 模式
// ──────────────────────────────────────────────

// QuerySummary 返回数据库整体概要
func (s *Store) QuerySummary() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 会话数
	var sessionCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM scan_sessions`).Scan(&sessionCount)
	result["session_count"] = sessionCount

	// 最新会话
	var latest model.ScanSession
	err := s.DB.QueryRow(
		`SELECT id, project_root, scan_time, duration_ms, file_count, func_count, var_count
		 FROM scan_sessions ORDER BY id DESC LIMIT 1`,
	).Scan(&latest.ID, &latest.ProjectRoot, &latest.ScanTime,
		&latest.Duration, &latest.FileCount, &latest.FuncCount, &latest.VarCount)
	if err == nil {
		result["latest_session"] = &latest
	}

	// 函数总数
	var funcCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM functions`).Scan(&funcCount)
	result["func_count"] = funcCount

	// 全局变量总数
	var varCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM global_vars`).Scan(&varCount)
	result["var_count"] = varCount

	// 依赖总数
	var depCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM function_deps`).Scan(&depCount)
	result["dep_count"] = depCount

	// 各语言函数分布
	langRows, _ := s.DB.Query(
		`SELECT language, COUNT(*) FROM functions GROUP BY language ORDER BY COUNT(*) DESC`,
	)
	if langRows != nil {
		defer langRows.Close()
		langMap := make(map[string]int)
		for langRows.Next() {
			var lang string
			var cnt int
			langRows.Scan(&lang, &cnt)
			langMap[lang] = cnt
		}
		result["lang_dist"] = langMap
	}

	// 各语言变量分布
	varLangRows, _ := s.DB.Query(
		`SELECT language, COUNT(*) FROM global_vars GROUP BY language ORDER BY COUNT(*) DESC`,
	)
	if varLangRows != nil {
		defer varLangRows.Close()
		varLangMap := make(map[string]int)
		for varLangRows.Next() {
			var lang string
			var cnt int
			varLangRows.Scan(&lang, &cnt)
			varLangMap[lang] = cnt
		}
		result["var_lang_dist"] = varLangMap
	}

	// 总函数体行数
	var totalLines int
	s.DB.QueryRow(`SELECT COALESCE(SUM(line_end - line_start + 1), 0) FROM functions`).Scan(&totalLines)
	result["total_lines"] = totalLines

	return result, nil
}

// QueryAllFunctions 返回所有函数（不含 body 以节省内存）
type FuncBrief struct {
	ID           int64
	Name         string
	PackageName  string
	Language     string
	FilePath     string
	LineStart    int
	LineEnd      int
	LineCount    int
	CallCount    int
	NestingDepth int
}

func (s *Store) QueryAllFunctions() ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth
		 FROM functions ORDER BY file_path, line_start`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*FuncBrief
	for rows.Next() {
		f := &FuncBrief{}
		if err := rows.Scan(&f.ID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, err
		}
		f.LineCount = f.LineEnd - f.LineStart + 1
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}

// QueryFuncByName 按名称模糊查找函数（支持前缀匹配）
func (s *Store) QueryFuncByName(name string) ([]*model.Function, error) {
	rows, err := s.DB.Query(
		`SELECT id, session_id, name, package_name, language, file_path,
		        line_start, line_end, body, call_count, nesting_depth
		 FROM functions WHERE name = ? ORDER BY file_path, line_start`, name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*model.Function
	for rows.Next() {
		f := &model.Function{}
		if err := rows.Scan(&f.ID, &f.SessionID, &f.Name, &f.PackageName, &f.Language,
			&f.FilePath, &f.LineStart, &f.LineEnd, &f.Body, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}

// QueryAllGlobalVars 返回所有全局变量
func (s *Store) QueryAllGlobalVars() ([]*model.GlobalVariable, error) {
	rows, err := s.DB.Query(
		`SELECT id, session_id, name, var_type, language, package_name, visibility, file_path, line_num, is_const
		 FROM global_vars ORDER BY file_path, line_num`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vars []*model.GlobalVariable
	for rows.Next() {
		v := &model.GlobalVariable{}
		if err := rows.Scan(&v.ID, &v.SessionID, &v.Name, &v.VarType, &v.Language,
			&v.PackageName, &v.Visibility, &v.FilePath, &v.LineNum, &v.IsConst); err != nil {
			return nil, err
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

// QueryDeadFunctions 返回 call_count = 0 的函数（可能死代码）
func (s *Store) QueryDeadFunctions() ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth
		 FROM functions WHERE call_count = 0
		 ORDER BY file_path, line_start`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*FuncBrief
	for rows.Next() {
		f := &FuncBrief{}
		if err := rows.Scan(&f.ID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, err
		}
		f.LineCount = f.LineEnd - f.LineStart + 1
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}

// QueryTopFunctions 返回按行数排序的最大函数
func (s *Store) QueryTopFunctions(limit int) ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth
		 FROM functions ORDER BY (line_end - line_start + 1) DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*FuncBrief
	for rows.Next() {
		f := &FuncBrief{}
		if err := rows.Scan(&f.ID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, err
		}
		f.LineCount = f.LineEnd - f.LineStart + 1
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}

// QueryMissingDeps 返回被调用但找不到定义的函数名
func (s *Store) QueryMissingDeps() ([]string, error) {
	// 获取所有被引用的 callee_name
	calleeRows, err := s.DB.Query(`SELECT DISTINCT callee_name FROM function_deps`)
	if err != nil {
		return nil, err
	}
	defer calleeRows.Close()

	callees := make(map[string]bool)
	for calleeRows.Next() {
		var name string
		calleeRows.Scan(&name)
		callees[name] = true
	}
	if err := calleeRows.Err(); err != nil {
		return nil, err
	}

	// 获取所有定义的函数名（含包前缀）
	funcRows, err := s.DB.Query(`SELECT name, package_name FROM functions`)
	if err != nil {
		return nil, err
	}
	defer funcRows.Close()

	defined := make(map[string]bool)
	definedLower := make(map[string]bool)
	for funcRows.Next() {
		var name, pkg string
		funcRows.Scan(&name, &pkg)
		defined[name] = true
		definedLower[strings.ToLower(name)] = true
		if pkg != "" {
			prefixed := pkg + "." + name
			defined[prefixed] = true
			definedLower[strings.ToLower(prefixed)] = true
		}
	}
	if err := funcRows.Err(); err != nil {
		return nil, err
	}

	var missing []string
	for callee := range callees {
		if !defined[callee] && !definedLower[strings.ToLower(callee)] && !isKnownStdFunc(callee) {
			missing = append(missing, callee)
		}
	}
	return missing, nil
}

// isKnownStdFunc 判断函数名是否为 Go 标准库或已知第三方库函数
// 与 internal/parser/go_parser.go 中的 goKeywords 保持同步
func isKnownStdFunc(name string) bool {
	switch name {
	// Go 语言关键字和内置函数
	case "if", "for", "switch", "select", "case", "default", "return", "go", "defer",
		"range", "break", "continue", "fallthrough", "else", "map", "chan", "type",
		"struct", "interface", "func", "make", "new", "append", "len", "cap", "copy",
		"close", "delete", "panic", "recover", "print", "println", "error", "nil",
		"true", "false", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "complex64", "complex128",
		"byte", "rune", "string", "bool", "uintptr",
		// fmt
		"Printf", "Fprint", "Fprintf", "Fprintln", "Sprint", "Sprintf", "Sprintln",
		"Errorf", "Scanf", "Scanln", "Fscanf", "Fscan", "Fscanln",
		"Print", "Println", "Format",
		// os / filepath
		"Open", "Create", "OpenFile", "ReadFile", "WriteFile",
		"Stat", "Mkdir", "MkdirAll", "Remove", "RemoveAll", "Rename",
		"Getenv", "Setenv", "Getwd", "Chdir", "Exit", "Getpid",
		"ReadDir", "Readlink", "TempDir", "UserHomeDir",
		"ReadAll", "CopyN", "ReadFull", "WriteString", "ReadAtLeast", "LimitReader",
		"NewWriter", "NewReadWriter",
		"Dir", "Executable", "Ext", "IsDir", "IsNotExist",
		"ModTime", "Name", "SameFile", "Size", "Walk", "Rel",
		"ExecContext",
		// strings
		"Join", "Split", "SplitN", "Contains", "ContainsAny",
		"HasPrefix", "HasSuffix", "Replace", "ReplaceAll",
		"Trim", "TrimSpace", "TrimLeft", "TrimRight",
		"TrimPrefix", "TrimSuffix", "ToLower", "ToUpper",
		"ToTitle", "Repeat", "Index", "LastIndex", "IndexByte",
		"Count", "Fields", "EqualFold", "NewReader", "NewReplacer",
		// strconv
		"Atoi", "Itoa", "ParseInt", "ParseUint", "ParseFloat",
		"FormatInt", "FormatUint", "FormatFloat", "Quote", "Unquote",
		// encoding/json
		"Marshal", "Unmarshal", "NewDecoder", "NewEncoder",
		"Encode", "Decode", "MarshalIndent", "Compact", "Indent",
		// net/http
		"Handle", "HandleFunc", "ListenAndServe", "ListenAndServeTLS",
		"NewRequest", "NewServeMux", "Redirect", "NotFound",
		"Head", "PostForm", "ReadRequest", "ReadResponse",
		// time
		"Now", "Since", "Until", "Sleep", "Milliseconds", "Unix",
		"NewTicker", "NewTimer", "After", "AfterFunc",
		"ParseDuration",
		// sync / context
		"Wait", "Done", "Add", "Once",
		"Lock", "Unlock", "RLock", "RUnlock", "NewCond", "Pool",
		"Background", "TODO",
		"WithCancel", "WithDeadline", "WithTimeout", "WithValue",
		// sort
		"Sort", "Slice", "SliceStable", "Search", "SearchInts",
		"Ints", "Float64s", "Strings", "Reverse", "IsSorted",
		// math
		"Ceil", "Floor", "Round", "Max", "Min",
		"Pow", "Sqrt", "Sin", "Cos", "Tan", "Log", "Exp", "Mod",
		// log
		"Fatal", "Fatalf", "Fatalln",
		// database/sql
		"Begin", "Commit", "Conn", "LastInsertId", "Prepare", "QueryRow",
		"Rollback", "Err", "Next",
		// regexp
		"Compile", "MustCompile", "FindAllStringSubmatch", "FindStringSubmatch",
		"FindStringSubmatchIndex", "SubexpIndex",
		// flag
		"BoolVar", "IntVar", "Int64Var", "StringVar", "NArg", "Arg",
		// bytes
		"WriteByte",
		// crypto
		"Sum256",
		// unicode/utf8
		"IsLetter", "IsUpper", "Valid",
		// std 遗漏补充 + 已知非函数引用的项目变量/参数名
		"NumCPU", "Abs", "Bytes", "Exec", "Grow", "Query", "Read", "String",
		"cleanup", "makeStringMask", "skipFn", "flushBatch", "allUpperSkip":
		return true
	}
	return false
}

// QueryDepStats 返回调用统计
type DepStat struct {
	FuncName       string
	CallerCount    int // 多少函数调用了它
	CalleeCount    int // 它调用了多少不同函数
	TotalCallCount int // 函数内部总调用次数
}

func (s *Store) QueryDepStats() ([]*DepStat, error) {
	rows, err := s.DB.Query(
		`SELECT f.id, f.name,
		        (SELECT COUNT(DISTINCT d2.caller_id) FROM function_deps d2 WHERE d2.callee_name = f.name) AS caller_cnt,
		        (SELECT COUNT(*) FROM function_deps d3 WHERE d3.caller_id = f.id) AS callee_cnt,
		        f.call_count
		 FROM functions f
		 ORDER BY caller_cnt DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*DepStat
	for rows.Next() {
		ds := &DepStat{}
		var id int64
		if err := rows.Scan(&id, &ds.FuncName, &ds.CallerCount, &ds.CalleeCount, &ds.TotalCallCount); err != nil {
			return nil, err
		}
		stats = append(stats, ds)
	}
	return stats, rows.Err()
}

// QueryFuncBody 返回指定函数的 body（大字段单独查，避免每次都加载）
func (s *Store) QueryFuncBody(funcID int64) (string, error) {
	var body string
	err := s.DB.QueryRow(`SELECT body FROM functions WHERE id = ?`, funcID).Scan(&body)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("function id %d not found", funcID)
	}
	return body, err
}

// QueryDeepNesting 返回嵌套深度 >= threshold 的函数
func (s *Store) QueryDeepNesting(threshold int) ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth
		 FROM functions WHERE nesting_depth >= ?
		 ORDER BY nesting_depth DESC, (line_end - line_start + 1) DESC`, threshold,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*FuncBrief
	for rows.Next() {
		f := &FuncBrief{}
		if err := rows.Scan(&f.ID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, err
		}
		f.LineCount = f.LineEnd - f.LineStart + 1
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}
