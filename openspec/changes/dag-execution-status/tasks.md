## 1. ELK 布局引擎替换 dagre

- [x] 1.1 安装 elkjs npm 包，移除 dagre npm 包
- [x] 1.2 重写 DagFlow.vue 的 buildLayout() 函数，使用 ELK layered 算法替代 dagre
- [x] 1.3 实现变长节点尺寸计算：根据节点类型和展开状态动态计算 width/height 传入 ELK
- [x] 1.4 更新 autoLayout() 函数调用新 buildLayout() + fitView()
- [x] 1.5 验证一键美化后无重叠：包含展开 group/while 节点的 DAG 不再重叠

## 2. WebSocket 后端推送

- [x] 2.1 添加 gorilla/websocket Go 依赖
- [x] 2.2 实现 internal/ws/hub.go：Hub 结构体，管理 Client 连接和订阅（subscribe by run_id）
- [x] 2.3 实现 internal/ws/client.go：Client 结构体，读写 pump goroutine，订阅管理
- [x] 2.4 编写 ws 包单元测试：Hub 订阅/取消订阅、广播、连接上限
- [x] 2.5 在 internal/api/server.go 注册 /ws 路由
- [x] 2.6 在 trace.go 的 SpanBuilder.Finish() / Skip() / FinishCanceled() 中添加 Hub 广播调用
- [x] 2.7 在 executor.go 的循环执行中添加 loop iteration 中间事件的广播（status=running, loop_index=i）
- [x] 2.8 编写 trace 广播集成测试：验证 Span 完成后 WS 客户端收到正确事件

## 3. 前端 WebSocket 客户端

- [x] 3.1 实现 src/composables/useExecutionWs.ts：WebSocket 连接管理、subscribe、自动重连（指数退避）
- [x] 3.2 编写 composable 单元测试：连接/断连/重连逻辑

## 4. DAG 执行状态叠加展示

- [x] 4.1 实现 src/composables/useExecutionStatus.ts：维护 nodeStatusMap（聚合+单 chain），处理 span_update 事件
- [x] 4.2 在 DagSceneNode.vue 中添加聚合视图状态 badge（✓✗⊘⟳计数）
- [x] 4.3 在 DagSceneNode.vue 中添加单链路视图状态样式（边框颜色+背景+半透明）
- [x] 4.4 在 DagSceneNode.vue 中添加循环进度 badge（L2/3）
- [x] 4.5 实现链路选择器组件：聚合视图/单链路视图 tab + chain 下拉
- [x] 4.6 在 DagFlow.vue 中实现单链路视图的边样式（活跃路径高亮，非活跃半透明）
- [x] 4.7 实现 DAG 执行态/编辑态切换：根据场景运行状态控制 nodesDraggable 和状态显示
- [x] 4.8 集成 WebSocket + 状态展示到 SceneDetailPage.vue

## 5. 清理与文档

- [x] 5.1 删除 web/demo/dag-execution-status.html 静态 demo 文件
- [x] 5.2 更新 .knowledge/L3-project/pitfalls.md 记录 ELK 布局和 WS 相关注意事项
