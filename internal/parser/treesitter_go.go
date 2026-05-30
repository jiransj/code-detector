package parser

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"code-detector/internal/model"
)

// TreeSitterGoParser 基于 tree-sitter AST 的 Go 语言解析器
type TreeSitterGoParser struct{}

func NewTreeSitterGoParser() *TreeSitterGoParser { return &TreeSitterGoParser{} }
func (p *TreeSitterGoParser) Language() string   { return "go" }

var tsCtx = context.Background()

// parserPool 复用 sitter.Parser 对象，避免每次解析重复 CGo 分配与销毁
var parserPool = sync.Pool{
	New: func() interface{} {
		p := sitter.NewParser()
		if p == nil {
			return nil
		}
		p.SetLanguage(golang.GetLanguage())
		return p
	},
}

// 缓存预编译的 tree-sitter 查询，避免每次 Parse 重复 CGo 分配
var (
	cachedQFmtDecl    *sitter.Query
	cachedQMethodDecl *sitter.Query
	cachedQCall       *sitter.Query
	cachedQSelCall    *sitter.Query
	cachedQPkg        *sitter.Query
	cachedQVar        *sitter.Query
	cachedQConst      *sitter.Query
)

var initQueriesOnce sync.Once

func initQueries() {
	lang := golang.GetLanguage()
	cachedQFmtDecl, _ = sitter.NewQuery([]byte(qFuncDecl), lang)
	cachedQMethodDecl, _ = sitter.NewQuery([]byte(qMethodDecl), lang)
	cachedQCall, _ = sitter.NewQuery([]byte(qCall), lang)
	cachedQSelCall, _ = sitter.NewQuery([]byte(qSelCall), lang)
	cachedQPkg, _ = sitter.NewQuery([]byte(qPkg), lang)
	cachedQVar, _ = sitter.NewQuery([]byte(qVar), lang)
	cachedQConst, _ = sitter.NewQuery([]byte(qConst), lang)
}

// getCachedQuery 根据查询字符串返回缓存的 Query 对象
func getCachedQuery(queryStr string) *sitter.Query {
	initQueriesOnce.Do(initQueries)
	switch queryStr {
	case qFuncDecl:
		return cachedQFmtDecl
	case qMethodDecl:
		return cachedQMethodDecl
	case qCall:
		return cachedQCall
	case qSelCall:
		return cachedQSelCall
	case qPkg:
		return cachedQPkg
	case qVar:
		return cachedQVar
	case qConst:
		return cachedQConst
	}
	return nil
}

// 树-sitter 查询字符串
const (
	qFuncDecl   = `(function_declaration name: (identifier) @name body: (block) @body) @func`
	qMethodDecl = `(method_declaration name: (field_identifier) @name body: (block) @body) @func`
	qCall       = `(call_expression function: (identifier) @callee) @call`
	qSelCall    = `(call_expression function: (selector_expression (field_identifier) @callee) @qualified) @call`
	qPkg        = `(source_file (package_clause (package_identifier) @pkg))`
	qVar        = `(var_declaration (var_spec name: (identifier) @name type: (_)? @type) @spec) @decl`
	qConst      = `(const_declaration (const_spec name: (identifier) @name type: (_)? @type) @spec) @decl`
)

func (p *TreeSitterGoParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	root, lang, err := tsParseRoot(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}

	pkgName := tsFirst(root, qPkg, "pkg", content)
	results := make([]*model.Function, 0)

	// 顶层函数 + 方法
	tsEach(root, qFuncDecl, content, func(name, body, fullText string) {
		if fn := tsMakeFunc(name, body, fullText, pkgName, filePath, root, qFuncDecl, content, lang); fn != nil {
			results = append(results, fn)
		}
	})
	tsEach(root, qMethodDecl, content, func(name, body, fullText string) {
		if fn := tsMakeFunc(name, body, fullText, pkgName, filePath, root, qMethodDecl, content, lang); fn != nil {
			results = append(results, fn)
		}
	})
	return results, nil
}

