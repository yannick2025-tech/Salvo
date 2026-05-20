## 1. 修改字段类型

- [x] 1.1 将 `EnhancedReportContext` 结构体中的 `JSONData` 字段类型从 `string` 改为 `template.JS`

## 2. 验证

- [x] 2.1 运行 `go test ./internal/api/...` 确认现有测试通过
- [x] 2.2 运行 `go build ./...` 确认编译通过
- [x] 2.3 重启后端服务，导出 HTML 报告，确认所有图表正常渲染