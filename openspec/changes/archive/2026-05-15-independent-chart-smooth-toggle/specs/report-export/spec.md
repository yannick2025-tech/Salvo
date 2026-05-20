## ADDED Requirements

### Requirement: Exported HTML Report Chart Toggle Independence

**MUST** update the exported HTML report template to support independent smooth/step toggle controls for each chart, matching the behavior of online pages.

As a user viewing an exported HTML report offline
I want to independently control each chart's display mode
So that I can analyze the report with the same flexibility as the online version

#### Scenario: Exported report includes toggle buttons for all charts

- **WHEN** user exports a report as enhanced HTML file
- **THEN** generated HTML **MUST** include smooth/step toggle buttons for:
  - Error rate trend chart
  - QPS trend chart
  - Latency trend chart
  - Each node detail chart

#### Scenario: Exported report charts toggle independently

- **WHEN** user clicks "阶梯" on latency trend chart in exported HTML
- **THEN** only latency trend chart **MUST** switch to step mode
- AND error rate, QPS, and all node charts **MUST** remain in their current modes
- AND JavaScript console **MUST NOT** show errors

#### Scenario: Exported report uses JavaScript object for state management

- **WHEN** exported HTML report initializes
- **THEN** it **MUST** create a `chartTypes` object to store per-chart state:
  ```javascript
  const chartTypes = {
    errorRate: 'smooth',
    qps: 'smooth',
    latTrend: 'smooth',
    node0: 'smooth',
    node1: 'smooth',
    // ... dynamic per node
  };
  ```
- AND each `switchChartType(chartId, type)` function call **MUST** update only the specified chart's state
- AND corresponding chart **MUST** re-render using `echarts.setOption()` with updated `smooth` and `step` properties

#### Scenario: Exported report toggle buttons match online styling

- **WHEN** user views toggle buttons in exported HTML
- **THEN** they **MUST** visually match online version (same CSS classes, colors, spacing)
- AND hover effects **MUST** work identically (background change on hover)
- AND active state **MUST** show primary color background with white text
