package model

import "time"

// LanguageConfig 扫描语言配置（从 YAML 加载）
type LanguageConfig struct {
	Name           string         `yaml:"name"`            // 语言名称
	Extensions     []string       `yaml:"extensions"`      // 文件扩展名，如 [".go", ".py"]
	FunctionRegex  string         `yaml:"function_regex"`  // 函数定义行正则（带 (?P<name>...) 命名组）
	SingleComment  []string       `yaml:"single_comment"`  // 单行注释标记，如 ["//", "#"]
	BodyStrategy   string         `yaml:"body_strategy"`   // 函数体边界策略: "brace" / "indent" / "end"
	BlockComment   [][2]string    `yaml:"block_comment"`   // 块注释标记对，如 [["/*","*/"]]
}

// Function 单个函数的完整信息
type Function struct {
	ID           int64    `json:"id"`
	SessionID    int64    `json:"session_id,omitempty"`
	Name         string   `json:"name"`
	PackageName  string   `json:"package_name,omitempty"`  // 所属包/命名空间
	Language     string   `json:"language"`
	FilePath     string   `json:"file_path"` // 相对于项目根目录的路径
	LineStart    int      `json:"line_start"`
	LineEnd      int      `json:"line_end"`
	Body         string   `json:"body"` // 函数完整源码
	Dependencies []string `json:"dependencies,omitempty"` // 函数内部调用的其他函数名
	Hash        string   `json:"hash,omitempty"`          // 内容哈希（用于去重）

	CallCount    int `json:"call_count"`    // 函数内部调用总次数（含重复调用）
	NestingDepth int `json:"nesting_depth"` // 最大调用嵌套深度（括号嵌套层级）
}

// CallStats 函数调用统计数据，由各语言解析器计算后填入 Function
type CallStats struct {
	Callees      []string // 去重的被调用函数名列表（即 Dependencies）
	CallCount    int      // 调用总次数（含重复调用同一个函数）
	NestingDepth int      // 最大调用嵌套深度（括号嵌套层级）
}

// GlobalVariable 跨文件可使用的全局变量
type GlobalVariable struct {
	ID          int64  `json:"id"`
	SessionID   int64  `json:"session_id,omitempty"`
	Name        string `json:"name"`
	VarType     string `json:"var_type"`     // 变量类型（如 int, string, []byte 等）
	Language    string `json:"language"`
	PackageName string `json:"package_name,omitempty"` // 所属包/命名空间
	Visibility  string `json:"visibility,omitempty"`   // "public" 或 "private"
	FilePath    string `json:"file_path"`
	LineNum     int    `json:"line_num"`
	IsConst     bool   `json:"is_const"`     // 是否为常量（不可变）
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
	Duration     time.Duration
	FileCount    int
	SkipCount    int
	Errors       []string
}
