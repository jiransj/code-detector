# code-detector v1.2

多编程语言自动扫描工具 — 性能优化与稳定性增强版

## 核心改进

### ⚡ 性能优化
- **tree-sitter 加速**：消除每函数多次冗余 CGo cursor.Exec 调用，首次扫描提速 ~25%
- **AST 结果缓存**：基于 FNV 内容哈希的跨会话缓存，重复扫描跳过 tree-sitter 解析，提速 **6x**
- **文件变更缓存**：`file_cache` 全局化，增量扫描零文件 I/O 进入，二次扫描仅需 **~0.2s**
- **智能增量**：数据库存在时自动启用增量扫描，无需手动指定 `-incremental`

### 🗄️ 数据库与缓存
- **ON DELETE CASCADE**：所有外键支持级联删除，prune 操作从 7 表逐条 DELETE 降为 1 条
- **会话数据迁移**：prune 时自动将旧会话数据迁移到最新会话，避免数据丢失
- **去重写入**：跨会话函数去重（`DISTINCT file:name:line`），避免重复存储
- **写锁优化**：`fileCacheMu` / `astCacheMu` 互斥锁 + `busy_timeout=5000`，消除 SQLITE_BUSY

### 🐛 Bug 修复
- `pruneOldSessionsLocked` 补充缺失的 `file_metrics` / `type_defs` 表删除
- `ComputeFileMetrics` 改用 `INSERT OR REPLACE` 修复 UNIQUE 约束冲突
- `waitForExit` 非终端模式直接返回，消除 3 秒 stdin 等待
- 跳过文件计数 (`SkipCount`) 现在正确统计增量缓存跳过的文件
- 语言分布统计从 DB `DISTINCT` 查询，不再因增量跳过而波动

### 📊 显示改进
- "发现函数" / "全局变量" 统一从 DB 去重查询，5 轮扫描全部 **266 / 53 稳定**
- 全缓存跳过时仍正确显示库中实际函数/变量总数

## 版本兼容性

- Go 版本: 1.26+
- 数据库: SQLite (modernc.org/sqlite v1.51+)
- 操作系统: Windows / Linux
