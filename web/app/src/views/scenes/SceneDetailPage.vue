<template>
  <div class="scene-detail">
    <!-- ===== 紧凑工具栏 ===== -->
    <div class="toolbar">
      <div class="toolbar-left">
        <button class="btn-back" @click="$router.push('/scenes')">← 返回</button>
        <h2 v-if="scene">{{ scene.name }}</h2>
        <span v-if="scene" :class="['status-badge', scene.status]">{{ scene.status }}</span>
      </div>
      <div class="toolbar-right">
        <button class="btn-sm toolbar-btn" :disabled="!canWriteScene" :title="canWriteScene ? '场景设置（变量 / 数据源）' : '您当前的角色没有编辑权限'" @click="$router.push(`/scenes/${$route.params.id}/settings`)">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>
          设置
        </button>
        <button class="btn-outline btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有创建权限'" @click="showCopyModal = true">复制</button>
        <button class="btn-primary btn-sm" :disabled="!canRunScene" :title="canRunScene ? '' : '您当前的角色没有运行权限'" @click="showRunConfig = true">▶ 启动测试</button>
      </div>
    </div>

    <!-- ===== 主工作区：DAG 画布 ===== -->
    <div class="main-workspace">
      <div class="dag-section">
        <div class="section-header">
          <h3>DAG 请求流</h3>
          <div class="dag-actions">
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('setup')">+ 初始化</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('http')">+ HTTP</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('delay')">+ 延迟</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('condition')">+ 条件</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('if-else')">+ IF-ELSE</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('group')">+ 分组</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('timer')">+ 定时器</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('while')">+ While</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('parallel')">+ 并行</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('sub_flow')">+ 子流程</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('loop')">+ 循环</button>
            <button class="btn-sm" :disabled="!canWriteScene" :title="canWriteScene ? '' : '您当前的角色没有编辑权限'" @click="addNode('teardown')">+ 清理</button>
          </div>
        </div>

        <div class="dag-canvas" ref="canvasRef">
          <div v-if="nodes.length === 0" class="dag-empty">
            <div class="empty-icon">⬡</div>
            <p>暂无请求节点，点击上方按钮添加</p>
          </div>

          <DagFlow
            v-else
            :nodes="nodes"
            :edges="edges"
            :is-running="isSceneRunning"
            :run-id="currentRunId"
            :scene-id="route.params.id as string"
            @edit="editNode"
            @delete-node="handleDeleteNode"
            @add-edge="onDagAddEdge"
            @delete-edge="handleDeleteEdge"
            @update-edge="handleUpdateEdge"
            @node-select="selectNode"
            @node-position-update="saveNodePosition"
          />
        </div>
      </div>

      <!-- 节点配置侧边栏 -->
      <div v-if="selectedNode" class="config-sidebar">
        <div class="node-config-panel">
          <div class="panel-header">
            <h4>{{ selectedNode.name }} - 配置</h4>
            <button class="btn-close" @click="selectedNode = null"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
          </div>

      <div v-if="selectedNode.type === 'http' || selectedNode.type === 'setup' || selectedNode.type === 'teardown'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>请求方法</label>
          <select v-model="httpConfig.method" @change="saveNodeConfig">
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
            <option value="PATCH">PATCH</option>
          </select>
        </div>
        <div class="form-row">
          <label>URL</label>
          <input v-model="httpConfig.url" placeholder="http://localhost:9090/mock/api/users" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>Headers (JSON)</label>
          <textarea v-model="httpConfig.headers" rows="3" placeholder='{"Content-Type":"application/json","Authorization":"Bearer ${token}"}' @change="saveNodeConfig"></textarea>
        </div>
        <div class="form-row">
          <label>Body</label>
          <textarea v-model="httpConfig.body" rows="4" placeholder='{"email":"admin@example.com","password":"admin123"}' @change="saveNodeConfig"></textarea>
        </div>
        <div class="form-row inline">
          <label>超时(ms)</label>
          <input v-model="httpConfig.timeout" type="text" placeholder="数值或 ${variable} 引用" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>期望状态码</label>
          <input v-model="httpConfig.expect_status" type="text" placeholder="数值或 ${variable} 引用" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>变量提取 (JSON)</label>
          <textarea v-model="httpConfig.extract" rows="2" placeholder='{"token":"$.data.token","user_id":"$.data.user.id"}' @change="saveNodeConfig"></textarea>
        </div>
        <div class="form-row">
          <label>参数生成器</label>
          <div class="generator-selector">
            <div class="generator-dropdown-row">
              <select v-model="selectedGeneratorCategory" class="gen-category-select" @change="selectedGeneratorName = ''">
                <option value="">选择分类</option>
                <option v-for="cat in generatorCategories" :key="cat.key" :value="cat.key">{{ cat.label }}</option>
              </select>
              <select v-model="selectedGeneratorName" class="gen-name-select" @change="applyGenerator">
                <option value="">选择函数</option>
                <option v-for="gen in filteredGenerators" :key="gen.name" :value="gen.name">{{ gen.label }} - {{ gen.description }}</option>
              </select>
              <button class="btn-sm" @click="insertGeneratorToBody" :disabled="!selectedGeneratorName">插入</button>
            </div>
            <textarea v-model="httpConfig.generator" rows="3" placeholder='{"email":{"type":"string","format":"email"},"age":{"type":"integer","minimum":18,"maximum":65}}' @change="saveNodeConfig"></textarea>
          </div>
        </div>
      </div>

      <div v-else-if="selectedNode.type === 'delay'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>延迟时间(ms)</label>
          <input v-model="delayConfig.ms" type="text" placeholder="数值或 ${variable} 引用" @change="saveNodeConfig" />
        </div>
      </div>

      <div v-else-if="selectedNode.type === 'condition'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>条件表达式</label>
          <input v-model="conditionConfig.expr" placeholder='${status_code} == 200' @change="saveNodeConfig" />
        </div>
      </div>

      <div v-else-if="selectedNode.type === 'if-else'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>条件表达式</label>
          <input v-model="ifElseConfig.expr" placeholder='${order_id} != ""' @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>IF 分支目标 (true)</label>
          <select v-model="ifElseConfig.trueTarget" @change="saveIfElseBranches">
            <option value="">选择 IF 分支目标节点</option>
            <option v-for="n in availableIfElseTargets" :key="n.id" :value="n.id">{{ n.name }} ({{ nodeTypeLabel(n.type) }})</option>
          </select>
        </div>
        <div class="form-row">
          <label>ELSE 分支目标 (false)</label>
          <select v-model="ifElseConfig.falseTarget" @change="saveIfElseBranches">
            <option value="">选择 ELSE 分支目标节点</option>
            <option v-for="n in availableIfElseTargets" :key="n.id" :value="n.id">{{ n.name }} ({{ nodeTypeLabel(n.type) }})</option>
          </select>
        </div>
        <div class="form-row">
          <label class="hint-label">表达式求值为 true 时走 IF 分支，false 时走 ELSE 分支。</label>
        </div>
      </div>

      <div v-else-if="selectedNode.type === 'group'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row group-child-config">
          <label>子节点（按执行顺序排列）</label>
          <div class="group-available-nodes">
            <div v-for="n in availableGroupChildren" :key="n.id" class="child-node-check">
              <input type="checkbox" :value="n.id" v-model="groupConfig.node_ids" @change="onGroupChildrenChange" />
              <span>{{ n.name }} ({{ nodeTypeLabel(n.type) }})</span>
            </div>
          </div>
          <div v-if="groupOrderedChildren.length > 0" class="group-ordered-list">
            <div v-for="(childId, idx) in groupConfig.node_ids" :key="childId" class="ordered-child-row">
              <span class="order-index">{{ idx + 1 }}</span>
              <span class="order-arrow" v-if="idx < groupConfig.node_ids.length - 1">↓</span>
              <span class="order-name">{{ getNodeName(childId) }}</span>
              <div class="order-actions">
                <button class="order-btn" :disabled="idx === 0" @click="moveChildUp(idx)" title="上移">↑</button>
                <button class="order-btn" :disabled="idx === groupConfig.node_ids.length - 1" @click="moveChildDown(idx)" title="下移">↓</button>
                <button class="order-btn remove-btn" @click="removeChild(childId)" title="移除"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
              </div>
            </div>
          </div>
          <label class="hint-label">勾选子节点后可调整执行顺序，顺序决定内部 DAG 连线方向</label>
        </div>
        <div class="form-row inline">
          <label>循环次数</label>
          <input v-model="groupConfig.loop_count" type="text" placeholder="数值或 ${variable} 引用" @change="saveNodeConfig" />
        </div>
      </div>

      <div v-else-if="selectedNode.type === 'timer'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>模式</label>
          <select v-model="timerConfig.mode" @change="saveNodeConfig">
            <option value="delay">延迟执行（单次）</option>
            <option value="interval">间隔执行（循环）</option>
          </select>
        </div>
        <div class="form-row inline">
          <label>{{ timerConfig.mode === 'delay' ? '延迟时间(秒)' : '间隔时间(秒)' }}</label>
          <input v-model="timerConfig.seconds" type="text" placeholder="数值或 ${variable} 引用" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label class="hint-label">定时器节点始终异步执行</label>
        </div>
      </div>

      <!-- ➕ While 节点配置 -->
      <div v-else-if="selectedNode.type === 'while'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>退出条件</label>
          <div class="condition-rows">
            <div v-for="(cond, idx) in whileConfig.exit_conditions" :key="idx" class="condition-row">
              <input v-model="cond.variable" placeholder="变量" class="cond-input" @change="saveNodeConfig" />
              <select v-model="cond.operator" class="cond-op" @change="saveNodeConfig">
                <option value="==">==</option>
                <option value="!=">!=</option>
                <option value=">">&gt;</option>
                <option value="<">&lt;</option>
                <option value=">=">&gt;=</option>
                <option value="<=">&lt;=</option>
              </select>
              <input v-model="cond.value" placeholder="值" class="cond-input" @change="saveNodeConfig" />
              <button class="btn-icon btn-del-var" @click="removeWhileCondition(idx)" title="删除"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
            </div>
          </div>
          <button class="btn-sm drawer-add-btn" @click="addWhileCondition">+ 添加条件</button>
        </div>
        <div class="form-row inline">
          <label>轮询间隔(秒)</label>
          <input v-model.number="whileConfig.interval_seconds" type="number" min="1" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>最大迭代</label>
          <input v-model.number="whileConfig.max_iterations" type="number" min="1" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>最大时长(分)</label>
          <input v-model.number="whileConfig.max_duration_minutes" type="number" min="0" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>连续失败次数</label>
          <input v-model.number="whileConfig.fail_after_consecutive" type="number" min="0" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>失败消息</label>
          <input v-model="whileConfig.fail_message" placeholder="可选" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label class="hint-label">While 节点会循环执行子步骤，直到条件满足或达到上限</label>
        </div>
      </div>

      <!-- ➕ Parallel 节点配置 -->
      <div v-else-if="selectedNode.type === 'parallel'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>异步执行</label>
          <input type="checkbox" v-model="parallelConfig.async" true-value="true" false-value="false" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label class="hint-label">并行节点中的步骤将同时执行。启用异步后不会阻塞后续节点。</label>
        </div>
      </div>

      <!-- ➕ Sub_flow 节点配置 -->
      <div v-else-if="selectedNode.type === 'sub_flow'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>目标场景</label>
          <select v-model="subFlowConfig.scene_id" @change="saveNodeConfig">
            <option value="">选择场景</option>
            <option v-for="s in availableScenes" :key="s.id" :value="s.id">{{ s.name }}</option>
          </select>
        </div>
        <div class="form-row inline">
          <label>异步执行</label>
          <input type="checkbox" v-model="subFlowConfig.async" true-value="true" false-value="false" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label class="hint-label">子流程节点将执行另一个场景的全部 DAG 节点。异步时不等待完成。</label>
        </div>
      </div>

      <!-- ➕ Loop 节点配置 -->
      <div v-else-if="selectedNode.type === 'loop'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>循环次数</label>
          <input v-model.number="loopConfig.loop_count" type="number" min="1" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label class="hint-label">循环节点会重复执行子步骤指定的次数。</label>
        </div>
      </div>

      <div class="panel-footer">
        <button class="btn-primary btn-save-panel" @click="saveNodeConfig" :disabled="!selectedNode || !canWriteScene">保存配置</button>
      </div>
    </div>
    </div>
    </div>

    <div v-if="showRunConfig" class="modal-overlay" @click.self="showRunConfig = false">
      <div class="modal">
        <h3>启动测试</h3>
        <div v-if="nodes.length === 0" class="run-warning">
          ⚠ 当前场景没有配置 DAG 请求流，请先添加节点
        </div>
        <div class="form-group">
          <label>并发数</label>
          <input v-model="runConfig.workers" type="number" min="1" max="1000" step="1" @input="normalizeNumber($event, 'workers')" />
        </div>
        <div class="form-group">
          <label>运行模式</label>
          <CustomSelect
            v-model="runConfig.run_mode"
            :options="runModeOptions"
          />
        </div>
        <div v-if="runConfig.run_mode === 'count'" class="form-group">
          <label>总次数</label>
          <input v-model="runConfig.count" type="number" min="1" max="1000000" step="1" @input="normalizeNumber($event, 'count')" />
        </div>
        <div v-if="runConfig.run_mode === 'duration'" class="form-group">
          <label>持续时间(秒)</label>
          <input v-model="runConfig.duration" type="number" min="1" max="86400" step="1" @input="normalizeNumber($event, 'duration')" />
        </div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showRunConfig = false">取消</button>
          <button class="btn-primary" :disabled="nodes.length === 0 || !canRunScene" @click="handleStart">
            {{ nodes.length === 0 ? '无 DAG 节点' : '启动' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showCopyModal" class="modal-overlay" @click.self="showCopyModal = false">
      <div class="modal">
        <h3>复制场景</h3>
        <div class="form-group">
          <label>新场景名称</label>
          <input v-model="copyName" placeholder="输入新场景名称" />
        </div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showCopyModal = false">取消</button>
          <button class="btn-primary" @click="handleCopyScene">确认复制</button>
        </div>
      </div>
    </div>

    <div v-if="showNodeEditor" class="modal-overlay" @click.self="closeNodeEditor">
      <div class="modal">
        <h3>{{ editingNode ? '编辑节点' : '添加节点' }}</h3>
        <div class="form-group">
          <label>节点名称</label>
          <input v-model="nodeForm.name" placeholder="如: Login / GetUsers / CreateOrder" />
        </div>
        <div class="form-group">
          <label>节点类型</label>
          <select v-model="nodeForm.type" :disabled="!!editingNode">
            <option value="setup">Setup (初始化)</option>
            <option value="http">HTTP 请求</option>
            <option value="delay">延迟</option>
            <option value="condition">条件判断</option>
            <option value="if-else">IF-ELSE 分支</option>
            <option value="group">分组</option>
            <option value="timer">定时器</option>
            <option value="while">While 循环</option>
            <option value="parallel">并行执行</option>
            <option value="sub_flow">子流程</option>
            <option value="loop">循环</option>
            <option value="teardown">Teardown (清理)</option>
          </select>
        </div>
        <div v-if="!editingNode && nodes.length > 0" class="form-group">
          <label>连接到（父节点）</label>
          <select v-model="nodeForm.parentId">
            <option value="">不连接（作为起始节点）</option>
            <option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }} ({{ nodeTypeLabel(n.type) }})</option>
          </select>
          <span class="field-hint">选择新节点的上游父节点，留空则作为DAG起始节点</span>
        </div>
        <template v-if="nodeForm.type === 'http' || nodeForm.type === 'setup' || nodeForm.type === 'teardown'">
          <div class="form-group">
            <label>请求方法</label>
            <select v-model="nodeForm.httpMethod">
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="DELETE">DELETE</option>
            </select>
          </div>
          <div class="form-group">
            <label>URL</label>
            <input v-model="nodeForm.url" placeholder="http://localhost:9090/mock/api/users" />
          </div>
          <div class="form-group">
            <label>Headers (JSON)</label>
            <textarea v-model="nodeForm.headers" rows="2" placeholder='{"Content-Type":"application/json"}'></textarea>
          </div>
          <div class="form-group">
            <label>Body</label>
            <textarea v-model="nodeForm.body" rows="3" placeholder='{"key":"value"}'></textarea>
          </div>
          <div class="form-group">
            <label>变量提取 (JSON)</label>
            <textarea v-model="nodeForm.extract" rows="2" placeholder='{"token":"$.data.token"}'></textarea>
          </div>
        </template>
        <template v-if="nodeForm.type === 'delay'">
          <div class="form-group">
            <label>延迟时间(ms)</label>
            <input v-model="nodeForm.delayMs" type="text" placeholder="数值或 ${variable} 引用" />
          </div>
        </template>
        <template v-if="nodeForm.type === 'condition'">
          <div class="form-group">
            <label>条件表达式</label>
            <input v-model="nodeForm.conditionExpr" placeholder='${status_code} == 200' />
          </div>
        </template>
        <template v-if="nodeForm.type === 'if-else'">
          <div class="form-group">
            <label>条件表达式</label>
            <input v-model="nodeForm.conditionExpr" placeholder='${order_id} != ""' />
          </div>
        </template>
        <template v-if="nodeForm.type === 'generator'">
          <div class="form-group">
            <label>表达式</label>
            <input v-model="nodeForm.generatorExpr" placeholder='${__random(1, 100)}' />
          </div>
          <div class="form-group">
            <label>变量名</label>
            <input v-model="nodeForm.generatorVar" placeholder="my_var" />
          </div>
        </template>
        <template v-if="nodeForm.type === 'timer'">
          <div class="form-group">
            <label>模式</label>
            <select v-model="nodeForm.timerMode">
              <option value="delay">延迟执行（单次）</option>
              <option value="interval">间隔执行（循环）</option>
            </select>
          </div>
          <div class="form-group">
            <label>{{ nodeForm.timerMode === 'delay' ? '延迟时间(秒)' : '间隔时间(秒)' }}</label>
            <input v-model="nodeForm.timerSeconds" type="text" placeholder="数值或 ${variable} 引用" />
          </div>
        </template>
        <template v-if="nodeForm.type === 'group'">
          <div class="form-group">
            <label>循环次数</label>
            <input v-model="nodeForm.loop_count" type="text" placeholder="数值或 ${variable} 引用" />
            <span class="field-hint">节点执行的循环次数，默认 1 次</span>
          </div>
        </template>
        <template v-if="nodeForm.type === 'while'">
          <div class="form-group">
            <label>退出条件</label>
            <div v-for="(cond, ci) in nodeForm.whileExitConditions" :key="ci" class="exit-condition-row">
              <input v-model="cond.variable" placeholder="变量名" class="cond-input" />
              <select v-model="cond.operator" class="cond-select">
                <option value="equals">等于 (==)</option>
                <option value="not_equals">不等于 (!=)</option>
                <option value="greater_than">大于 (&gt;)</option>
                <option value="less_than">小于 (&lt;)</option>
                <option value="greater_than_or_equal">大于等于 (&gt;=)</option>
                <option value="less_than_or_equal">小于等于 (&lt;=)</option>
              </select>
              <input v-model="cond.value" placeholder="值" class="cond-input" />
              <button class="btn-icon" @click="nodeForm.whileExitConditions.splice(ci, 1)" title="删除"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
            </div>
            <button class="btn-sm" @click="nodeForm.whileExitConditions.push({ variable: '', operator: 'equals', value: '' })">+ 添加条件</button>
          </div>
          <div class="form-group">
            <label>循环间隔(秒)</label>
            <input v-model="nodeForm.whileIntervalSeconds" type="text" placeholder="数值或 ${variable} 引用" />
          </div>
          <div class="form-group">
            <label>最大迭代次数</label>
            <input v-model="nodeForm.whileMaxIterations" type="text" placeholder="数值或 ${variable} 引用" />
          </div>
          <div class="form-group">
            <label>最大持续时间(分钟)</label>
            <input v-model="nodeForm.whileMaxDurationMinutes" type="text" placeholder="0 表示无限制" />
          </div>
        </template>
        <template v-if="nodeForm.type === 'loop'">
          <div class="form-group">
            <label>循环次数</label>
            <input v-model="nodeForm.loop_count" type="text" placeholder="数值或 ${variable} 引用" />
            <span class="field-hint">节点执行的循环次数，默认 1 次</span>
          </div>
        </template>
        <template v-if="nodeForm.type !== 'generator' && nodeForm.type !== 'timer' && nodeForm.type !== 'group' && nodeForm.type !== 'while' && nodeForm.type !== 'loop' && nodeForm.type !== 'http' && nodeForm.type !== 'setup' && nodeForm.type !== 'teardown' && nodeForm.type !== 'delay' && nodeForm.type !== 'condition' && nodeForm.type !== 'if-else'">
          <div class="form-group">
            <label>循环次数</label>
            <input v-model="nodeForm.loop_count" type="text" placeholder="数值或 ${variable} 引用" />
            <span class="field-hint">节点执行的循环次数，默认 1 次</span>
          </div>
        </template>
        <div class="modal-actions">
          <button class="btn-secondary" @click="closeNodeEditor">取消</button>
          <button class="btn-primary" @click="handleSaveNode">{{ editingNode ? '保存' : '添加' }}</button>
        </div>
      </div>
    </div>

    <div v-if="toastMsg" class="toast" :class="toastType">{{ toastMsg }}</div>

    <!-- ===== 全屏 CSV 数据编辑器 ===== -->
    <div v-if="showDsPreview" class="csv-editor-overlay" @click.self="showDsPreview = false">
      <div class="csv-editor">
        <!-- 精简标题栏 -->
        <div class="csv-editor-header">
          <h3>{{ dsPreview?.file_name }}</h3>
          <button class="btn-icon modal-close" @click="showDsPreview = false"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
        </div>

        <!-- 表格区域（含底部浮动工具栏） -->
        <div class="csv-editor-body">
          <div class="csv-table-card">
          <table class="csv-table">
            <thead>
              <tr>
                <th class="col-num">#</th>
                <th v-for="(col, ci) in dsEditColumns" :key="ci" class="col-editable">
                  <input
                    class="col-name-input"
                    :value="col"
                    @change="dsRenameColumn(ci, ($event.target as HTMLInputElement).value)"
                  />
                  <button class="btn-icon col-del-btn" @click="dsDeleteColumn(ci)" title="删除列"><svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
                </th>
                <th class="col-action"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, ri) in pagedRows" :key="ri + (dsPage - 1) * dsPageSize">
                <td class="col-num">{{ (dsPage - 1) * dsPageSize + ri + 1 }}</td>
                <td v-for="col in dsEditColumns" :key="col">
                  <input
                    class="cell-input"
                    :value="row[col] || ''"
                    @change="dsUpdateCell((dsPage - 1) * dsPageSize + ri, col, ($event.target as HTMLInputElement).value)"
                  />
                </td>
                <td class="col-action">
                  <button class="btn-icon row-del-btn" @click="dsDeleteRow((dsPage - 1) * dsPageSize + ri)" title="删除行"><svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
                </td>
              </tr>
            </tbody>
          </table>
          </div>

          <!-- 底部浮动工具栏 -->
          <div class="csv-footer-bar">
            <span class="csv-meta">{{ dsEditColumns.length }} 列 · {{ dsEditRows.length }} 行</span>
            <div class="csv-pagination" v-if="totalDsPages > 1">
              <button class="btn-sm" :disabled="dsPage <= 1" @click="dsPage--">‹</button>
              <span class="page-info">{{ dsPage }} / {{ totalDsPages }}</span>
              <button class="btn-sm" :disabled="dsPage >= totalDsPages" @click="dsPage++">›</button>
              <select class="page-size-select" v-model.number="dsPageSize">
                <option :value="25">25/页</option>
                <option :value="50">50/页</option>
                <option :value="100">100/页</option>
                <option :value="200">200/页</option>
              </select>
            </div>
            <div style="flex:1"></div>
            <button class="btn-sm" @click="dsAddRow">+ 行</button>
            <button class="btn-sm" @click="dsAddColumn">+ 列</button>
            <button class="btn-primary btn-sm" @click="dsSaveEdit" :disabled="dsSaving">{{ dsSaving ? '保存中...' : '保存' }}</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showConfirm" class="modal-overlay" @click.self="showConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
        </div>
        <h3 class="confirm-title">确认删除</h3>
        <p class="confirm-msg">{{ confirmMessage }}</p>
        <div class="confirm-actions">
          <button class="btn-cancel" @click="showConfirm = false">取消</button>
          <button class="btn-danger-confirm" @click="confirmDeleteAction">确认删除</button>
        </div>
      </div>
    </div>

    <!-- ===== CSV 添加列弹窗 ===== -->
    <div v-if="showAddColumnModal" class="modal-overlay" @click.self="showAddColumnModal = false">
      <div class="modal">
        <h3>添加列</h3>
        <div class="form-group">
          <label>列名</label>
          <input v-model="newColumnName" placeholder="仅支持英文字母、数字、下划线" @keyup.enter="dsConfirmAddColumn" />
        </div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showAddColumnModal = false">取消</button>
          <button class="btn-primary" @click="dsConfirmAddColumn">确认添加</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getScene, createScene, startScene, batchSetVariables, listScenes, sceneStatus, listRuns } from '@/api/scene'
