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
	qSelCall    = `(call_expression function: (selector_expression (field_identifier) @callee)) @call`
	qPkg        = `(source_file (package_clause (package_identifier) @pkg))`
	qVar        = `(var_declaration (var_spec name: (identifier) @name type: (_)? @type)) @decl`
	qConst      = `(const_declaration (const_spec name: (identifier) @name type: (_)? @type)) @decl`
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
	tsEachTopLevel(root, qVar, content, func(name, typeStr string) {
		results = append(results, &model.GlobalVariable{
			Name: name, VarType: typeStr, Language: "go",
			PackageName: pkgName, Visibility: visibilityFromName(name),
			FilePath: filepath.ToSlash(filePath), IsConst: false,
		})
	})
	tsEachTopLevel(root, qConst, content, func(name, typeStr string) {
		results = append(results, &model.GlobalVariable{
			Name: name, VarType: typeStr, Language: "go",
			PackageName: pkgName, Visibility: visibilityFromName(name),
			FilePath: filepath.ToSlash(filePath), IsConst: true,
		})
	})

	return results, nil
}

// visibilityFromName 根据首字母判断可见性
func visibilityFromName(name string) string {
	if len(name) == 0 {
		return "private"
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return "public"
	}
	return "private"
}

// tsEachTopLevel 只匹配 source_file 直接子级的 var/const 声明（排除局部变量）
func tsEachTopLevel(root *sitter.Node, queryStr string, content []byte, fn func(name, typeStr string)) {
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

		var name, typeStr string
		var isTopLevel bool
		for _, c := range m.Captures {
			if c.Node == nil {
				continue
			}
			switch q.CaptureNameForId(c.Index) {
			case "decl":
				parent := c.Node.Parent()
				isTopLevel = parent != nil && parent.Type() == "source_file"
			case "name":
				name = strings.TrimSpace(c.Node.Content(content))
			case "type":
				typeStr = strings.TrimSpace(c.Node.Content(content))
			}
		}
		if name != "" && isTopLevel {
			fn(name, typeStr)
		}
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
				for _, c := range m.Captures {
					if q2.CaptureNameForId(c.Index) == "callee" && c.Node != nil {
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

	// 从 AST 提取参数/返回值/接收器
	tsExtractFuncSignature(name, root, queryStr, content, lang, &params, &returnTypes, &receiver)

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
		ParameterCount:  tsCountParams(params),
		ReturnCount:     returnCount,
		StatementCount:  statementCount,
		AnonymousFuncs:  anonCount,
	}
}

// tsExtractFuncSignature 从 AST 提取函数参数、返回类型和接收器
func tsExtractFuncSignature(name string, root *sitter.Node, queryStr string, content []byte, lang *sitter.Language, params, returnTypes, receiver *string) {
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
		// 遍历子节点找 parameters / result / receiver
		for i := 0; i < int(funcNode.ChildCount()); i++ {
			child := funcNode.Child(i)
			if child == nil {
				continue
			}
			switch child.Type() {
			case "parameter_list":
				if *params == "" {
					*params = child.Content(content)
				}
			case "receiver":
				// 方法接收器: 取其内部的 parameter_list
				for j := 0; j < int(child.ChildCount()); j++ {
					grand := child.Child(j)
					if grand != nil && grand.Type() == "parameter_list" {
						*receiver = grand.Content(content)
						break
					}
				}
			default:
				// 可能为返回类型
				if child.Type() == "type_identifier" || child.Type() == "pointer_type" ||
					child.Type() == "qualified_type" || child.Type() == "generic_type" ||
					child.Type() == "function_type" || child.Type() == "interface_type" ||
					child.Type() == "array_type" || child.Type() == "slice_type" ||
					child.Type() == "map_type" || child.Type() == "channel_type" ||
					child.Type() == "struct_type" || child.Type() == "named_type" {
					if *returnTypes == "" {
						*returnTypes = child.Content(content)
					} else {
						*returnTypes = *returnTypes + ", " + child.Content(content)
					}
				}
			}
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

// tsCountParams 从参数字符串统计参数个数
func tsCountParams(params string) int {
	if params == "" || params == "()" {
		return 0
	}
	// 简单的括号内逗号计数
	inner := strings.TrimSpace(params[1 : len(params)-1])
	if inner == "" {
		return 0
	}
	return strings.Count(inner, ",") + 1
}
