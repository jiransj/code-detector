package db

import (
	"context"
	"hash/fnv"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	"code-detector/internal/model"
)

// Store 封装所有数据库 CRUD 操作
//
// ⚠️ 锁获取定序（必须遵守，否则可能死锁）：
//   sessionMu → funcMu → depsMu → globalVarMu
// 任何需要持有多个锁的代码必须以该顺序获取
//
type Store struct {
	DB          *sql.DB
	sessionMu   sync.Mutex // [1] 保护会话创建/更新/清理
	funcMu      sync.Mutex // [2] 保护函数批量写入
	depsMu      sync.Mutex // [3] 保护依赖关系批量写入
	globalVarMu sync.Mutex // [4] 保护全局变量批量写入

	// 缓存预处理语句，避免每次批次重新 prepare
	funcInsertStmt      *sql.Stmt
	depsInsertStmt      *sql.Stmt
	globalVarInsertStmt *sql.Stmt
}

// NewStore 创建 Store 实例
func NewStore(db *sql.DB) *Store {
	s := &Store{DB: db}
	// 预处理 INSERT 语句，避免每次批次重新编译
	s.funcInsertStmt, _ = db.Prepare(
		`INSERT INTO functions (session_id, name, package_name, language, file_path, line_start, line_end, body, hash, call_count, nesting_depth,
		                        parameters, return_types, receiver, is_method, visibility, cyclomatic, parameter_count, return_count, statement_count, anonymous_funcs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		         ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	s.depsInsertStmt, _ = db.Prepare(`INSERT OR IGNORE INTO function_deps (caller_id, callee_name) VALUES (?, ?)`)
	s.globalVarInsertStmt, _ = db.Prepare(
		`INSERT INTO global_vars (session_id, name, var_type, language, package_name, visibility, file_path, line_num, is_const, hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	return s
}

// CreateSession 创建一次扫描会话，返回 session_id
// 创建后会清理超过 3 个的旧历史会话，防止 DB 膨胀
func (s *Store) CreateSession(root string) (int64, error) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	now := time.Now()
	res, err := s.DB.Exec(
		`INSERT INTO scan_sessions (project_root, scan_time) VALUES (?, ?)`,
		root, now,
	)
	if err != nil {
		return 0, fmt.Errorf("create session: %w", err)
	}

	newID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}

	// 保留最多 3 个历史 session，删除更旧的
	if err := s.pruneOldSessionsLocked(3); err != nil {
		return 0, fmt.Errorf("prune old sessions: %w", err)
	}

	return newID, nil
}

// pruneOldSessionsLocked 清理超出 keepCount 的旧会话（调用方需持有 mu 锁）
// 按 session_id 保留最新的 keepCount 条，删除其余及其关联数据
func (s *Store) pruneOldSessionsLocked(keepCount int) error {
	// 查出需要删除的旧 session ID（跳过最新的 keepCount 个）
	rows, err := s.DB.Query(
		`SELECT id FROM scan_sessions ORDER BY id DESC LIMIT -1 OFFSET ?`, keepCount,
	)
	if err != nil {
		return fmt.Errorf("query old sessions: %w", err)
	}
	defer rows.Close()

	var oldIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan old session id: %w", err)
		}
		oldIDs = append(oldIDs, id)
	}
	rows.Close()

	if len(oldIDs) == 0 {
		return nil
	}

	// 构建 IN 子句
	for _, sid := range oldIDs {
		// 按外键依赖顺序删除：function_deps → functions → global_vars → file_cache → scan_sessions
		if _, err := s.DB.Exec(
			`DELETE FROM function_deps WHERE caller_id IN (SELECT id FROM functions WHERE session_id = ?)`, sid,
		); err != nil {
			return fmt.Errorf("delete deps for session %d: %w", sid, err)
		}
		if _, err := s.DB.Exec(`DELETE FROM functions WHERE session_id = ?`, sid); err != nil {
			return fmt.Errorf("delete functions for session %d: %w", sid, err)
		}
		if _, err := s.DB.Exec(`DELETE FROM global_vars WHERE session_id = ?`, sid); err != nil {
			return fmt.Errorf("delete vars for session %d: %w", sid, err)
		}
		if _, err := s.DB.Exec(`DELETE FROM file_cache WHERE session_id = ?`, sid); err != nil {
			return fmt.Errorf("delete cache for session %d: %w", sid, err)
		}
		if _, err := s.DB.Exec(`DELETE FROM scan_sessions WHERE id = ?`, sid); err != nil {
			return fmt.Errorf("delete session %d: %w", sid, err)
		}
	}

	return nil
}

