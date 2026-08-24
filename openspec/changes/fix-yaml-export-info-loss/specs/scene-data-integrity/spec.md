## MODIFIED Requirements

### Requirement: YAML export includes new node types and data sources
The YAML export SHALL be produced by the backend `/api/v1/scenes/export` endpoint (not by frontend `DagFlow.vue`), SHALL serialize group and timer nodes with their full config, SHALL include a `data_sources` section if the scene has linked data sources, and SHALL preserve node `block_on_error` and scene `default_timeout`.

#### Scenario: Export scene with group node
- **WHEN** user exports YAML from a scene containing a group node
- **THEN** the YAML includes the group node with type, node_ids (as names), and loop_count

#### Scenario: Export scene with data sources
- **WHEN** user exports YAML from a scene with linked data sources
- **THEN** the YAML includes a `data_sources` section listing each data source name and columns/rows

#### Scenario: Export preserves block_on_error
- **WHEN** a node has `block_on_error` set to true
- **THEN** the exported YAML node SHALL include `block_on_error: true`

#### Scenario: Export preserves default_timeout
- **WHEN** the scene has a non-zero `default_timeout`
- **THEN** the exported YAML SHALL include `default_timeout` at scene level with the same value

## ADDED Requirements

### Requirement: YAML 导入记录节点 lifecycle 分节
The YAML import handler SHALL record each node's source section by setting `Node.Lifecycle` to `setup` for nodes imported from the `setup:` block, `teardown` for nodes from the `teardown:` block, and empty string for nodes from the `nodes:` block.

#### Scenario: Import records setup section
- **WHEN** YAML contains a `setup:` block with a node `{name: A, type: generator}`
- **THEN** the imported Node A SHALL have `lifecycle = "setup"` and `type = "generator"` (真实类型不变)

#### Scenario: Import records teardown section
- **WHEN** YAML contains a `teardown:` block with a node `{name: B, type: http}`
- **THEN** the imported Node B SHALL have `lifecycle = "teardown"` and `type = "http"`

#### Scenario: Import records main section
- **WHEN** YAML contains a `nodes:` block with a node `{name: C, type: while}`
- **THEN** the imported Node C SHALL have `lifecycle = ""` (main)

#### Scenario: Export reconstructs sections from lifecycle
- **WHEN** exporting a scene whose nodes have `lifecycle` set to `setup`/`teardown`/`""`
- **THEN** the exported YAML SHALL place nodes into `setup:`/`teardown:`/`nodes:` blocks according to their `lifecycle` value (NOT according to `type`)
