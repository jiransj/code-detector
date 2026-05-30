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

func makeStringMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inRaw := false
	inInterp := false
	for i, line := range lines {
		if inRaw {
			mask[i] = true
			if strings.Contains(line, "`") {
				inRaw = false
			}
			continue
		}
		if inInterp {
			mask[i] = true
			if strings.Contains(line, "\"") {
				inInterp = false
			}
			continue
		}
		if strings.Contains(line, "`") {
			mask[i] = true
			if !strings.HasSuffix(strings.TrimSpace(line), "`") {
				inRaw = true
			}
		}
		if strings.Contains(line, "\"") && !mask[i] {
			mask[i] = true
			if !strings.HasSuffix(strings.TrimSpace(line), "\"") {
				inInterp = true
			}
		}
	}
	return mask
}

// makeMasks 合并 makeCommentMask + makeStringMask 为单趟扫描
// 减少对 lines 的完整遍历次数：从 2 次降为 1 次
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

		// ── 字符串检测 ──
		if inRaw {
			stringMask[i] = true
			if strings.Contains(line, "`") {
				inRaw = false
			}
		} else if inInterp {
			stringMask[i] = true
			if strings.Contains(line, "\"") {
				inInterp = false
			}
		} else {
			if strings.Contains(line, "`") {
				stringMask[i] = true
				if !strings.HasSuffix(strings.TrimSpace(line), "`") {
					inRaw = true
				}
			}
			if strings.Contains(line, "\"") && !stringMask[i] {
				stringMask[i] = true
				if !strings.HasSuffix(strings.TrimSpace(line), "\"") {
					inInterp = true
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

func findBraceInLine(line string) int {
	inSingle, inDouble, inBacktick := false, false, false
	for i, ch := range line {
		if ch == '\'' && !inDouble && !inBacktick {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle && !inBacktick {
			inDouble = !inDouble
		} else if ch == '`' && !inSingle && !inDouble {
			inBacktick = !inBacktick
		} else if ch == '{' && !inSingle && !inDouble && !inBacktick {
			return i
		}
	}
	return -1
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
