package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// ErbParser 纯 Go ERB 解析器
// ERB 语法: <% code %>, <%= expr %>, <%# comment %>
// 通过状态机分离出 Ruby 代码块，再用正则提取函数定义和调用
type ErbParser struct{}

func NewErbParser() *ErbParser { return &ErbParser{} }

func (p *ErbParser) Language() string { return "erb" }

// erbBlock 表示一个 ERB 代码块
type erbBlock struct {
	code     string // Ruby 代码内容（不含 <% %> 标签）
	lineFrom int    // 起始行（0-based）
	lineTo   int    // 终止行（0-based）
}

// extractErbBlocks 将 ERB 文件拆分为 Ruby 代码块和文本
func extractErbBlocks(content []byte) []erbBlock {
	text := string(content)
	var blocks []erbBlock

	type stateType int
	const (
		stText stateType = iota
		stDirective // <% ... %>
		stOutput    // <%= ... %>
		stComment   // <%# ... %>
	)

	curState := stText
	blockStart := 0 // 代码块起始位置

	// 行号追踪函数
	lineOf := func(pos int) int {
		if pos < 0 {
			return 0
		}
		n := 0
		for i := 0; i < pos && i < len(text); i++ {
			if text[i] == '\n' {
				n++
			}
		}
		return n
	}

	for i := 0; i < len(text); i++ {
		switch curState {
		case stText:
			if text[i] == '<' && i+1 < len(text) && text[i+1] == '%' {
				if i+2 < len(text) && text[i+2] == '=' {
					curState = stOutput
					blockStart = i + 3
					i += 2
				} else if i+2 < len(text) && text[i+2] == '#' {
					curState = stComment
					blockStart = i + 3
					i += 2
				} else {
					curState = stDirective
					blockStart = i + 2
					i++
				}
			}
		case stDirective, stOutput:
			if text[i] == '%' && i+1 < len(text) && text[i+1] == '>' {
				code := strings.TrimSpace(text[blockStart:i])
				if code != "" {
					blocks = append(blocks, erbBlock{
						code:     code,
						lineFrom: lineOf(blockStart),
						lineTo:   lineOf(i),
					})
				}
				curState = stText
				i++
			}
		case stComment:
			if text[i] == '%' && i+1 < len(text) && text[i+1] == '>' {
				curState = stText
				i++
			}
		}
	}
	return blocks
}

// rubyDefRegex 匹配 Ruby 函数定义: def name / def self.name / def name(args)
var rubyDefRegex = regexp.MustCompile(`^\s*def\s+(?:self\.)?(?P<name>[a-zA-Z_]\w*)\b`)

// stripErbTags 从一行中去除 ERB 标签和 HTML，只取 Ruby 代码
func stripErbTags(line string) string {
	s := line
	// 找到 <% 和 %> 之间的内容
	start := strings.Index(s, "<%")
	if start < 0 {
		return ""
	}
	s = s[start+2:] // 跳过 <%
	// 跳过 = 或 #
	if len(s) > 0 && (s[0] == '=' || s[0] == '#') {
		return ""
	}
	end := strings.Index(s, "%>")
	if end < 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:end])
}

// rubyEndLine 判断一行中的 Ruby 代码是否为 end
func isRubyEndLine(line string) bool {
	code := stripErbTags(line)
	if code == "" {
		return false
	}
	return code == "end" || code == "end," || code == "end)"
}

// rubyBlockStart 判断一行中的 Ruby 代码是否开启新块
func rubyBlockStart(line string) bool {
	code := stripErbTags(line)
	if code == "" {
		return false
	}
	if strings.HasPrefix(code, "def ") || strings.HasPrefix(code, "def(") {
		return true
	}
	if strings.HasPrefix(code, "class ") || strings.HasPrefix(code, "module ") {
		return true
	}
	if strings.HasPrefix(code, "if ") || strings.HasPrefix(code, "unless ") {
		return true
	}
	if strings.HasPrefix(code, "case ") || strings.HasPrefix(code, "begin ") {
		return true
	}
	if code == "do" || strings.HasPrefix(code, "do ") || strings.HasPrefix(code, "do|") {
		return true
	}
	if strings.HasPrefix(code, "for ") || strings.HasPrefix(code, "while ") || strings.HasPrefix(code, "until ") {
		return true
	}
	return false
}

// erbCallRegex 匹配 Ruby 函数调用（兼容 extractCallStatsSimple 的 2-group 格式）
// group 1 = prefix (before dot, 可选), group 2 = function name
var erbCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *ErbParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	blocks := extractErbBlocks(content)
	if len(blocks) == 0 {
		return nil, nil
	}

	// 将文件按行分割
	lines := strings.Split(string(content), "\n")

	// 先收集每个代码块内部的函数定义
	type funcMatch struct {
		name    string
		defLine int // 函数定义行号（0-based）
		endLine int // 函数体结束行号
		body    string
	}

	var funcDefs []funcMatch

	for _, block := range blocks {
		// 代码块可能跨多行，按行处理
		blockLines := strings.Split(block.code, "\n")
		for j, line := range blockLines {
			m := rubyDefRegex.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			nameIdx := rubyDefRegex.SubexpIndex("name")
			if nameIdx < 0 || nameIdx >= len(m) {
				continue
			}
			name := m[nameIdx]
			defLine := block.lineFrom + j

			// 从 def 的行号开始，在原始文件中找匹配的 end（追踪 def...end 嵌套）
			endLine := defLine
			nestLevel := 1
			bodyLines := []string{}
			for k := defLine; k < len(lines); k++ {
				bodyLines = append(bodyLines, lines[k])
				if k > defLine && rubyBlockStart(lines[k]) {
					nestLevel++
				}
				if k > defLine && isRubyEndLine(lines[k]) {
					nestLevel--
					if nestLevel == 0 {
						endLine = k
						break
					}
				}
			}

			funcDefs = append(funcDefs, funcMatch{
				name:    name,
				defLine: defLine,
				endLine: endLine,
				body:    strings.Join(bodyLines, "\n"),
			})
		}
	}

	if len(funcDefs) == 0 {
		return nil, nil
	}

	// 构建结果
	result := make([]*model.Function, 0, len(funcDefs))
	for _, fd := range funcDefs {
		// 从 body 中提取调用
		callStats := extractCallStatsSimple(fd.body, erbCallRegex, func(name string) bool {
			return goKeywords[name]
		})

		f := &model.Function{
			Name:         fd.name,
			PackageName:  "",
			Language:     "erb",
			FilePath:     filePath,
			LineStart:    fd.defLine + 1,
			LineEnd:      fd.endLine + 1,
			Body:         fd.body,
			Dependencies: callStats.Callees,
			CallCount:    callStats.CallCount,
			NestingDepth: callStats.NestingDepth,
		}
		result = append(result, f)
	}

	return result, nil
}

func (p *ErbParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	return nil, nil
}
