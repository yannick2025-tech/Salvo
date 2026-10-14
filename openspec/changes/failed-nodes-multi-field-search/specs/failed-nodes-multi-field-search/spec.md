## ADDED Requirements

### Requirement: 失败节点多字段独立查询界面
测试报告和导出 HTML 报告的失败节点详情区 SHALL 提供五个独立查询字段：节点名、NodeID、错误码、错误信息、协议。每个字段 SHALL 独立输入，互不影响。

#### Scenario: 五个独立查询框
- **WHEN** 用户打开失败节点详情区
- **THEN** 显示五个独立查询字段：节点名（text）、NodeID（text）、错误码（text）、错误信息（text）、协议（select 下拉）

#### Scenario: 协议下拉选项
- **WHEN** 用户点击协议下拉框
- **THEN** 显示选项：全部（默认）、http、db、mq（其他协议按数据中实际存在的值动态补充）

### Requirement: 字段间 AND 逻辑联合过滤
多个查询字段 SHALL 使用 AND 逻辑：只有同时满足所有已填写字段条件的记录才显示。空字段 SHALL 不参与过滤（视为无约束）。

#### Scenario: 单字段查询
- **WHEN** 用户只在"节点名"框输入"订单"，其他框为空
- **THEN** 显示所有节点名包含"订单"的失败记录（忽略大小写）

#### Scenario: 多字段 AND 查询
- **WHEN** 用户在"节点名"框输入"订单"，在"错误码"框输入"503"
- **THEN** 只显示节点名包含"订单" **且** 错误码包含"503"的记录

#### Scenario: 全部字段为空
- **WHEN** 所有查询字段均为空
- **THEN** 显示全部失败记录（无过滤）

#### Scenario: 无匹配记录
- **WHEN** 用户输入的条件导致无任何记录匹配
- **THEN** 显示"没有匹配的记录"提示文案，不显示任何失败节点卡片

### Requirement: 模糊匹配与大小写忽略
节点名、NodeID、错误码、错误信息四个文本字段 SHALL 使用模糊匹配（includes），且 SHALL 忽略大小写。协议字段 SHALL 精确匹配（"全部"除外）。

#### Scenario: 大小写不敏感
- **WHEN** 用户在"错误信息"框输入"TIMEOUT"
- **THEN** 匹配错误信息含 "timeout"、"Timeout"、"TIMEOUT" 等任意大小写组合的记录

#### Scenario: 协议精确匹配
- **WHEN** 用户在协议下拉选择"http"
- **THEN** 只显示 protocol 为 "http" 的记录；protocol 为空的记录默认按 "http" 处理

### Requirement: 独立清空与整体布局
每个查询字段 SHALL 支持独立清空。查询区 SHALL 紧凑布局，使用网格一行排列，在窄屏下自动换行。

#### Scenario: 独立清空
- **WHEN** 用户点击某字段右侧的清空按钮（✕）
- **THEN** 仅该字段清空，其他字段保持不变，结果实时更新

#### Scenario: 实时过滤
- **WHEN** 用户在任一字段输入或清空内容
- **THEN** 失败节点列表立即重新过滤，无需点击搜索按钮

### Requirement: 导出 HTML 报告同步支持多字段查询
导出的静态 HTML 报告 SHALL 包含与前端 SPA 一致的五个独立查询字段，AND 逻辑过滤通过原生 JavaScript 实现，无需后端请求。

#### Scenario: 导出报告离线查询
- **WHEN** 用户打开导出的 HTML 报告，在"节点名"输入"订单"，在"错误码"输入"503"
- **THEN** 仅显示同时满足两个条件的失败节点卡片，实时过滤，无需网络请求

#### Scenario: 导出报告 data 属性
- **WHEN** 导出 HTML 生成时
- **THEN** 每个失败节点卡片元素 SHALL 包含 data-name、data-id、data-code、data-msg、data-protocol 属性，供 JS 过滤函数读取
