# independent-chart-toggle Specification

## Purpose
TBD - created by archiving change independent-chart-smooth-toggle. Update Purpose after archive.
## Requirements
### Requirement: Independent Chart Type Toggle for Dashboard

**MUST** provide independent smooth/step toggle controls for each line chart in the Dashboard page, ensuring that toggling one chart's display mode does not affect other charts.

As a user viewing the Dashboard
I want to independently control the smooth/step display mode for each chart
So that I can choose the best visualization style for each individual metric

#### Scenario: Dashboard displays toggle buttons for all line charts

- **WHEN** user loads the Dashboard page
- **THEN** system **MUST** display smooth/step toggle buttons for all line charts including:
  - Error rate trend chart
  - QPS trend chart (if exists)
  - Latency trend chart (if exists)
  - Each node detail chart

#### Scenario: User toggles error rate chart independently

- **WHEN** user clicks "阶梯" button on error rate chart
- **THEN** only the error rate chart **MUST** switch to step mode
- AND all other charts (QPS, latency, node charts) **MUST** remain in their current mode unchanged
- AND the error rate toggle button **MUST** show "阶梯" as active state

#### Scenario: User toggles node chart independently

- **WHEN** user clicks "平滑" button on node-3 chart
- **THEN** only node-3 chart **MUST** switch to smooth mode
- AND node-1, node-2, and other node charts **MUST** remain in their current modes unchanged
- AND global charts (error rate, QPS, latency) **MUST** remain unaffected

#### Scenario: Chart type state persists during session

- **WHEN** user toggles multiple charts to different modes
- **THEN** each chart **MUST** maintain its独立状态 until page refresh or manual toggle
- AND switching between browser tabs **MUST NOT** reset chart states

---

### Requirement: Independent Chart Type Toggle for Report Detail Page

**SHALL** provide independent smooth/step toggle controls for each line chart in the Report Detail Page, with per-chart state isolation.

As a user viewing a test report
I want to independently control the smooth/step mode for each report chart
So that I can analyze different metrics with optimal visualization styles

#### Scenario: Report detail page shows toggle buttons for all charts

- **WHEN** user opens a completed test report
- **THEN** system **SHALL** display smooth/step toggle buttons for:
  - Error rate trend chart
  - QPS trend chart
  - Latency trend chart (P50/P90/P95/P99)
  - Each node performance chart

#### Scenario: Toggling QPS chart does not affect latency chart

- **WHEN** user switches QPS chart from smooth to step
- **THEN** QPS chart **SHALL** immediately re-render in step mode
- AND latency trend chart **SHALL** remain in its previous mode (smooth or step)
- AND error rate chart **SHALL** remain unchanged
- AND all node charts **SHALL** remain unchanged

#### Scenario: Multiple node charts have independent states

- **WHEN** user sets node-0 to smooth, node-1 to step, and node-2 to smooth
- **THEN** each node chart **SHALL** display its respective mode correctly
- AND toggling any single node chart **SHALL** not alter others

---

### Requirement: Unified Toggle Button UI Consistency

**MUST** ensure all chart type toggle buttons follow consistent visual design and interaction patterns across Dashboard, Report Detail Page, and exported HTML reports.

As a user
I want toggle buttons to look and behave consistently everywhere
So that I can intuitively use them without re-learning

#### Scenario: Toggle buttons have consistent styling

- **WHEN** toggle buttons appear on any chart
- **THEN** they **MUST** use identical CSS classes: `.chart-type-toggle` container, `.type-btn` for buttons
- AND active button **MUST** have `.active` class with primary color background
- AND non-active buttons **MUST** have transparent background with border

#### Scenario: Toggle buttons positioned consistently

- **WHEN** toggle buttons are rendered
- **THEN** they **MUST** appear below the chart body (margin-top: 8px)
- AND **MUST** be horizontally centered within the chart card
- AND **MUST** contain exactly two buttons: "平滑" and "阶梯"

#### Scenario: Toggle interaction feedback is immediate

- **WHEN** user clicks a toggle button
- **THEN** target chart **MUST** re-render within 100ms showing new mode
- AND button active state **MUST** update instantly without delay