import { listNodes, addNode as apiAddNode, updateNode as apiUpdateNode, deleteNode as apiDeleteNode, listEdges, addEdge, deleteEdge } from '@/api/node'
import { listGenerators } from '@/api/generator'
import { listDataSources, uploadDataSource, deleteDataSource } from '@/api/datasource'
import { previewDataSource } from '@/api/datasource'
import type { DataSourceDTO, DataSourcePreviewDTO } from '@/api/datasource'
import type { SceneDTO, NodeDTO, EdgeDTO, GeneratorCategoryInfo, GeneratorInfo } from '@/types'
import { useAuthStore } from '@/stores/auth'
import DagFlow from './DagFlow.vue'
import CustomSelect from '@/components/CustomSelect.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const canWriteScene = computed(() => authStore.canAccess(['scene:write']))
const canRunScene = computed(() => authStore.canAccess(['scene:run']))

const scene = ref<SceneDTO | null>(null)
const nodes = ref<NodeDTO[]>([])
const edges = ref<EdgeDTO[]>([])
const selectedNode = ref<NodeDTO | null>(null)
const currentRunId = ref('')

const isSceneRunning = computed(() => {
  if (route.query.mode === 'exec') return true
  return scene.value?.status === 'running'
})

const showRunConfig = ref(false)
const showCopyModal = ref(false)
const showNodeEditor = ref(false)
const editingNode = ref<NodeDTO | null>(null)
const copyName = ref('')

