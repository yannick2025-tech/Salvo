## ADDED Requirements

### Requirement: ELK layout engine replaces dagre
The system SHALL use ELK (Eclipse Layout Kernel) via elkjs as the DAG layout engine, replacing dagre. The `buildLayout()` function in DagFlow.vue SHALL call ELK's `layout()` method with the `layered` algorithm and DOWN direction.

#### Scenario: Basic DAG layout with ELK
- **WHEN** a DAG with 5+ nodes and edges is loaded
- **THEN** the system SHALL produce a layered top-to-bottom layout with no node overlaps using ELK

#### Scenario: ELK layout with variable-size nodes
- **WHEN** a DAG contains an expanded group node (height > 90px) or while node
- **THEN** the system SHALL pass each node's actual computed dimensions to ELK, and ELK SHALL produce a layout where no nodes overlap

### Requirement: Variable node dimensions for layout
The system SHALL compute and pass actual node dimensions (width, height) to the layout engine. For expanded group nodes, the height SHALL be calculated from the number of child nodes. For expanded while nodes, the height SHALL be calculated from the number of steps. For collapsed nodes, the default size SHALL be 280×56.

#### Scenario: Expanded group node sizing
- **WHEN** a group node is expanded showing 3 child nodes
- **THEN** the layout engine SHALL receive a height value that accounts for all children plus padding

#### Scenario: Collapsed node sizing
- **WHEN** all nodes are in collapsed state
- **THEN** each node SHALL report dimensions of 280×56 to the layout engine

### Requirement: One-click beautify with no overlaps
The "美化布局" button SHALL re-run ELK layout and fit the view. After beautification, no two nodes SHALL overlap and no edge SHALL be hidden behind a node.

#### Scenario: Beautify a complex DAG with IF-ELSE branches
- **WHEN** user clicks "美化布局" on a DAG with IF-ELSE conditional branches and 8+ nodes
- **THEN** all nodes SHALL be positioned without overlaps, TRUE/FALSE edges SHALL be clearly visible, and the view SHALL be centered with padding

### Requirement: Remove dagre dependency
The system SHALL remove the `dagre` npm dependency and add `elkjs` as a dependency.

#### Scenario: Package.json after migration
- **WHEN** the migration is complete
- **THEN** package.json SHALL list elkjs as a dependency and SHALL NOT list dagre
