## MODIFIED Requirements

### Requirement: Group execution semantics
When the DAG executor encounters a Group node, it SHALL execute the child nodes in the order specified by `node_ids`, repeating the sequence `loop_count` times. Group child execution SHALL use the shared child-chain runner (runChildChain) with group-style soft-failure semantics: a child reporting failure via Output.Error does not abort the chain, and the first child soft failure SHALL surface on the group's Output.Error. After each child step, the input variable map SHALL be refreshed from the executor's shared variable scope, so variables extracted by one child are immediately visible to subsequent children within the same chain pass. The Group's sync/async mode determines whether downstream nodes wait for all loops to complete:
- **Sync Group**: downstream nodes wait until all loop iterations finish
- **Async Group**: downstream nodes proceed immediately after the Group starts its first iteration

#### Scenario: Sync group with loop_count=2
- **WHEN** a sync Group [D1→D2→D3] has loop_count=2
- **THEN** the execution order is D1→D2→D3→D1→D2→D3, and downstream nodes wait until the second D3 completes

#### Scenario: Async group does not block downstream
- **WHEN** an async Group [D1→D2] has loop_count=3
- **THEN** downstream nodes start immediately after the Group begins, while D1→D2 repeats 3 times in the background

#### Scenario: Group with single child node
- **WHEN** a Group contains only one child node [D1] with loop_count=5
- **THEN** D1 is executed 5 times sequentially (equivalent to setting LoopCount=5 on D1 directly)

#### Scenario: Child extract visible to next child within a pass
- **WHEN** a Group [A→B] where A extracts variable `token` and B's request references `${token}`
- **THEN** B reads the value extracted by A in the same chain pass (variables refreshed after each child step)

#### Scenario: Child soft failure does not abort the group
- **WHEN** a Group [A→D→B] where D reports a soft failure via Output.Error
- **THEN** B still executes and the group's Output.Error carries D's error
