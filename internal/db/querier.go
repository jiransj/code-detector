package db

import (
	"database/sql"
	"fmt"

	"code-detector/internal/model"
)

// FuncBrief 函数摘要（用于列表展示）
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
	// AST 增强字段
	Parameters   string
	ReturnTypes  string
	Receiver     string
	IsMethod     bool
	Visibility   string
	Cyclomatic   int
	ParamCount   int
	ReturnCount  int
	StmtCount    int
	AnonFuncs    int
}

// DepStat 调用统计单条
type DepStat struct {
	FuncName       string
	CallerCount    int
	CalleeCount    int
	TotalCallCount int
}

// ─── 查询 ──────────────────────────────────────────────

func (s *Store) QuerySummary() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	var sessionCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM scan_sessions`).Scan(&sessionCount)
	result["session_count"] = sessionCount
	var latest model.ScanSession
	err := s.DB.QueryRow(
		`SELECT id, project_root, scan_time, duration_ms, file_count, func_count, var_count
		 FROM scan_sessions ORDER BY id DESC LIMIT 1`,
	).Scan(&latest.ID, &latest.ProjectRoot, &latest.ScanTime,
		&latest.Duration, &latest.FileCount, &latest.FuncCount, &latest.VarCount)
	if err == nil {
		result["latest_session"] = &latest
	}
	var funcCount, varCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM functions`).Scan(&funcCount)
	s.DB.QueryRow(`SELECT COUNT(*) FROM global_vars`).Scan(&varCount)
	result["func_count"] = funcCount
	result["var_count"] = varCount
	// 依赖关系总数
	var depCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM function_deps`).Scan(&depCount)
	result["dep_count"] = depCount
	// 函数体总行数
	var bodyLines int
	s.DB.QueryRow(`SELECT COALESCE(SUM(line_end - line_start + 1), 0) FROM functions`).Scan(&bodyLines)
	result["body_lines"] = bodyLines
	langRows, err := s.DB.Query(`SELECT language, COUNT(*) FROM functions GROUP BY language ORDER BY COUNT(*) DESC`)
	if err == nil {
		defer langRows.Close()
		langDist := make(map[string]int)
		for langRows.Next() {
			var lang string
			var cnt int
			langRows.Scan(&lang, &cnt)
			langDist[lang] = cnt
		}
		result["lang_dist"] = langDist
	}
	// 全局变量语言分布
	varLangRows, err := s.DB.Query(`SELECT language, COUNT(*) FROM global_vars GROUP BY language ORDER BY COUNT(*) DESC`)
	if err == nil {
		defer varLangRows.Close()
		varLangDist := make(map[string]int)
		for varLangRows.Next() {
			var lang string
			var cnt int
			varLangRows.Scan(&lang, &cnt)
			varLangDist[lang] = cnt
		}
		result["var_lang_dist"] = varLangDist
	}
	return result, nil
}

func (s *Store) QueryAllFunctions() ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth,
		        parameters, return_types, receiver, is_method, visibility,
		        cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs
		 FROM functions ORDER BY file_path, line_start`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

