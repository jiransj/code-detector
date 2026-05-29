package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// PythonParser 解析 Python 源文件
type PythonParser struct{}

func NewPythonParser() *PythonParser { return &PythonParser{} }

func (p *PythonParser) Language() string { return "python" }

func (p *PythonParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makePythonCommentMask(lines)

	var vars []*model.GlobalVariable
	pyVarRegex := regexp.MustCompile(`^\s*(?P<name>\w+)\s*(?::\s*(?P<type>\w+(?:\[.*?\])?))?\s*=\s*`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "import ") ||
			strings.HasPrefix(trimmed, "from ") || strings.HasPrefix(trimmed, "return ") ||
			strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "elif ") ||
			strings.HasPrefix(trimmed, "else") || strings.HasPrefix(trimmed, "for ") ||
			strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "try") ||
			strings.HasPrefix(trimmed, "except") || strings.HasPrefix(trimmed, "with ") {
			continue
		}

		matches := pyVarRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := pyVarRegex.SubexpIndex("name")
		typeIdx := pyVarRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" && !isPyKeyword(name) {
			vars = append(vars, &model.GlobalVariable{
				Name:     name,
				VarType:  varType,
				Language: "python",
				FilePath: filePath,
				LineNum:  i + 1,
				IsConst:  strings.ToUpper(name) == name && len(name) > 1,
			})
		}
	}
	return vars, nil
}

// pyFuncRegex 匹配 def 定义行（必须在行首非空字符开始）
var pyFuncRegex = regexp.MustCompile(`^\s*def\s+(?P<name>\w+)\s*\(`)

// pyCallRegex 匹配函数调用
var pyCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *PythonParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	text := string(content)
	lines := strings.Split(text, "\n")

	// Python 注释 + 多行字符串
	commentMask := makePythonCommentMask(lines)
	stringMask := makePythonStringMask(lines)

	// 定位函数定义
	type pyFuncStart struct {
		lineIdx  int
		name     string
		indent   int // 函数定义行的缩进级别
	}
	var starts []pyFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		matches := pyFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := pyFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		indent := countIndent(line)
		starts = append(starts, pyFuncStart{
			lineIdx: i,
			name:    name,
			indent:  indent,
		})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))

	for _, fs := range starts {
		// Python 函数体结束条件：遇到缩进 <= 本函数定义缩进且非空/非注释/非装饰器的行
		bodyStart := fs.lineIdx + 1
		if bodyStart >= len(lines) {
			continue
		}

		bodyEnd := len(lines) - 1
		foundEnd := false
		for j := bodyStart; j < len(lines); j++ {
			if commentMask[j] || stringMask[j] {
				continue
			}
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			// 空行或装饰器不结束
			if strings.HasPrefix(trimmed, "@") {
				continue
			}
			indent := countIndent(lines[j])
			if indent <= fs.indent {
				bodyEnd = j - 1
				foundEnd = true
				break
			}
		}
		if !foundEnd {
			bodyEnd = len(lines) - 1
		}

		// 提取函数体
		bodyLines := lines[fs.lineIdx : bodyEnd+1]
		body := strings.Join(bodyLines, "\n")

		// 提取调用统计（只在函数体内）
		callStats := extractCallStatsSimple(body, pyCallRegex, func(name string) bool {
			return isPyKeyword(name) || name[0] == '_'
		})

		f := &model.Function{
			Name:         fs.name,
			Language:     "python",
			FilePath:     filePath,
			LineStart:    fs.lineIdx + 1,
			LineEnd:      bodyEnd + 1,
			Body:         body,
			Dependencies: callStats.Callees,
			CallCount:    callStats.CallCount,
			NestingDepth: callStats.NestingDepth,
		}
		allFuncs = append(allFuncs, f)
	}

	return allFuncs, nil
}

// makePythonCommentMask 标记注释行（# 及多行字符串中的行）
func makePythonCommentMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inTripleSingle := false
	inTripleDouble := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if inTripleSingle || inTripleDouble {
			mask[i] = true
			if inTripleSingle && strings.Contains(line, "'''") {
				// 检查是否结束（简单处理：只检查是否出现结束标记）
				// 但可能在同一行开始和结束，需要更精细处理
				idx := strings.Index(line, "'''")
				if idx >= 0 {
					remaining := line[idx+3:]
					if !strings.Contains(remaining, "'''") {
						inTripleSingle = false
					}
				}
			}
			if inTripleDouble && strings.Contains(line, `"""`) {
				idx := strings.Index(line, `"""`)
				if idx >= 0 {
					remaining := line[idx+3:]
					if !strings.Contains(remaining, `"""`) {
						inTripleDouble = false
					}
				}
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			mask[i] = true
			continue
		}

		// 检查多行字符串开始
		if strings.Contains(line, `"""`) {
			// 简单启发：如果出现奇数个 """
			count := strings.Count(line, `"""`)
			if count%2 == 1 {
				inTripleDouble = true
				mask[i] = true
				continue
			}
		}
		if strings.Contains(line, "'''") {
			count := strings.Count(line, "'''")
			if count%2 == 1 {
				inTripleSingle = true
				mask[i] = true
				continue
			}
		}
	}

	return mask
}

