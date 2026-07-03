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

## 5. 集成测试与验证

- [ ] 5.1 使用 card.yaml 进行端到端测试，验证所有功能正常工作
- [ ] 5.2 验证向后兼容性：确保现有的 while/parallel/loop step extract/retry 不受影响
- [ ] 5.3 编写集成测试，覆盖统一后处理、while generator、multipart 兼容
- [ ] 5.4 更新文档，说明新的 extract、retry 配置方式
- [ ] 5.5 代码审查和性能测试