// UpdateSession 扫描完成后更新会话统计
func (s *Store) UpdateSession(sessionID int64, duration time.Duration, fileCount, funcCount, varCount int) error {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	_, err := s.DB.Exec(
		`UPDATE scan_sessions SET duration_ms=?, file_count=?, func_count=?, var_count=? WHERE id=?`,
		duration.Milliseconds(), fileCount, funcCount, varCount, sessionID,
	)
	return err
}

// InsertFunction 插入一条函数记录，返回函数 ID
// FuncHash 计算函数的唯一哈希值
func FuncHash(f *model.Function) string {
	sort.Strings(f.Dependencies)
	h := fnv.New128a()
	h.Write([]byte(f.Name))
	h.Write([]byte(f.PackageName))
	h.Write([]byte(f.Language))
	h.Write([]byte(f.FilePath))
	h.Write([]byte(fmt.Sprintf("%d", f.LineStart)))
	h.Write([]byte(f.Body))
	h.Write([]byte(fmt.Sprintf("%v", f.Dependencies)))
	h.Write([]byte(fmt.Sprintf("%d", f.CallCount)))
	h.Write([]byte(fmt.Sprintf("%d", f.NestingDepth)))
	h.Write([]byte(f.Parameters))
	h.Write([]byte(f.ReturnTypes))
	h.Write([]byte(f.Receiver))
	h.Write([]byte(fmt.Sprintf("%v", f.IsMethod)))
	h.Write([]byte(f.Visibility))
	h.Write([]byte(fmt.Sprintf("%d", f.Cyclomatic)))
	h.Write([]byte(fmt.Sprintf("%d", f.ParameterCount)))
	h.Write([]byte(fmt.Sprintf("%d", f.ReturnCount)))
	h.Write([]byte(fmt.Sprintf("%d", f.StatementCount)))
	h.Write([]byte(fmt.Sprintf("%d", f.AnonymousFuncs)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// VarHash 计算全局变量的唯一哈希值
func VarHash(v *model.GlobalVariable) string {
	h := fnv.New128a()
	h.Write([]byte(v.Name))
	h.Write([]byte(v.VarType))
	h.Write([]byte(v.Language))
	h.Write([]byte(v.PackageName))
	h.Write([]byte(v.Visibility))
	h.Write([]byte(v.FilePath))
	h.Write([]byte(fmt.Sprintf("%d", v.LineNum)))
	h.Write([]byte(fmt.Sprintf("%v", v.IsConst)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// buildInClause 安全构建 IN 查询的前半部分（不含 WHERE 前缀）和参数列表
// 返回: "column IN (?, ?, ...)" 和对应的参数切片
func buildInClause(column string, values []string) (string, []interface{}) {
	if len(values) == 0 {
		return column + " IN (NULL)", nil
	}
	args := make([]interface{}, len(values))
	clause := column + " IN (?"
	for i := 1; i < len(values); i++ {
		clause += ", ?"
	}
	clause += ")"
	for i, v := range values {
		args[i] = v
	}
	return clause, args
}

// CheckExistingFuncHashes 查询已有函数的 hash，返回 map[hash]id
// 限定当前 session_id，防止跨 session 误删历史数据
func (s *Store) CheckExistingFuncHashes(sessionID int64, funcs []*model.Function) (map[string]int64, map[string]int64, error) {
	existing := make(map[string]int64) // "file:name:line" → id
	hashMap := make(map[string]int64)  // hash → id

	if len(funcs) == 0 {
		return existing, hashMap, nil
	}

	// 收集唯一的文件路径，只查询相关文件（避免全表扫描）
	seenFiles := make(map[string]bool)
	filePaths := make([]string, 0, len(funcs))
	for _, f := range funcs {
		if !seenFiles[f.FilePath] {
			seenFiles[f.FilePath] = true
			filePaths = append(filePaths, f.FilePath)
		}
	}

	// 使用安全辅助函数构建 IN 查询
	inClause, args := buildInClause("file_path", filePaths)
	query := `SELECT id, file_path, name, line_start, hash FROM functions WHERE session_id = ? AND ` + inClause
	queryArgs := append([]interface{}{sessionID}, args...)

	rows, err := s.DB.Query(query, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var fp, name, hash string
		var ls int
		rows.Scan(&id, &fp, &name, &ls, &hash)
		key := fmt.Sprintf("%s:%s:%d", fp, name, ls)
		existing[key] = id
		if hash != "" {
			hashMap[hash] = id
		}
	}
	return existing, hashMap, rows.Err()
}

// BatchInsertFunctions 批量插入函数（事务内），带哈希去重
// 返回 (新插入的ID列表, 跳过的数量, 变更的数量)
// 当 skipHashCheck 为 true（全量扫描）时跳过哈希查询与比对，直接全量写入
func (s *Store) BatchInsertFunctions(functions []*model.Function, sessionID int64, skipHashCheck bool) ([]int64, int, int, error) {
	s.funcMu.Lock()
	defer s.funcMu.Unlock()

	if skipHashCheck {
		// 全量扫描：直接批量插入，跳过哈希比对
		tx, err := s.DB.Begin()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback()

		stmt := tx.Stmt(s.funcInsertStmt)
		ids := make([]int64, 0, len(functions))

		for _, f := range functions {
			f.Hash = FuncHash(f)
			res, err := stmt.Exec(sessionID, f.Name, f.PackageName, f.Language, f.FilePath, f.LineStart, f.LineEnd, f.Body, f.Hash,
				f.CallCount, f.NestingDepth,
				f.Parameters, f.ReturnTypes, f.Receiver, boolToInt(f.IsMethod), f.Visibility,
				f.Cyclomatic, f.ParameterCount, f.ReturnCount, f.StatementCount, f.AnonymousFuncs)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("insert function %s: %w", f.Name, err)
			}
			newID, _ := res.LastInsertId()
			ids = append(ids, newID)
		}

		if err := tx.Commit(); err != nil {
			return nil, 0, 0, fmt.Errorf("commit tx: %w", err)
		}
		return ids, 0, 0, nil
	}

	// 增量扫描：先查询已有的函数进行哈希去重
	existing, hashMap, err := s.CheckExistingFuncHashes(sessionID, functions)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("query existing: %w", err)
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt := tx.Stmt(s.funcInsertStmt)
	ids := make([]int64, 0, len(functions))
	skipped := 0
	changed := 0

	// 构建反向映射 id→hash，避免 O(n²) 查找
	idToHash := make(map[int64]string, len(hashMap))
	for h, id := range hashMap {
		idToHash[id] = h
	}

	for _, f := range functions {
		hash := FuncHash(f)
		f.Hash = hash

		// 检查是否已存在（相同 session + file + name + line）
		key := fmt.Sprintf("%s:%s:%d", f.FilePath, f.Name, f.LineStart)
		if oldID, exists := existing[key]; exists {
			// 存在：用反向映射 O(1) 比较 hash
			if oldHash, ok := idToHash[oldID]; ok && oldHash == hash {
				// 完全一致，跳过
				ids = append(ids, oldID)
				skipped++
				continue
			}
			// hash 不同：内容已变更，删除旧记录，插入新记录
			if _, err := tx.Exec(`DELETE FROM function_deps WHERE caller_id = ?`, oldID); err != nil {
				return nil, 0, 0, fmt.Errorf("delete old deps: %w", err)
			}
			if _, err := tx.Exec(`DELETE FROM functions WHERE id = ?`, oldID); err != nil {
				return nil, 0, 0, fmt.Errorf("delete old func: %w", err)
			}
			changed++
		}

		// 插入新记录
		res, err := stmt.Exec(sessionID, f.Name, f.PackageName, f.Language, f.FilePath, f.LineStart, f.LineEnd, f.Body, hash,
			f.CallCount, f.NestingDepth,
			f.Parameters, f.ReturnTypes, f.Receiver, boolToInt(f.IsMethod), f.Visibility,
			f.Cyclomatic, f.ParameterCount, f.ReturnCount, f.StatementCount, f.AnonymousFuncs)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("insert function %s: %w", f.Name, err)
		}
		newID, _ := res.LastInsertId()
		ids = append(ids, newID)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, 0, fmt.Errorf("commit tx: %w", err)
	}
	return ids, skipped, changed, nil
}

// BatchInsertDeps 批量插入依赖关系（事务内）
func (s *Store) BatchInsertDeps(deps map[int64][]string) error {
	s.depsMu.Lock()
	defer s.depsMu.Unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt := tx.Stmt(s.depsInsertStmt)
	for callerID, callees := range deps {
		for _, callee := range callees {
			if _, err := stmt.Exec(callerID, callee); err != nil {
				return fmt.Errorf("insert dep %d->%s: %w", callerID, callee, err)
			}
		}
	}

	return tx.Commit()
}

// QueryFunctionsByLanguage 按语言查询函数
func (s *Store) QueryFunctionsByLanguage(lang string) ([]*model.Function, error) {
	rows, err := s.DB.Query(
		`SELECT id, session_id, name, package_name, language, file_path, line_start, line_end, body, call_count, nesting_depth
		 FROM functions WHERE language = ? ORDER BY file_path, line_start`, lang,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*model.Function
	for rows.Next() {
		f := &model.Function{}
		if err := rows.Scan(&f.ID, &f.SessionID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.Body, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}

// QueryDependenciesBySession 通过 JOIN 一次性查询一个 session 的所有函数依赖关系
// 返回 map[caller_id][]callee_name — 消除 N+1 查询
func (s *Store) QueryDependenciesBySession(sessionID int64) (map[int64][]string, error) {
	rows, err := s.DB.Query(
		`SELECT d.caller_id, d.callee_name
		 FROM function_deps d
		 JOIN functions f ON f.id = d.caller_id
		 WHERE f.session_id = ?`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query deps by session: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]string)
	for rows.Next() {
		var callerID int64
		var callee string
		if err := rows.Scan(&callerID, &callee); err != nil {
			return nil, fmt.Errorf("scan dep row: %w", err)
		}
		result[callerID] = append(result[callerID], callee)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// QueryDependencies 查询指定函数的直接依赖
func (s *Store) QueryDependencies(funcID int64) ([]string, error) {
	rows, err := s.DB.Query(
		`SELECT callee_name FROM function_deps WHERE caller_id = ? ORDER BY callee_name`, funcID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		deps = append(deps, name)
	}
	return deps, rows.Err()
}

// QueryCallers 查询调用指定函数的所有调用方
func (s *Store) QueryCallers(funcName string) ([]*model.Function, error) {
	rows, err := s.DB.Query(
		`SELECT f.id, f.session_id, f.name, f.package_name, f.language, f.file_path, f.line_start, f.line_end, f.body, f.call_count, f.nesting_depth
		 FROM functions f
		 INNER JOIN function_deps d ON f.id = d.caller_id
		 WHERE d.callee_name = ?
		 ORDER BY f.file_path, f.line_start`, funcName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var funcs []*model.Function
	for rows.Next() {
		f := &model.Function{}
		if err := rows.Scan(&f.ID, &f.SessionID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.Body, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}
	return funcs, rows.Err()
}

// BatchInsertGlobalVars 批量插入全局变量（事务内）
func (s *Store) BatchInsertGlobalVars(vars []*model.GlobalVariable, sessionID int64) (int, int, error) {
	s.globalVarMu.Lock()
	defer s.globalVarMu.Unlock()

	// 查询已有的全局变量
	existing := make(map[string]int64) // "file:name:line" → id
	hashMap := make(map[string]int64)  // hash → id

	rows, err := s.DB.Query(
		`SELECT id, file_path, name, line_num, hash FROM global_vars`,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("query existing vars: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var fp, name, hash string
		var ln int
		rows.Scan(&id, &fp, &name, &ln, &hash)
		key := fmt.Sprintf("%s:%s:%d", fp, name, ln)
		existing[key] = id
		if hash != "" {
			hashMap[hash] = id
		}
	}
	rows.Close()

	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt := tx.Stmt(s.globalVarInsertStmt)
	skipped := 0
	changed := 0

	for _, v := range vars {
		isConst := 0
		if v.IsConst {
			isConst = 1
		}
		hash := VarHash(v)
		key := fmt.Sprintf("%s:%s:%d", v.FilePath, v.Name, v.LineNum)

		if oldID, exists := existing[key]; exists {
			found := false
			for h, id := range hashMap {
				if id == oldID && h == hash {
					found = true
					break
				}
			}
			if found {
				skipped++
				continue
			}
			if _, err := tx.Exec(`DELETE FROM global_vars WHERE id = ?`, oldID); err != nil {
				return 0, 0, fmt.Errorf("delete old var: %w", err)
			}
			changed++
		}

		if _, err := stmt.Exec(sessionID, v.Name, v.VarType, v.Language, v.PackageName, v.Visibility, v.FilePath, v.LineNum, isConst, hash); err != nil {
			return 0, 0, fmt.Errorf("insert global var %s: %w", v.Name, err)
		}
	}

	return skipped, changed, tx.Commit()
}

// QueryGlobalVarsByLanguage 按语言查询全局变量
func (s *Store) QueryGlobalVarsByLanguage(lang string) ([]*model.GlobalVariable, error) {
	rows, err := s.DB.Query(
		`SELECT id, session_id, name, var_type, language, package_name, visibility, file_path, line_num, is_const
		 FROM global_vars WHERE language = ? ORDER BY file_path, line_num`, lang,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vars []*model.GlobalVariable
	for rows.Next() {
		v := &model.GlobalVariable{}
		var isConst int
		if err := rows.Scan(&v.ID, &v.SessionID, &v.Name, &v.VarType, &v.Language,
			&v.PackageName, &v.Visibility, &v.FilePath, &v.LineNum, &isConst); err != nil {
			return nil, err
		}
		v.IsConst = isConst != 0
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

// ---- 文件缓存（增量扫描） ----

// GetFileCache 查询文件的缓存信息（mtime + hash）
// 返回 (mtime_unix, hash, found, error)
func (s *Store) GetFileCache(filePath string) (int64, string, bool, error) {
	row := s.DB.QueryRow(
		`SELECT mtime, hash FROM file_cache WHERE file_path = ?`, filePath,
	)
	var mtime int64
	var hash string
	err := row.Scan(&mtime, &hash)
	if err == sql.ErrNoRows {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("query file cache: %w", err)
	}
	return mtime, hash, true, nil
}

// UpsertFileCache 插入或更新文件缓存
func (s *Store) UpsertFileCache(filePath string, mtime int64, hash string, sessionID int64) error {
	_, err := s.DB.Exec(
		`INSERT INTO file_cache (file_path, mtime, hash, session_id)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(file_path) DO UPDATE SET mtime=excluded.mtime, hash=excluded.hash, session_id=excluded.session_id`,
		filePath, mtime, hash, sessionID,
	)
	if err != nil {
		return fmt.Errorf("upsert file cache: %w", err)
	}
	return nil
}

// Checkpoint 将 WAL 内容回写到主数据库文件
func (s *Store) Checkpoint() {
	conn, err := s.DB.Conn(context.Background())
	if err != nil {
		return
	}
	defer conn.Close()
	conn.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")
}

// ComputeFileMetrics 从 functions 表聚合计算文件级指标并写入 file_metrics
func (s *Store) ComputeFileMetrics(sessionID int64) error {
	rows, err := s.DB.Query(
		`SELECT file_path, language,
		        COUNT(*) AS func_count,
		        SUM(cyclomatic) AS total_cyclomatic,
		        MAX(cyclomatic) AS max_cyclomatic,
		        AVG(CAST(cyclomatic AS REAL)) AS avg_cyclomatic,
		        SUM(parameter_count) AS total_params,
		        MAX(parameter_count) AS max_params,
		        SUM(return_count) AS total_returns,
		        SUM(statement_count) AS total_stmts,
		        SUM(anonymous_funcs) AS total_anon,
		        SUM(CASE WHEN visibility = 'public' THEN 1 ELSE 0 END) AS public_cnt,
		        SUM(CASE WHEN visibility = 'private' THEN 1 ELSE 0 END) AS private_cnt,
		        SUM(CASE WHEN is_method = 1 THEN 1 ELSE 0 END) AS methods_cnt,
		        MAX(line_end) AS max_line
		 FROM functions WHERE session_id = ?
		 GROUP BY file_path, language`, sessionID,
	)
	if err != nil {
		return fmt.Errorf("query file metrics: %w", err)
	}
	defer rows.Close()

	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 清除旧数据
	if _, err := tx.Exec(`DELETE FROM file_metrics WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete old metrics: %w", err)
	}

	stmt, err := tx.Prepare(
		`INSERT INTO file_metrics (session_id, file_path, language, func_count, total_lines,
		 avg_cyclomatic, max_cyclomatic, total_parameters, max_parameters,
		 total_returns, total_statements, total_anon_funcs,
		 public_funcs, private_funcs, methods_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for rows.Next() {
		var fp, lang string
		var funcCnt, totalCyc, maxCyc, totalParams, maxParams, totalRets, totalStmts, totalAnon, pubCnt, privCnt, methodsCnt, maxLine int
		var avgCyc float64
		if err := rows.Scan(&fp, &lang, &funcCnt, &totalCyc, &maxCyc, &avgCyc, &totalParams, &maxParams, &totalRets, &totalStmts, &totalAnon, &pubCnt, &privCnt, &methodsCnt, &maxLine); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		if _, err := stmt.Exec(sessionID, fp, lang, funcCnt, maxLine,
			avgCyc, maxCyc, totalParams, maxParams,
			totalRets, totalStmts, totalAnon,
			pubCnt, privCnt, methodsCnt,
		); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}
	return tx.Commit()
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	if s.funcInsertStmt != nil {
		s.funcInsertStmt.Close()
	}
	if s.depsInsertStmt != nil {
		s.depsInsertStmt.Close()
	}
	if s.globalVarInsertStmt != nil {
		s.globalVarInsertStmt.Close()
	}
	s.Checkpoint()
	return s.DB.Close()
}

// boolToInt 布尔值转 0/1
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