func (p *TreeSitterGoParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	root, _, err := tsParseRoot(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}

	pkgName := tsFirst(root, qPkg, "pkg", content)
	var results []*model.GlobalVariable

	// 只提取顶层 var/const（parent == source_file），排除函数体内的局部变量
	tsEachTopLevel(root, qVar, content, func(name, typeStr string, lineNum int, valueNode *sitter.Node) {
		if typeStr == "" {
			typeStr = inferTypeFromValue(valueNode, content)
		}
		results = append(results, &model.GlobalVariable{
			Name: name, VarType: typeStr, Language: "go",
			PackageName: pkgName, Visibility: visibilityFromName(name),
			FilePath: filepath.ToSlash(filePath), LineNum: lineNum, IsConst: false,
		})
	})
	tsEachTopLevel(root, qConst, content, func(name, typeStr string, lineNum int, valueNode *sitter.Node) {
		if typeStr == "" {
			typeStr = inferTypeFromValue(valueNode, content)
		}
		results = append(results, &model.GlobalVariable{
			Name: name, VarType: typeStr, Language: "go",
			PackageName: pkgName, Visibility: visibilityFromName(name),
			FilePath: filepath.ToSlash(filePath), LineNum: lineNum, IsConst: true,
		})
	})

	return results, nil
}

// tsEachTopLevel 只匹配 source_file 直接子级的 var/const 声明（排除局部变量）
// 注意：一个 match 中可能包含多个 name/type 对（如 var (a int; b string)），
// 因此需要收集所有 (name, type, line, value) 四元组并逐个回调。
func tsEachTopLevel(root *sitter.Node, queryStr string, content []byte, fn func(name, typeStr string, lineNum int, valueNode *sitter.Node)) {
	q := getCachedQuery(queryStr)
	if q == nil {
		return
	}

	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return
	}
	defer cursor.Close()

	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var isTopLevel bool
		// 一个 match 中可能有多个 name/type/line 组合（grouped var/const）
		type quad struct {
			name      string
			typeStr   string
			lineNum   int
			valueNode *sitter.Node
		}
		var pairs []quad
		for _, c := range m.Captures {
			if c.Node == nil {
				continue
			}
			switch q.CaptureNameForId(c.Index) {
			case "decl":
				parent := c.Node.Parent()
				isTopLevel = parent != nil && parent.Type() == "source_file"
			case "name":
				// 从 name 节点的父节点（var_spec / const_spec）查找 value 子节点
				// 使用 tree-sitter 原生 field "value" 直接定位
				var valueNode *sitter.Node
				if parentSpec := c.Node.Parent(); parentSpec != nil {
					valueNode = parentSpec.ChildByFieldName("value")
				}
				pairs = append(pairs, quad{
					name:      strings.TrimSpace(c.Node.Content(content)),
					lineNum:   int(c.Node.StartPoint().Row) + 1,
					valueNode: valueNode,
				})
			case "type":
				if len(pairs) > 0 {
					pairs[len(pairs)-1].typeStr = strings.TrimSpace(c.Node.Content(content))
				}
			}
		}
		if isTopLevel {
			for _, p := range pairs {
				if p.name != "" {
					fn(p.name, p.typeStr, p.lineNum, p.valueNode)
				}
			}
		}
	}
}

// findValueChild 使用 tree-sitter 原生 field "value" 获取变量初始值表达式节点
// Go grammar: var_spec / const_spec 都有 value: (_)? 字段
func findValueChild(specNode *sitter.Node) *sitter.Node {
	if specNode == nil {
		return nil
	}
	return specNode.ChildByFieldName("value")
}