// makePythonStringMask 标记 Python 单行字符串（单引号/双引号内的行）
// 注意：三引号字符串已在 makePythonCommentMask 中处理，
// 但单行 "..." 和 '...' 中的函数调用（如 f"{x}"）需要被屏蔽，避免假阳性。
func makePythonStringMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	for i, line := range lines {
		inDouble := false
		inSingle := false
		escaped := false
		for _, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' && !inSingle {
				inDouble = !inDouble
			} else if ch == '\'' && !inDouble {
				inSingle = !inSingle
			}
		}
		// 如果行结束时空引号未闭合，说明这是跨行字符串（Python 中很少见，
		// 多行字符串通常用三引号），按未闭合标记
		mask[i] = inDouble || inSingle
	}
	return mask
}

// extractPythonCalls 提取 Python 函数调用（委托给通用提取器）
func extractPythonCalls(body string, callRegex *regexp.Regexp) []string {
	return extractCallsSimple(body, callRegex, func(name string) bool {
		return isPyKeyword(name) || name[0] == '_'
	})
}

// pyKeywords 是 Python 关键字和常用标准库名的集合（map 数据驱动）
var pyKeywords = map[string]bool{
	"if": true, "elif": true, "else": true, "for": true, "while": true, "with": true, "as": true,
	"try": true, "except": true, "finally": true, "raise": true, "return": true, "yield": true,
	"import": true, "from": true, "class": true, "def": true, "lambda": true, "pass": true,
	"break": true, "continue": true, "and": true, "or": true, "not": true, "in": true, "is": true,
	"True": true, "False": true, "None": true, "self": true, "cls": true, "super": true,
	"print": true, "len": true, "range": true, "int": true, "str": true, "float": true, "list": true,
	"dict": true, "set": true, "tuple": true, "type": true,
	"isinstance": true, "hasattr": true, "getattr": true, "setattr": true, "delattr": true,
	"open": true, "zip": true, "map": true, "filter": true, "sorted": true, "reversed": true,
	"enumerate": true, "iter": true, "next": true, "input": true, "format": true,
	"abs": true, "all": true, "any": true, "bin": true, "bool": true, "bytearray": true, "bytes": true,
	"callable": true, "chr": true, "classmethod": true, "compile": true, "complex": true,
	"dir": true, "divmod": true, "eval": true, "exec": true, "frozenset": true,
	"globals": true, "hash": true, "hex": true, "id": true, "issubclass": true,
	"locals": true, "max": true, "memoryview": true, "min": true, "object": true,
	"oct": true, "ord": true, "pow": true, "property": true, "repr": true,
	"round": true, "slice": true, "staticmethod": true, "sum": true, "vars": true,
	"__import__": true,
	// os 模块
	"abort": true, "chdir": true, "chmod": true, "chown": true, "cpu_count": true, "ctermid": true,
	"environ": true, "getcwd": true, "getenv": true, "getpid": true, "getppid": true,
	"kill": true, "listdir": true, "makedirs": true, "mkdir": true, "path": true,
	"readlink": true, "remove": true, "removedirs": true, "rename": true, "replace": true,
	"rmdir": true, "stat": true, "symlink": true, "system": true, "uname": true, "unlink": true, "walk": true,
	// json 模块
	"dump": true, "dumps": true, "load": true, "loads": true,
	// re 模块
	"search": true, "match": true, "findall": true, "finditer": true,
	"split": true, "sub": true, "subn": true, "fullmatch": true, "escape": true,
	// datetime 模块
	"now": true, "today": true, "utcnow": true, "fromtimestamp": true, "strftime": true, "strptime": true,
	"timedelta": true, "time": true, "date": true, "datetime": true,
	// math 模块
	"ceil": true, "comb": true, "copysign": true, "fabs": true, "factorial": true, "floor": true,
	"fmod": true, "frexp": true, "fsum": true, "gcd": true, "isclose": true, "isfinite": true,
	"isinf": true, "isnan": true, "isqrt": true, "ldexp": true, "modf": true, "nextafter": true,
	"perm": true, "prod": true, "remainder": true, "trunc": true,
	"exp": true, "log": true, "log2": true, "log10": true,
	"sqrt": true, "cos": true, "sin": true, "tan": true, "acos": true, "asin": true, "atan": true,
	"atan2": true, "cosh": true, "sinh": true, "tanh": true, "degrees": true, "radians": true,
	// subprocess 模块
	"run": true, "call": true, "check_call": true, "check_output": true, "Popen": true,
	// sys 模块
	"argv": true, "exit": true, "getdefaultencoding": true, "getrecursionlimit": true,
	"platform": true, "setrecursionlimit": true, "stdin": true, "stdout": true, "stderr": true,
	"version": true, "version_info": true,
}

// isPyKeyword 过滤 Python 关键字
func isPyKeyword(name string) bool {
	return pyKeywords[name]
}

// countIndent 计算行的缩进级别（按空格计）
func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4 // tab 视为 4 空格
		} else {
			break
		}
	}
	return count
}