const showVarPanel = ref(false)
const varEntries = ref<{ key: string; value: string }[]>([])

// Data source state
const dataSources = ref<DataSourceDTO[]>([])
const dsUploading = ref(false)
// CSV editor pagination
const dsPage = ref(1)
const dsPageSize = ref(50)
const totalDsPages = computed(() => Math.max(1, Math.ceil(dsEditRows.value.length / dsPageSize.value)))
const pagedRows = computed(() => {
  const start = (dsPage.value - 1) * dsPageSize.value
  return dsEditRows.value.slice(start, start + dsPageSize.value)
})

// Group node config
const groupConfig = reactive({ node_ids: [] as string[], loop_count: '1', async: false })
// Timer node config
const timerConfig = reactive({ mode: 'delay', seconds: '1' })
const whileConfig = reactive({ exit_conditions: [{ variable: '', operator: '==', value: '' }], interval_seconds: '1', max_iterations: '100', max_duration_minutes: '0', fail_after_consecutive: '0', fail_message: '' })
const parallelConfig = reactive({ async: false })
const subFlowConfig = reactive({ scene_id: '', async: false })
const loopConfig = reactive({ loop_count: '3' })

function parseVariables(sceneVars: string): { key: string; value: string }[] {
  try {
    const obj = JSON.parse(sceneVars)
    if (obj && typeof obj === 'object' && !Array.isArray(obj)) {
      return Object.entries(obj).map(([k, v]) => ({ key: k, value: String(v) }))
    }
  } catch { /* ignore */ }
  return []
}

function addVariableRow() {
  varEntries.value.push({ key: '', value: '' })
  showVarPanel.value = true
}

function removeVariableRow(idx: number) {
  confirmMessage.value = '确定要删除变量 "' + (varEntries.value[idx]?.key || '未命名') + '" 吗？'
  showConfirm.value = true
  pendingDeleteAction.value = 'var'
  pendingDeleteParam.value = idx
}

async function saveVariables() {
  const sceneId = route.params.id as string
  if (!sceneId) return
  const vars: Record<string, string> = {}
  for (const e of varEntries.value) {
    if (e.key.trim()) {
      vars[e.key.trim()] = e.value
    }
  }
  try {
    await batchSetVariables(sceneId, vars)
  } catch { /* ignore */ }
}

// Data source methods
async function fetchDataSources() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await listDataSources(id)
    if (resp.code === 0) {
      const items = Array.isArray(resp.data) ? resp.data : (resp.data?.items || [])
      dataSources.value = items
        .map((ds: any) => ({
          ...ds,
          columns: ds.columns || [],
          row_count: ds.row_count ?? 0,
        }))
        .filter((ds: any) => ds.file_name && (ds.columns?.length > 0 || ds.row_count > 0))
    }
  } catch (e) { /* ignore */ }
}

async function handleDsUpload(file: File) {
  if (file.size > 10 * 1024 * 1024) {
    showToast('文件大小不能超过 10MB', 'error')
    return
  }
  dsUploading.value = true
  try {
    const text = await file.text()
    const sceneId = route.params.id as string
    const resp = await uploadDataSource(sceneId, file.name, text)
    if (resp.code !== 0) {
      showToast(resp.message || '上传失败', 'error')
      return
    }
    showToast('上传成功', 'success')
    fetchDataSources()
  } catch (e) {
    showToast('上传失败', 'error')
  } finally {
    dsUploading.value = false
  }
}

async function handleDsDelete(dsId: string) {
  pendingDeleteAction.value = 'datasource'
  pendingDeleteParam.value = dsId
  confirmMessage.value = '确定要删除该数据源吗？此操作不可撤销。'
  showConfirm.value = true
}

function onDsFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files && input.files[0]) handleDsUpload(input.files[0])
  input.value = ''
}

// Data source preview
const showDsPreview = ref(false)
const dsPreview = ref<DataSourcePreviewDTO | null>(null)
const dsEditColumns = ref<string[]>([])
const dsEditRows = ref<Record<string, string>[]>([])
const dsSaving = ref(false)

async function handleDsPreview(ds: DataSourceDTO) {
  try {
    const resp = await previewDataSource(ds.id)
    if (resp.code === 0) {
      dsPreview.value = resp.data
      dsEditColumns.value = [...(resp.data.columns || [])]
      // 过滤掉所有字段均为空的无效行
      const raw = (resp.data.rows || []) as Record<string, string>[]
      dsEditRows.value = raw
        .filter(r => r && typeof r === 'object' && Object.keys(r).length > 0 && Object.values(r).some(v => v != null && v !== ''))
        .map(r => ({ ...r }))
      dsPage.value = 1
      showDsPreview.value = true
    }
  } catch { showToast('预览失败', 'error') }
}

function dsUpdateCell(ri: number, col: string, value: string) {
  if (dsEditRows.value[ri]) {
    dsEditRows.value[ri][col] = value
  }
}

function dsAddRow() {
  const newRow: Record<string, string> = {}
  for (const col of dsEditColumns.value) {
    newRow[col] = ''
  }
  dsEditRows.value.push(newRow)
}

function dsDeleteRow(ri: number) {
  pendingDeleteAction.value = 'csv-row'
  pendingDeleteParam.value = ri
  confirmMessage.value = '确定要删除第 ' + ((dsPage.value - 1) * dsPageSize.value + ri + 1) + ' 行吗？'
  showConfirm.value = true
}

function dsAddColumn() {
  newColumnName.value = ''
  showAddColumnModal.value = true
}

function dsConfirmAddColumn() {
  const name = newColumnName.value.trim()
  if (!name || !/^[a-zA-Z0-9_]+$/.test(name)) {
    showToast('列名只能包含英文字母、数字、下划线', 'error')
    return
  }
  if (dsEditColumns.value.includes(name)) {
    showToast('列名已存在', 'error')
    return
  }
  dsEditColumns.value.push(name)
  for (const row of dsEditRows.value) {
    row[name] = ''
  }
  showAddColumnModal.value = false
}

function dsRenameColumn(ci: number, newName: string) {
  const trimmed = newName.trim()
  if (!trimmed || !/^[a-zA-Z0-9_]+$/.test(trimmed)) {
    showToast('列名只能包含英文字母、数字、下划线', 'error')
    return
  }
  if (dsEditColumns.value.includes(trimmed) && dsEditColumns.value[ci] !== trimmed) {
    showToast('列名已存在', 'error')
    return
  }
  const oldName = dsEditColumns.value[ci]
  dsEditColumns.value[ci] = trimmed
  if (oldName !== trimmed) {
    for (const row of dsEditRows.value) {
      row[trimmed] = row[oldName] || ''
      delete row[oldName]
    }
  }
}

function dsDeleteColumn(ci: number) {
  pendingDeleteAction.value = 'csv-column'
  pendingDeleteParam.value = ci
  confirmMessage.value = '确定要删除列 "' + dsEditColumns.value[ci] + '" 吗？该列所有数据将被清除。'
  showConfirm.value = true
}

async function dsSaveEdit() {
  if (!dsPreview.value) return
  dsSaving.value = true
  try {
    const csvLines: string[] = [dsEditColumns.value.join(',')]
    for (const row of dsEditRows.value) {
      csvLines.push(dsEditColumns.value.map(c => row[c] || '').join(','))
    }
    const csvContent = csvLines.join('\n')
    const sceneId = route.params.id as string
    await uploadDataSource(sceneId, dsPreview.value.file_name, csvContent)
    showToast('保存成功', 'success')
    fetchDataSources()
    showDsPreview.value = false
  } catch {
    showToast('保存失败', 'error')
  } finally {
    dsSaving.value = false
  }
}

const toastMsg = ref('')
const toastType = ref('info')
const showConfirm = ref(false)
const confirmMessage = ref('')
// 通用确认删除状态（支持节点、数据源、CSV行/列、变量）
const pendingDeleteAction = ref<'node' | 'datasource' | 'csv-row' | 'csv-column' | 'var'>('node')
const pendingDeleteParam = ref<string | number>('')

// CSV 添加列弹窗
const showAddColumnModal = ref(false)
const newColumnName = ref('')

const runConfig = reactive({
  workers: 10,
  run_mode: 'count',
  count: 100,
  duration: 60,
})

const runModeOptions = [
  { value: 'count', label: '按次数' },
  { value: 'duration', label: '按时间' },
]

const nodeForm = reactive({
  name: '',
  type: 'http',
  httpMethod: 'GET',
  url: '',
  headers: '',
  body: '',
  delayMs: '1000',
  conditionExpr: '',
  extract: '',
  parentId: '',
  loop_count: '1',
  generatorExpr: '',
  generatorVar: '',
  timerMode: 'delay',
  timerSeconds: '1',
  whileExitConditions: [{ variable: '', operator: '==', value: '' }],
  whileIntervalSeconds: '1',
  whileMaxIterations: '100',
  whileMaxDurationMinutes: '0',
})

const editingConfig = reactive({ name: '' })
const httpConfig = reactive({
  method: 'GET',
  url: '',
  headers: '',
  body: '',
  timeout: '5000',
  expect_status: '200',
  extract: '',
  generator: '',
})
const delayConfig = reactive({ ms: '1000' })
const conditionConfig = reactive({ expr: '' })
const ifElseConfig = reactive({ expr: '', trueTarget: '', falseTarget: '' })

