package parser

import (
	"regexp"

	"code-detector/internal/model"
)

// extractCalls 在函数体内提取函数调用（返回去重的被调用函数名列表）
func extractCalls(body string, callRegex *regexp.Regexp, stringMask, commentMask []bool, startLine, endLine int) []string {
	stats := extractCallStats(body, callRegex, stringMask, commentMask, startLine, endLine, isKeyword, isAllUpper)
	return stats.Callees
}

// extractCallStats 在函数体内提取函数调用统计数据
// 包含：去重的被调用函数名、调用总次数（含重复）、最大嵌套深度
func extractCallStats(body string, callRegex *regexp.Regexp,
	stringMask, commentMask []bool, startLine, endLine int,
	skipFn func(string) bool, allUpperSkip func(string) bool) model.CallStats {

	seen := make(map[string]bool)
	callCount := 0
	maxNesting := 0
	currentNesting := 0

	// 跨行状态：字符串/注释跨行持续追踪（假阳性修复）
	inBlockComment := false
	inRawString := false
	inDoubleString := false
	inSingleString := false

	// 逐字符扫描 body，按 \n 切分行，避免 strings.Split 分配
	lineIdx := 0
	lineStart := 0
	for pos := 0; pos <= len(body); pos++ {
		if pos < len(body) && body[pos] != '\n' {
			continue
		}
		line := body[lineStart:pos]
		lineStart = pos + 1
		globalLine := startLine + lineIdx
		lineIdx++

		commentMasked := globalLine < len(commentMask) && commentMask[globalLine]
		stringMasked := globalLine < len(stringMask) && stringMask[globalLine]
		if commentMasked || stringMasked {
			continue
		}

		// 追踪括号嵌套深度（跨行），区分字符串/注释内的括号（假阳性修复）
		escaped := false
		lineComment := false
		for j, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && (inDoubleString || inSingleString) {
				escaped = true
				continue
			}

			// 行注释 //
			if !inBlockComment && !inDoubleString && !inSingleString && !inRawString && !lineComment {
				if ch == '/' && j+1 < len(line) && line[j+1] == '/' {
					lineComment = true
					break
				}
				if ch == '/' && j+1 < len(line) && line[j+1] == '*' {
					inBlockComment = true
					continue
				}
			}
			if lineComment {
				break
			}

			// 块注释
			if inBlockComment {
				if ch == '*' && j+1 < len(line) && line[j+1] == '/' {
					inBlockComment = false
				}
				continue
			}

			// 字符串边界
			if !inSingleString && !inDoubleString && !inRawString {
				if ch == '"' {
					inDoubleString = true
					continue
				}
				if ch == '\'' {
					inSingleString = true
					continue
				}
				if ch == '`' {
					inRawString = true
					continue
				}
			} else if inRawString {
				if ch == '`' {
					inRawString = false
				}
				continue
			} else if inDoubleString || inSingleString {
				continue
			}

			// 只在非字符串、非注释中追踪括号
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

		// 匹配函数调用
		matches := callRegex.FindAllStringSubmatch(line, -1)
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
				if skipFn != nil && skipFn(name) {
					continue
				}
				if allUpperSkip != nil && allUpperSkip(name) {
					continue
				}
				callCount++
				seen[name] = true
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

// extractCallsSimple 通用函数调用提取器（不需要行 mask 的简化版）
func extractCallsSimple(body string, callRegex *regexp.Regexp, skipFn func(string) bool) []string {
	stats := extractCallStatsSimple(body, callRegex, skipFn)
	return stats.Callees
}

// extractCallStatsSimple 通用函数调用统计提取器（不需要行 mask 的简化版）
// 返回：去重的被调用函数名、调用总次数、最大嵌套深度
func extractCallStatsSimple(body string, callRegex *regexp.Regexp, skipFn func(string) bool) model.CallStats {
	seen := make(map[string]bool)
	callCount := 0
	maxNesting := 0
	currentNesting := 0

	inBlockComment := false
	inRawString := false
	inDoubleString := false
	inSingleString := false
	lineStart := 0
	for pos := 0; pos <= len(body); pos++ {
		if pos < len(body) && body[pos] != '\n' {
			continue
		}
		line := body[lineStart:pos]
		lineStart = pos + 1

		// 追踪括号嵌套深度（跨行），区分字符串/注释内的括号（假阳性修复）
		escaped := false
		lineComment := false
		for j, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && (inDoubleString || inSingleString) {
				escaped = true
				continue
			}

			if !inBlockComment && !inDoubleString && !inSingleString && !inRawString && !lineComment {
				if ch == '/' && j+1 < len(line) && line[j+1] == '/' {
					lineComment = true
					break
				}
				if ch == '/' && j+1 < len(line) && line[j+1] == '*' {
					inBlockComment = true
					continue
				}
			}
			if lineComment {
				break
			}

			if inBlockComment {
				if ch == '*' && j+1 < len(line) && line[j+1] == '/' {
					inBlockComment = false
				}
				continue
			}

			if !inSingleString && !inDoubleString && !inRawString {
				if ch == '"' {
					inDoubleString = true
					continue
				}
				if ch == '\'' {
					inSingleString = true
					continue
				}
				if ch == '`' {
					inRawString = true
					continue
				}
			} else if inRawString {
				if ch == '`' {
					inRawString = false
				}
				continue
			} else if inDoubleString || inSingleString {
				continue
			}

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

		// 匹配函数调用
		matches := callRegex.FindAllStringSubmatch(line, -1)
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