// inferTypeFromValue 当变量没有显式类型标注时，从 value 节点推断类型
// 先尝试 AST 节点类型推断，失败后回退到源码文本分析
func inferTypeFromValue(valueNode *sitter.Node, content []byte) string {
	if valueNode == nil {
		return ""
	}
	// 方法1: 从 value 源码文本推断
	valText := valueNode.Content(content)

	// 字符串
	if len(valText) >= 2 && (valText[0] == '"' || valText[0] == '`') {
		if valText[0] == '`' || valText[len(valText)-1] == '"' {
			return "string"
		}
	}
	// 布尔
	if valText == "true" || valText == "false" {
		return "bool"
	}
	// nil
	if valText == "nil" {
		return "untyped nil"
	}
	// 数字（int 或 float）
	if len(valText) > 0 && (valText[0] >= '0' && valText[0] <= '9') {
		isFloat := false
		for _, ch := range valText {
			if ch == '.' || ch == 'e' || ch == 'E' {
				isFloat = true
				break
			}
		}
		if isFloat {
			return "float64"
		}
		return "int"
	}
	// 复合字面量: Type{...}
	if len(valText) > 0 && valText[len(valText)-1] == '}' {
		// 提取 { 之前的类型名
		if braceIdx := strings.IndexByte(valText, '{'); braceIdx > 0 {
			typePart := strings.TrimSpace(valText[:braceIdx])
			// 去除 & 前缀 (如 &Type{})
			typePart = strings.TrimPrefix(typePart, "&")
			if typePart != "" {
				return typePart
			}
		}
	}
	// 函数调用: 提取函数名作为类型提示
	if parenIdx := strings.IndexByte(valText, '('); parenIdx > 0 {
		funcName := strings.TrimSpace(valText[:parenIdx])
		if funcName != "" {
			return funcName
		}
	}

	// 方法2: 展开 expression 包装后 AST 节点类型推断
	n := valueNode
	for n != nil && n.ChildCount() > 0 {
		if n.Type() == "expression" {
			n = n.Child(0)
			continue
		}
		break
	}
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "interpreted_string_literal", "raw_string_literal":
		return "string"
	case "int_literal":
		return "int"
	case "float_literal":
		return "float64"
	case "imaginary_literal":
		return "complex128"
	case "boolean_literal", "true", "false":
		return "bool"
	case "nil":
		return "untyped nil"
	case "composite_literal":
		// 使用 tree-sitter 原生 field "type" 获取复合字面量的类型名
		if typeNode := n.ChildByFieldName("type"); typeNode != nil {
			return typeNode.Content(content)
		}
		return ""
	case "call_expression":
		// 使用 tree-sitter 原生 field "function" 获取函数名
		if funcNode := n.ChildByFieldName("function"); funcNode != nil {
			return funcNode.Content(content)
		}
		return ""
	case "type_conversion_expression":
		// 使用 tree-sitter 原生 field "type" 获取转换目标类型
		if typeNode := n.ChildByFieldName("type"); typeNode != nil {
			return typeNode.Content(content)
		}
		return ""
	case "unary_expression":
		if n.ChildCount() >= 2 {
			operand := n.Child(int(n.ChildCount()) - 1)
			return inferTypeFromValue(operand, content)
		}
		return ""
	case "binary_expression":
		return "bool"
	case "function_literal":
		return "func"
	case "slice_literal":
		return "[]int"
	default:
		return ""
	}
}

// ─── 树操作 ──────────────────────────────────────────

func tsParseRoot(content []byte) (*sitter.Node, *sitter.Language, error) {
	p := parserPool.Get().(*sitter.Parser)
	if p == nil {
		return nil, nil, fmt.Errorf("NewParser failed")
	}
	defer parserPool.Put(p)

	tree, err := p.ParseCtx(tsCtx, nil, content)
	if err != nil || tree == nil {
		return nil, nil, fmt.Errorf("parse: %v", err)
	}
	root := tree.RootNode()
	if root == nil {
		tree.Close()
		return nil, nil, fmt.Errorf("nil root")
	}
	return root, golang.GetLanguage(), nil
}

func tsEach(root *sitter.Node, queryStr string, content []byte, fn func(name, body, fullText string)) {
	q := getCachedQuery(queryStr)
	if q == nil {
		return
	}

	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return
	}
	defer cursor.Close()

	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var name, body, fullText string
		for _, c := range m.Captures {
			if c.Node == nil {
				continue
			}
			switch q.CaptureNameForId(c.Index) {
			case "name":
				name = strings.TrimSpace(c.Node.Content(content))
			case "body":
				body = c.Node.Content(content)
			case "func", "decl":
				fullText = c.Node.Content(content)
			case "type":
				if body == "" {
					body = strings.TrimSpace(c.Node.Content(content))
				}
			}
		}
		if name != "" && fullText != "" {
			fn(name, body, fullText)
		}
	}
}