func (s *Store) QueryFuncDetail(name string) ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth,
		        parameters, return_types, receiver, is_method, visibility,
		        cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs
		 FROM functions WHERE name = ? ORDER BY file_path, line_start`, name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

func (s *Store) QueryVars() ([]*model.GlobalVariable, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, var_type, language, package_name, visibility, file_path, line_num, is_const
		 FROM global_vars ORDER BY file_path, line_num`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vars []*model.GlobalVariable
	for rows.Next() {
		v := &model.GlobalVariable{}
		var isConst int
		if err := rows.Scan(&v.ID, &v.Name, &v.VarType, &v.Language, &v.PackageName, &v.Visibility, &v.FilePath, &v.LineNum, &isConst); err != nil {
			return nil, err
		}
		v.IsConst = isConst != 0
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

func (s *Store) QueryDead() ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT f.id, f.name, f.package_name, f.language, f.file_path,
		        f.line_start, f.line_end, f.call_count, f.nesting_depth,
		        f.parameters, f.return_types, f.receiver, f.is_method, f.visibility,
		        f.cyclomatic, f.parameter_count, f.return_count, f.statement_count, f.anonymous_funcs
		 FROM functions f
		 WHERE f.name NOT IN (SELECT DISTINCT callee_name FROM function_deps)
		 ORDER BY (f.line_end - f.line_start + 1) DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

func (s *Store) QueryMissing() ([]string, error) {
	rows, err := s.DB.Query(
		`SELECT DISTINCT d.callee_name FROM function_deps d
		 WHERE NOT EXISTS (SELECT 1 FROM functions f WHERE f.name = d.callee_name)
		 ORDER BY d.callee_name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *Store) QueryTop(limit int) ([]*FuncBrief, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth,
		        parameters, return_types, receiver, is_method, visibility,
		        cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs
		 FROM functions ORDER BY (line_end - line_start + 1) DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

func (s *Store) QueryDepStats() ([]*DepStat, error) {
	rows, err := s.DB.Query(
		`SELECT f.name,
		        (SELECT COUNT(DISTINCT d2.caller_id) FROM function_deps d2 WHERE d2.callee_name = f.name) AS caller_cnt,
		        (SELECT COUNT(DISTINCT d3.callee_name) FROM function_deps d3 WHERE d3.caller_id IN (SELECT id FROM functions WHERE name = f.name)) AS callee_cnt,
		        MAX(f.call_count) AS call_count
		 FROM functions f
		 GROUP BY f.name
		 ORDER BY caller_cnt DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []*DepStat
	for rows.Next() {
		ds := &DepStat{}
		if err := rows.Scan(&ds.FuncName, &ds.CallerCount, &ds.CalleeCount, &ds.TotalCallCount); err != nil {
			return nil, err
		}
		stats = append(stats, ds)
	}
	return stats, rows.Err()
}

func (s *Store) QueryFuncBody(funcID int64) (string, error) {
	var body string
	err := s.DB.QueryRow(`SELECT body FROM functions WHERE id = ?`, funcID).Scan(&body)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("function id %d not found", funcID)
	}
	return body, err
}

func (s *Store) QueryDeepNesting(threshold int) ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth,
		        parameters, return_types, receiver, is_method, visibility,
		        cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs
		 FROM functions WHERE nesting_depth >= ?
		 ORDER BY nesting_depth DESC, (line_end - line_start + 1) DESC`, threshold,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

// ═══ 🆕 AST 增强查询 ═══

func (s *Store) QueryByComplexity(limit int) ([]*FuncBrief, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth,
		        parameters, return_types, receiver, is_method, visibility,
		        cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs
		 FROM functions WHERE cyclomatic > 0
		 ORDER BY cyclomatic DESC, (line_end - line_start + 1) DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

func (s *Store) QueryByParams(threshold int) ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth,
		        parameters, return_types, receiver, is_method, visibility,
		        cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs
		 FROM functions WHERE parameter_count >= ?
		 ORDER BY parameter_count DESC, (line_end - line_start + 1) DESC`, threshold,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

func (s *Store) QueryAnonFuncs() ([]*FuncBrief, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, package_name, language, file_path,
		        line_start, line_end, call_count, nesting_depth,
		        parameters, return_types, receiver, is_method, visibility,
		        cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs
		 FROM functions WHERE anonymous_funcs > 0
		 ORDER BY anonymous_funcs DESC, (line_end - line_start + 1) DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFuncBriefs(rows)
}

func (s *Store) QueryFileMetrics() ([]*model.FileMetrics, error) {
	rows, err := s.DB.Query(
		`SELECT file_path, language, total_lines, code_lines, comment_lines, blank_lines,
		        func_count, type_count, avg_cyclomatic, max_cyclomatic,
		        total_parameters, max_parameters, total_returns, total_statements,
		        total_anon_funcs, public_funcs, private_funcs, methods_count
		 FROM file_metrics ORDER BY total_lines DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var metrics []*model.FileMetrics
	for rows.Next() {
		m := &model.FileMetrics{}
		if err := rows.Scan(
			&m.FilePath, &m.Language, &m.TotalLines, &m.CodeLines, &m.CommentLines, &m.BlankLines,
			&m.FuncCount, &m.TypeCount, &m.AvgCyclomatic, &m.MaxCyclomatic,
			&m.TotalParameters, &m.MaxParameters, &m.TotalReturns, &m.TotalStatements,
			&m.TotalAnonFuncs, &m.PublicFuncs, &m.PrivateFuncs, &m.MethodsCount,
		); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

func (s *Store) QueryTypeDefs() ([]*model.TypeDef, error) {
	rows, err := s.DB.Query(
		`SELECT name, kind, language, package_name, file_path, line_start, line_end, body, fields
		 FROM type_defs ORDER BY file_path, line_start`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var defs []*model.TypeDef
	for rows.Next() {
		d := &model.TypeDef{}
		if err := rows.Scan(
			&d.Name, &d.Kind, &d.Language, &d.PackageName, &d.FilePath,
			&d.LineStart, &d.LineEnd, &d.Body, &d.Fields,
		); err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

// scanFuncBriefs 通用 FuncBrief 行扫描（含 AST 字段）
func scanFuncBriefs(rows *sql.Rows) ([]*FuncBrief, error) {
	var funcs []*FuncBrief
	for rows.Next() {
		f := &FuncBrief{}
		var isMethod int
		if err := rows.Scan(
			&f.ID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.CallCount, &f.NestingDepth,
			&f.Parameters, &f.ReturnTypes, &f.Receiver, &isMethod, &f.Visibility,
			&f.Cyclomatic, &f.ParamCount, &f.ReturnCount, &f.StmtCount, &f.AnonFuncs,
		); err != nil {
			return nil, err
		}
		f.IsMethod = isMethod != 0
		f.LineCount = f.LineEnd - f.LineStart + 1
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}
