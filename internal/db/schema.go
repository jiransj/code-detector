package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "modernc.org/sqlite"
)

// Schema DDL 语句
const (
	createScanSessionsTable = `
	CREATE TABLE IF NOT EXISTS scan_sessions (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		project_root TEXT    NOT NULL,
		scan_time    DATETIME DEFAULT CURRENT_TIMESTAMP,
		duration_ms  INTEGER DEFAULT 0,
		file_count   INTEGER DEFAULT 0,
		func_count   INTEGER DEFAULT 0,
		var_count    INTEGER DEFAULT 0
	);`

	createFunctionsTable = `
	CREATE TABLE IF NOT EXISTS functions (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id      INTEGER NOT NULL,
		name            TEXT    NOT NULL,
		language        TEXT    NOT NULL,
		file_path       TEXT    NOT NULL,
		line_start      INTEGER DEFAULT 0,
		line_end        INTEGER DEFAULT 0,
		body            TEXT    DEFAULT '',
		hash            TEXT    DEFAULT '',
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id) ON DELETE CASCADE
	);`

	createFunctionDepsTable = `
	CREATE TABLE IF NOT EXISTS function_deps (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		caller_id   INTEGER NOT NULL,
		callee_name TEXT    NOT NULL,
		UNIQUE(caller_id, callee_name),
		FOREIGN KEY (caller_id) REFERENCES functions(id) ON DELETE CASCADE
	);`

	createGlobalVarsTable = `
	CREATE TABLE IF NOT EXISTS global_vars (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		name       TEXT    NOT NULL,
		var_type   TEXT    DEFAULT '',
		language   TEXT    NOT NULL,
		file_path  TEXT    NOT NULL,
		line_num   INTEGER DEFAULT 0,
		is_const   INTEGER DEFAULT 0,
		hash       TEXT    DEFAULT '',
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id) ON DELETE CASCADE
	);`

	createFileMetricsTable = `
	CREATE TABLE IF NOT EXISTS file_metrics (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id      INTEGER NOT NULL,
		file_path       TEXT    NOT NULL UNIQUE,
		language        TEXT    NOT NULL,
		total_lines     INTEGER DEFAULT 0,
		code_lines      INTEGER DEFAULT 0,
		comment_lines   INTEGER DEFAULT 0,
		blank_lines     INTEGER DEFAULT 0,
		func_count      INTEGER DEFAULT 0,
		type_count      INTEGER DEFAULT 0,
		avg_cyclomatic  REAL    DEFAULT 0.0,
		max_cyclomatic  INTEGER DEFAULT 0,
		total_parameters INTEGER DEFAULT 0,
		max_parameters   INTEGER DEFAULT 0,
		total_returns   INTEGER DEFAULT 0,
		total_statements INTEGER DEFAULT 0,
		total_anon_funcs INTEGER DEFAULT 0,
		public_funcs    INTEGER DEFAULT 0,
		private_funcs   INTEGER DEFAULT 0,
		methods_count   INTEGER DEFAULT 0,
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id) ON DELETE CASCADE
	);`

	createTypeDefsTable = `
	CREATE TABLE IF NOT EXISTS type_defs (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id  INTEGER NOT NULL,
		name        TEXT    NOT NULL,
		kind        TEXT    DEFAULT '',
		language    TEXT    NOT NULL,
		package_name TEXT   DEFAULT '',
		file_path   TEXT    NOT NULL,
		line_start  INTEGER DEFAULT 0,
		line_end    INTEGER DEFAULT 0,
		body        TEXT    DEFAULT '',
		fields      TEXT    DEFAULT '',
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id) ON DELETE CASCADE
	);`

	// 索引
	createIndexFuncName   = `CREATE INDEX IF NOT EXISTS idx_functions_name   ON functions(name);`
	createIndexFuncLang   = `CREATE INDEX IF NOT EXISTS idx_functions_lang   ON functions(language);`
	createIndexFuncFile   = `CREATE INDEX IF NOT EXISTS idx_functions_file   ON functions(file_path);`
	createIndexDepCall    = `CREATE INDEX IF NOT EXISTS idx_deps_caller      ON function_deps(caller_id);`
	createIndexDepCallee  = `CREATE INDEX IF NOT EXISTS idx_deps_callee      ON function_deps(callee_name);`
	createIndexVarName    = `CREATE INDEX IF NOT EXISTS idx_global_vars_name ON global_vars(name);`
	createIndexVarLang    = `CREATE INDEX IF NOT EXISTS idx_global_vars_lang ON global_vars(language);`

	createASTCacheTable = `
	CREATE TABLE IF NOT EXISTS ast_cache (
		file_path    TEXT    PRIMARY KEY,
		content_hash TEXT    NOT NULL,
		funcs_json   TEXT    DEFAULT '[]',
		globals_json TEXT    DEFAULT '[]',
		updated_at   INTEGER NOT NULL
	);`

	createFileCacheTable = `
	CREATE TABLE IF NOT EXISTS file_cache (
		file_path   TEXT    PRIMARY KEY,
		mtime       INTEGER NOT NULL,
		hash        TEXT    DEFAULT '',
		session_id  INTEGER NOT NULL,
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id) ON DELETE CASCADE
	);`
)