const generatorCategories = ref<GeneratorCategoryInfo[]>([])
const selectedGeneratorCategory = ref('')
const selectedGeneratorName = ref('')

const filteredGenerators = computed<GeneratorInfo[]>(() => {
  if (!selectedGeneratorCategory.value) return []
  const cat = generatorCategories.value.find(c => c.key === selectedGeneratorCategory.value)
  return cat?.generators || []
})

const availableIfElseTargets = computed(() => {
  if (!selectedNode.value) return []
  return nodes.value.filter(n => n.id !== selectedNode.value?.id)
})

const availableGroupChildren = computed(() => {
  if (!selectedNode.value) return []
  return nodes.value.filter(n => n.id !== selectedNode.value?.id && n.type !== 'group')
})

const groupOrderedChildren = computed(() => {
  const nodeMap = new Map(nodes.value.map(n => [n.id, n]))
  return groupConfig.node_ids.map(id => nodeMap.get(id)).filter(Boolean)
})

function getNodeName(nodeId: string): string {
  const node = nodes.value.find(n => n.id === nodeId)
  return node ? `${node.name} (${nodeTypeLabel(node.type)})` : nodeId
}

function onGroupChildrenChange() { saveNodeConfig() }

function moveChildUp(idx: number) {
  if (idx <= 0) return
  const arr = [...groupConfig.node_ids]
  ;[arr[idx - 1], arr[idx]] = [arr[idx], arr[idx - 1]]
  groupConfig.node_ids = arr
  saveNodeConfig()
}

function moveChildDown(idx: number) {
  if (idx >= groupConfig.node_ids.length - 1) return
  const arr = [...groupConfig.node_ids]
  ;[arr[idx], arr[idx + 1]] = [arr[idx + 1], arr[idx]]
  groupConfig.node_ids = arr
  saveNodeConfig()
}

function removeChild(childId: string) {
  groupConfig.node_ids = groupConfig.node_ids.filter(id => id !== childId)
  saveNodeConfig()
}

function addWhileCondition() {
  whileConfig.exit_conditions.push({ variable: '', operator: '==', value: '' })
  saveNodeConfig()
}

function removeWhileCondition(idx: number) {
  whileConfig.exit_conditions.splice(idx, 1)
  saveNodeConfig()
}

const availableScenes = computed(() => {
  return scenes.value.filter(s => s.id !== route.params.id)
})

const scenes = ref<{ id: string; name: string }[]>([])

async function fetchSceneList() {
  try {
    const resp = await listScenes({ limit: 100 })
    if (resp.code === 0) scenes.value = (resp.data.items || []).map((s: any) => ({ id: s.id, name: s.name }))
  } catch { /* ignore */ }
}

async function saveIfElseBranches() {
  if (!selectedNode.value) return
  const sceneId = route.params.id as string
  const nodeId = selectedNode.value.id
  
  const existingEdges = edges.value.filter(e => e.from_node === nodeId)
  for (const e of existingEdges) {
    await deleteEdge(e.id)
  }
  
  if (ifElseConfig.trueTarget) {
    await addEdge({ scene_id: sceneId, from_node: nodeId, to_node: ifElseConfig.trueTarget, condition: '__if_true__' })
  }
  if (ifElseConfig.falseTarget) {
    await addEdge({ scene_id: sceneId, from_node: nodeId, to_node: ifElseConfig.falseTarget, condition: '__if_false__' })
  }
  
  fetchEdges()
  showToast('分支配置已保存')
}

interface DagTreeNode {
  node: NodeDTO
  level: number
  children: { edge: EdgeDTO; child: DagTreeNode }[]
}

const dagTree = computed((): DagTreeNode[] => {
  if (nodes.value.length === 0) return []

  const nodeMap = new Map<string, NodeDTO>()
  for (const n of nodes.value) nodeMap.set(n.id, n)

  const outEdgesMap = new Map<string, EdgeDTO[]>()
  for (const e of edges.value) {
    outEdgesMap.set(e.from_node, [...(outEdgesMap.get(e.from_node) || []), e])
  }

  const rootNodes = nodes.value.filter(n => !edges.value.some(e => e.to_node === n.id))
  const visited = new Set<string>()

  function buildTree(nodeId: string, level: number): DagTreeNode | null {
    if (visited.has(nodeId)) return null
    visited.add(nodeId)
    
    const node = nodeMap.get(nodeId)
    if (!node) return null
    
    const childrenOut = outEdgesMap.get(nodeId) || []
    const children: { edge: EdgeDTO; child: DagTreeNode }[] = []
    
    for (const e of childrenOut) {
      const childTree = buildTree(e.to_node, level + 1)
      if (childTree) {
        children.push({ edge: e, child: childTree })
      }
    }

    return { node, level, children }
  }

  const result: DagTreeNode[] = []
  for (const root of rootNodes) {
    const tree = buildTree(root.id, 0)
    if (tree) result.push(tree)
  }

  for (const n of nodes.value) {
    if (!visited.has(n.id)) {
      result.push({ node: n, level: 0, children: [] })
    }
  }

  return result
})

function flattenDagLevels(): { nodes: NodeDTO[]; isBranch: boolean; branchFrom?: string; branchLabel?: string }[][] {
  const levels: { nodes: NodeDTO[]; isBranch: boolean; branchFrom?: string; branchLabel?: string }[][] = [[]]
  const added = new Set<string>()
  
  function addNodeToLevel(node: NodeDTO, levelIdx: number, branchInfo?: { from: string; label: string }) {
    while (levels.length <= levelIdx) levels.push([])
    if (!added.has(node.id)) {
      added.add(node.id)
      levels[levelIdx].push({
        nodes: [node],
        isBranch: !!branchInfo,
        branchFrom: branchInfo?.from,
        branchLabel: branchInfo?.label,
      })
    }
  }

  function traverse(treeNodes: DagTreeNode[], startLevel: number) {
    for (const tree of treeNodes) {
      addNodeToLevel(tree.node, startLevel)
      
      if (tree.children.length > 1) {
        for (const c of tree.children) {
          const label = c.edge.condition === '__if_true__' ? 'TRUE' : 
                       c.edge.condition === '__if_false__' ? 'FALSE' : 
                       c.edge.condition || ''
          addNodeToLevel(c.child.node, startLevel + 1, { from: tree.node.id, label })
          traverse([c.child], startLevel + 2)
        }
      } else if (tree.children.length === 1) {
        traverse([tree.children[0].child], startLevel + 1)
      }
    }
  }

  traverse(dagTree.value, 0)
  return levels
}

const dagLevels = computed(() => flattenDagLevels())

function getOutEdgesOfNode(nodeId: string): EdgeDTO[] {
  return edges.value.filter(e => e.from_node === nodeId)
}

function getInEdgeOfNode(nodeId: string): EdgeDTO | undefined {
  return edges.value.find(e => e.to_node === nodeId)
}

const sortedNodes = computed(() => {
  if (nodes.value.length === 0) return []

  const nodeMap = new Map<string, NodeDTO>()
  for (const n of nodes.value) nodeMap.set(n.id, n)

  const outEdges = new Map<string, EdgeDTO[]>()
  const inEdges = new Map<string, EdgeDTO[]>()
  for (const e of edges.value) {
    outEdges.set(e.from_node, [...(outEdges.get(e.from_node) || []), e])
    inEdges.set(e.to_node, [...(inEdges.get(e.to_node) || []), e])
  }

  const rootNodes = nodes.value.filter(n => !edges.value.some(e => e.to_node === n.id))

  const result: NodeDTO[] = []
  const visited = new Set<string>()

  function dfs(nodeId: string) {
    if (visited.has(nodeId)) return
    visited.add(nodeId)
    const n = nodeMap.get(nodeId)
    if (n) result.push(n)
    const children = outEdges.get(nodeId) || []
    for (const e of children) {
      dfs(e.to_node)
    }
  }

  for (const root of rootNodes) {
    dfs(root.id)
  }

  for (const n of nodes.value) {
    if (!visited.has(n.id)) result.push(n)
  }

  return result
})

function getEdgeCondition(fromId: string, toId: string): string {
  const edge = edges.value.find(e => e.from_node === fromId && e.to_node === toId)
  return edge?.condition || ''
}

function nodeIcon(type: string) {
  switch (type) {
    case 'setup': return '▶'
    case 'http': return '⇄'
    case 'delay': return '⏱'
    case 'condition': return '◇'
    case 'if-else': return '⑂'
    case 'loop': return '↻'
    case 'while': return '↺'
    case 'parallel': return '∥'
    case 'sub_flow': return '⊞'
    case 'teardown': return '■'
    default: return '○'
  }
}

function nodeTypeLabel(type: string) {
  switch (type) {
    case 'setup': return 'SETUP'
    case 'http': return 'HTTP'
    case 'delay': return 'DELAY'
    case 'condition': return 'CONDITION'
    case 'if-else': return 'IF-ELSE'
    case 'loop': return 'LOOP'
    case 'while': return 'WHILE'
    case 'parallel': return 'PARALLEL'
    case 'sub_flow': return 'SUBFLOW'
    case 'teardown': return 'TEARDOWN'
    default: return type.toUpperCase()
  }
}

function showToast(msg: string, type = 'info') {
  toastMsg.value = msg
  toastType.value = type
  setTimeout(() => { toastMsg.value = '' }, 3000)
}

function normalizeNumber(event: Event, field: 'workers' | 'count' | 'duration') {
  const input = event.target as HTMLInputElement
  const cursorPos = input.selectionStart
  const raw = input.value

  // Strip non-digits
  const cleaned = raw.replace(/[^0-9]/g, '')

  // Only update if content actually changed (avoids cursor jump)
  if (cleaned !== raw) {
    input.value = cleaned
    // Restore cursor position, clamped to new length
    const newPos = Math.min(cursorPos, cleaned.length)
    input.setSelectionRange(newPos, newPos)
  }

  // Update reactive value as string (no auto-correction during typing)
  if (field === 'workers') runConfig.workers = cleaned
  else if (field === 'count') runConfig.count = cleaned
  else if (field === 'duration') runConfig.duration = cleaned
}

function validateRunConfig(): string | null {
  const limits: Record<string, { min: number; max: number; label: string }> = {
    workers: { min: 1, max: 1000, label: '并发数' },
    count: { min: 1, max: 1000000, label: '总次数' },
    duration: { min: 1, max: 86400, label: '持续时间' },
  }

  // Always validate workers
  const w = parseInt(String(runConfig.workers), 10)
  if (isNaN(w) || w < 1) return `${limits.workers.label}必须为大于0的整数`
  if (w > limits.workers.max) return `${limits.workers.label}不能超过${limits.workers.max}`

  if (runConfig.run_mode === 'count') {
    const c = parseInt(String(runConfig.count), 10)
    if (isNaN(c) || c < 1) return `${limits.count.label}必须为大于0的整数`
    if (c > limits.count.max) return `${limits.count.label}不能超过${limits.count.max}`
  } else {
    const d = parseInt(String(runConfig.duration), 10)
    if (isNaN(d) || d < 1) return `${limits.duration.label}必须为大于0的整数`
    if (d > limits.duration.max) return `${limits.duration.label}不能超过${limits.duration.max}`
  }
  return null
}

async function fetchScene() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await getScene(id)
    if (resp.code === 0) {
      scene.value = resp.data
      if (resp.data.variables) {
        varEntries.value = parseVariables(resp.data.variables)
      }
      // If scene is running and we don't have a runId, fetch status to get it
      if (resp.data.status === 'running' && !currentRunId.value) {
        fetchRunId(id)
      }
      // If entered via mode=exec but scene is not running in DB, still try to get runId
      // (scene may have just finished, we want to show final status)
      if (route.query.mode === 'exec' && !currentRunId.value) {
        fetchRunId(id)
      }
      // Clear runId if scene is no longer running and not in exec mode
      if (resp.data.status !== 'running' && !route.query.mode && currentRunId.value) {
        currentRunId.value = ''
      }
    }
  } catch { /* ignore */ }
}

