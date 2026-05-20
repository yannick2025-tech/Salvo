# Generator Function Selector & IF-ELSE Branch Node Design

## Overview

Two enhancements to the Salvo scene editor:

1. **Generator Function Selector** — expose built-in generators in the GUI with a two-level categorized dropdown, replacing the raw JSON textarea experience.
2. **IF-ELSE Branch Node** — add a new `if-else` node type for explicit dual-branch routing, alongside the existing `condition` node.

---

## 1. Generator Function Selector

### 1.1 Problem

The current generator configuration is a raw JSON textarea where users must hand-write JSON Schema. This is error-prone and requires knowledge of the internal generator system.

### 1.2 Solution

Add a `+ Add Generator` button next to the generator textarea. Clicking it opens a two-level categorized dropdown. Selecting a generator inserts its JSON Schema template into the textarea.

### 1.3 Backend: Generator Catalog API

**Endpoint**: `GET /api/generators`

**Response**:

```json
{
  "categories": [
    {
      "key": "string",
      "label": "String",
      "generators": [
        {
          "name": "uuid",
          "label": "UUID v4",
          "description": "Generate random UUID v4",
          "schema_template": { "type": "string", "format": "uuid" },
          "params": []
        },
        {
          "name": "email",
          "label": "Email",
          "description": "Generate random email address",
          "schema_template": { "type": "string", "format": "email" },
          "params": []
        },
        {
          "name": "random-string",
          "label": "Random String",
          "description": "Random alphanumeric string",
          "schema_template": { "type": "string", "minLength": 8, "maxLength": 16 },
          "params": [
            { "key": "minLength", "type": "integer", "default": 8 },
            { "key": "maxLength", "type": "integer", "default": 16 }
          ]
        },
        {
          "name": "enum-string",
          "label": "Enum String",
          "description": "Pick from enum values",
          "schema_template": { "type": "string", "enum": ["option1", "option2"] },
          "params": [
            { "key": "enum", "type": "array", "required": true }
          ]
        },
        {
          "name": "format-string",
          "label": "Format String",
          "description": "Formatted string (ipv4/ipv6/url/date etc.)",
          "schema_template": { "type": "string", "format": "ipv4" },
          "params": [
            { "key": "format", "type": "string", "enum": ["ipv4", "ipv6", "hostname", "uri", "url", "byte", "password"], "required": true }
          ]
        }
      ]
    },
    {
      "key": "number",
      "label": "Number",
      "generators": [
        {
          "name": "random-int",
          "label": "Random Integer",
          "description": "Random integer in range",
          "schema_template": { "type": "integer", "minimum": 0, "maximum": 100 },
          "params": [
            { "key": "minimum", "type": "integer", "default": 0 },
            { "key": "maximum", "type": "integer", "default": 100 }
          ]
        },
        {
          "name": "increment-int",
          "label": "Increment Integer",
          "description": "Auto-incrementing integer",
          "schema_template": { "type": "integer", "minimum": 0 },
          "params": [
            { "key": "minimum", "type": "integer", "default": 0 }
          ]
        },
        {
          "name": "random-float",
          "label": "Random Float",
          "description": "Random float in range",
          "schema_template": { "type": "number", "minimum": 0, "maximum": 100, "multipleOf": 0.01 },
          "params": [
            { "key": "minimum", "type": "number", "default": 0 },
            { "key": "maximum", "type": "number", "default": 100 },
            { "key": "multipleOf", "type": "number", "default": 0.01 }
          ]
        }
      ]
    },
    {
      "key": "boolean",
      "label": "Boolean",
      "generators": [
        {
          "name": "random-bool",
          "label": "Random Boolean",
          "description": "Random true/false",
          "schema_template": { "type": "boolean" },
          "params": []
        },
        {
          "name": "weighted-bool",
          "label": "Weighted Boolean",
          "description": "Boolean with weighted true ratio",
          "schema_template": { "type": "boolean" },
          "params": [
            { "key": "trueWeight", "type": "number", "default": 0.5 }
          ]
        }
      ]
    },
    {
      "key": "composite",
      "label": "Composite",
      "generators": [
        {
          "name": "array",
          "label": "Array",
          "description": "Array of generated items",
          "schema_template": { "type": "array", "minItems": 1, "maxItems": 5 },
          "params": [
            { "key": "minItems", "type": "integer", "default": 1 },
            { "key": "maxItems", "type": "integer", "default": 5 }
          ]
        },
        {
          "name": "object",
          "label": "Object",
          "description": "Object with generated properties",
          "schema_template": { "type": "object" },
          "params": []
        }
      ]
    }
  ]
}
```