// columnExists 检查 SQLite 表中是否存在指定列
func columnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return false, fmt.Errorf("pragma table_info(%s): %w", tableName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan pragma row: %w", err)
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
}

// migrateCascadeFK 将已有表的外键升级为 ON DELETE CASCADE（SQLite 不支持 ALTER TABLE 改外键）
func migrateCascadeFK(db *sql.DB) error {
	// 检查 functions 表的 session_id 外键是否已有 CASCADE
	hasCascade := false
	rows, err := db.Query("PRAGMA foreign_key_list(functions)")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			continue
		}
		if from == "session_id" && onDelete == "CASCADE" {
			hasCascade = true
		}
	}
	rows.Close()
	if hasCascade {
		return nil // 已迁移
	}

	// 需要迁移的表对：(旧表, 新表DDL)
	type tableDef struct {
		name string
		ddl  string
	}
	tables := []tableDef{
		{"function_deps", createFunctionDepsTable},
		{"functions", createFunctionsTable},
		{"file_metrics", createFileMetricsTable},
		{"type_defs", createTypeDefsTable},
		{"global_vars", createGlobalVarsTable},
		{"file_cache", createFileCacheTable},
	}

	// 禁用外键，避免重建时的引用检查
	db.Exec("PRAGMA foreign_keys=OFF")

	for _, t := range tables {
		tmpName := t.name + "_cascade"
		// 创建新表（DDL 中定义了 CASCADE）
		ddl := "CREATE TABLE IF NOT EXISTS " + tmpName + " " + t.ddl[strings.Index(t.ddl, "("):]
		if _, err := db.Exec(ddl); err != nil {
			db.Exec("PRAGMA foreign_keys=ON")
			return fmt.Errorf("create %s: %w", tmpName, err)
		}
		// 复制数据
		if _, err := db.Exec("INSERT INTO " + tmpName + " SELECT * FROM " + t.name); err != nil {
			db.Exec("PRAGMA foreign_keys=ON")
			return fmt.Errorf("copy %s: %w", t.name, err)
		}
		// 删除原表
		if _, err := db.Exec("DROP TABLE " + t.name); err != nil {
			db.Exec("PRAGMA foreign_keys=ON")
			return fmt.Errorf("drop %s: %w", t.name, err)
		}
		// 重命名新表
		if _, err := db.Exec("ALTER TABLE " + tmpName + " RENAME TO " + t.name); err != nil {
			db.Exec("PRAGMA foreign_keys=ON")
			return fmt.Errorf("rename %s: %w", t.name, err)
		}
	}

	db.Exec("PRAGMA foreign_keys=ON")
	return nil
}