func tsFirst(root *sitter.Node, queryStr, capName string, content []byte) string {
	q := getCachedQuery(queryStr)
	if q == nil {
		return ""
	}
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return ""
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	m, ok := cursor.NextMatch()
	if !ok {
		return ""
	}
	for _, c := range m.Captures {
		if q.CaptureNameForId(c.Index) == capName && c.Node != nil {
			return strings.TrimSpace(c.Node.Content(content))
		}
	}
	return ""
}

// tsFindFuncNode 在已执行的查询游标中找当前函数的完整节点（用于获取行号）
// 由于 tsEach 已经消耗了游标，我们单独执行一次查询来定位行号
func tsFindLine(name string, root *sitter.Node, queryStr string, content []byte) (int, int) {
	q := getCachedQuery(queryStr)
	if q == nil {
		return 0, 0
	}
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return 0, 0
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var foundName string
		var funcNode *sitter.Node
		for _, c := range m.Captures {
			switch q.CaptureNameForId(c.Index) {
			case "name":
				if c.Node != nil {
					foundName = strings.TrimSpace(c.Node.Content(content))
				}
			case "func":
				funcNode = c.Node
			}
		}
		if foundName == name && funcNode != nil {
			return int(funcNode.StartPoint().Row) + 1, int(funcNode.EndPoint().Row) + 1
		}
	}
	return 0, 0
}

// ─── 调用分析 ───────────────────────────────────────

func tsAnalyzeCalls(bodyNode *sitter.Node, content []byte, lang *sitter.Language) *model.CallStats {
	stats := &model.CallStats{}
	if bodyNode == nil {
		return stats
	}
	seen := make(map[string]bool)

	// 1) 普通调用 foo()（使用缓存查询）
	q := getCachedQuery(qCall)
	if q != nil {
		cursor := sitter.NewQueryCursor()
		if cursor != nil {
			defer cursor.Close()
			cursor.Exec(q, bodyNode)
			for {
				m, ok := cursor.NextMatch()
				if !ok {
					break
				}
				for _, c := range m.Captures {
					if q.CaptureNameForId(c.Index) == "callee" && c.Node != nil {
						callee := strings.TrimSpace(c.Node.Content(content))
						if callee != "" && !goKeywords[callee] {
							if !seen[callee] {
								stats.Callees = append(stats.Callees, callee)
								seen[callee] = true
							}
							stats.CallCount++
						}
					}
				}
			}
		}
	}

	// 2) selector 调用 obj.method()（使用缓存查询）
	q2 := getCachedQuery(qSelCall)
	if q2 != nil {
		cursor2 := sitter.NewQueryCursor()
		if cursor2 != nil {
			defer cursor2.Close()
			cursor2.Exec(q2, bodyNode)
			for {
				m, ok := cursor2.NextMatch()
				if !ok {
					break
				}
				calleeName := ""
				qualifiedName := ""
				for _, c := range m.Captures {
					switch q2.CaptureNameForId(c.Index) {
					case "callee":
						if c.Node != nil {
							calleeName = strings.TrimSpace(c.Node.Content(content))
						}
					case "qualified":
						if c.Node != nil {
							qualifiedName = strings.TrimSpace(c.Node.Content(content))
						}
					}
				}
				// 添加简单名（如 "New"）
				if calleeName != "" && !goKeywords[calleeName] {
					if !seen[calleeName] {
						stats.Callees = append(stats.Callees, calleeName)
						seen[calleeName] = true
					}
					stats.CallCount++
				}
				// 额外添加完整限定名（如 "fscanner.New"），让 BuildCallGraph 能直配
				// 同时过滤掉限定名中的部分（如 "fmt.Sprintf" 中 Sprintf 是关键字）
				lastDot := strings.LastIndex(qualifiedName, ".")
				qualFunc := qualifiedName
				if lastDot >= 0 {
					qualFunc = qualifiedName[lastDot+1:]
				}
				if qualifiedName != "" && qualifiedName != calleeName &&
					!goKeywords[qualifiedName] && !goKeywords[qualFunc] {
					if !seen[qualifiedName] {
						stats.Callees = append(stats.Callees, qualifiedName)
						seen[qualifiedName] = true
					}
				}
			}
		}
	}

	// 3) 嵌套深度（内联计算，消除独立递归函数调用）
	stats.NestingDepth = tsGoNestingDepth(bodyNode)
	return stats
}

