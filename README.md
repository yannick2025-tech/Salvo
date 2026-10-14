# Salvo

**配置驱动的通用性能测试平台**

Salvo 是一个专为 API 和微服务架构设计的性能测试平台，通过 YAML 配置实现零代码业务适配，支持复杂的 DAG 请求流编排、多场景并发测试和实时监控。

## 核心特性

### 配置驱动
- **YAML 场景定义**：通过 YAML 文件定义完整的测试场景，无需编写代码
- **零代码业务适配**：只需编写 YAML 配置即可适应不同的业务系统
- **内置生成器**：13+ 种数据生成器（UUID、邮箱、日期、随机数等）

### DAG 编排
- **可视化编排**：支持 HTTP 请求、延迟、条件分支、循环、定时触发等节点类型
- **独立分支并行**：自动识别并并行执行无依赖关系的分支
- **变量传递**：支持响应数据提取、变量引用、数据源参数化

### 实时监控
- **WebSocket 推送**：实时展示执行进度、QPS、延迟、错误率
- **聚合视图**：按节点聚合展示通过/失败/跳过/运行中数量
- **单链路视图**：选择具体链路查看循环进度和节点状态
- **4 层 Trace 追踪**：Run → Chain → Node → Span 完整链路追踪

### 测试报告
- **自动生成**：包含总体指标、节点详情、失败请求分析、图表可视化
- **HTML 导出**：支持单个或批量导出独立的 HTML 报告
- **失败请求详情**：包含请求 Method/URL/Headers/Body 和响应 Status/Headers/Body

### 扩展性架构
- **插件系统**：
  - Go Plugin（SO 插件）：已实现，支持加密解密、自定义协议处理
  - Lua Plugin：计划中，用于限速、流量控制
- **协议扩展**：预留接口，可扩展到 DATABASE、FTP/SFTP、gRPC、WebSocket 等
- **加密解密**：内置 AES-CBC/GCM 加密插件，支持业务定制加密方案

## 快速开始

### 环境要求
- Go 1.21+
- Node.js 18+
- SQLite 3（默认数据库）

### 安装

```bash
# 克隆项目
git clone https://github.com/your-org/salvo.git
cd salvo

# 构建后端
go build -o salvo-server ./cmd/server

# 构建前端
cd web
npm install
npm run build
```

### 启动

```bash
# 启动服务
./salvo-server

# 访问 Web UI
open http://localhost:8080
```

### 创建第一个测试场景

1. **导入 YAML 场景**：
```yaml
name: 简单 HTTP 测试
nodes:
  - name: 请求首页
    type: http
    config:
      method: GET
      url: "https://api.example.com/home"
      extract:
        user_id: "$.data.userId"
```

2. **配置运行参数**：
   - 并发数：10
   - 持续时间：60 秒（或按次数：1000 次）
   - 运行模式：按时间 / 按次数

3. **启动测试**：点击"启动场景"，实时查看执行进度和指标

## 文档

### 使用指南
- [功能清单](docs/features.md) - 平台完整功能列表
- [YAML 场景配置指南](docs/biz-migration/salvo-yaml-guide.md) - DAG 编排、节点类型、变量系统

### 设计文档
详见 [docs/design/](docs/design/) 目录：
- [Salvo 整体设计](docs/design/2026-04-30-salvo-design-zh.md)
- [Web UI 与 RBAC 设计](docs/design/2026-05-02-web-ui-rbac-design.md)
- [生成器与条件分支设计](docs/design/2026-05-03-generator-selector-ifelse-design.md)
- [加密模块设计](docs/design/crypto.md)
- [SO 插件架构](docs/design/so-plugin-architecture.md)

### 知识库
团队约定、已知陷阱、工作流等详见 [.knowledge/](.knowledge/) 目录。

## 技术栈

### 后端
- **语言**：Go 1.21+
- **Web 框架**：标准库 net/http
- **数据库**：SQLite（默认）/ MySQL / PostgreSQL
- **实时通信**：WebSocket
- **插件系统**：Go Plugin（动态链接库）

### 前端
- **框架**：Vue 3 + Composition API
- **构建工具**：Vite
- **UI 组件**：Element Plus
- **状态管理**：Pinia
- **图表库**：ECharts

## 项目结构

```
salvo/
├── cmd/                    # 命令行入口
│   └── server/            # 服务端启动
├── internal/              # 内部包
│   ├── api/               # HTTP API 处理
│   ├── core/              # 核心业务逻辑
│   │   ├── dag/           # DAG 执行引擎
│   │   ├── scene/         # 场景管理
│   │   └── runner/        # 测试运行器
│   ├── model/             # 数据模型
│   ├── plugin/            # 插件系统
│   ├── protocol/          # 协议实现（HTTP）
│   └── storage/           # 存储层
├── web/                   # 前端代码
│   ├── src/
│   │   ├── components/    # Vue 组件
│   │   ├── views/         # 页面视图
│   │   ├── stores/        # Pinia 状态管理
│   │   └── utils/         # 工具函数
│   └── public/            # 静态资源
├── plugins/               # 内置插件
│   └── aes/               # AES 加密插件
├── docs/                  # 文档
│   ├── design/            # 设计文档
│   └── biz-migration/     # 业务迁移文档
└── .knowledge/            # 知识库
    ├── L1-conventions/    # 团队约定
    ├── L3-project/        # 项目知识
    └── L4-workflows/      # 工作流
```

## 贡献指南

欢迎贡献！请遵循以下流程：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

### 开发规范
- 遵循 [代码风格规范](.knowledge/L1-conventions/coding-style.md)
- 遵循 [Git 提交规范](.knowledge/L1-conventions/git-commit.md)
- 提交前运行代码审查清单

## 路线图

- [ ] Lua Plugin 支持（限速、流量控制）
- [ ] 数据库协议扩展（MySQL、PostgreSQL、Redis）
- [ ] gRPC 协议支持
- [ ] WebSocket 协议支持
- [ ] 更丰富的数据生成器（Faker 集成）
- [ ] 分布式测试支持

## 许可证

[MIT License](LICENSE)

## 致谢

感谢所有贡献者和用户的支持！

---

**Star History**

如果这个项目对你有帮助，欢迎给个 Star ⭐ 支持一下！
