package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// CSharpParser 解析 C# 源文件
type CSharpParser struct{}

func NewCSharpParser() *CSharpParser { return &CSharpParser{} }
func (p *CSharpParser) Language() string { return "csharp" }

func (p *CSharpParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//", "///"}, [][2]string{{"/*", "*/"}})

	var vars []*model.GlobalVariable
	csStaticRegex := regexp.MustCompile(`(?:public|private|protected|internal|static|readonly|const|volatile)\s+(?:static\s+)?(?:(?P<type>\w+(?:\<[^\>]*\>)?(?:\[\])*))\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		matches := csStaticRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := csStaticRegex.SubexpIndex("name")
		typeIdx := csStaticRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" {
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: "csharp",
				FilePath: filePath, LineNum: i + 1,
				IsConst: strings.Contains(trimmed, "const "),
			})
		}
	}
	return vars, nil
}

var csFuncRegex = regexp.MustCompile(
	`(?:(?:public|private|protected|internal|static|virtual|override|abstract|async|unsafe|sealed|readonly|partial|new|extern)\s+)*(?:\w+(?:\[\])*(?:\<[^\>]+\>)?)\s+(?P<name>\w+)\s*\(`,
)
var csCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *CSharpParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//", "///"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)

	type csFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []csFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
			continue
		}

		matches := csFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := csFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		starts = append(starts, csFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
		braceOffset := -1
		for j := fs.lineIdx; j < len(lines); j++ {
			line := lines[j]
			idx := strings.Index(line, "{")
			if idx >= 0 && !commentMask[j] {
				braceOffset = fl.LineOffset(j) + idx
				break
			}
		}
		if braceOffset < 0 {
			continue
		}

		closeOffset, err := matchBrace(text, braceOffset)
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

		callStats := extractCallStats(body, csCallRegex, stringMask, commentMask, startLine, endLine, isCSKeyword, nil)

		f := &model.Function{
			Name:         fs.name,
			Language:     "csharp",
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

// csKeywords 是 C# 关键字和常用标准库名的集合（map 数据驱动）
var csKeywords = map[string]bool{
	"if": true, "else": true, "for": true, "foreach": true, "while": true, "do": true, "switch": true, "case": true,
	"default": true, "break": true, "continue": true, "return": true, "throw": true, "try": true, "catch": true, "finally": true,
	"new": true, "this": true, "base": true, "as": true, "is": true, "in": true, "out": true, "ref": true, "sizeof": true,
	"typeof": true, "nameof": true, "checked": true, "unchecked": true, "unsafe": true, "fixed": true, "stackalloc": true,
	"await": true, "async": true, "yield": true, "from": true, "select": true, "where": true, "join": true, "group": true,
	"orderby": true, "let": true, "ascending": true, "descending": true, "equals": true, "by": true, "on": true, "into": true,
	"int": true, "long": true, "double": true, "float": true, "bool": true, "char": true, "byte": true, "short": true,
	"uint": true, "ulong": true, "ushort": true, "sbyte": true, "decimal": true, "string": true, "object": true, "void": true,
	"null": true, "true": true, "false": true, "var": true, "dynamic": true, "class": true, "struct": true, "enum": true,
	"interface": true, "record": true, "namespace": true, "using": true, "partial": true, "sealed": true, "static": true,
	"virtual": true, "override": true, "abstract": true, "public": true, "private": true, "protected": true, "internal": true,
	"readonly": true, "volatile": true, "const": true, "event": true, "delegate": true, "add": true, "remove": true,
	"set": true, "get": true, "value": true, "params": true, "implicit": true, "explicit": true, "operator": true,
	"Console": true, "WriteLine": true, "Write": true, "ReadLine": true, "Read": true, "Convert": true, "Math": true,
	"String": true, "Task": true, "List": true, "Dictionary": true, "IEnumerable": true,
	"Array": true, "Buffer": true, "Byte": true, "Char": true, "DateTime": true, "Decimal": true,
	"Double": true, "Enum": true, "Environment": true, "Exception": true, "Guid": true,
	"Int16": true, "Int32": true, "Int64": true, "IntPtr": true, "Lazy": true, "Nullable": true,
	"Object": true, "Random": true, "Stream": true, "StringBuilder": true,
	"TextReader": true, "TextWriter": true, "TimeSpan": true, "Tuple": true, "Type": true, "Uri": true, "Version": true,
	"File": true, "FileInfo": true, "Directory": true, "DirectoryInfo": true, "Path": true,
	"StreamReader": true, "StreamWriter": true, "FileStream": true, "MemoryStream": true,
	"Open": true, "OpenRead": true, "OpenWrite": true, "Create": true, "CreateText": true,
	"ReadAllText": true, "ReadAllLines": true, "ReadAllBytes": true,
	"WriteAllText": true, "WriteAllLines": true, "WriteAllBytes": true,
	"AppendAllText": true, "AppendAllLines": true, "Copy": true, "Delete": true,
	"Exists": true, "Move": true, "GetFiles": true, "GetDirectories": true,
	"Select": true, "Where": true, "OrderBy": true, "OrderByDescending": true,
	"ThenBy": true, "ThenByDescending": true, "GroupBy": true, "Join": true,
	"Skip": true, "Take": true, "SkipWhile": true, "TakeWhile": true,
	"First": true, "FirstOrDefault": true, "Last": true, "LastOrDefault": true,
	"Single": true, "SingleOrDefault": true, "Any": true, "All": true, "Count": true,
	"Min": true, "Max": true, "Sum": true, "Average": true, "Aggregate": true,
	"Distinct": true, "Union": true, "Intersect": true, "Except": true,
	"ToList": true, "ToArray": true, "ToDictionary": true, "ToLookup": true,
	"Run": true, "Start": true, "Wait": true, "Result": true, "ConfigureAwait": true,
	"Delay": true, "WhenAll": true, "WhenAny": true, "FromResult": true,
	"Ok": true, "BadRequest": true, "NotFound": true, "Redirect": true, "View": true,
	"Content": true, "Json": true, "PhysicalFile": true,
	"ModelState": true, "IsValid": true, "TryValidateModel": true,
	"ToString": true, "Equals": true, "GetHashCode": true, "GetType": true,
	"CompareTo": true, "Contains": true, "StartsWith": true, "EndsWith": true,
	"IndexOf": true, "LastIndexOf": true, "Substring": true, "Replace": true,
	"Split": true, "Trim": true, "TrimStart": true, "TrimEnd": true, "ToLower": true,
	"ToUpper": true, "ToCharArray": true, "PadLeft": true, "PadRight": true,
	"Format": true, "Concat": true, "Empty": true, "IsNullOrEmpty": true,
	"IsNullOrWhiteSpace": true, "Append": true, "AppendLine": true,
	"Clear": true, "Remove": true, "Insert": true, "CopyTo": true,
}

func isCSKeyword(name string) bool {
	return csKeywords[name]
}
