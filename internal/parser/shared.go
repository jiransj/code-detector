package parser

import (
	"strings"
	"unicode"
)

// ─── 共用工具函数 ────────────────────────────────────

func isKeyword(name string) bool { return goKeywords[name] }

func isAllUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 'a' && b <= 'z' {
			return false
		}
		if b < 0x80 {
			continue
		}
		for _, r := range s[i:] {
			if unicode.IsLetter(r) && !unicode.IsUpper(r) {
				return false
			}
		}
		return true
	}
	return true
}

func makeCommentMask(lines []string, singleComments []string, blockComments [][2]string) []bool {
	mask := make([]bool, len(lines))
	inBlock := false
	for i, line := range lines {
		if inBlock {
			mask[i] = true
			for _, bc := range blockComments {
				if idx := strings.Index(line, bc[1]); idx >= 0 {
					inBlock = false
					break
				}
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		for _, sc := range singleComments {
			if strings.HasPrefix(trimmed, sc) {
				mask[i] = true
				break
			}
		}
		if mask[i] {
			continue
		}
		for _, bc := range blockComments {
			if idx := strings.Index(line, bc[0]); idx >= 0 {
				if endIdx := strings.Index(line[idx+len(bc[0]):], bc[1]); endIdx >= 0 {
					continue
				}
				inBlock = true
				mask[i] = true
				break
			}
		}
	}
	return mask
}

// makeMasks 单趟扫描同时对注释和字符串行做掩码（按字符状态机，正确处理转义符）
func makeMasks(lines []string, singleComments []string, blockComments [][2]string) (commentMask, stringMask []bool) {
	commentMask = make([]bool, len(lines))
	stringMask = make([]bool, len(lines))
	inBlock := false
	inRaw := false
	inInterp := false
	for i, line := range lines {
		// ── 注释检测 ──
		if inBlock {
			commentMask[i] = true
			for _, bc := range blockComments {
				if idx := strings.Index(line, bc[1]); idx >= 0 {
					inBlock = false
					break
				}
			}
		} else {
			trimmed := strings.TrimSpace(line)
			for _, sc := range singleComments {
				if strings.HasPrefix(trimmed, sc) {
					commentMask[i] = true
					break
				}
			}
			if !commentMask[i] {
				for _, bc := range blockComments {
					if idx := strings.Index(line, bc[0]); idx >= 0 {
						if endIdx := strings.Index(line[idx+len(bc[0]):], bc[1]); endIdx >= 0 {
							continue
						}
						inBlock = true
						commentMask[i] = true
						break
					}
				}
			}
		}

		// ── 字符串检测（按字符状态机，正确处理转义） ──
		if inRaw {
			stringMask[i] = true
			// 逐字符扫描反引号结束
			for j := 0; j < len(line); j++ {
				if line[j] == '`' {
					inRaw = false
					break
				}
			}
		} else if inInterp {
			stringMask[i] = true
			// 逐字符扫描，处理 \" 转义
			escaped := false
			for j := 0; j < len(line); j++ {
				if escaped {
					escaped = false
					continue
				}
				if line[j] == '\\' {
					escaped = true
					continue
				}
				if line[j] == '"' {
					inInterp = false
					break
				}
			}
		} else {
			// 不处于任何字符串中 → 逐字符扫描检测字符串开启
			escaped := false
			for j := 0; j < len(line); j++ {
				if escaped {
					escaped = false
					continue
				}
				ch := line[j]
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '`' {
					stringMask[i] = true
					// 反引号开/关切换：若本行之后还有 ` 则已关闭
					remain := line[j+1:]
					if idx := strings.IndexByte(remain, '`'); idx >= 0 {
						j += idx + 1 // 跳过闭合的反引号
					} else {
						inRaw = true // 未闭合 → 跨行
						break
					}
					continue
				}
				if ch == '"' && !commentMask[i] {
					stringMask[i] = true
					// 双引号开/关切换（处理 \" 转义）
					remain := line[j+1:]
					idx := strings.IndexByte(remain, '"')
					for idx >= 0 && idx > 0 && remain[idx-1] == '\\' {
						// 检查是否是真正的转义 \\" vs \"
						bsCount := 0
						for k := idx - 1; k >= 0 && remain[k] == '\\'; k-- {
							bsCount++
						}
						if bsCount%2 == 1 { // 奇数反斜杠 → 转义引号
							next := strings.IndexByte(remain[idx+1:], '"')
							if next < 0 {
								idx = -1
							} else {
								idx = idx + 1 + next
							}
						} else {
							break // 偶数反斜杠 → 真正的引号结束
						}
					}
					if idx >= 0 {
						j += idx + 1
					} else {
						inInterp = true
						break
					}
					continue
				}
			}
		}
	}
	return
}

func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4
		} else {
			break
		}
	}
	return count
}


// goKeywords Go 关键字和常用内置函数（用于调用分析过滤）
var goKeywords = map[string]bool{
	"if": true, "else": true, "for": true, "range": true, "switch": true,
	"case": true, "default": true, "break": true, "continue": true, "return": true,
	"go": true, "defer": true, "select": true,
	"var": true, "const": true, "type": true, "struct": true, "interface": true,
	"func": true, "map": true, "chan": true, "package": true, "import": true,
	"make": true, "new": true, "append": true, "len": true, "cap": true,
	"copy": true, "delete": true, "close": true, "panic": true, "recover": true,
	"print": true, "println": true,
	"nil": true, "true": true, "false": true, "iota": true,
	"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"bool": true, "byte": true, "rune": true, "uintptr": true, "any": true, "error": true,
	// 避免误报的常用标准库函数
	"Close": true, "Read": true, "Write": true, "Seek": true, "Stat": true,
	"Open": true, "Create": true, "OpenFile": true,
	"ReadFile": true, "WriteFile": true, "ReadAll": true, "ReadDir": true,
	"Mkdir": true, "MkdirAll": true, "Remove": true, "RemoveAll": true, "Rename": true,
	"Scan": true, "Scanf": true, "Scanln": true, "Sscanf": true,
	"Split": true, "Join": true, "Contains": true, "HasPrefix": true, "HasSuffix": true,
	"Replace": true, "ReplaceAll": true, "Trim": true, "TrimSpace": true,
	"ToLower": true, "ToUpper": true, "Index": true, "LastIndex": true,
	"Atoi": true, "Itoa": true, "ParseInt": true, "FormatInt": true, "ParseFloat": true,
	"Set": true, "Get": true, "Load": true, "Store": true,
	"LoadOrStore": true, "LoadAndDelete": true,
	"Error": true, "Errorf": true, "Wrap": true, "Wrapf": true,
	"Is": true, "As": true, "Unwrap": true,
	"Context": true, "Cancel": true, "Deadline": true, "Done": true, "Err": true,
	"Marshal": true, "Unmarshal": true, "MarshalIndent": true,
	"Decode": true, "Encode": true,
	// testing
	"Run": true, "Log": true, "Logf": true,
	"Fatal": true, "Fatalf": true, "Fail": true, "FailNow": true, "Skip": true,
	// db
	"Exec": true, "Query": true, "QueryRow": true, "Prepare": true,
	"Begin": true, "Commit": true, "Rollback": true, "Ping": true,
}