### 1.4 Frontend: Dropdown Selector

- Button `+ Add Generator` placed next to the generator textarea label
- Dropdown shows two-level menu: category → generator name
- On selection, inserts the `schema_template` as a new field in the generator JSON
- The field name is auto-generated from the generator name (e.g., `email_1`, `age_1`) or user can customize
- Existing textarea remains editable for advanced users

### 1.5 Files to Modify

**Backend**:
- `internal/api/handler.go` — add `ListGenerators` handler
- `internal/api/dto/` — add `GeneratorCatalogResponse` DTO
- `internal/generator/builtin/builtin.go` — add `Catalog()` function returning structured metadata

**Frontend**:
- `web/app/src/views/scenes/SceneDetailPage.vue` — add dropdown selector component

---

## 2. IF-ELSE Branch Node

### 2.1 Problem

The current `condition` node only evaluates a single expression. IF-ELSE routing requires manually creating two edges with complementary conditions (e.g., `"true"` and `"false"`), which is unintuitive.

### 2.2 Solution

Add a new `if-else` node type that explicitly models dual-branch routing. The existing `condition` node is retained for simple conditional edge evaluation.

### 2.3 Backend: Node Type & Edge Convention

**New node type**: `NodeTypeIfElse = "if-else"`

**if-else node config**:
```json
{
  "expr": "${order_id} != ''"
}
```

**Edge convention**: if-else node's outgoing edges use special condition markers:
- `condition: "__if_true__"` — the TRUE branch
- `condition: "__if_false__"` — the FALSE branch

**DAG executor behavior**: When the executor encounters an if-else node, it evaluates `expr` and only traverses the matching branch edge. This is identical to how `EdgeCondition` works today, but the condition values are standardized.

**YAML example**:
```yaml
nodes:
  - name: Check Payment
    type: if-else
    config:
      expr: "${order_id} != ''"

edges:
  - from: Check Payment
    to: Pay Order
    condition: "__if_true__"
  - from: Check Payment
    to: Skip Payment
    condition: "__if_false__"
```

### 2.4 Frontend: IF-ELSE Node UI

- Add `+ IF-ELSE` button in the DAG section header
- if-else node displays with a diamond-shaped icon (◇) to distinguish from condition node
- Configuration panel contains:
  - Node name input
  - Condition expression input
- When connecting edges FROM an if-else node, the edge displays `TRUE` or `FALSE` label
- The DAG canvas shows two distinct output ports on the if-else node

### 2.5 Files to Modify

**Backend**:
- `internal/store/model/model.go` — add `NodeTypeIfElse = "if-else"` constant
- `internal/runner/runner.go` — add `case model.NodeTypeIfElse` handling in `sceneNode.Execute()`
- `internal/core/dag/executor.go` — no change needed (existing EdgeCondition mechanism handles it)
- `internal/api/handler.go` — ensure if-else nodes are properly serialized in YAML import/export

**Frontend**:
- `web/app/src/views/scenes/SceneDetailPage.vue` — add if-else node type, button, config panel, edge labels

---

## 3. Default YAML Template Update

Update the default scene YAML template to use the new if-else node instead of condition node for the payment branch:

```yaml
  - name: Check Payment
    type: if-else
    config:
      expr: "${order_id} != ''"

edges:
  - from: Check Payment
    to: Pay Order
    condition: "__if_true__"
  - from: Check Payment
    to: Skip Payment
    condition: "__if_false__"
```

---

## 4. Scope & Non-Goals

**In scope**:
- Generator catalog API and frontend dropdown
- if-else node type (backend + frontend)
- Default YAML template update

**Out of scope** (future work):
- Lua custom generator in the dropdown (requires Lua plugin system integration)
- Visual DAG editor with drag-and-drop nodes
- Generator parameter form (beyond template insertion)
