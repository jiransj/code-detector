# code-detector v1.1 版本说明

## 🆕 新功能

### `-query=tree=NAME` — 递归依赖树提取
新增查询模式，可递归提取指定函数及其**所有传递依赖**（含完整函数体）。

```bash
code-detector -query tree=initScanner
code-detector -query tree=printBanner
code-detector -format json -query tree=main
```

- BFS 遍历传递闭包，visited 集合防循环
- 树形缩进排版输出，深度标记清晰
- 支持 text 和 JSON 两种输出格式

---

## 🔧 重构与优化

### 全面迁移至 tree-sitter 原生 field 提取
原本多处使用**子节点遍历 + 硬编码类型黑名单**的自编方案提取函数签名信息，现已全部替换为 tree-sitter 原生 field 名访问：

| 字段 | 旧方案 | 新方案 |
|------|--------|--------|
| 参数列表 | 遍历子节点按 `Type()` 匹配 | `ChildByFieldName("parameters")` |
| 返回值类型 | 13 种硬编码 type 名黑名单匹配 | `ChildByFieldName("result")` |
| 方法接收器 | 两层子节点遍历 | `ChildByFieldName("receiver")` |
| 参数个数 | `strings.Count(",", inner) + 1` 逗号计数 | `NamedChildCount()` |
| 变量初始值 | 全子节点遍历 + 类型黑名单排除 | `ChildByFieldName("value")` |
| 复合字面量类型 | 子节点排除 `{`/`}`/`literal_value` | `ChildByFieldName("type")` |
| 函数调用名 | 子节点找 `identifier`/`selector_expression` | `ChildByFieldName("function")` |
| 类型转换目标 | 子节点排除 `(`/`)`/`argument_list` | `ChildByFieldName("type")` |

**效果**：
- 消除脆弱硬编码，对复杂/泛型/多返回值类型提取更准确
- 方法接收器与参数正确分离（旧方案混在一起）
- 参数计数不再受逗号格式干扰

### 其他
- `tsCountParams` 废弃，彻底移除逗号计数逻辑

---

## 📦 文件变更

```
internal/parser/treesitter_go.go  |  大幅精简
cmd/code-detector/main.go         |  新增 tree 查询分支 + printQueryTree
internal/db/querier.go            |  新增 QueryFuncTree / FuncTreeItem
README.md                         |  文档更新 tree=NAME 模式
README_EN.md                      |  同上（英文版）
CHANGELOG-v1.1.md                 |  本版本说明
```

---

## ✅ 验证

- 编译通过，零错误
- 全量扫描正常（72 文件 / 924 函数 / 74 全局变量 / 14 种语言）
- `-query func=` 多返回值正确显示（如 `parseFlags` 的 13 个命名返回值）
- `-query tree=` 递归依赖提取工作正常
- `-format json` 格式输出正确
