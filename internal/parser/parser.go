package parser

import (
	"code-detector/internal/model"
)

// DebugMode 全局调试开关，开启时解析器会输出内部调试信息
var DebugMode bool

// Parser 接口：每种语言实现一个 Parser
type Parser interface {
	// Language 返回语言名称
	Language() string
	// Parse 解析文件内容，提取所有函数
	Parse(filePath string, content []byte) ([]*model.Function, error)
	// Globals 解析文件内容，提取跨文件使用的全局变量
	// 不实现则返回 nil
	Globals(filePath string, content []byte) ([]*model.GlobalVariable, error)
}

// Registry 解析器注册表
type Registry struct {
	extToParser map[string]Parser // 按扩展名索引
	langToParser map[string]Parser // 按语言名索引
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{
		extToParser:  make(map[string]Parser),
		langToParser: make(map[string]Parser),
	}
}

// Register 注册一个解析器及其支持的所有扩展名
func (r *Registry) Register(p Parser, extensions ...string) {
	r.langToParser[p.Language()] = p
	for _, ext := range extensions {
		r.extToParser[ext] = p
	}
}

// GetByExt 按文件扩展名获取解析器
func (r *Registry) GetByExt(ext string) Parser {
	return r.extToParser[ext]
}

// GetByLang 按语言名称获取解析器
func (r *Registry) GetByLang(lang string) Parser {
	return r.langToParser[lang]
}

// SupportedExts 返回所有支持的扩展名列表
func (r *Registry) SupportedExts() []string {
	exts := make([]string, 0, len(r.extToParser))
	for ext := range r.extToParser {
		exts = append(exts, ext)
	}
	return exts
}

// ParserRegistration 描述一个解析器及其关联的文件扩展名
type ParserRegistration struct {
	Parser     Parser
	Extensions []string
}

// DefaultParsers 返回内置的解析器列表，用于自动注册
func DefaultParsers() []ParserRegistration {
	return []ParserRegistration{
		{NewTreeSitterGoParser(), []string{".go"}},
		{NewTreeSitterParser(".py"), []string{".py"}},
		{NewTreeSitterParser(".java"), []string{".java"}},
		{NewTreeSitterParser(".js"), []string{".js", ".jsx", ".mjs"}},
		{NewTreeSitterParser(".cs"), []string{".cs"}},
		{NewTreeSitterParser(".cpp"), []string{".cpp", ".cxx", ".cc", ".c", ".h", ".hpp"}},
		{NewTreeSitterParser(".rs"), []string{".rs"}},
		{NewTreeSitterParser(".rb"), []string{".rb"}},
		{NewTreeSitterParser(".ts"), []string{".ts", ".tsx"}},
		{NewTreeSitterParser(".swift"), []string{".swift"}},
		{NewTreeSitterParser(".kt"), []string{".kt", ".kts"}},
		{NewTreeSitterParser(".php"), []string{".php"}},
		{NewTreeSitterParser(".lua"), []string{".lua"}},
		{NewTreeSitterParser(".scala"), []string{".scala"}},
	}
}
