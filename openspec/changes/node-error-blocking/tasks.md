## 1. Database Schema Migration

- [x] 1.1 Create migration script to add `block_on_error BOOLEAN DEFAULT FALSE` column to `nodes` table
- [x] 1.2 Verify migration executes successfully on existing database
- [x] 1.3 Confirm all existing nodes have `block_on_error = false` after migration

## 2. Model Layer Changes

- [x] 2.1 Add `BlockOnError bool` field to `model.Node` struct in `internal/store/model/model.go`
- [x] 2.2 Update Node JSON tags to include `block_on_error,omitempty`
- [x] 2.3 Verify model serialization/deserialization works with new field

## 3. DAG Interface Extension

- [x] 3.1 Add `BlockOnError() bool` method to `dag.Node` interface in `internal/core/dag/dag.go`
- [x] 3.2 Implement `BlockOnError() bool` method in `sceneNode` struct in `internal/runner/runner.go`
- [x] 3.3 Ensure default implementation returns `false` for backward compatibility

## 4. Runner Configuration Parsing

- [x] 4.1 Update `buildDAGNode()` in `internal/runner/runner.go` to parse `block_on_error` from `model.Node` and set it in `sceneNode`
- [x] 4.2 Add `blockOnError bool` field to `sceneNode` struct
- [x] 4.3 Update Group node child construction to also parse `block_on_error`
- [x] 4.4 Verify YAML scene import correctly parses `block_on_error` field

## 5. HTTP Node Error Handling

- [x] 5.1 Modify `executeHTTP()` in `internal/runner/runner.go` to return error when `blockOnError == true` and HTTP status is non-2xx
- [x] 5.2 Fix `expect_body` assertion failure to return error instead of `Output{Error}, nil` (change to `nil, error`)
- [x] 5.3 Test HTTP 404 with `block_on_error: true` triggers chain cancellation
- [x] 5.4 Test HTTP 500 with `block_on_error: false` continues chain execution
- [x] 5.5 Test `expect_body` assertion failure with `block_on_error: true` triggers chain cancellation

## 6. DAG Executor Chain Cancellation

- [x] 6.1 Modify `Executor.Execute()` in `internal/core/dag/executor.go` to hold a reference to the context cancel function
- [x] 6.2 When a node fails and `BlockOnError() == true`, call `cancel()` to cancel the entire chain
- [x] 6.3 Ensure context cancellation propagates to all running nodes
- [x] 6.4 Verify chain status is marked as "failed" when cancelled due to `block_on_error`
- [x] 6.5 Test that nodes with `block_on_error: false` do NOT trigger cancellation on failure

## 7. While Loop Step-Level Error Blocking

- [x] 7.1 Add `BlockOnError bool` field to `stepConfig` struct in `internal/runner/while_node.go`
- [x] 7.2 Modify while step execution logic to abort the loop immediately when step fails and `step.BlockOnError == true`
- [x] 7.3 Ensure while step `block_on_error` takes precedence over `fail_after_consecutive`
- [x] 7.4 Verify while node itself respects its own `block_on_error` setting when returning error
- [x] 7.5 Test while step with `block_on_error: true` aborts loop and propagates error to chain

## 8. API Layer Support

- [x] 8.1 Update node creation API handler to accept `block_on_error` field in request body
- [x] 8.2 Update node update API handler to accept `block_on_error` field in request body
- [x] 8.3 Verify node query API returns `block_on_error` field in response
- [x] 8.4 Test API CRUD operations with `block_on_error: true/false`

## 9. Logging Enhancements

- [x] 9.1 Add `block_on_error: true/false` field to "node execution started" log in `sceneNode.Execute()`
- [x] 9.2 Add `block_on_error: true/false` field to "node execution completed" log (success case)
- [x] 9.3 Add `block_on_error: true/false` field to "node execution failed" log (failure case)
- [x] 9.4 Log specific message "chain cancelled due to block_on_error" when chain is cancelled
- [x] 9.5 Include `node_id`, `node_name`, and original error in chain cancellation log

## 10. Testing and Validation

- [x] 10.1 Write unit test for `sceneNode.BlockOnError()` method
- [x] 10.2 Write unit test for HTTP node error handling with `block_on_error: true`
- [x] 10.3 Write integration test for chain cancellation when node with `block_on_error: true` fails
- [x] 10.4 Write integration test for chain continuation when node with `block_on_error: false` fails
- [x] 10.5 Write integration test for while loop step-level `block_on_error`
- [x] 10.6 Test backward compatibility: existing scenes without `block_on_error` behave identically
- [x] 10.7 Test database migration on existing database with nodes

## 11. Documentation

- [x] 11.1 Update API documentation to include `block_on_error` field in node schema
- [x] 11.2 Add example YAML configuration showing `block_on_error` usage
- [x] 11.3 Document the interaction between `block_on_error` and `fail_after_consecutive` in while loops

## 12. card.yaml 实战配置

- [x] 12.1 为"创建订单"节点添加 `block_on_error: true`（启动充电关键接口，失败需阻断）
- [x] 12.2 其余 HTTP 节点保持默认 `block_on_error: false`（不阻断）