async function fetchRunId(sceneId: string) {
  try {
    const resp = await sceneStatus(sceneId)
    if (resp.code === 0 && resp.data?.run_id) {
      currentRunId.value = resp.data.run_id
      return
    }
  } catch { /* scene may not be running */ }

  // Fallback: get most recent run from run history
  try {
    const resp = await listRuns({ scene_id: sceneId, limit: 1 })
    if (resp.code === 0 && resp.data?.items?.length) {
      const latestRun = resp.data.items[0]
      if (latestRun.run_id) {
        currentRunId.value = latestRun.run_id
      }
    }
  } catch { /* ignore */ }
}

async function fetchNodes() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await listNodes(id)
    if (resp.code === 0) {
      nodes.value = resp.data.items || []
    }
  } catch { /* ignore */ }
  try {
    const resp = await listEdges(id)
    if (resp.code === 0) {
      edges.value = resp.data.items || []
    }
  } catch { /* ignore */ }
}

async function fetchEdges() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await listEdges(id)
    if (resp.code === 0) {
      edges.value = resp.data.items || []
    }
  } catch { /* ignore */ }
}

function addNode(type: string) {
  editingNode.value = null
  nodeForm.name = ''
  nodeForm.type = type
  nodeForm.httpMethod = 'GET'
  nodeForm.url = ''
  nodeForm.headers = ''
  nodeForm.body = ''
  nodeForm.delayMs = '1000'
  nodeForm.conditionExpr = ''
  nodeForm.extract = ''
  nodeForm.parentId = ''
  showNodeEditor.value = true
}

function editNode(node: NodeDTO) {
  editingNode.value = node
  nodeForm.name = node.name
  nodeForm.type = node.type
  try {
    const cfg = JSON.parse(node.config || '{}')
    nodeForm.httpMethod = cfg.method || 'GET'
    nodeForm.url = cfg.url || ''
    nodeForm.headers = cfg.headers ? JSON.stringify(cfg.headers, null, 2) : ''
    nodeForm.body = cfg.body || ''
    nodeForm.delayMs = cfg.ms != null ? String(cfg.ms) : '1000'
    nodeForm.conditionExpr = cfg.expr || ''
    nodeForm.extract = cfg.extract ? JSON.stringify(cfg.extract, null, 2) : ''
    nodeForm.loop_count = cfg.loop_count != null ? String(cfg.loop_count) : '1'

    // Generator
    nodeForm.generatorExpr = cfg.expression || ''
    nodeForm.generatorVar = cfg.variable || ''

    // Timer
    nodeForm.timerMode = cfg.mode || 'delay'
    nodeForm.timerSeconds = cfg.seconds != null ? String(cfg.seconds) : '1'

    // While - operator names match backend directly (equals, not_equals, etc.)
    nodeForm.whileExitConditions = cfg.exit_conditions && cfg.exit_conditions.length > 0
      ? cfg.exit_conditions.map((c: any) => ({ variable: c.variable || '', operator: c.operator || 'equals', value: c.value != null ? String(c.value) : '' }))
      : [{ variable: '', operator: 'equals', value: '' }]
    nodeForm.whileIntervalSeconds = cfg.interval_seconds != null ? String(cfg.interval_seconds) : '1'
    nodeForm.whileMaxIterations = cfg.max_iterations != null ? String(cfg.max_iterations) : '100'
    nodeForm.whileMaxDurationMinutes = cfg.max_duration_minutes != null ? String(cfg.max_duration_minutes) : '0'

    if (node.type === 'group') {
      groupConfig.node_ids = cfg.node_ids || []
      groupConfig.loop_count = cfg.loop_count != null ? String(cfg.loop_count) : '1'
    }
    if (node.type === 'timer') {
      timerConfig.mode = cfg.mode || 'delay'
      timerConfig.seconds = cfg.seconds != null ? String(cfg.seconds) : '1'
    }
    if (node.type === 'loop') {
      loopConfig.loop_count = cfg.loop_count != null ? String(cfg.loop_count) : '3'
    }
  } catch {
    nodeForm.httpMethod = 'GET'
    nodeForm.url = ''
    nodeForm.headers = ''
    nodeForm.body = ''
    nodeForm.delayMs = '1000'
    nodeForm.conditionExpr = ''
    nodeForm.extract = ''
    nodeForm.loop_count = '1'
    nodeForm.generatorExpr = ''
    nodeForm.generatorVar = ''
    nodeForm.timerMode = 'delay'
    nodeForm.timerSeconds = '1'
    nodeForm.whileExitConditions = [{ variable: '', operator: '==', value: '' }]
    nodeForm.whileIntervalSeconds = '1'
    nodeForm.whileMaxIterations = '100'
    nodeForm.whileMaxDurationMinutes = '0'
  }
  showNodeEditor.value = true
}

function closeNodeEditor() {
  showNodeEditor.value = false
  editingNode.value = null
}

