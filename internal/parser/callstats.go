package parser

import (
	"bytes"
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// stripStringContent 用空格替换字符串字面量和注释内容，使调用正则不会误匹配字符串/注释中的文本
// 保留非字符串部分的位置对齐，确保行号/列号相关的调试信息不受影响
func stripStringContent(line string) string {
	inDouble := false
	inSingle := false
	inBacktick := false
	inBlockComment := false
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

		// 处理字符串边界
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
			// 单行注释 // — 忽略本行剩余内容
			if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
				// 保留注释标记之前的非字符串内容
				return buf.String()
			}
			// 块注释 /* — 忽略至 */
			if ch == '/' && i+1 < len(line) && line[i+1] == '*' {
				inBlockComment = true
				buf.WriteByte(' ')
				i++ // skip '*'
				continue
			}
		}

		if inStringLike(inDouble, inSingle, inBacktick, inBlockComment) {
			buf.WriteByte(' ')
		} else {
			buf.WriteByte(ch)
		}

		// 字符串/注释结束检测
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
	return strings.TrimRight(buf.String(), " ")
}

// inStringLike 判断当前是否在字符串或注释内部
func inStringLike(inDouble, inSingle, inBacktick, inBlockComment bool) bool {
	return inDouble || inSingle || inBacktick || inBlockComment
}

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

		// 匹配函数调用（cleanLine 已剔除字符串/注释内的文本，防止假阳性）
		cleanLine := stripStringContent(line)
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

		// 匹配函数调用（cleanLine 已剔除字符串/注释内的文本，防止假阳性）
		cleanLine := stripStringContent(line)
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
