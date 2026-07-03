## 1. 统一节点后处理框架

- [ ] 1.1 在 runner.go 中定义 RetryConfig 结构体（包含 max_attempts、initial_backoff、multiplier、max_backoff、jitter、on_status 字段）
- [ ] 1.2 实现 parseRetryConfig 方法，从节点 config JSON 中解析 retry 配置
- [ ] 1.3 实现 calculateBackoff 方法，计算指数退避时间（支持 jitter）
- [ ] 1.4 实现 parseExtractConfig 方法，从节点 config JSON 中解析 extract 配置
- [ ] 1.5 实现 applyExtract 方法，从节点响应中提取变量并写入共享作用域
- [ ] 1.6 重构 sceneNode.Execute 方法，在节点执行后调用统一后处理（extract）
- [ ] 1.7 重构 sceneNode.Execute 方法，在节点执行前包装 retry 逻辑
- [ ] 1.8 为统一 extract 编写单元测试（HTTP 节点、Generator 节点）
- [ ] 1.9 为统一 retry 编写单元测试（指数退避、jitter、max_backoff）

## 2. While 步骤支持 Generator 类型

- [ ] 2.1 扩展 while_node.go 中的 stepConfig 结构体，添加 Type 和 Config 字段
- [ ] 2.2 实现 executeWhileStepGenerator 方法，执行 generator 表达式并存储结果
- [ ] 2.3 修改 executeWhile 方法，根据 step.Type 分发到 HTTP 或 Generator 执行逻辑
- [ ] 2.4 确保 generator 步骤的变量写入 loopVars，可供后续步骤和退出条件使用
- [ ] 2.5 为 while generator step 编写单元测试（基本执行、变量传播、extract）

## 3. Multipart 格式兼容性

- [ ] 3.1 修改 executeHTTP 方法，支持解析 multipart 扁平格式配置
- [ ] 3.2 实现自动推断逻辑：根据值模式（包含文件扩展名或路径分隔符）判断是 field 还是 file
- [ ] 3.3 确保嵌套格式（form.fields/files）仍然正常工作
- [ ] 3.4 为 multipart 兼容性编写单元测试（扁平格式、嵌套格式、推断逻辑）

## 4. Card YAML 重构

- [ ] 4.1 分析 card.yaml 中所有 think_time 使用位置（11 处）
- [ ] 4.2 为每个 think_time 创建对应的 delay 节点
- [ ] 4.3 更新 edges，在 HTTP 节点和 delay 节点之间建立正确的依赖关系
- [ ] 4.4 验证重构后的 YAML 语法正确性
- [ ] 4.5 运行完整流程测试，确保重构后功能正常

## 5. 节点级独立超时与场景默认超时配置

- [ ] 5.1 在 DAG executor 中实现节点级独立超时：为每个节点创建独立的 timeout context
- [ ] 5.2 在 Scene 模型中添加 `default_timeout` 字段（秒，0 表示使用系统默认）
- [ ] 5.3 实现数据库迁移：为 scenes 表添加 default_timeout 列
- [ ] 5.4 更新 Scene Repository 的 CRUD 方法，支持 default_timeout 字段
- [ ] 5.5 更新 API DTO 和 Handler，支持创建/更新/查询场景时传递 default_timeout
- [ ] 5.6 在 Runner 中使用场景的 default_timeout：如果为 0 则使用系统默认 600 秒
- [ ] 5.7 在前端场景编辑页面添加"默认超时"输入框
- [ ] 5.8 为节点级超时和场景默认超时编写单元测试

## 6. 前端全节点编辑支持变量引用

- [ ] 6.1 修改 delay 节点的 ms 字段：从 type="number" 改为 type="text"，支持变量引用
- [ ] 6.2 修改 timer 节点的 seconds 字段：从 type="number" 改为 type="text"，支持变量引用
- [ ] 6.3 修改 group 节点的 loop_count 字段：从 type="number" 改为 type="text"，支持变量引用
- [ ] 6.4 修改 http 节点的 timeout 和 expect_status 字段：从 type="number" 改为 type="text"，支持变量引用
- [ ] 6.5 更新 reactive 初始值：将数值类型改为字符串类型（如 '5000'、'200'）
- [ ] 6.6 更新配置读取逻辑：使用 String() 转换，确保显示原始值
- [ ] 6.7 验证保存逻辑：确保变量引用不被破坏
- [ ] 6.8 为前端全节点编辑编写端到端测试

## 7. 集成测试与验证

- [ ] 7.1 使用 card.yaml 进行端到端测试，验证所有功能正常工作
- [ ] 7.2 验证向后兼容性：确保现有的 while/parallel/loop step extract/retry 不受影响
- [ ] 7.3 编写集成测试，覆盖统一后处理、while generator、multipart 兼容、节点级超时、场景默认超时
- [ ] 7.4 更新文档，说明新的 extract、retry、超时配置方式
- [ ] 7.5 代码审查和性能测试
