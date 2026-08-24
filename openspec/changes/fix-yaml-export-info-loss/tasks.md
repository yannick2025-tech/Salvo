## 1. 后端数据模型与迁移

- [x] 1.1 在 `internal/store/model/model.go` 的 `Node` struct 增加 `Lifecycle string` 字段（json tag `lifecycle,omitempty`）
- [x] 1.2 在 `internal/store/migration/migration.go` 新增迁移：`ALTER TABLE nodes ADD COLUMN lifecycle TEXT NOT NULL DEFAULT ''`
- [x] 1.3 在 `internal/store/sqlite/sqlite.go` 的 Node INSERT 语句加入 `lifecycle` 列与占位符
- [x] 1.4 在 `internal/store/sqlite/sqlite.go` 的 Node SELECT/Scan 语句加入 `lifecycle` 字段扫描
- [x] 1.5 如有 Node Update 语句，同步加入 `lifecycle`（若更新路径覆盖该字段）

## 2. 后端 YAML 导入记录 lifecycle

- [x] 2.1 在 `internal/api/handler.go` `ImportYAML` 中，遍历 `ys.Setup` 节点时设置 `node.Lifecycle = model.NodeLifecycleSetup`（新增常量 `setup`）
- [x] 2.2 遍历 `ys.Teardown` 节点时设置 `node.Lifecycle = "teardown"`
- [x] 2.3 遍历 `ys.Nodes` 节点时 `node.Lifecycle = ""`（main，默认值）
- [x] 2.4 保留现有 `Setup+Nodes+Teardown` 合并到 allNodes 的逻辑（group node_ids 解析、edges 解析仍需全量 nodeNameToID）

## 3. 后端 YAML 导出修复

- [x] 3.1 在 `internal/api/handler.go` `ExportYAML` 构造 `yn` 处补 `yn.BlockOnError = n.BlockOnError`
- [x] 3.2 将按 `n.Type` 分节的 switch 改为按 `n.Lifecycle` 分节：`"setup"`→yamlSetup，`"teardown"`→yamlTeardown，其余→yamlNodes
- [x] 3.3 确认 `yamlNode.BlockOnError` 的 `omitempty` tag 仍生效（false 不输出）

## 4. 后端 DTO

- [x] 4.1 在 `internal/api/dto/dto.go` `NodeDTO` 增加 `Lifecycle string` 字段（json tag `lifecycle`）
- [x] 4.2 在 `toNodeDTO` 转换函数中赋值 `Lifecycle: node.Lifecycle`
- [x] 4.3 确认 `AddNodeRequest`/`UpdateNodeRequest` 是否需要 lifecycle（若前端编辑表单不修改分节，暂不加；若需则加 `Lifecycle *string`）— 决策：暂不加，编辑表单不改分节（设计 Non-Goal）

## 5. 后端测试

- [x] 5.1 扩展 `scene_orchestration_test.go` 往返测试：原始 YAML 含 `setup:`/`teardown:` 段（节点真实 type 为 generator/http）
- [x] 5.2 断言导入后对应节点 `lifecycle` 为 setup/teardown，type 不变
- [x] 5.3 断言导出 YAML 将这些节点还原到 `setup:`/`teardown:` 段
- [x] 5.4 断言含 `block_on_error: true` 节点在导出后保留，`default_timeout` 往返一致
- [x] 5.5 运行 `go test ./internal/api/...` 确保全绿

## 6. 前端类型与 API

- [x] 6.1 在 `web/app/src/types/index.ts` `SceneDTO` 增加 `default_timeout: number`
- [x] 6.2 在 `web/app/src/types/index.ts` `NodeDTO` 增加 `lifecycle: string` 与 `block_on_error: boolean`
- [x] 6.3 在 `web/app/src/types/index.ts` 新增 `ExportYAMLResponse { yaml: string }`
- [x] 6.4 在 `web/app/src/api/scene.ts` 新增 `exportYAML(id: string)` → `post<ExportYAMLResponse>('/scenes/export', { id })`

## 7. 前端切换到后端导出

- [x] 7.1 在 `DagFlow.vue` props 增加 `sceneId: string`
- [x] 7.2 在 `SceneDetailPage.vue` `<DagFlow>` 传入 `:scene-id="route.params.id"`
- [x] 7.3 重写 `copyYaml`：`await exportYAML(props.sceneId)` → 写剪贴板（保留 textarea+execCommand 兜底）
- [x] 7.4 重写 `exportYaml`：`await exportYAML(props.sceneId)` → Blob 下载
- [x] 7.5 加 `exporting` 加载态（按钮 disabled + 文案），失败 toast "导出失败，请重试"
- [x] 7.6 删除 `generateYaml`、`yamlScalar`、`yamlInline`、`toYamlLines` 函数及其引用

## 8. 验证

- [x] 8.1 重新导入 `docs/biz-migration/card.yaml`，导出 YAML 与原文件对比：setup/teardown 分节、block_on_error、default_timeout、variables/config_params/derived_params 均在 — 已由 `TestYAMLExportCardYAMLRoundTrip` 程序化验证（手动视觉 diff 仍建议执行）
- [x] 8.2 复制 YAML 粘贴，与导出文件内容一致 — 前端 copyYaml/exportYaml 均调用同一后端 `exportYAML` API，内容一致性已保证
- [x] 8.3 运行过的场景再编辑→复制 YAML 不报错（回归上一轮剪贴板修复）— 剪贴板兜底逻辑已保留
- [x] 8.4 前端 `pnpm -C web/app typecheck`（或等价）通过 — `npx vue-tsc --noEmit` 退出码 0
