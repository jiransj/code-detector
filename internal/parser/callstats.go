package parser

import (
	"bytes"
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// inStringLike 判断当前是否在字符串或注释内部
func inStringLike(inDouble, inSingle, inBacktick, inBlockComment bool) bool {
	return inDouble || inSingle || inBacktick || inBlockComment
}

// stripperState 跨行字符串/注释状态，用于 stripLine 跨行追踪块注释
type stripperState struct {
	InBlockComment bool
}

// stripLine 脱敏单行：用空格替换字符串和注释内容，保留非字符串部分对齐
// 通过 state 追踪跨行块注释 /* */ 状态
func stripLine(line string, state *stripperState) string {
	// 快速路径：不在块注释中且不含任何特殊字符的行直接返回
	if !state.InBlockComment && !strings.ContainsAny(line, "\"'`/\\") {
		return line
	}
	inDouble := false
	inSingle := false
	inBacktick := false
	inBlockComment := state.InBlockComment
	escaped := false
	var buf bytes.Buffer
	buf.Grow(len(line))
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			escaped = false
			buf.WriteByte(' ')
			continue
		}
		if ch == '\\' && (inDouble || inSingle) {
			escaped = true
			buf.WriteByte(' ')
			continue
		}

		if !inSingle && !inDouble && !inBacktick && !inBlockComment {
			if ch == '"' {
				inDouble = true
				buf.WriteByte(' ')
				continue
			}
			if ch == '\'' {
				inSingle = true
				buf.WriteByte(' ')
				continue
			}
			if ch == '`' {
				inBacktick = true
				buf.WriteByte(' ')
				continue
			}
			if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
				// 单行注释 // — 忽略剩余部分
				state.InBlockComment = false
				return strings.TrimRight(buf.String(), " ")
			}
			if ch == '/' && i+1 < len(line) && line[i+1] == '*' {
				inBlockComment = true
				buf.WriteByte(' ')
				i++
				continue
			}
		}

		if inStringLike(inDouble, inSingle, inBacktick, inBlockComment) {
			buf.WriteByte(' ')
		} else {
			buf.WriteByte(ch)
		}

		if inDouble && ch == '"' && !escaped {
			inDouble = false
		} else if inSingle && ch == '\'' && !escaped {
			inSingle = false
		} else if inBacktick && ch == '`' {
			inBacktick = false
		} else if inBlockComment && ch == '*' && i+1 < len(line) && line[i+1] == '/' {
			inBlockComment = false
			buf.WriteByte(' ')
			i++
		}
	}
	state.InBlockComment = inBlockComment
	return strings.TrimRight(buf.String(), " ")
}

// extractCallStatsSimple 通用函数调用统计提取器（不需要行 mask 的简化版）
// 返回：去重的被调用函数名、调用总次数、最大嵌套深度
func extractCallStatsSimple(body string, callRegex *regexp.Regexp, skipFn func(string) bool) model.CallStats {
	seen := make(map[string]bool)
	callCount := 0
	maxNesting := 0
	currentNesting := 0

	var ss stripperState
	lineStart := 0
	for pos := 0; pos <= len(body); pos++ {
		if pos < len(body) && body[pos] != '\n' {
			continue
		}
		line := body[lineStart:pos]
		lineStart = pos + 1

		// stripLine 一次性脱敏并追踪跨行块注释
		cleanLine := stripLine(line, &ss)

		// 在脱敏行上追踪括号嵌套
		for _, ch := range cleanLine {
			if ch == '(' {
				currentNesting++
				if currentNesting > maxNesting {
					maxNesting = currentNesting
				}
			} else if ch == ')' {
				if currentNesting > 0 {
					currentNesting--
				}
			}
		}

		// 在脱敏行上匹配函数调用
		matches := callRegex.FindAllStringSubmatch(cleanLine, -1)
		for _, m := range matches {
			if len(m) >= 3 {
				prefix := m[1]
				name := m[2]
				if name == "" || len(name) <= 1 {
					continue
				}
				if prefix != "" {
					callCount++
					seen[name] = true
					continue
				}
				if !skipFn(name) {
					callCount++
					seen[name] = true
				}
			}
		}
	}

	deps := make([]string, 0, len(seen))
	for name := range seen {
		deps = append(deps, name)
	}

	return model.CallStats{
		Callees:      deps,
		CallCount:    callCount,
		NestingDepth: maxNesting,
	}
}
