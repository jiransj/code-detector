package analyzer

import (
	"fmt"
	"sort"

	"code-detector/internal/db"
	"code-detector/internal/model"
)

// Analyzer 依赖分析器：交叉引用索引
type Analyzer struct {
	Store *db.Store
}

// New 创建分析器
func New(store *db.Store) *Analyzer {
	return &Analyzer{Store: store}
}

// CallGraph 调用图
type CallGraph struct {
	// Nodes 按限定名索引: "pkg.Name" 或 "Name"（无包名时）
	Nodes map[string]*FuncNode
	// IDIndex 按函数 ID 索引
	IDIndex map[int64]*FuncNode
	// plainNameIndex 按普通函数名索引（用于依赖匹配回退）
	plainNameIndex map[string][]*FuncNode
}

// FuncNode 调用图节点
type FuncNode struct {
	Function  *model.Function
	Qualified string   // 限定名: "pkg.Name" 或 "Name"
	Callers   []string // 调用此函数的函数名（限定名）
	Callees   []string // 此函数调用的函数名（原始文本中的名字）
}

// qualifiedName 生成函数的限定名
func qualifiedName(f *model.Function) string {
	if f.PackageName != "" {
		return fmt.Sprintf("%s.%s", f.PackageName, f.Name)
	}
	return f.Name
}

// BuildCallGraph 构建完整调用图（使用限定名去重）
func (a *Analyzer) BuildCallGraph(sessionID int64) (*CallGraph, error) {
	// 查询该 session 的所有函数（含 package_name）
	rows, err := a.Store.DB.Query(
		`SELECT id, name, package_name, language, file_path, line_start, line_end, body, call_count, nesting_depth
		 FROM functions WHERE session_id = ?`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query functions: %w", err)
	}
	defer rows.Close()

	graph := &CallGraph{
		Nodes:          make(map[string]*FuncNode),
		IDIndex:        make(map[int64]*FuncNode),
		plainNameIndex: make(map[string][]*FuncNode),
	}

	// 第一遍：收集所有函数节点
	for rows.Next() {
		f := &model.Function{}
		if err := rows.Scan(&f.ID, &f.Name, &f.PackageName, &f.Language, &f.FilePath,
			&f.LineStart, &f.LineEnd, &f.Body, &f.CallCount, &f.NestingDepth); err != nil {
			return nil, fmt.Errorf("scan function: %w", err)
		}
		qn := qualifiedName(f)
		node := &FuncNode{
			Function:  f,
			Qualified: qn,
			Callers:   []string{},
			Callees:   []string{},
		}
		graph.Nodes[qn] = node
		graph.IDIndex[f.ID] = node
		graph.plainNameIndex[f.Name] = append(graph.plainNameIndex[f.Name], node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 第二遍：填充调用/被调用关系
	for _, node := range graph.Nodes {
		deps, err := a.Store.QueryDependencies(node.Function.ID)
		if err != nil {
			return nil, fmt.Errorf("query deps for %s: %w", node.Qualified, err)
		}
		for _, dep := range deps {
			node.Callees = append(node.Callees, dep)

			// 尝试匹配限定名：如果调用方有包名，尝试 pkg.dep
			matched := false
			if node.Function.PackageName != "" {
				qd := fmt.Sprintf("%s.%s", node.Function.PackageName, dep)
				if calleeNode, ok := graph.Nodes[qd]; ok {
					calleeNode.Callers = append(calleeNode.Callers, node.Qualified)
					matched = true
				}
			}

			// 回退：按普通名匹配（处理跨包调用）
			if !matched {
				if candidates, ok := graph.plainNameIndex[dep]; ok {
					for _, calleeNode := range candidates {
						calleeNode.Callers = append(calleeNode.Callers, node.Qualified)
					}
					matched = true
				}
			}
		}
	}

	return graph, nil
}

// FindOrphanFunctions 查找未被任何函数调用的函数（可能的死代码）
func (g *CallGraph) FindOrphanFunctions() []*FuncNode {
	var orphans []*FuncNode
	for _, node := range g.Nodes {
		if len(node.Callers) == 0 {
			orphans = append(orphans, node)
		}
	}
	return orphans
}

// FindHotFunctions 查找被调用最多的函数（按调用次数排序）
func (g *CallGraph) FindHotFunctions(limit int) []*FuncNode {
	type kv struct {
		node *FuncNode
		cnt  int
	}
	var sorted []kv
	for _, node := range g.Nodes {
		sorted = append(sorted, kv{node: node, cnt: len(node.Callers)})
	}
	// 标准库排序 O(n log n)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].cnt > sorted[j].cnt
	})
	if limit > 0 && limit < len(sorted) {
		sorted = sorted[:limit]
	}
	result := make([]*FuncNode, len(sorted))
	for i, kv := range sorted {
		result[i] = kv.node
	}
	return result
}
