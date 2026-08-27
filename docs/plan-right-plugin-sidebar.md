# 右侧可收缩插件容器 — 实施计划（已归档）

见计划模式产出：右侧插件容器固定宽度 300dp + 32dp 收起窄条，水平并排于主内容区右侧，`FrameMetrics.Sidebar` 纳入 `SetBounds` 宽度计算，状态持久化于 `config`，内容复用 `extension` 管理器，展开/收起双入口（侧栏内 + 工具栏）。

实施顺序：Config 扩展 → FrameMetrics/side state → LayoutRoot 重构 → plugin_sidebar.go → 工具栏与 app 接线 → 联调与文档。
