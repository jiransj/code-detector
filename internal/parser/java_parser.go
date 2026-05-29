package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// JavaParser 解析 Java/Kotlin 源文件
type JavaParser struct {
	language string // "java" 或 "kotlin"
}

func NewJavaParser() *JavaParser     { return &JavaParser{language: "java"} }
func NewKotlinParser() *JavaParser   { return &JavaParser{language: "kotlin"} }

func (p *JavaParser) Language() string { return p.language }

func (p *JavaParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})

	var vars []*model.GlobalVariable
	// 匹配 public static Type NAME = value; 等静态字段
	staticRegex := regexp.MustCompile(`(?:public|private|protected|static|final)\s+(?:static\s+)?(?:(?P<type>\w+(?:\<[^\>]*\>)?(?:\[\])*))\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		matches := staticRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := staticRegex.SubexpIndex("name")
		typeIdx := staticRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" {
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: p.language,
				FilePath: filePath, LineNum: i + 1,
				IsConst: strings.Contains(trimmed, "final"),
			})
		}
	}
	return vars, nil
}

// javaFuncRegex 匹配方法定义
var javaFuncRegex = regexp.MustCompile(
	`(?:(?:public|private|protected|static|final|abstract|synchronized|native|transient|volatile|strictfp|default)\s+)*(?:\w+(?:\[\])*(?:\<[^\>]*\>)?)\s+(?P<name>\w+)\s*\(`,
)

// kotlinFuncRegex 匹配 Kotlin fun 定义
var kotlinFuncRegex = regexp.MustCompile(
	`(?:fun\s+)(?P<name>\w+)\s*\(`,
)

// javaCallRegex 匹配 Java 方法调用
var javaCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *JavaParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)

	funcRegex := javaFuncRegex
	if p.language == "kotlin" {
		funcRegex = kotlinFuncRegex
	}

	type javaFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []javaFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 排除注解行、接口方法、抽象方法（没有方法体）
		if strings.HasPrefix(trimmed, "@") {
			continue
		}

		matches := funcRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := funcRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		starts = append(starts, javaFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))

	for _, fs := range starts {
		// 找到第一个 '{' 开始匹配
		offset := fl.LineOffset(fs.lineIdx)
		line := lines[fs.lineIdx]
		braceIdx := strings.Index(line, "{")
		if braceIdx < 0 {
			// 查找后续行的 {
			found := false
			for j := fs.lineIdx + 1; j < len(lines); j++ {
				if commentMask[j] {
					continue
				}
				if idx := strings.Index(lines[j], "{"); idx >= 0 {
					braceIdx = idx
					offset = fl.LineOffset(j) + idx
					found = true
					break
				}
			}
			if !found {
				continue
			}
		} else {
			offset += braceIdx
		}

		closeOffset, err := matchBrace(text, offset)
		if err != nil {
			continue
		}

		startLine := fs.lineIdx
		endLine := fl.LineFromOffset(closeOffset)

		bodyStart := fl.LineOffset(startLine)
		bodyEnd := closeOffset + 1
		if bodyEnd > len(text) {
			bodyEnd = len(text)
		}
		body := text[bodyStart:bodyEnd]

		callStats := extractCallStats(body, javaCallRegex, stringMask, commentMask, startLine, endLine, isJavaKeyword, nil)

		f := &model.Function{
			Name:         fs.name,
			Language:     p.language,
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

// isJavaKeyword 过滤 Java/Kotlin 关键字、常见类型
func isJavaKeyword(name string) bool {
	switch name {
	case "if", "else", "for", "while", "do",
		"switch", "case", "default", "break",
		"continue", "return", "throw", "try",
		"catch", "finally", "new", "this",
		"super", "instanceof", "synchronized",
		"int", "long", "double", "float",
		"boolean", "char", "byte", "short",
		"void", "String", "Integer", "Boolean",
		"Double", "Float", "Long", "Character",
		"Byte", "Short", "Object", "Class",
		"System", "null", "true", "false",
		"print", "println", "printf",
		"override", "Deprecated", "SuppressWarnings":
		return true
	}
	return false
}