// tsGoNestingDepth 递归计算函数体内的调用嵌套最大深度
func tsGoNestingDepth(node *sitter.Node) int {
	if node == nil {
		return 0
	}
	maxDepth := 0
	var walk func(n *sitter.Node, depth int)
	walk = func(n *sitter.Node, depth int) {
		if n == nil {
			return
		}
		if n.Type() == "call_expression" {
			if depth > maxDepth {
				maxDepth = depth
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i), depth+1)
			}
		} else {
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i), depth)
			}
		}
	}
	walk(node, 0)
	return maxDepth
}

// tsFindBodyNode 根据函数名从 root 中找到 body node
func tsFindBodyNode(name string, root *sitter.Node, queryStr string, content []byte) *sitter.Node {
	q := getCachedQuery(queryStr)
	if q == nil {
		return nil
	}
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return nil
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var foundName string
		var bodyNode *sitter.Node
		for _, c := range m.Captures {
			switch q.CaptureNameForId(c.Index) {
			case "name":
				if c.Node != nil {
					foundName = strings.TrimSpace(c.Node.Content(content))
				}
			case "body":
				bodyNode = c.Node
			}
		}
		if foundName == name {
			return bodyNode
		}
	}
	return nil
}

// ─── 构建 model ──────────────────────────────────────

func tsMakeFunc(name, body, fullText, pkgName, filePath string, root *sitter.Node, queryStr string, content []byte, lang *sitter.Language) *model.Function {
	if name == "" || fullText == "" {
		return nil
	}

	// 行号
	lineStart, lineEnd := 0, 0
	q, _ := sitter.NewQuery([]byte(queryStr), lang)
	if q != nil {
		defer q.Close()
		cursor := sitter.NewQueryCursor()
		if cursor != nil {
			defer cursor.Close()
			cursor.Exec(q, root)
			for {
				m, ok := cursor.NextMatch()
				if !ok {
					break
				}
				var foundName string
				var funcNode *sitter.Node
				for _, c := range m.Captures {
					switch q.CaptureNameForId(c.Index) {
					case "name":
						if c.Node != nil {
							foundName = strings.TrimSpace(c.Node.Content(content))
						}
					case "func":
						funcNode = c.Node
					}
				}
				if foundName == name && funcNode != nil {
					lineStart = int(funcNode.StartPoint().Row) + 1
					lineEnd = int(funcNode.EndPoint().Row) + 1
					break
				}
			}
		}
	}

	// 调用分析
	bodyNode := tsFindBodyNode(name, root, queryStr, content)
	stats := tsAnalyzeCalls(bodyNode, content, lang)

	// ═══ AST 增强提取 ═══
	isMethod := strings.HasPrefix(queryStr, "(method_declaration")
	receiver := ""
	params := ""
	returnTypes := ""
	visibility := "public"
	if len(name) > 0 && name[0] >= 'a' && name[0] <= 'z' {
		visibility = "private"
	}

	// 从 AST 提取参数/返回值/接收器 + 参数个数（AST 原生计数）
	paramCount := 0
	tsExtractFuncSignature(name, root, queryStr, content, lang, &params, &returnTypes, &receiver, &paramCount)

	// 复杂度分析
	cyclomatic := 0
	returnCount := 0
	statementCount := 0
	anonCount := 0
	if bodyNode != nil {
		cyclomatic = tsCountCyclomatic(bodyNode)
		returnCount = tsCountNodeType(bodyNode, "return_statement")
		statementCount = tsCountStatements(bodyNode)
		anonCount = tsCountNodeType(bodyNode, "function_literal")
	}

	return &model.Function{
		Name:            name,
		PackageName:     pkgName,
		Language:        "go",
		FilePath:        filepath.ToSlash(filePath),
		LineStart:       lineStart,
		LineEnd:         lineEnd,
		Body:            fullText,
		Dependencies:    stats.Callees,
		CallCount:       stats.CallCount,
		NestingDepth:    stats.NestingDepth,
		// AST 字段
		Parameters:      params,
		ReturnTypes:     returnTypes,
		Receiver:        receiver,
		IsMethod:        isMethod,
		Visibility:      visibility,
		Cyclomatic:      cyclomatic,
		ParameterCount:  paramCount,
		ReturnCount:     returnCount,
		StatementCount:  statementCount,
		AnonymousFuncs:  anonCount,
	}
}

