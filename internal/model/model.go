package model

import "time"

// LanguageConfig 扫描语言配置（从 YAML 加载）
type LanguageConfig struct {
	Name           string         `yaml:"name"`
	Extensions     []string       `yaml:"extensions"`
	FunctionRegex  string         `yaml:"function_regex"`
	SingleComment  []string       `yaml:"single_comment"`
	BodyStrategy   string         `yaml:"body_strategy"`
	BlockComment   [][2]string    `yaml:"block_comment"`
}

// Function 单个函数的完整信息（含 AST 提取的丰富字段）
type Function struct {
	ID              int64    `json:"id"`
	SessionID       int64    `json:"session_id,omitempty"`
	Name            string   `json:"name"`
	PackageName     string   `json:"package_name,omitempty"`
	Language        string   `json:"language"`
	FilePath        string   `json:"file_path"`
	LineStart       int      `json:"line_start"`
	LineEnd         int      `json:"line_end"`
	Body            string   `json:"body"`
	Dependencies    []string `json:"dependencies,omitempty"`
	Hash            string   `json:"hash,omitempty"`

	// 调用统计
	CallCount       int `json:"call_count"`
	NestingDepth    int `json:"nesting_depth"`

	// ═══ AST 增强字段 ═══
	Parameters      string `json:"parameters,omitempty"`       // 参数列表 "(a int, b string)"
	ReturnTypes     string `json:"return_types,omitempty"`     // 返回类型 "(int, error)"
	Receiver        string `json:"receiver,omitempty"`         // 接收器 "(s *Server)"
	IsMethod        bool   `json:"is_method"`                  // 是否为方法
	Visibility      string `json:"visibility,omitempty"`       // "public" / "private"

	// 复杂度指标
	Cyclomatic      int `json:"cyclomatic"`                    // 圈复杂度
	ParameterCount  int `json:"parameter_count"`               // 参数个数
	ReturnCount     int `json:"return_count"`                  // return 语句数
	StatementCount  int `json:"statement_count"`               // body 中语句数
	AnonymousFuncs  int `json:"anonymous_funcs"`               // 内部匿名函数数
}

// CallStats 函数调用统计数据
type CallStats struct {
	Callees       []string
	CallCount     int
	NestingDepth  int
}

// GlobalVariable 跨文件可使用的全局变量
type GlobalVariable struct {
	ID          int64  `json:"id"`
	SessionID   int64  `json:"session_id,omitempty"`
	Name        string `json:"name"`
	VarType     string `json:"var_type"`
	Language    string `json:"language"`
	PackageName string `json:"package_name,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	FilePath    string `json:"file_path"`
	LineNum     int    `json:"line_num"`
	IsConst     bool   `json:"is_const"`
}

// FileMetrics 单个文件的 AST 维度统计
type FileMetrics struct {
	ID              int64  `json:"id"`
	SessionID       int64  `json:"session_id,omitempty"`
	FilePath        string `json:"file_path"`
	Language        string `json:"language"`

	// 基础
	TotalLines      int `json:"total_lines"`      // 文件总行数
	CodeLines       int `json:"code_lines"`       // 代码行数（非空非注释）
	CommentLines    int `json:"comment_lines"`    // 注释行数
	BlankLines      int `json:"blank_lines"`      // 空行数
	FuncCount       int `json:"func_count"`       // 函数数
	TypeCount       int `json:"type_count"`       // 类型定义数

	// AST 维度
	AvgCyclomatic   float64 `json:"avg_cyclomatic"`    // 平均圈复杂度
	MaxCyclomatic   int     `json:"max_cyclomatic"`    // 最高圈复杂度
	TotalParameters int     `json:"total_parameters"`  // 参数总数
	MaxParameters   int     `json:"max_parameters"`    // 单函数最多参数
	TotalReturns    int     `json:"total_returns"`     // return 总数
	TotalStatements int     `json:"total_statements"`  // 语句总数
	TotalAnonFuncs  int     `json:"total_anon_funcs"`  // 匿名函数总数
	PublicFuncs     int     `json:"public_funcs"`      // 公开函数数
	PrivateFuncs    int     `json:"private_funcs"`     // 私有函数数
	MethodsCount    int     `json:"methods_count"`     // 方法数
}

// TypeDef 类型定义信息
type TypeDef struct {
	ID          int64  `json:"id"`
	SessionID   int64  `json:"session_id,omitempty"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`         // "struct" / "interface" / "alias" / "enum"
	Language    string `json:"language"`
	PackageName string `json:"package_name,omitempty"`
	FilePath    string `json:"file_path"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
	Body        string `json:"body,omitempty"`        // 完整源码
	Fields      string `json:"fields,omitempty"`      // 结构化字段描述 (JSON)
}

// ScanSession 一次扫描会话记录
type ScanSession struct {
	ID          int64     `json:"id"`
	ProjectRoot string    `json:"project_root"`
	ScanTime    time.Time `json:"scan_time"`
	Duration    int64     `json:"duration_ms"`
	FileCount   int       `json:"file_count"`
	FuncCount   int       `json:"func_count"`
	VarCount    int       `json:"var_count"`
}

// ScanResult 一次扫描的完整结果
type ScanResult struct {
	Session      ScanSession
	Functions    []*Function
	GlobalVars   []*GlobalVariable
	FileMetrics  []*FileMetrics
	TypeDefs     []*TypeDef
	Duration     time.Duration
	FileCount    int
	SkipCount    int
	Errors       []string
}


