## 1. Backend: Variable Enhancement

- [x] 1.1 Add recursive resolution to `variable.ResolveString`: support nested `${var}` references up to 10 levels deep, return error on circular reference
- [x] 1.2 Add unit tests for nested resolution: A→B→C, circular detection, max depth exceeded, expression concatenation
- [x] 1.3 Add variable CRUD API endpoints: `GET /api/scenes/:id/variables`, `PUT /api/scenes/:id/variables` (batch update)
- [x] 1.4 Ensure `buildScope()` in runner.go applies recursive resolution when building the variable scope

## 2. Backend: CSV Data Source

- [x] 2.1 Create `DataSource` model in `internal/store/model/model.go`: ID, SceneID, Name, FileName, Columns (JSON), Rows (JSON), RowCount, CreatedAt
- [x] 2.2 Create `data_sources` table migration (SQLite ALTER TABLE)
- [x] 2.3 Implement `DataSourceRepo` with Create, GetBySceneID, GetByName, Delete methods
- [x] 2.4 Implement CSV upload API: `POST /api/scenes/:id/data-sources` (multipart form, parse CSV, validate headers, store rows as JSON)
- [x] 2.5 Implement CSV download/preview API: `GET /api/scenes/:id/data-sources/:dsId` (return metadata + first 5 rows)
- [x] 2.6 Implement data source delete API: `DELETE /api/scenes/:id/data-sources/:dsId`
- [x] 2.7 Create `RowIterator` in `internal/runner/datasource.go`: atomic row index, `Next() map[string]string`, wrap-around on exhaustion
- [x] 2.8 Integrate RowIterator into Runner: on Run() start, create iterators for all scene data sources; on each chain iteration, inject current row values into variable scope as `${datasource_name.column_name}`
- [x] 2.9 Add 10MB file size limit validation on upload
- [x] 2.10 Add file name validation: must match `[a-zA-Z0-9_]+` (letters, digits, underscores only)
- [x] 2.11 Add duplicate column name detection on CSV parse
- [x] 2.12 Write unit tests for RowIterator: sequential access, wrap-around, concurrent access (race detector)

## 3. Backend: Group Node Execution

- [x] 3.1 Add `NodeTypeGroup` execution case in `sceneNode.Execute()`: parse config for `node_ids` and `loop_count`
- [x] 3.2 Implement Group expansion in DAG executor: when encountering a Group node, execute child nodes in order, repeat `loop_count` times
- [x] 3.3 Implement async Group behavior: if Group node is async, signal downstream immediately and run child loops in background goroutine
- [x] 3.4 Add validation: Group `node_ids` must reference existing nodes in the same scene
- [x] 3.5 Add validation: Group nodes cannot be nested (a Group cannot contain another Group)
- [x] 3.6 Write unit tests for Group execution: sync loop, async loop, single child, loop_count=1

## 4. Backend: Timer Node Execution

- [x] 4.1 Add `NodeTypeTimer` constant to model.go (if not already defined)
- [x] 4.2 Add `timer` execution case in `sceneNode.Execute()`: parse config for `mode` and `seconds`
- [x] 4.3 Implement delay timer: start goroutine with `time.After(duration)`, then trigger downstream nodes via signal channel
- [x] 4.4 Implement interval timer: start goroutine with `time.NewTicker(duration)`, repeatedly trigger downstream nodes until context cancelled
- [x] 4.5 Ensure timer goroutines are cleaned up on context cancellation (test stop)
- [x] 4.6 Timer nodes SHALL always be async — never block the DAG main chain
- [x] 4.7 Write unit tests: delay trigger fires at correct time, interval fires repeatedly, context cancellation stops timer

## 5. Backend: YAML Import/Export Extension

- [x] 5.1 Extend `yamlScene` struct to include `data_sources` field
- [x] 5.2 Extend `yamlNode` struct to support `group` type with `node_ids` (names) and `loop_count` in config
- [x] 5.3 Extend `yamlNode` struct to support `timer` type with `mode` and `seconds` in config
- [x] 5.4 Update `ImportYAML` handler: parse data_sources section, link existing data sources by name
- [x] 5.5 Update `ImportYAML` handler: resolve group node_ids from names to IDs
- [x] 5.6 Update YAML export in DagFlow.vue: serialize group/timer nodes, include data_sources section

## 6. Frontend: Variable Editing Panel

- [x] 6.1 Add variable editing section to SceneDetailPage.vue (below scene info, above DAG editor)
- [x] 6.2 Implement variable list with key-value input rows, add/delete buttons
- [x] 6.3 Implement auto-save on blur (call POST /api/v1/scenes/variables/batch-set)
- [x] 6.4 Display variables parsed from YAML in import dialog preview

## 7. Frontend: Data Source Management

- [x] 7.1 Add data source management section to SceneDetailPage.vue
- [x] 7.2 Implement CSV file upload button with drag-and-drop support
- [x] 7.3 Implement data source list showing name, column count, row count
- [x] 7.4 Implement data source preview (click to show first 5 rows in a table)
- [x] 7.5 Implement data source delete with confirmation dialog
- [x] 7.6 Add file size validation on frontend (10MB limit, show error before upload)

## 8. Frontend: LoopCount Configuration

- [x] 8.1 Add "Loop Count" input field to node editor form (all node types)
- [x] 8.2 Default value: 1, minimum: 1, only positive integers
- [x] 8.3 Display loop count badge on DAG canvas nodes when > 1 (e.g., "HTTP x3")

## 9. Frontend: Group Node Visualization

- [x] 9.1 Add "+ Group" button in DAG section header
- [x] 9.2 Implement Group node type in DagFlow.vue: render as collapsible node with group icon
- [x] 9.3 Implement collapsed view: show group name + loop count (e.g., "Login Flow x3")
- [x] 9.4 Implement expand/collapse toggle on double-click
- [x] 9.5 Implement expanded view: bordered region containing child nodes with group label
- [x] 9.6 Implement Group node config panel: child node selector (multi-select from existing nodes), loop count input
- [x] 9.7 Ensure external edges connect to Group node ports, not to child nodes directly

## 10. Frontend: Timer Node Visualization

- [x] 10.1 Add "+ Timer" button in DAG section header
- [x] 10.2 Implement Timer node type in DagFlow.vue: render with clock icon and label (e.g., "⏱ 30s delay")
- [x] 10.3 Implement Timer node config panel: mode dropdown (delay/interval), seconds input
- [x] 10.4 Timer nodes display with output port only (no input dependency line from main chain)

## 11. Integration Testing

- [x] 11.1 End-to-end test: create scene with variables, run test, verify variable resolution in HTTP requests
- [x] 11.2 End-to-end test: upload CSV, create scene referencing data source columns, run test, verify row iteration
- [x] 11.3 End-to-end test: create Group node with loop_count, run test, verify child nodes execute correct number of times
- [x] 11.4 End-to-end test: create timer node (delay mode), run test, verify downstream triggers at correct time
- [x] 11.5 End-to-end test: create timer node (interval mode), run test, verify periodic triggers
- [x] 11.6 End-to-end test: YAML import/export round-trip with all new node types and data sources