// tsExtractFuncSignature 从 AST 提取函数参数、返回类型和接收器
// 使用 tree-sitter 原生 field 名：parameters / result / receiver
func tsExtractFuncSignature(name string, root *sitter.Node, queryStr string, content []byte, lang *sitter.Language, params, returnTypes, receiver *string, paramCount *int) {
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil || q == nil {
		return
	}
	defer q.Close()
	cursor := sitter.NewQueryCursor()
	if cursor == nil {
		return
	}
	defer cursor.Close()
	cursor.Exec(q, root)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var foundName string
		var funcNode *sitter.Node
		for _, c := range m.Captures {
			switch q.CaptureNameForId(c.Index) {
			case "name":
				if c.Node != nil {
					foundName = strings.TrimSpace(c.Node.Content(content))
				}
			case "func":
				funcNode = c.Node
			}
		}
		if foundName != name || funcNode == nil {
			continue
		}

		// 使用 tree-sitter 原生 field 名提取签名信息
		// Go grammar: function_declaration/method_declaration 定义了
		//   parameters: (parameter_list)
		//   result: (_)?           ← 返回值节点（单值或 parameter_list）
		//   receiver: (parameter_list)  ← 仅 method_declaration

		if paramNode := funcNode.ChildByFieldName("parameters"); paramNode != nil {
			*params = paramNode.Content(content)
			// 使用 AST 原生 NamedChildCount 计数参数个数
			// 每个 parameter_declaration 是一个 named child
			*paramCount = int(paramNode.NamedChildCount())
		}
		if resultNode := funcNode.ChildByFieldName("result"); resultNode != nil {
			*returnTypes = resultNode.Content(content)
		}
		if recvNode := funcNode.ChildByFieldName("receiver"); recvNode != nil {
			// 接收器本身是 parameter_list，取其内部文本
			*receiver = recvNode.Content(content)
		}
		break
	}
}

// tsCountCyclomatic 计算圈复杂度（if/else/for/range/switch/case/&&/||）
func tsCountCyclomatic(node *sitter.Node) int {
	if node == nil {
		return 0
	}
	count := 1 // 基准复杂度
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "if_statement", "else_statement", "for_statement", "range_clause",
			"switch_statement", "select_statement", "case_clause", "communication_case",
			"short_circuit":
			count++
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return count
}

// tsCountNodeType 统计 AST 中指定类型的节点数
func tsCountNodeType(node *sitter.Node, nodeType string) int {
	if node == nil {
		return 0
	}
	count := 0
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == nodeType {
			count++
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return count
}

// tsCountStatements 统计函数体中的语句数
func tsCountStatements(node *sitter.Node) int {
	if node == nil {
		return 0
	}
	count := 0
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "expression_statement" || n.Type() == "return_statement" ||
			n.Type() == "short_var_declaration" || n.Type() == "var_declaration" ||
			n.Type() == "assign_statement" || n.Type() == "if_statement" ||
			n.Type() == "for_statement" || n.Type() == "switch_statement" ||
			n.Type() == "select_statement" || n.Type() == "defer_statement" ||
			n.Type() == "go_statement" || n.Type() == "send_statement" ||
			n.Type() == "inc_statement" || n.Type() == "dec_statement" ||
			n.Type() == "label_definition" || n.Type() == "type_declaration" ||
			n.Type() == "block" {
			count++
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return count
}

// tsCountParams 已废弃 —— 改用 tree-sitter AST 原生 ChildByFieldName("parameters").NamedChildCount()
// 保留该函数仅用于兼容外部调用，直接返回 0 表示不应再被使用
func tsCountParams(params string) int { return 0 }
