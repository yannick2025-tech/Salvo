## 1. 前端 SPA — ReportDetailPage.vue

- [ ] 1.1 替换单搜索框为多字段查询区 HTML：5 个输入框（节点名 text、NodeID text、错误码 text、错误信息 text、协议 select），紧凑网格布局
- [ ] 1.2 每个文本字段添加独立清空按钮（✕），协议下拉用默认箭头
- [ ] 1.3 查询区顶部添加提示文案"多个字段同时匹配（AND）"
- [ ] 1.4 新增 5 个 ref：failedNodeSearchName、failedNodeSearchId、failedNodeSearchCode、failedNodeSearchMsg、failedNodeSearchProtocol
- [ ] 1.5 重写 filteredFailedNodes computed：AND 逻辑 + 空字段不参与过滤 + 文本 includes 忽略大小写 + 协议精确匹配
- [ ] 1.6 新增 CSS：多字段查询区网格布局（grid auto-fit minmax 140px）、独立清空按钮样式、提示文案样式
- [ ] 1.7 验证：单字段查询、多字段 AND 查询、全空显示全部、无匹配显示提示

## 2. 导出 HTML 报告 — report_generator_enhanced.go

- [ ] 2.1 替换 HTML 模板中的单搜索栏为多字段查询区（5 个 input/select），与前端一致
- [ ] 2.2 每个失败节点卡片元素添加 data-protocol 属性（补充现有 data-name/data-id/data-code/data-msg）
- [ ] 2.3 重写 filterFailedNodes() JS 函数：读取 5 个输入值，AND 逻辑过滤，实时显示/隐藏卡片
- [ ] 2.4 添加 clearField(fieldName) JS 函数支持独立清空单个字段
- [ ] 2.5 添加多字段查询区 CSS（网格布局、清空按钮、提示文案），与前端风格一致
- [ ] 2.6 验证：导出 HTML 报告后离线测试多字段 AND 查询

## 3. 知识库与文档更新

- [ ] 3.1 检查 .knowledge/L1-conventions/ 是否有 UI 搜索模式约定，如有则更新
- [ ] 3.2 在 .knowledge/L3-project/pitfalls.md 或相关文件记录"多字段 AND 查询"的实现模式（前端 computed + 导出 HTML JS 双实现）
