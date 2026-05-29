package db

import (
	"database/sql"
	"fmt"
	"log"

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
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		name       TEXT    NOT NULL,
		language   TEXT    NOT NULL,
		file_path  TEXT    NOT NULL,
		line_start INTEGER DEFAULT 0,
		line_end   INTEGER DEFAULT 0,
		body       TEXT    DEFAULT '',
		hash       TEXT    DEFAULT '',
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id)
	);`

	createFunctionDepsTable = `
	CREATE TABLE IF NOT EXISTS function_deps (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		caller_id   INTEGER NOT NULL,
		callee_name TEXT    NOT NULL,
		UNIQUE(caller_id, callee_name),
		FOREIGN KEY (caller_id) REFERENCES functions(id)
	);`

	createIndexFuncName = `CREATE INDEX IF NOT EXISTS idx_functions_name  ON functions(name);`
	createIndexFuncLang = `CREATE INDEX IF NOT EXISTS idx_functions_lang  ON functions(language);`
	createIndexFuncFile = `CREATE INDEX IF NOT EXISTS idx_functions_file ON functions(file_path);`
	createIndexDepCall  = `CREATE INDEX IF NOT EXISTS idx_deps_caller   ON function_deps(caller_id);`
	createIndexDepCallee = `CREATE INDEX IF NOT EXISTS idx_deps_callee  ON function_deps(callee_name);`

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
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id)
	);`

	createIndexVarName = `CREATE INDEX IF NOT EXISTS idx_global_vars_name  ON global_vars(name);`
	createIndexVarLang = `CREATE INDEX IF NOT EXISTS idx_global_vars_lang  ON global_vars(language);`

	createFileCacheTable = `
	CREATE TABLE IF NOT EXISTS file_cache (
		file_path   TEXT    PRIMARY KEY,
		mtime       INTEGER NOT NULL,
		hash        TEXT    DEFAULT '',
		session_id  INTEGER NOT NULL,
		FOREIGN KEY (session_id) REFERENCES scan_sessions(id)
	);`
)

// InitDB 初始化数据库，创建所有表与索引
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// 启用 WAL 模式 — 写入性能好，支持并发读
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("warn: failed to set WAL mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		log.Printf("warn: failed to enable foreign keys: %v", err)
	}

	ddl := []string{
		createScanSessionsTable,
		createFunctionsTable,
		createFunctionDepsTable,
		createGlobalVarsTable,
		createFileCacheTable,
		createIndexFuncName,
		createIndexFuncLang,
		createIndexFuncFile,
		createIndexDepCall,
		createIndexDepCallee,
		createIndexVarName,
		createIndexVarLang,
	}
	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("exec schema: %w\nstmt: %s", err, stmt)
		}
	}

	// 数据库迁移：为旧数据库补充新列
	migrations := []string{
		`ALTER TABLE functions ADD COLUMN package_name TEXT DEFAULT ''`,
		`ALTER TABLE global_vars ADD COLUMN package_name TEXT DEFAULT ''`,
		`ALTER TABLE global_vars ADD COLUMN visibility  TEXT DEFAULT ''`,
		`ALTER TABLE scan_sessions ADD COLUMN var_count INTEGER DEFAULT 0`,
		`ALTER TABLE functions ADD COLUMN call_count INTEGER DEFAULT 0`,
		`ALTER TABLE functions ADD COLUMN nesting_depth INTEGER DEFAULT 0`,
	}
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			log.Printf("migration (ignored if column exists): %v", err)
		}
	}

	return db, nil
}