// InitDB 初始化数据库，创建所有表与索引，执行迁移
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("warn: failed to set WAL mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		log.Printf("warn: failed to enable foreign keys: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		log.Printf("warn: failed to set busy timeout: %v", err)
	}

	// 创建基础表
	tables := []string{
		createScanSessionsTable,
		createFunctionsTable,
		createFunctionDepsTable,
		createGlobalVarsTable,
		createFileMetricsTable,
		createTypeDefsTable,
		createFileCacheTable,
		createASTCacheTable,
	}
	for _, stmt := range tables {
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("exec schema: %w\nstmt: %s", err, stmt)
		}
	}

	// 创建索引
	indexes := []string{
		createIndexFuncName, createIndexFuncLang, createIndexFuncFile,
		createIndexDepCall, createIndexDepCallee,
		createIndexVarName, createIndexVarLang,
	}
	for _, stmt := range indexes {
		if _, err := db.Exec(stmt); err != nil {
			log.Printf("warn: create index: %v\nstmt: %s", err, stmt)
		}
	}

	// 数据库迁移：为新列补充
	migrations := []struct {
		table  string
		column string
		stmt   string
	}{
		// 已有迁移
		{"functions", "package_name", `ALTER TABLE functions ADD COLUMN package_name TEXT DEFAULT ''`},
		{"global_vars", "package_name", `ALTER TABLE global_vars ADD COLUMN package_name TEXT DEFAULT ''`},
		{"global_vars", "visibility",  `ALTER TABLE global_vars ADD COLUMN visibility TEXT DEFAULT ''`},
		{"scan_sessions", "var_count", `ALTER TABLE scan_sessions ADD COLUMN var_count INTEGER DEFAULT 0`},
		{"functions", "call_count",    `ALTER TABLE functions ADD COLUMN call_count INTEGER DEFAULT 0`},
		{"functions", "nesting_depth", `ALTER TABLE functions ADD COLUMN nesting_depth INTEGER DEFAULT 0`},
		// ═══ AST 增强迁移 ═══
		{"functions", "parameters",     `ALTER TABLE functions ADD COLUMN parameters TEXT DEFAULT ''`},
		{"functions", "return_types",   `ALTER TABLE functions ADD COLUMN return_types TEXT DEFAULT ''`},
		{"functions", "receiver",       `ALTER TABLE functions ADD COLUMN receiver TEXT DEFAULT ''`},
		{"functions", "is_method",      `ALTER TABLE functions ADD COLUMN is_method INTEGER DEFAULT 0`},
		{"functions", "visibility",     `ALTER TABLE functions ADD COLUMN visibility TEXT DEFAULT ''`},
		{"functions", "cyclomatic",     `ALTER TABLE functions ADD COLUMN cyclomatic INTEGER DEFAULT 0`},
		{"functions", "parameter_count", `ALTER TABLE functions ADD COLUMN parameter_count INTEGER DEFAULT 0`},
		{"functions", "return_count",   `ALTER TABLE functions ADD COLUMN return_count INTEGER DEFAULT 0`},
		{"functions", "statement_count",`ALTER TABLE functions ADD COLUMN statement_count INTEGER DEFAULT 0`},
		{"functions", "anonymous_funcs",`ALTER TABLE functions ADD COLUMN anonymous_funcs INTEGER DEFAULT 0`},
	}
	for _, m := range migrations {
		exists, err := columnExists(db, m.table, m.column)
		if err != nil {
			log.Printf("warn: check column %s.%s: %v", m.table, m.column, err)
			continue
		}
		if exists {
			continue
		}
		if _, err := db.Exec(m.stmt); err != nil {
			log.Printf("warn: migration %s.%s: %v", m.table, m.column, err)
		}
	}

	// 外键 CASCADE 迁移：确保旧库的级联删除支持
	if err := migrateCascadeFK(db); err != nil {
		log.Printf("warn: cascade FK migration: %v", err)
	}

	return db, nil
}
