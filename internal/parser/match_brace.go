package parser

import "fmt"

// matchBrace 从 openPos 处（应指向 '{'）开始栈匹配，返回匹配的 '}' 位置
func matchBrace(text string, openPos int) (int, error) {
	if openPos >= len(text) || text[openPos] != '{' {
		return -1, fmt.Errorf("not a brace at position %d", openPos)
	}
	stack := 0
	inSingle := false
	inDouble := false
	inRaw := false  // Go 原生字符串 `...`
	escaped := false

	for i := openPos; i < len(text); i++ {
		ch := text[i]

		if escaped {
			escaped = false
			continue
		}

		// 处理转义
		if ch == '\\' && (inDouble || inSingle) {
			escaped = true
			continue
		}

		// C++ 原始字符串字面量 R"delimiter(...)delimiter"
		if !inRaw && !inSingle && !inDouble && ch == 'R' && i+1 < len(text) && text[i+1] == '"' {
			// 提取分隔符: R"delimiter(
			delimStart := i + 2
			delimEnd := delimStart
			for delimEnd < len(text) && text[delimEnd] != '(' {
				delimEnd++
			}
			if delimEnd < len(text) && text[delimEnd] == '(' {
				delim := text[delimStart:delimEnd]
				// 跳过原始字符串体直到 )delimiter"
				i = delimEnd + 1 // 跳过 '('
				for i < len(text) {
					if text[i] == ')' && i+1+len(delim) < len(text) &&
						text[i+1:i+1+len(delim)] == delim &&
						text[i+1+len(delim)] == '"' {
						i = i + 1 + len(delim) + 1 // 跳过 )delimiter"
						break
					}
					i++
				}
				if i >= len(text) {
					i = len(text) - 1
				}
				continue
			}
			// 不是有效的 R"( 语法，继续正常处理
		}

		// 处理字符串边界
		if !inRaw && !inSingle && !inDouble {
			if ch == '`' {
				inRaw = true
				continue
			}
		}
		if inRaw {
			if ch == '`' {
				inRaw = false
			}
			continue
		}
		if !inSingle && !inDouble {
			if ch == '"' {
				inDouble = true
				continue
			}
			if ch == '\'' {
				inSingle = true
				continue
			}
		} else {
			if inDouble && ch == '"' {
				inDouble = false
			} else if inSingle && ch == '\'' {
				inSingle = false
			}
			continue
		}

		// 单行注释 // — 跳过到行尾
		if ch == '/' && i+1 < len(text) && text[i+1] == '/' {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		}
		// 块注释 /* */
		if ch == '/' && i+1 < len(text) && text[i+1] == '*' {
			i += 2
			for i < len(text) {
				if text[i] == '*' && i+1 < len(text) && text[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			i-- // for loop will increment
			continue
		}

		if ch == '{' {
			stack++
		} else if ch == '}' {
			stack--
			if stack == 0 {
				return i, nil
			}
		}
	}

	return -1, fmt.Errorf("unmatched opening brace at %d", openPos)
}
