package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"code-detector/internal/model"
)

// Config 顶层配置结构
type Config struct {
	Languages []model.LanguageConfig `yaml:"languages"`
}

// DefaultConfig 返回内置默认配置（数据驱动，无深层嵌套）
func DefaultConfig() *Config {
	return &Config{Languages: defaultLanguages()}
}

// defaultLanguages 返回内置语言配置列表（数据驱动，避免超长函数）
func defaultLanguages() []model.LanguageConfig {
	return []model.LanguageConfig{
		{Name: "go", Extensions: []string{".go"}, FunctionRegex: `func\s+(?P<name>\w+)\s*\(`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "python", Extensions: []string{".py"}, FunctionRegex: `^\s*def\s+(?P<name>\w+)\s*\(`, BodyStrategy: "indent", SingleComment: []string{"#"}, BlockComment: [][2]string{{`"""`, `"""`}, {"'''", "'''"}}},
		{Name: "java", Extensions: []string{".java"}, FunctionRegex: `(?:(?:public|private|protected|static|final|abstract|synchronized)\s+)*(?:\w+(?:\[\])*)\s+(?P<name>\w+)\s*\(`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "kotlin", Extensions: []string{".kt", ".kts"}, FunctionRegex: `(?:fun\s+)(?P<name>\w+)\s*\(`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "javascript", Extensions: []string{".js", ".jsx", ".mjs"}, FunctionRegex: `(?:(?:async\s+)?function\s+(?P<name>\w+)\s*\(|(?:\w+)\s*[:=]\s*(?:async\s+)?function\s*\(|(?:\w+)\s*[:=]\s*\([^)]*\)\s*=>)`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "typescript", Extensions: []string{".ts", ".tsx"}, FunctionRegex: `(?:(?:async\s+)?function\s+(?P<name>\w+)\s*\(|(?:\w+)\s*[:=]\s*(?:async\s+)?function\s*\(|(?:\w+)\s*[:=]\s*\([^)]*\)\s*=>)`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "csharp", Extensions: []string{".cs"}, FunctionRegex: `(?:(?:public|private|protected|internal|static|virtual|override|abstract|async|unsafe)\s+)*(?:\w+(?:\[\])*(?:<[^>]+>)?)\s+(?P<name>\w+)\s*\(`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "cpp", Extensions: []string{".cpp", ".cxx", ".cc", ".c", ".h", ".hpp"}, FunctionRegex: `(?:(?:\w+(?:\[\])*(?:\s*<[^>]+>)?\s+)+)(?P<name>\w+)\s*\(`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "rust", Extensions: []string{".rs"}, FunctionRegex: `(?:pub\s+(?:unsafe\s+)?)?fn\s+(?P<name>\w+)\s*\(`, BodyStrategy: "brace", SingleComment: []string{"//"}, BlockComment: [][2]string{{"/*", "*/"}}},
		{Name: "ruby", Extensions: []string{".rb"}, FunctionRegex: `^\s*(?:def\s+)(?P<name>\w+(?:[?!])?)\s*[\(;]`, BodyStrategy: "end", SingleComment: []string{"#"}, BlockComment: [][2]string{{"=begin", "=end"}}},
	}
}

// LoadConfig 从 YAML 文件加载配置，如果文件不存在则返回默认配置
// 如果 YAML 中 languages 为空，则使用内置默认配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	// 先解析到一个临时结构检查是否为空
	var raw struct {
		Languages []model.LanguageConfig `yaml:"languages"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := DefaultConfig()
	if len(raw.Languages) > 0 {
		cfg.Languages = raw.Languages
	}
	return cfg, nil
}

// GetLanguageConfigsMap 返回按扩展名索引的语言配置
func (c *Config) GetLanguageConfigsMap() map[string]*model.LanguageConfig {
	m := make(map[string]*model.LanguageConfig)
	for i := range c.Languages {
		lang := &c.Languages[i]
		for _, ext := range lang.Extensions {
			m[ext] = lang
		}
	}
	return m
}

// GetLanguageByName 按名称查找语言配置
func (c *Config) GetLanguageByName(name string) *model.LanguageConfig {
	for i := range c.Languages {
		if c.Languages[i].Name == name {
			return &c.Languages[i]
		}
	}
	return nil
}