const SQL_INJECTION_PATTERN = /['";]|--|\/\*|\*\//

function validateNodeName(name: string, excludeId?: string): string | null {
  if (!name || !name.trim()) return '节点名称不能为空'
  const trimmed = name.trim()
  if (trimmed.length > 50) return '节点名称不能超过50个字符'
  if (SQL_INJECTION_PATTERN.test(trimmed)) return '节点名称包含非法字符（不允许引号、分号、注释符）'
  const duplicate = nodes.value.find(n => n.name === trimmed && n.id !== excludeId)
  if (duplicate) return `节点名称 "${trimmed}" 已存在，请使用唯一名称`
  return null
}

async function handleSaveNode() {
  const nameError = validateNodeName(nodeForm.name, editingNode.value?.id)
  if (nameError) {
    showToast(nameError, 'error')
    return
  }

  const sceneId = route.params.id as string
  let config = '{}'

  if (nodeForm.type === 'http' || nodeForm.type === 'setup' || nodeForm.type === 'teardown') {
    let headers: Record<string, string> = {}
    try { headers = JSON.parse(nodeForm.headers || '{}') } catch { /* ignore */ }
    let extract: Record<string, string> = {}
    try { extract = JSON.parse(nodeForm.extract || '{}') } catch { /* ignore */ }
    config = JSON.stringify({
      method: nodeForm.httpMethod,
      url: nodeForm.url,
      headers,
      body: nodeForm.body,
      timeout: httpConfig.timeout || 5000,
      expect_status: httpConfig.expect_status || 200,
      extract,
    })
  } else if (nodeForm.type === 'delay') {
    config = JSON.stringify({ ms: nodeForm.delayMs })
  } else if (nodeForm.type === 'condition') {
    config = JSON.stringify({ expr: nodeForm.conditionExpr })
  } else if (nodeForm.type === 'if-else') {
    config = JSON.stringify({ expr: nodeForm.conditionExpr })
  } else if (nodeForm.type === 'group') {
    config = JSON.stringify({ node_ids: groupConfig.node_ids, loop_count: nodeForm.loop_count, async: false })
  } else if (nodeForm.type === 'timer') {
    config = JSON.stringify({ mode: nodeForm.timerMode, seconds: nodeForm.timerSeconds })
  } else if (nodeForm.type === 'generator') {
    config = JSON.stringify({ expression: nodeForm.generatorExpr, variable: nodeForm.generatorVar })
  } else if (nodeForm.type === 'while') {
    const exitConditions = nodeForm.whileExitConditions
      .filter(c => c.variable !== '')
      .map(c => ({ variable: c.variable, operator: c.operator, value: c.value }))
    config = JSON.stringify({
      exit_conditions: exitConditions,
      interval_seconds: nodeForm.whileIntervalSeconds,
      max_iterations: nodeForm.whileMaxIterations,
      max_duration_minutes: nodeForm.whileMaxDurationMinutes,
    })
  } else if (nodeForm.type === 'parallel') {
    config = JSON.stringify(parallelConfig)
  } else if (nodeForm.type === 'sub_flow') {
    config = JSON.stringify(subFlowConfig)
  } else if (nodeForm.type === 'loop') {
    config = JSON.stringify({ loop_count: nodeForm.loop_count })
  }

  // Add loop_count for types that support it (except group/timer/while/loop which handle it themselves)
  if (config !== '{}' && nodeForm.type !== 'group' && nodeForm.type !== 'timer' && nodeForm.type !== 'while' && nodeForm.type !== 'loop' && parseInt(nodeForm.loop_count) > 1) {
    try {
      const cfg = JSON.parse(config)
      cfg.loop_count = parseInt(nodeForm.loop_count)
      config = JSON.stringify(cfg)
    } catch { /* ignore */ }
  }

  if (editingNode.value) {
    const resp = await apiUpdateNode({
      id: editingNode.value.id,
      name: nodeForm.name,
      type: nodeForm.type,
      config,
    })
    if (resp.code === 0) {
      showToast('节点已更新')
      fetchNodes()
    } else {
      showToast(resp.message || '更新失败', 'error')
    }
  } else {
    const resp = await apiAddNode({
      scene_id: sceneId,
      name: nodeForm.name,
      type: nodeForm.type,
      config,
    })
    if (resp.code === 0) {
      const newNode = resp.data
      if (nodeForm.parentId) {
        await addEdge({
          scene_id: sceneId,
          from_node: nodeForm.parentId,
          to_node: newNode.id,
        })
      } else if (nodes.value.length > 0) {
        const lastNode = sortedNodes.value[sortedNodes.value.length - 1]
        if (lastNode) {
          await addEdge({
            scene_id: sceneId,
            from_node: lastNode.id,
            to_node: newNode.id,
          })
        }
      }
      showToast('节点已添加')
      fetchNodes()
    } else {
      showToast(resp.message || '添加失败', 'error')
    }
  }

  closeNodeEditor()
}

async function handleDeleteNode(id: string) {
  pendingDeleteAction.value = 'node'
  pendingDeleteParam.value = id
  confirmMessage.value = '确定要删除该节点吗？'
  showConfirm.value = true
}

async function confirmDeleteAction() {
  const action = pendingDeleteAction.value
  const param = pendingDeleteParam.value
  showConfirm.value = false

  if (action === 'node') {
    const id = param as string
    const relatedEdges = edges.value.filter(e => e.from_node === id || e.to_node === id)
    for (const edge of relatedEdges) {
      await deleteEdge(edge.id)
    }
    const resp = await apiDeleteNode(id)
    if (resp.code === 0) {
      showToast('节点已删除（关联连线已自动清理）')
      if (selectedNode.value?.id === id) selectedNode.value = null
      fetchNodes()
      fetchEdges()
    } else {
      showToast(resp.message || '删除失败', 'error')
    }
  } else if (action === 'datasource') {
    const dsId = param as string
    try {
      await deleteDataSource(dsId)
      showToast('数据源已删除', 'success')
      fetchDataSources()
    } catch {
      showToast('删除失败', 'error')
    }
  } else if (action === 'csv-row') {
    const ri = param as number
    dsEditRows.value.splice(ri, 1)
  } else if (action === 'csv-column') {
    const ci = param as number
    const colName = dsEditColumns.value[ci]
    dsEditColumns.value.splice(ci, 1)
    for (const row of dsEditRows.value) {
      delete row[colName]
    }
  } else if (action === 'var') {
    const idx = param as number
    varEntries.value.splice(idx, 1)
    saveVariables()
  }

  pendingDeleteAction.value = 'node'
  pendingDeleteParam.value = ''
}

async function deleteEdgeBetween(idx: number) {
  const sorted = sortedNodes.value
  if (idx >= sorted.length - 1) return
  const fromNode = sorted[idx]
  const toNode = sorted[idx + 1]
  const edge = edges.value.find(e => String(e.from_node) === String(fromNode.id) && String(e.to_node) === String(toNode.id))
  if (edge) {
    await deleteEdge(edge.id)
    fetchNodes()
  }
}

function selectNode(node: NodeDTO | null) {
  if (!node) {
    selectedNode.value = null
    return
  }
  selectedNode.value = node
  editingConfig.name = node.name
  try {
    const cfg = JSON.parse(node.config || '{}')
    httpConfig.method = cfg.method || 'GET'
    httpConfig.url = cfg.url || ''
    httpConfig.headers = cfg.headers ? JSON.stringify(cfg.headers, null, 2) : ''
    httpConfig.body = cfg.body || ''
    httpConfig.timeout = cfg.timeout != null ? String(cfg.timeout) : '5000'
    httpConfig.expect_status = cfg.expect_status != null ? String(cfg.expect_status) : '200'
    httpConfig.extract = cfg.extract ? JSON.stringify(cfg.extract, null, 2) : ''
    httpConfig.generator = cfg.generator ? JSON.stringify(cfg.generator, null, 2) : ''
    delayConfig.ms = cfg.ms != null ? String(cfg.ms) : '1000'
    conditionConfig.expr = cfg.expr || ''
    ifElseConfig.expr = cfg.expr || ''

    groupConfig.node_ids = cfg.node_ids || []
    groupConfig.loop_count = cfg.loop_count != null ? String(cfg.loop_count) : '1'
    groupConfig.async = cfg.async || false
    timerConfig.mode = cfg.mode || 'delay'
    timerConfig.seconds = cfg.seconds != null ? String(cfg.seconds) : '1'
    
    // Parse configs for new node types
    whileConfig.exit_conditions = cfg.exit_conditions || [{ variable: '', operator: '==', value: '' }]
    whileConfig.interval_seconds = cfg.interval_seconds || 1
    whileConfig.max_iterations = cfg.max_iterations || 100
    whileConfig.max_duration_minutes = cfg.max_duration_minutes || 0
    whileConfig.fail_after_consecutive = cfg.fail_after_consecutive || 0
    whileConfig.fail_message = cfg.fail_message || ''
    
    parallelConfig.async = cfg.async ?? false
    
    subFlowConfig.scene_id = cfg.scene_id || ''
    subFlowConfig.async = cfg.async ?? false
    
    loopConfig.loop_count = cfg.loop_count || 3
    
    if (node.type === 'if-else') {
      const nodeEdges = edges.value.filter(e => e.from_node === node.id)
      const trueEdge = nodeEdges.find(e => e.condition === '__if_true__')
      const falseEdge = nodeEdges.find(e => e.condition === '__if_false__')
      ifElseConfig.trueTarget = trueEdge?.to_node || ''
      ifElseConfig.falseTarget = falseEdge?.to_node || ''
    } else {
      ifElseConfig.trueTarget = ''
      ifElseConfig.falseTarget = ''
    }
  } catch { /* ignore */ }
}

async function onDagAddEdge(from: string, to: string) {
  const sceneId = route.params.id as string
  await addEdge({ scene_id: sceneId, from_node: from, to_node: to })
  fetchEdges()
  showToast('连线已创建')
}

async function handleDeleteEdge(id: string) {
  await deleteEdge(id)
  fetchEdges()
  showToast('连线已删除')
}

async function handleUpdateEdge(edgeId: string, newSource: string, newTarget: string, _newSourceHandle?: string, _newTargetHandle?: string) {
  const edge = edges.value.find(e => e.id === edgeId)
  if (!edge) return
  const sceneId = route.params.id as string
  await deleteEdge(edgeId)
  await addEdge({ scene_id: sceneId, from_node: newSource, to_node: newTarget, condition: edge.condition || undefined })
  const idx = edges.value.findIndex(e => e.id === edgeId)
  if (idx >= 0) {
    edges.value[idx] = { ...edges.value[idx], from_node: newSource, to_node: newTarget }
  }
  showToast('连线已重新连接')
}

async function saveNodePosition(id: string, x: number, y: number) {
  const sceneId = route.params.id as string
  await apiUpdateNode({ id, position: JSON.stringify({ x: Math.round(x), y: Math.round(y) }) })
}

async function saveNodeConfig() {
  if (!selectedNode.value) return

  const newName = editingConfig.name || selectedNode.value.name
  const nameError = validateNodeName(newName, selectedNode.value.id)
  if (nameError) {
    showToast(nameError, 'error')
    return
  }

  let config = '{}'
  const nodeType = selectedNode.value.type
  if (nodeType === 'http' || nodeType === 'setup' || nodeType === 'teardown') {
    let headers: Record<string, string> = {}
    try { headers = JSON.parse(httpConfig.headers || '{}') } catch { /* ignore */ }
    let extract: Record<string, string> = {}
    try { extract = JSON.parse(httpConfig.extract || '{}') } catch { /* ignore */ }
    let generator: Record<string, any> = {}
    try { generator = JSON.parse(httpConfig.generator || '{}') } catch { /* ignore */ }
    const cfg: Record<string, any> = {
      method: httpConfig.method,
      url: httpConfig.url,
      headers,
      body: httpConfig.body,
      timeout: httpConfig.timeout,
      expect_status: httpConfig.expect_status,
    }
    if (Object.keys(extract).length > 0) cfg.extract = extract
    if (Object.keys(generator).length > 0) cfg.generator = generator
    config = JSON.stringify(cfg)
  } else if (nodeType === 'delay') {
    config = JSON.stringify({ ms: delayConfig.ms })
  } else if (nodeType === 'condition') {
    config = JSON.stringify({ expr: conditionConfig.expr })
  } else if (nodeType === 'if-else') {
    config = JSON.stringify({ expr: ifElseConfig.expr })
  } else if (nodeType === 'group') {
    config = JSON.stringify({ node_ids: groupConfig.node_ids, loop_count: groupConfig.loop_count, async: groupConfig.async })
  } else if (nodeType === 'timer') {
    config = JSON.stringify(timerConfig)
  } else if (nodeType === 'while') {
    config = JSON.stringify(whileConfig)
  } else if (nodeType === 'parallel') {
    config = JSON.stringify(parallelConfig)
  } else if (nodeType === 'sub_flow') {
    config = JSON.stringify(subFlowConfig)
  } else if (nodeType === 'loop') {
    config = JSON.stringify(loopConfig)
  }

  try {
    const resp = await apiUpdateNode({
      id: selectedNode.value.id,
      name: newName,
      config,
    })
    if (resp.code === 0) {
      showToast('配置已保存')
      fetchNodes()
    } else {
      showToast(resp.message || '保存失败', 'error')
    }
  } catch (e: any) {
    showToast('保存失败: ' + (e.message || '网络错误'), 'error')
  }
}

async function handleStart() {
  if (nodes.value.length === 0) {
    showToast('请先添加 DAG 节点', 'error')
    return
  }
  // Validate numeric inputs
  const validationError = validateRunConfig()
  if (validationError) {
    showToast(validationError, 'error')
    return
  }
  const sceneId = route.params.id as string
  try {
    const resp = await startScene({
      scene_id: sceneId,
      workers: parseInt(String(runConfig.workers), 10),
      run_mode: runConfig.run_mode,
      count: parseInt(String(runConfig.count), 10),
      duration: parseInt(String(runConfig.duration), 10),
    })
    if (resp.code === 0) {
      // Store run_id for execution status tracking
      if (resp.data?.run_id) {
        currentRunId.value = resp.data.run_id
      }
      // Refresh scene to get updated status
      fetchScene()
      showToast('测试已启动')
      showRunConfig.value = false
      router.push('/runner')
    } else {
      showToast(resp.message || '启动失败', 'error')
    }
  } catch (e: any) {
    showToast('启动失败: ' + (e.message || ''), 'error')
  }
}

async function handleCopyScene() {
  if (!copyName.value) {
    showToast('请输入新场景名称', 'error')
    return
  }
  try {
    const resp = await createScene({
      name: copyName.value,
      description: scene.value?.description ? `复制自: ${scene.value.name}` : '',
    })
    if (resp.code === 0) {
      showToast('场景已复制')
      showCopyModal.value = false
      router.push(`/scenes/${resp.data.id}`)
    } else {
      showToast(resp.message || '复制失败', 'error')
    }
  } catch (e: any) {
    showToast('复制失败', 'error')
  }
}

function formatTime(t: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function fetchGenerators() {
  try {
    const resp = await listGenerators()
    if (resp.code === 0) {
      generatorCategories.value = resp.data.categories || []
    }
  } catch { /* ignore */ }
}

function applyGenerator() {
  if (!selectedGeneratorCategory.value || !selectedGeneratorName.value) return
  const cat = generatorCategories.value.find(c => c.key === selectedGeneratorCategory.value)
  if (!cat) return
  const gen = cat.generators.find(g => g.name === selectedGeneratorName.value)
  if (!gen) return

  let current: Record<string, any> = {}
  try { current = JSON.parse(httpConfig.generator || '{}') } catch { /* ignore */ }
  current[gen.name] = gen.schema_template
  httpConfig.generator = JSON.stringify(current, null, 2)
  saveNodeConfig()
}

function insertGeneratorToBody() {
  if (!selectedGeneratorCategory.value || !selectedGeneratorName.value) return
  const cat = generatorCategories.value.find(c => c.key === selectedGeneratorCategory.value)
  if (!cat) return
  const gen = cat.generators.find(g => g.name === selectedGeneratorName.value)
  if (!gen) return

  let body: Record<string, any> = {}
  try {
    const parsed = JSON.parse(httpConfig.body || '{}')
    if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
      body = parsed
    }
  } catch { /* ignore */ }

  body[gen.name] = `\${generator.${gen.name}}`
  httpConfig.body = JSON.stringify(body, null, 2)
  saveNodeConfig()
}

onMounted(() => {
  fetchScene()
  fetchNodes()
  fetchEdges()
  fetchGenerators()
  fetchDataSources()
  fetchSceneList()
})
</script>

<style scoped>
/* ===================================================================
   SceneDetailPage — 完整 UI 重设计
   设计理念：年轻、扁平、清爽、信息层级清晰
   基于 Salvo CSS 变量系统，兼容深色/浅色主题
   =================================================================== */

/* ---- 布局结构 ---- */
.scene-detail {
  display: flex; flex-direction: column;
  height: calc(100vh - var(--header-height, 52px)); overflow: hidden;
  background: var(--bg-primary);
}

/* ---- 工具栏 ---- */
.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 20px; background: var(--bg-card);
  border-bottom: 1px solid var(--border-primary);
  flex-shrink: 0; gap: 12px;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}
.toolbar-left {
  display: flex; align-items: center; gap: 12px; min-width: 0;
}
.toolbar-left h2 {
  font-size: 16px; font-weight: 700; margin: 0;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  color: var(--text-primary); letter-spacing: -0.3px;
}
.toolbar-right {
  display: flex; align-items: center; gap: 8px; flex-shrink: 0;
}

/* ---- 通用按钮系统 ---- */

/* 基础按钮 mixin — 通过 class 组合使用 */
.btn-base {
  display: inline-flex; align-items: center; gap: 5px;
  font-family: inherit; cursor: pointer; white-space: nowrap;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  flex-shrink: 0;
}

/* 返回按钮 */
.btn-back {
  padding: 6px 12px;
  border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  background: transparent; color: var(--text-secondary);
  font-size: 13px; cursor: pointer;
  transition: all 0.2s ease;
}
.btn-back:hover {
  color: var(--text-primary); border-color: var(--text-tertiary);
  background: var(--bg-hover);
}

/* 次要按钮 */
.btn-secondary {
  padding: 7px 16px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-md); background: transparent;
  color: var(--text-secondary); font-size: 13px; cursor: pointer;
  transition: all 0.2s ease;
}
.btn-secondary:hover { background: var(--bg-hover); color: var(--text-primary); }

/* 描边按钮（青色边框） */
.btn-outline {
  padding: 5px 12px; border: 1px solid var(--accent-primary);
  border-radius: var(--radius-md); background: transparent;
  color: var(--accent-primary); font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.2s ease;
}
.btn-outline:hover { background: rgba(0,229,255,0.08); }
.btn-outline:disabled { opacity: 0.5; cursor: not-allowed; }

/* 小型按钮 */
.btn-sm {
  padding: 5px 10px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-md); background: transparent;
  color: var(--text-secondary); font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.2s ease; white-space: nowrap;
}
.btn-sm:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sm:hover { border-color: var(--accent-primary); color: var(--accent-primary); background: rgba(0,229,255,0.04); }

