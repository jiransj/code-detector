# code-detector v1.0

## 全局变量探测全面升级

### P0 修复
- **LineNum 始终为 0** — 从 AST `StartPoint().Row` 获取，所有变量现在显示正确行号
- **分组 `var (...)` 漏检** — 修复 `tsEachTopLevel` 单 match 多 capture 覆盖写 bug，7 个缓存查询变量全部检出

### P1 增强
- **推断类型** — `var x = "text"` 等无显式类型注解的变量，从 value 源码文本 + AST 节点双重推断：
  - 字符串/数字/布尔/nil → 直接映射
  - 复合字面量 `Type{...}` → 提取类型名
  - 函数调用 `pkg.Func()` → 函数名
- **跨语言支持** — 14 种语言添加 `VarQuery`/`ConstQuery`，`TreeSitterParser.Globals()` 自动提取模块级全局变量
- **Python 去重** — ConstQuery 与 VarQuery 相同时自动跳过，避免重复

### 调用图与死代码分析
- **`printQueryDead`** 改用 `BuildCallGraph` 分析，自动分类：test/fixture/runtime/mcp/lang/iface/func-ref/closure/cgap/dead
- **`qualifiedIndex`** 新增，支持同名函数多文件索引
- **调用匹配** — 同时输出简单名 + 限定名，支持同包优先、限定名退平、全局回退三级策略

### 清理
- 删除废弃的 `QueryDead()` SQL 查询
- 清理 `goKeywords` 重复条目
