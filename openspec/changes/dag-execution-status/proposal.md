## Why

当前 Salvo 的 DAG 编辑器存在两个核心体验问题：(1) 一键美化后节点与连线重叠，尤其展开的 group/while 节点因 dagre 不支持变长节点尺寸而与周围节点重叠；(2) 场景执行时无法实时看到各节点的执行状态（PASS/FAILED/SKIP/RUNNING），只能事后查看 trace 日志。这两个问题严重影响 DAG 的可用性和执行监控效率。

## What Changes

- 将 DAG 布局引擎从 dagre 替换为 ELK (Eclipse Layout Kernel)，支持变长节点尺寸，消除一键美化后的重叠问题
- 新增 WebSocket 实时推送机制，后端在每个 Span 状态变更时主动推送到前端
- 在 DAG 节点上叠加执行状态角标：聚合视图显示各状态计数（✓3 ✗1 ⚠1），单链路视图显示节点整体状态着色
- 新增链路选择器，支持在聚合视图和单 chain 视图间切换
- 循环节点显示当前循环进度（L2/5）
- DAG 在场景执行期间进入"执行态"（只读 + 状态展示），执行结束后保持最终状态

## Capabilities

### New Capabilities
- `elk-layout`: ELK 布局引擎替换 dagre，支持变长节点尺寸的一键美化布局
- `ws-execution-push`: WebSocket 实时推送 Span 状态变更事件到前端
- `dag-execution-overlay`: DAG 节点执行状态叠加展示（聚合视图 + 单链路视图 + 循环进度）

### Modified Capabilities

## Impact

- **前端**: DagFlow.vue 替换 dagre 为 elkjs；DagSceneNode.vue 新增状态角标和执行态样式；新增 WebSocket 客户端和链路选择器组件
- **后端**: 新增 internal/ws/ 包（WebSocket Hub）；trace.go 中 Span 完成时广播事件；api/server.go 注册 /ws 路由
- **依赖**: 新增 elkjs npm 包，移除 dagre npm 包；后端新增 gorilla/websocket 依赖
- **API**: 新增 WebSocket 端点 /ws，消息格式为 JSON（subscribe + span_update 事件）
- **数据**: 无数据库 schema 变更，实时状态仅通过 WS 推送不持久化