/* 工具栏按钮变体 */
.toolbar-btn { display: inline-flex; align-items: center; gap: 5px; }
.toolbar-btn.active {
  border-color: var(--accent-primary); color: var(--accent-primary);
  background: rgba(0,229,255,0.1);
}

/* 关闭按钮 */
.btn-close {
  border: none; background: transparent; color: var(--text-tertiary);
  font-size: 18px; cursor: pointer; padding: 4px 8px; line-height: 1;
  border-radius: var(--radius-sm); transition: all 0.2s ease;
}
.btn-close:hover { color: var(--text-primary); background: var(--bg-hover); }

/* 无样式图标按钮（用于操作列表中的小按钮） */
.btn-icon {
  border: none; background: transparent; cursor: pointer;
  padding: 4px; border-radius: var(--radius-sm);
  transition: all 0.2s ease;
}

/* ---- 状态标签 ---- */
.status-badge {
  font-size: 11px; font-weight: 600; padding: 3px 10px;
  border-radius: 100px; white-space: nowrap;
  letter-spacing: 0.2px; border: 1px solid transparent;
}
.status-badge.draft { background: var(--bg-tertiary); color: var(--text-secondary); border-color: var(--border-primary); }
.status-badge.ready { background: rgba(74,222,128,0.1); color: var(--accent-success); border-color: rgba(74,222,128,0.2); }
.status-badge.running { background: rgba(0,229,255,0.08); color: var(--accent-primary); border-color: rgba(0,229,255,0.15); }
.status-badge.completed { background: rgba(74,222,128,0.1); color: var(--accent-success); border-color: rgba(74,222,128,0.2); }

/* ---- 主工作区 ---- */
.main-workspace { display: flex; flex: 1; min-height: 0; overflow: hidden; }

/* ---- DAG 区块 ---- */
.dag-section {
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg); overflow: hidden;
  flex: 1; min-width: 0; display: flex; flex-direction: column;
  margin: 12px;
}
.section-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 16px; border-bottom: 1px solid var(--border-primary);
  flex-shrink: 0; background: var(--bg-secondary);
}
.section-header h3 {
  font-size: 13px; font-weight: 700; margin: 0;
  color: var(--text-primary); letter-spacing: 0.3px;
  text-transform: uppercase;
}
.dag-actions { display: flex; gap: 6px; flex-wrap: wrap; }
.dag-canvas { padding: 0; min-height: 200px; flex: 1; position: relative; overflow: auto; }
.dag-empty {
  text-align: center; padding: 80px 20px; color: var(--text-tertiary);
  display: flex; flex-direction: column; align-items: center; gap: 12px;
}
.empty-icon { font-size: 48px; opacity: 0.15; }
.dag-empty p { font-size: 14px; margin: 0; }

/* ---- 节点配置侧边栏 ---- */
.config-sidebar {
  width: 400px; max-width: 45%; min-width: 340px;
  flex-shrink: 0; overflow-y: auto; overflow-x: hidden;
  background: var(--bg-card);
  border-left: 1px solid var(--border-primary);
  box-shadow: -4px 0 16px rgba(0,0,0,0.08);
}
.node-config-panel { padding: 20px; min-width: 0; }
.panel-header {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 20px; padding-bottom: 12px;
  border-bottom: 1px solid var(--border-primary);
}
.panel-header h4 {
  font-size: 14px; font-weight: 700; margin: 0;
  color: var(--text-primary);
}

/* ---- 配置表单 ---- */
.config-form { display: flex; flex-direction: column; gap: 14px; }
.form-row { display: flex; flex-direction: column; gap: 5px; }
.form-row.inline { flex-direction: row; align-items: center; gap: 10px; }
.form-row.inline label { width: 110px; flex-shrink: 0; font-size: 12px; }
.form-row label {
  font-size: 12px; font-weight: 600; color: var(--text-secondary);
  text-transform: uppercase; letter-spacing: 0.3px;
}
.form-row input, .form-row select, .form-row textarea {
  width: 100%; box-sizing: border-box;
  padding: 8px 12px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-md); background: var(--bg-input);
  color: var(--text-primary); font-size: 13px; outline: none;
  font-family: inherit; word-break: break-all;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}
.form-row input:focus, .form-row select:focus, .form-row textarea:focus {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(0,229,255,0.08);
}
.form-row textarea {
  resize: vertical; min-height: 64px;
  font-family: var(--font-mono); font-size: 12px;
  line-height: 1.5;
}
.panel-footer { margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--border-primary); }
.btn-save-panel { width: 100%; padding: 10px; font-size: 13px; }

/* ---- 删除按钮 & 添加按钮（while条件等复用） ---- */
.btn-del-var {
  background: none; border: 1px solid transparent; color: var(--text-tertiary); cursor: pointer;
  font-size: 12px; padding: 3px 8px; border-radius: var(--radius-md);
  transition: all 0.2s ease; line-height: 1.4; flex-shrink: 0;
}
.btn-del-var:hover {
  color: #fff; background: var(--accent-danger);
  border-color: var(--accent-danger);
}

.drawer-add-btn {
  margin-top: 6px; border-style: dashed !important;
  width: 100%; justify-content: center;
  color: var(--accent-primary) !important; border-color: var(--accent-primary) !important;
}
.drawer-add-btn:hover {
  background: rgba(0,229,255,0.06) !important;
}

/* ---- 全屏 CSV 编辑器 ---- */
.csv-editor-overlay {
  position: fixed; inset: 0; z-index: 100;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex; align-items: center; justify-content: center;
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }

.csv-editor {
  width: 94vw; height: 90vh; max-width: 1500px;
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  display: flex; flex-direction: column;
  /* 多层阴影营造立体浮起效果（参考 SettingsPage .card） */
  box-shadow:
    0 4px 6px rgba(0, 0, 0, 0.04),
    0 12px 40px rgba(0, 0, 0, 0.12),
    0 24px 64px rgba(0, 0, 0, 0.08);
  overflow: hidden;
}
.csv-editor-header {
  padding: 10px 20px; border-bottom: 1px solid var(--border-secondary);
  flex-shrink: 0; background: var(--bg-secondary);
  display: flex; justify-content: space-between; align-items: center;
}
.csv-editor-header h3 { font-size: 14px; font-weight: 700; margin: 0; color: var(--text-primary); }

/* CSV 分页 */
.csv-pagination { display: flex; align-items: center; gap: 6px; }
.page-info {
  font-size: 12px; color: var(--text-secondary);
  min-width: 50px; text-align: center; font-weight: 600;
}
.page-size-select {
  padding: 4px 8px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-md); background: var(--bg-input);
  color: var(--text-primary); font-size: 12px; outline: none; cursor: pointer;
}
.page-size-select:focus { border-color: var(--accent-primary); }

/* CSV 表格体 — 背景块内边距，表格与背景块有间距 */
.csv-editor-body {
  flex: 1; overflow: auto;
  padding: 12px 16px;
  display: flex; flex-direction: column;
}

/* CSV 底部浮动工具栏 */
.csv-footer-bar {
  display: flex; align-items: center; gap: 8px;
  margin-top: 10px; padding-top: 10px;
  border-top: 1px solid var(--border-primary);
  flex-shrink: 0;
}
.csv-meta { font-size: 12px; color: var(--text-tertiary); white-space: nowrap; }

/* CSV 表格内层卡片 — 数据表与背景块的视觉分层 */
.csv-table-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  padding: 0;
  overflow-x: auto;
}

