## 1. 后端执行引擎（runner）

- [x] 1.1 修复 while_node.go 语法错误（恢复 executeWhileStepGenerator 函数签名）
- [x] 1.2 实现 executeWhileRefMode：复用 while 全部骨架（退出条件/max_iterations/max_duration/fail_on_max_*/interval），每轮 runChildChain + 退出条件评估用刷新后的 input.Variables；实现软失败语义（首个子节点软失败写入 Output.Error）
- [x] 1.3 loop_node.go 增加 `len(n.childNodes) > 0` 引用分支：loop_count 次子链执行、软失败语义；steps 兜底保留
- [x] 1.4 运行 composite_ref_test.go（TestExecuteWhileNodeIDsRefMode / TestExecuteWhileRefModeTakesPrecedenceOverSteps / TestExecuteLoopNodeIDsRefMode / TestBuildDAGCompositeChildren / TestBuildDAGWhileChildNestingForbidden）全部转绿

## 2. YAML 导入/导出（handler）

- [x] 2.1 YAML 导入：node_ids 校验循环类型扩展为三类复合节点（group/while/loop）；新增嵌套校验（while/loop 引用复合节点 → 400）
- [x] 2.2 YAML 导入：name→ID 解析循环类型扩展为三类
- [x] 2.3 YAML 导出：导出节点循环新增 node_ids ID→名字转换（nodeNameMap 反查，查不到原样保留），三类节点统一生效
- [x] 2.4 新增测试：TestYAMLImportWhileNodeIDs（校验+解析+嵌套拒绝）、TestYAMLExportNodeIDsAsNames（导出 ID→名字 + group/while/loop 导出→再导入闭环）

## 3. 后端回归

- [x] 3.1 `go test ./internal/...` 全绿（重点 runner/api/dag，group 既有测试全量回归）

## 4. 前端配置面板（SceneDetailPage.vue）

- [x] 4.1 修保存丢字段 bug：whileConfig 补 steps/node_ids/fail_on_max_iterations/fail_on_max_duration 字段，loopConfig 补 steps/node_ids；selectNode 加载这些字段；saveNodeConfig while/loop 分支改为基于原始 config 浅合并（JSON.parse 原对象 → 表单字段覆盖 → 序列化）；节点编辑弹窗（handleSaveNode）while/loop 分支同步改为浅合并
- [x] 4.2 保存成功后 fetchNodes() 并按 id 重同步 selectedNode（修陈旧引用）
- [x] 4.3 while/loop 面板新增"子节点"区：勾选列表（过滤复合类型与自身）+ 有序列表（上移/下移/移除），绑定 whileConfig.node_ids / loopConfig.node_ids；moveChildUp/Down/removeChild 泛化为通用函数
- [x] 4.4 双模式提示：config 同时含内嵌 steps 时显示"N 个 YAML 内嵌步骤（引用子节点优先生效）"
- [x] 4.5 group/while/loop 面板已选列表加"编辑"按钮：selectNode(childNode) + returnToStack 记录当前端 id（支持多级返回）；panel-header 加"← 返回"按钮（按 id 现查节点 selectNode）

## 5. 前端画布渲染（DagFlow.vue + DagSceneNode.vue）

- [x] 5.1 applyLayout：while/loop 有 node_ids → 反查 NodeDTO 挂 childNodes（同 group）；while 内嵌 steps 只读展示保留
- [x] 5.2 getNodeDimensions：while/loop 高度按 node_ids 子节点数计算（与 steps 取较大者）
- [x] 5.3 DagSceneNode：while/loop 展开渲染 childNodes（复用 group-children 样式与双击交互）；loop 新增展开分支；while 有子节点时内嵌 steps 不再显示（引用模式优先生效，仅无子节点时保留只读展示）

## 6. 前端验证与收尾

- [x] 6.1 `cd web/app && npx vue-tsc -b` 类型检查通过（与 HEAD 基线一致，零新增错误；基线的 8 个预存在错误位于本次未触碰的代码）
- [x] 6.2 全量回归：go test ./internal/... 全绿 + vue-tsc 无新增错误，codegraph sync 已执行（Added: 4, Modified: 8），变更后自检清单已过
