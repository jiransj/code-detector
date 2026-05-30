package parser

import "fmt"

// charClass 字符分类，用于状态机快速决策
type charClass int

const (
	ccOther charClass = iota
	ccBackslash
	ccSingleQuote
	ccDoubleQuote
	ccBacktick
	ccSlash
	ccStar
	ccOpenBrace
	ccCloseBrace
	ccOpenParen
	ccR
	ccNewline
)

var charClassTable [256]charClass

func init() {
	for i := range charClassTable {
		switch {
		case i == '\\':
			charClassTable[i] = ccBackslash
		case i == '\'':
			charClassTable[i] = ccSingleQuote
		case i == '"':
			charClassTable[i] = ccDoubleQuote
		case i == '`':
			charClassTable[i] = ccBacktick
		case i == '/':
			charClassTable[i] = ccSlash
		case i == '*':
			charClassTable[i] = ccStar
		case i == '{':
			charClassTable[i] = ccOpenBrace
		case i == '}':
			charClassTable[i] = ccCloseBrace
		case i == '(':
			charClassTable[i] = ccOpenParen
		case i == 'R':
			charClassTable[i] = ccR
		case i == '\n':
			charClassTable[i] = ccNewline
		}
	}
}

// matchBrace 从 openPos 处（应指向 '{'）开始栈匹配，返回匹配的 '}' 位置
func matchBrace(text string, openPos int) (int, error) {
	if openPos >= len(text) || text[openPos] != '{' {
		return -1, fmt.Errorf("not a brace at position %d", openPos)
	}

	stack := 1 // 已计入起始 '{'
	inSingle, inDouble, inRaw := false, false, false
	escaped := false

	for i := openPos + 1; i < len(text); i++ {
		ch := text[i]

		if escaped {
			escaped = false
			continue
		}

		cc := charClassTable[ch]

		// 转义符：仅在字符串内有效
		if cc == ccBackslash && (inDouble || inSingle) {
			escaped = true
			continue
		}

		// ── 字符串/注释状态跳转 ──
		if inRaw {
			if cc == ccBacktick {
				inRaw = false
			}
			continue
		}
		if inDouble {
			if cc == ccDoubleQuote {
				inDouble = false
			}
			continue
		}
		if inSingle {
			if cc == ccSingleQuote {
				inSingle = false
			}
			continue
		}

		// 不在任何字符串内
		switch cc {
		case ccBacktick:
			inRaw = true
		case ccDoubleQuote:
			inDouble = true
		case ccSingleQuote:
			inSingle = true
		case ccSlash:
			if i+1 < len(text) {
				next := text[i+1]
				if next == '/' {
					// 单行注释：跳过到行尾
					for i < len(text) && text[i] != '\n' {
						i++
					}
					continue
				}
				if next == '*' {
					// 块注释：跳过到 */
					i += 2
					for i < len(text) {
						if text[i] == '*' && i+1 < len(text) && text[i+1] == '/' {
							i += 2
							break
						}
						i++
					}
					i-- // for 循环会 ++
					continue
				}
			}
		case ccR:
			// C++ 原始字符串 R"delimiter(...)delimiter"
			if i+1 < len(text) && text[i+1] == '"' {
				i = skipCppRawString(text, i)
				continue
			}
		case ccOpenBrace:
			stack++
		case ccCloseBrace:
			stack--
			if stack == 0 {
				return i, nil
			}
		}
	}

	return -1, fmt.Errorf("unmatched opening brace at %d", openPos)
}

// skipCppRawString 跳过 C++ R"delimiter(...)delimiter" 原始字符串
// pos 指向 'R' 字符，返回应继续处理的位置
func skipCppRawString(text string, pos int) int {
	// R"delimiter(
	delimStart := pos + 2
	delimEnd := delimStart
	for delimEnd < len(text) && text[delimEnd] != '(' {
		delimEnd++
	}
	if delimEnd >= len(text) || text[delimEnd] != '(' {
		return pos // 不是合法的 R"(，回退
	}
	delim := text[delimStart:delimEnd]
	// 跳过到 )delimiter"
	i := delimEnd + 1 // 跳过 '('
	for i < len(text) {
		if text[i] == ')' && i+1+len(delim) < len(text) &&
			text[i+1:i+1+len(delim)] == delim &&
			text[i+1+len(delim)] == '"' {
			return i + 1 + len(delim) + 1 - 1 // -1 因为 for 循环会 ++
		}
		i++
	}
	return len(text) - 1
}
