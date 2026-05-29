package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// GenericParser 基于 YAML LanguageConfig 的通用正则解析器
// 使得用户可以通过 config.yaml 添加新语言，无需编写 Go 代码
type GenericParser struct {
	config model.LanguageConfig
}

// NewGenericParser 创建基于配置的通用解析器
func NewGenericParser(cfg model.LanguageConfig) *GenericParser {
	return &GenericParser{config: cfg}
}

func (p *GenericParser) Language() string { return p.config.Name }

// Parse 使用配置的 FunctionRegex + BodyStrategy 解析函数
func (p *GenericParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, p.config.SingleComment, p.config.BlockComment)
	stringMask := makeStringMask(lines)

	funcRegex, err := regexp.Compile(p.config.FunctionRegex)
	if err != nil {
		return nil, nil
	}

	pkgName := extractPkgNameForStrategy(lines, commentMask, p.config.BodyStrategy)

	_ = stringMask

	type funcMatch struct {
		name    string
		lineIdx int
	}

	var matches []funcMatch

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		locs := funcRegex.FindStringSubmatchIndex(line)
		if locs == nil {
			continue
		}
		nameIdx := funcRegex.SubexpIndex("name")
		if nameIdx < 0 || 2*nameIdx+1 >= len(locs) {
			continue
		}
		nameStart := locs[2*nameIdx]
		nameEnd := locs[2*nameIdx+1]
		if nameStart < 0 || nameEnd < 0 {
			continue
		}
		name := line[nameStart:nameEnd]

		matches = append(matches, funcMatch{name: name, lineIdx: i})
	}

	if len(matches) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(matches))

	for _, m := range matches {
		var body string
		var startLine, endLine int

		switch p.config.BodyStrategy {
		case "brace":
			braceOffset := -1
			for j := m.lineIdx; j < len(lines); j++ {
				if commentMask[j] {
					continue
				}
				if idx := strings.Index(lines[j], "{"); idx >= 0 {
					braceOffset = fl.LineOffset(j) + idx
					break
				}
			}
			if braceOffset < 0 {
				continue
			}
			closeOffset, err := matchBrace(text, braceOffset)
			if err != nil {
				continue
			}
			startLine = m.lineIdx
			endLine = fl.LineFromOffset(closeOffset)
			bodyStart := fl.LineOffset(startLine)
			bodyEnd := closeOffset + 1
			if bodyEnd > len(text) {
				bodyEnd = len(text)
			}
			body = text[bodyStart:bodyEnd]

		case "indent":
			indent := countIndent(lines[m.lineIdx])
			startLine = m.lineIdx
			endLine = m.lineIdx
			for j := m.lineIdx + 1; j < len(lines); j++ {
				if commentMask[j] || strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if countIndent(lines[j]) <= indent && strings.TrimSpace(lines[j]) != "" {
					endLine = j - 1
					break
				}
				endLine = j
			}
			bodyLines := lines[startLine : endLine+1]
			body = strings.Join(bodyLines, "\n")

		case "end":
			startLine = m.lineIdx
			endLine = m.lineIdx
			for j := m.lineIdx + 1; j < len(lines); j++ {
				if commentMask[j] {
					continue
				}
				trim := strings.TrimSpace(lines[j])
				if trim == "end" || trim == "end," {
					endLine = j
					break
				}
				endLine = j
			}
			bodyLines := lines[startLine : endLine+1]
			body = strings.Join(bodyLines, "\n")
		}

		if body == "" {
			continue
		}

		callStats := extractCallStatsSimple(body, genericCallRegex, func(string) bool { return false })

		f := &model.Function{
			Name:         m.name,
			PackageName:  pkgName,
			Language:     p.config.Name,
			FilePath:     filePath,
			LineStart:    startLine + 1,
			LineEnd:      endLine + 1,
			Body:         body,
			Dependencies: callStats.Callees,
			CallCount:    callStats.CallCount,
			NestingDepth: callStats.NestingDepth,
		}
		allFuncs = append(allFuncs, f)
	}

	return allFuncs, nil
}

func (p *GenericParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	return nil, nil
}

// extractPkgNameForStrategy 根据策略提取包名
func extractPkgNameForStrategy(lines []string, commentMask []bool, strategy string) string {
	switch strategy {
	case "brace":
		pkgRegex := regexp.MustCompile(`^\s*(?:package|namespace)\s+(?P<name>\w+)`)
		for i, line := range lines {
			if commentMask[i] {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if matches := pkgRegex.FindStringSubmatch(trimmed); matches != nil {
				nameIdx := pkgRegex.SubexpIndex("name")
				if nameIdx >= 0 && nameIdx < len(matches) {
					return matches[nameIdx]
				}
				break
			}
		}
	}
	return ""
}

// genericCallRegex 通用函数调用匹配正则
var genericCallRegex = regexp.MustCompile(`(?:\w+\.)?(\w+)\s*\(`)

// extractCallsFromBody 从函数体中提取调用（通用模式，委托给 extractCallStatsSimple）
func extractCallsFromBody(body string) []string {
	stats := extractCallStatsSimple(body, genericCallRegex, func(string) bool { return false })
	return stats.Callees
}