.csv-table {
  width: 100%; border-collapse: separate; border-spacing: 0;
  font-size: 13px; table-layout: fixed;
}
.csv-table th {
  text-align: left; padding: 10px 12px; background: var(--bg-tertiary);
  border-bottom: 2px solid var(--accent-primary);
  font-weight: 600; font-size: 12px; color: var(--text-primary);
  position: sticky; top: 0; z-index: 2;
  white-space: nowrap; vertical-align: middle;
}
.csv-table td {
  padding: 8px 12px; border-bottom: 1px solid var(--border-primary);
  color: var(--text-primary); vertical-align: middle;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.csv-table tbody tr { transition: background 0.12s ease; }
.csv-table tbody tr:hover { background: var(--bg-hover); }
.csv-table tbody tr:nth-child(even) { background: rgba(128,128,128,0.02); }
.csv-table tbody tr:nth-child(even):hover { background: var(--bg-hover); }

.col-num {
  width: 44px; text-align: center; color: var(--text-tertiary);
  font-size: 11px; user-select: none;
}
.col-action { width: 36px; text-align: center; }
.col-editable { position: relative; }

/* 列名输入 */
.col-name-input {
  background: transparent; border: none; color: var(--text-primary);
  font-weight: 600; font-size: 12px; width: calc(100% - 20px);
  padding: 2px 4px; outline: none; border-radius: 2px;
}
.col-name-input:focus { background: rgba(0,229,255,0.06); }
.col-del-btn {
  font-size: 10px; padding: 2px 5px; opacity: 0.3;
  cursor: pointer; transition: all 0.15s ease;
  vertical-align: middle; border: 1px solid transparent; background: none; color: var(--text-tertiary);
  border-radius: var(--radius-sm);
}
.col-del-btn:hover { opacity: 1; color: #fff; background: var(--accent-danger); border-color: var(--accent-danger); }

/* 单元格输入 */
.cell-input {
  background: transparent; border: none; color: var(--text-primary);
  font-size: 13px; width: 100%; padding: 4px 6px; outline: none;
  border-radius: var(--radius-sm); transition: all 0.15s ease;
  box-sizing: border-box;
}
.cell-input:focus {
  background: rgba(0,229,255,0.06);
  box-shadow: 0 0 0 2px rgba(0,229,255,0.12);
}
.row-del-btn {
  font-size: 11px; padding: 2px 6px; opacity: 0.3;
  transition: all 0.15s ease; color: var(--text-tertiary);
  border: 1px solid transparent; background: none; border-radius: var(--radius-sm); cursor: pointer;
}
.row-del-btn:hover { opacity: 1; color: #fff; background: var(--accent-danger); border-color: var(--accent-danger); }

/* ---- DAG 流程图元素 ---- */
.dag-flow { display: flex; flex-direction: column; align-items: center; gap: 0; }
.dag-branch-container { display: flex; flex-direction: column; align-items: center; width: 100%; padding: 20px 0; }
.dag-level { display: flex; justify-content: center; gap: 28px; width: 100%; position: relative; margin-bottom: 12px; }
.dag-level-item { display: flex; flex-direction: column; align-items: center; position: relative; }

.branch-item .dag-nodes-in-level::before {
  content: ''; position: absolute; top: -24px; left: 50%;
  width: 2px; height: 20px; background: var(--border-primary);
}
.branch-label { margin-bottom: 6px; }
.label-badge {
  font-size: 10px; font-weight: 700; letter-spacing: 0.8px;
  padding: 3px 12px; border-radius: 100px; text-transform: uppercase;
}
.true-label { background: rgba(74,222,128,0.12); color: var(--accent-success); border: 1px solid rgba(74,222,128,0.2); }
.false-label { background: rgba(239,68,68,0.1); color: var(--accent-danger); border: 1px solid rgba(239,68,68,0.2); }
.dag-nodes-in-level { display: flex; gap: 14px; }

/* 分支线 */
.branch-split-lines { display: flex; gap: 48px; margin-top: 6px; position: relative; }
.split-line-wrapper { display: flex; flex-direction: column; align-items: center; min-width: 80px; }
.split-line { display: flex; flex-direction: column; align-items: center; position: relative; }
.split-condition {
  font-size: 9px; font-weight: 700; letter-spacing: 0.8px;
  padding: 2px 8px; border-radius: 100px; margin-bottom: 4px;
  text-transform: uppercase;
}
.true-line .split-condition { background: rgba(74,222,128,0.1); color: var(--accent-success); }
.false-line .split-condition { background: rgba(239,68,68,0.08); color: var(--accent-danger); }
.split-svg { width: 40px; height: 50px; overflow: visible; }
.true-line .split-svg path { stroke: var(--accent-success); stroke-dasharray: none; opacity: 0.6; }
.false-line .split-svg path { stroke: var(--accent-danger); stroke-dasharray: none; opacity: 0.6; }

/* DAG 节点卡片 */
.dag-node {
  display: flex; align-items: center; gap: 12px;
  padding: 12px 18px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg); background: var(--bg-card);
  cursor: pointer; min-width: 300px;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: var(--shadow-sm);
  position: relative;
}
[data-theme='dark'] .dag-node {
  background: #1c2333;
  border-color: #3a4556;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
}
.dag-node:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 12px rgba(0,229,255,0.08);
  transform: translateY(-1px);
}
.dag-node.active {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(0,229,255,0.12), 0 2px 12px rgba(0,229,255,0.06);
}
.dag-node.setup { border-left: 3px solid var(--accent-success); }
.dag-node.http { border-left: 3px solid var(--accent-primary); }
.dag-node.delay { border-left: 3px solid var(--accent-warning); }
.dag-node.condition { border-left: 3px solid #a78bfa; }
.dag-node.if-else { border-left: 3px solid #fb923c; }
.dag-node.teardown { border-left: 3px solid var(--accent-danger); }
.dag-node.group { border-left: 3px solid #f472b6; }
.dag-node.timer { border-left: 3px solid #38bdf8; }

/* 节点图标 */
.node-icon {
  font-size: 14px; width: 32px; height: 32px;
  display: flex; align-items: center; justify-content: center;
  border-radius: var(--radius-md); flex-shrink: 0;
}
.node-icon.setup { background: rgba(74,222,128,0.1); color: var(--accent-success); }
.node-icon.http { background: rgba(0,229,255,0.08); color: var(--accent-primary); }
.node-icon.delay { background: rgba(234,179,8,0.1); color: var(--accent-warning); }
.node-icon.condition { background: rgba(167,139,250,0.1); color: #a78bfa; }
.node-icon.if-else { background: rgba(251,146,60,0.1); color: #fb923c; }
.node-icon.teardown { background: rgba(239,68,68,0.08); color: var(--accent-danger); }
.node-icon.group { background: rgba(244,114,182,0.1); color: #f472b6; }
.node-icon.timer { background: rgba(56,189,248,0.1); color: #38bdf8; }

.node-info { flex: 1; display: flex; align-items: center; gap: 10px; min-width: 0; }
.node-name { font-size: 13px; font-weight: 600; color: var(--text-primary); }
.node-type-badge {
  font-size: 10px; font-weight: 600; padding: 2px 8px;
  border-radius: 100px; background: var(--bg-tertiary);
  color: var(--text-tertiary); letter-spacing: 0.3px;
}
.node-actions { display: flex; gap: 4px; opacity: 0; transition: opacity 0.2s ease; }
.dag-node:hover .node-actions { opacity: 1; }
.node-btn {
  border: none; background: transparent; color: var(--text-tertiary);
  font-size: 12px; cursor: pointer; padding: 4px 8px;
  border-radius: var(--radius-sm); transition: all 0.15s ease;
}
.node-btn:hover { background: var(--bg-hover); color: var(--text-primary); }
.node-btn.danger:hover { color: var(--accent-danger); background: rgba(239,68,68,0.08); }

/* DAG 边（连线） */
.dag-edge {
  display: flex; flex-direction: column; align-items: center;
  position: relative; padding: 6px 0;
}
.edge-line { width: 2px; height: 14px; background: var(--border-primary); border-radius: 1px; }
[data-theme='dark'] .edge-line { background: #4a5568; }
.edge-condition {
  font-size: 10px; font-weight: 600; padding: 2px 10px;
  border-radius: 100px; background: rgba(167,139,250,0.1);
  color: #a78bfa; margin: 4px 0; letter-spacing: 0.3px;
}
.edge-arrow { color: var(--text-tertiary); font-size: 10px; line-height: 1; }
[data-theme='dark'] .edge-arrow { color: #718096; }
.edge-delete {
  position: absolute; right: -24px; top: 50%; transform: translateY(-50%);
  border: none; background: transparent; color: var(--text-tertiary);
  font-size: 10px; cursor: pointer; opacity: 0;
  transition: opacity 0.15s ease; padding: 4px; border-radius: var(--radius-sm);
}
.dag-edge:hover .edge-delete { opacity: 1; }
.edge-delete:hover { color: var(--accent-danger); background: rgba(239,68,68,0.08); }

/* 深色模式 DAG 连线增强 */
[data-theme='dark'] .branch-item .dag-nodes-in-level::before {
  background: #4a5568;
}
[data-theme='dark'] .split-svg path {
  opacity: 0.85;
}

/* ---- 运行警告 / 弹窗 / Toast / 确认框 ---- */
.run-warning {
  background: rgba(234,179,8,0.08);
  border: 1px solid rgba(234,179,8,0.2);
  border-radius: var(--radius-md); padding: 12px 16px;
  font-size: 13px; color: var(--accent-warning); margin-bottom: 12px;
}

/* 模态框 */
.modal-overlay {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.5);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex; align-items: center; justify-content: center;
  z-index: 100; animation: fadeIn 0.2s ease;
}
.modal {
  background: var(--bg-card); border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg); padding: 28px;
  width: 480px; max-width: 90vw; max-height: 80vh;
  overflow-y: auto; box-shadow: var(--shadow-lg);
  animation: modalScaleIn 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
@keyframes modalScaleIn {
  from { opacity: 0; transform: scale(0.96) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}
.modal h3 { font-size: 17px; font-weight: 700; margin: 0 0 20px; color: var(--text-primary); }

.form-group { margin-bottom: 16px; }
.form-group label {
  display: block; font-size: 12px; font-weight: 600;
  color: var(--text-secondary); margin-bottom: 5px;
  text-transform: uppercase; letter-spacing: 0.3px;
}
.form-group input, .form-group select, .form-group textarea {
  width: 100%; padding: 0 12px; height: 38px;
  border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  background: var(--bg-input); color: var(--text-primary);
  font-size: 13px; outline: none; font-family: inherit;
  box-sizing: border-box; transition: all 0.2s ease;
}
.form-group textarea {
  height: auto; min-height: 64px; padding: 10px 12px;
  resize: vertical; font-family: var(--font-mono); font-size: 12px;
}
.form-group input:focus, .form-group select:focus, .form-group textarea:focus {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(0,229,255,0.08);
}
.field-hint { font-size: 11px; color: var(--text-tertiary); margin-top: 4px; display: block; line-height: 1.4; }
.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 24px; }

/* ---- Toast 通知 ---- */
.toast {
  position: fixed; bottom: 24px; right: 24px;
  padding: 12px 24px; border-radius: var(--radius-md);
  font-size: 13px; font-weight: 600; z-index: 200;
  animation: toastSlideIn 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  color: #fff; box-shadow: var(--shadow-lg);
  display: flex; align-items: center; gap: 8px;
}
.toast.info { background: var(--accent-primary); }
.toast.error { background: var(--accent-danger); }
.toast.success { background: var(--accent-success); }
@keyframes toastSlideIn {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

/* ---- 确认对话框 ---- */
.confirm-dialog {
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: 32px;
  width: 380px;
  text-align: center;
  box-shadow: var(--shadow-lg);
  animation: modalScaleIn 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.confirm-icon {
  width: 52px; height: 52px; margin: 0 auto 18px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: rgba(239,68,68,0.1); color: var(--accent-danger);
}
.confirm-icon svg { width: 26px; height: 26px; }
.confirm-title { font-size: 17px; font-weight: 700; color: var(--text-primary); margin: 0 0 8px; }
.confirm-msg { font-size: 13px; color: var(--text-secondary); margin: 0 0 28px; line-height: 1.6; }
.confirm-actions { display: flex; justify-content: center; gap: 12px; }

.btn-cancel {
  padding: 9px 24px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-md); background: transparent;
  color: var(--text-primary); font-size: 13px; font-weight: 500;
  cursor: pointer; transition: all 0.2s ease;
}
.btn-cancel:hover { background: var(--bg-hover); }

.btn-danger-confirm {
  padding: 9px 24px; border: none; border-radius: var(--radius-md);
  background: var(--accent-danger); color: #fff; font-size: 13px; font-weight: 600;
  cursor: pointer; transition: all 0.2s ease;
}
.btn-danger-confirm:hover { opacity: 0.88; box-shadow: 0 2px 8px rgba(239,68,68,0.25); }

/* ---- 生成器选择器 ---- */
.generator-selector { display: flex; flex-direction: column; gap: 8px; }
.generator-dropdown-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.gen-category-select, .gen-name-select {
  flex: 1 1 120px; min-width: 0; padding: 7px 10px;
  border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  background: var(--bg-input); color: var(--text-primary);
  font-size: 13px; outline: none; box-sizing: border-box;
  transition: border-color 0.2s ease;
}
.gen-category-select:focus, .gen-name-select:focus {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(0,229,255,0.08);
}
.hint-label { font-size: 11px; color: var(--text-tertiary); line-height: 1.5; }

/* ---- 子节点选择器（Group） ---- */
.group-child-config { display: flex; flex-direction: column; gap: 8px; }
.group-available-nodes {
  display: flex; flex-direction: column; gap: 4px; margin-top: 4px;
  max-height: 140px; overflow-y: auto; padding: 4px 0;
}
.child-node-selector { display: flex; flex-direction: column; gap: 4px; margin-top: 4px; }
.child-node-check {
  display: flex; align-items: center; gap: 8px; font-size: 13px; padding: 4px 0;
}
.child-node-check input[type="checkbox"] { width: auto; margin: 0; accent-color: var(--accent-primary); }

.group-ordered-list {
  display: flex; flex-direction: column; gap: 4px; margin-top: 8px;
  border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  padding: 8px; background: var(--bg-secondary);
}
.ordered-child-row {
  display: flex; align-items: center; gap: 8px; padding: 6px 8px;
  border-radius: var(--radius-md); background: var(--bg-card);
  transition: all 0.15s ease; border: 1px solid transparent;
}
.ordered-child-row:hover { background: var(--bg-hover); border-color: var(--border-primary); }
.order-index {
  width: 24px; height: 24px; display: flex; align-items: center; justify-content: center;
  font-size: 11px; font-weight: 700; color: #fff;
  background: var(--accent-primary); border-radius: 50%; flex-shrink: 0;
}
.order-arrow { color: var(--accent-primary); font-size: 14px; font-weight: bold; flex-shrink: 0; margin-left: -2px; }
.order-name { font-size: 13px; color: var(--text-primary); flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.order-actions { display: flex; gap: 4px; flex-shrink: 0; opacity: 0; transition: opacity 0.2s ease; }
.ordered-child-row:hover .order-actions { opacity: 1; }
.order-btn {
  width: 26px; height: 26px; display: flex; align-items: center; justify-content: center;
  border: 1px solid var(--border-primary); border-radius: var(--radius-sm);
  background: var(--bg-card); cursor: pointer; font-size: 12px;
  color: var(--text-secondary); transition: all 0.15s ease; padding: 0; line-height: 1;
}
.order-btn:hover:not(:disabled) { border-color: var(--accent-primary); color: var(--accent-primary); background: rgba(0,229,255,0.06); }
.order-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.order-btn.remove-btn:hover { border-color: var(--accent-danger); color: var(--accent-danger); background: rgba(239,68,68,0.06); }

/* ---- 响应式 ---- */
@media (max-width: 900px) {
  .main-workspace { flex-direction: column; overflow-y: auto; }
  .config-sidebar { width: 100%; max-width: none; min-width: 0; border-left: none; border-top: 1px solid var(--border-primary); }
  .dag-section { min-width: 0; margin: 8px; }
  .scene-detail { height: auto; max-height: none; overflow: visible; }
  .toolbar { flex-wrap: wrap; gap: 6px; padding: 8px 12px; }
  .toolbar-right { flex-wrap: wrap; }
  .dag-actions { overflow-x: auto; flex-wrap: nowrap; }
  .csv-editor { width: 98vw; height: 95vh; }
}
</style>
