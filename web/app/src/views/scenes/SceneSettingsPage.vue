<template>
  <div class="settings-page">
    <!-- Header -->
    <div class="page-header">
      <div class="header-left">
        <button class="btn-back" @click="goBack">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
          返回场景
        </button>
        <h1 class="page-title">场景设置</h1>
        <span v-if="scene" class="scene-name-tag">{{ scene.name }}</span>
      </div>
    </div>

    <!-- Two-column layout -->
    <div class="page-body">
      <!-- Left nav -->
      <div class="settings-nav">
        <div
          v-for="tab in navTabs"
          :key="tab.key"
          :class="['nav-item', { active: activeTab === tab.key }]"
          @click="activeTab = tab.key"
        >
          <span class="nav-icon" v-html="tab.icon"></span>
          <span>{{ tab.label }}</span>
        </div>
      </div>

      <!-- Right content -->
      <div class="settings-content">
        <!-- ===== 基本信息 ===== -->
        <template v-if="activeTab === 'basic'">
          <div class="section-header">
            <h3 class="section-title">基本信息</h3>
            <p class="section-desc">场景的基础配置与状态信息</p>
          </div>
          <div class="card" v-if="scene">
            <div class="info-grid">
              <div class="info-item">
                <span class="info-label">场景名称</span>
                <span class="info-value">{{ scene.name }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">状态</span>
                <span :class="['status-badge', scene.status]">{{ scene.status }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">描述</span>
                <span class="info-value">{{ scene.description || '-' }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">创建时间</span>
                <span class="info-value">{{ formatTime(scene.created_at) }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">更新时间</span>
                <span class="info-value">{{ formatTime(scene.updated_at) }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">默认超时(秒)</span>
                <input
                  v-model.number="scene.default_timeout"
                  type="number"
                  min="0"
                  placeholder="0 表示使用系统默认"
                  class="timeout-input"
                  :disabled="!canWriteScene"
                  @change="saveSceneTimeout"
                />
              </div>
            </div>
          </div>
        </template>

        <!-- ===== 场景变量 ===== -->
        <template v-if="activeTab === 'variables'">
          <div class="section-header">
            <h3 class="section-title">场景变量</h3>
            <p class="section-desc">定义场景级别的变量，可在请求配置中通过 ${variable} 引用</p>
          </div>
          <div class="card">
            <div class="var-table-header">
              <span>变量名</span>
              <span></span>
              <span>值</span>
              <span></span>
            </div>
            <div v-if="varEntries.length === 0" class="var-empty">暂无变量，点击下方按钮添加</div>
            <div v-for="(entry, idx) in varEntries" :key="idx" class="var-row">
              <input v-model="entry.key" placeholder="变量名" class="var-input" :disabled="!canWriteScene" @blur="saveVariables" />
              <span class="var-eq">=</span>
              <input v-model="entry.value" placeholder="值（支持 ${other_var} 引用）" class="var-input" :disabled="!canWriteScene" @blur="saveVariables" />
              <button v-if="canWriteScene" class="btn-icon btn-del-var" @click="removeVariableRow(idx)" title="删除">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
            <div class="var-footer">
              <button v-if="canWriteScene" class="btn-add-var" @click="addVariableRow">+ 添加变量</button>
            </div>
          </div>
        </template>

        <!-- ===== 数据源 CSV ===== -->
        <template v-if="activeTab === 'datasource'">
          <div class="section-header">
            <h3 class="section-title">数据源 CSV</h3>
            <p class="section-desc">上传和管理 CSV 数据文件，用于参数化测试</p>
          </div>

          <div class="ds-toolbar">
            <label v-if="canWriteScene" class="btn-upload">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
              上传 CSV
              <input type="file" accept=".csv" style="display:none" @change="onDsFileChange" />
            </label>
          </div>

          <div v-if="dataSources.length === 0" class="ds-empty">暂无数据源，上传 CSV 文件开始使用</div>

          <div v-for="ds in dataSources" :key="ds.id" class="ds-card" @click="handleDsPreview(ds)">
            <div class="ds-icon">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="9" y1="21" x2="9" y2="9"/></svg>
            </div>
            <div class="ds-info">
              <div class="ds-name">{{ ds.file_name }}</div>
              <div class="ds-meta">{{ ds.columns?.length ?? 0 }} 列 · {{ ds.row_count ?? 0 }} 行</div>
            </div>
            <button v-if="canWriteScene" class="btn-icon btn-del-ds" @click.stop="handleDsDelete(ds.id)" title="删除">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg>
            </button>
          </div>

          <!-- Inline CSV Editor -->
          <div v-if="showDsPreview" class="csv-editor-wrapper">
            <div class="csv-editor-header">
              <h4>{{ dsPreview?.file_name }}</h4>
              <div class="csv-editor-actions">
                <span class="csv-meta">{{ dsEditColumns.length }} 列 · {{ dsEditRows.length }} 行</span>
                <button v-if="canWriteScene" class="btn-sm" @click="dsAddRow">+ 行</button>
                <button v-if="canWriteScene" class="btn-sm" @click="dsAddColumn">+ 列</button>
                <button v-if="canWriteScene" class="btn-primary btn-sm" @click="dsSaveEdit" :disabled="dsSaving">{{ dsSaving ? '保存中...' : '保存' }}</button>
                <button class="btn-sm" @click="showDsPreview = false">关闭</button>
              </div>
            </div>
            <div class="csv-editor-body">
              <table class="csv-table">
                <thead>
                  <tr>
                    <th class="col-num">#</th>
                    <th v-for="(col, ci) in dsEditColumns" :key="ci" class="col-editable">
                      <input
                        class="col-name-input"
                        :value="col"
                        :disabled="!canWriteScene"
                        @change="dsRenameColumn(ci, ($event.target as HTMLInputElement).value)"
                      />
                      <button v-if="canWriteScene" class="btn-icon col-del-btn" @click="dsDeleteColumn(ci)" title="删除列">
                        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                      </button>
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
                        :disabled="!canWriteScene"
                        @change="dsUpdateCell((dsPage - 1) * dsPageSize + ri, col, ($event.target as HTMLInputElement).value)"
                      />
                    </td>
                    <td class="col-action">
                      <button v-if="canWriteScene" class="btn-icon row-del-btn" @click="dsDeleteRow((dsPage - 1) * dsPageSize + ri)" title="删除行">
                        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="totalDsPages > 1" class="csv-footer-bar">
              <div class="csv-pagination">
                <button class="btn-sm" :disabled="dsPage <= 1" @click="dsPage--">&#8249;</button>
                <span class="page-info">{{ dsPage }} / {{ totalDsPages }}</span>
                <button class="btn-sm" :disabled="dsPage >= totalDsPages" @click="dsPage++">&#8250;</button>
                <select class="page-size-select" v-model.number="dsPageSize">
                  <option :value="25">25/页</option>
                  <option :value="50">50/页</option>
                  <option :value="100">100/页</option>
                  <option :value="200">200/页</option>
                </select>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Toast -->
    <div v-if="toastMsg" class="toast" :class="toastType">{{ toastMsg }}</div>

    <!-- Confirm Dialog -->
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

    <!-- Add Column Modal -->
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
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getScene, updateScene, batchSetVariables } from '@/api/scene'
import { listDataSources, uploadDataSource, deleteDataSource, previewDataSource } from '@/api/datasource'
import type { DataSourceDTO, DataSourcePreviewDTO } from '@/api/datasource'
import type { SceneDTO } from '@/types'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const canWriteScene = computed(() => authStore.canAccess(['scene:write']))

// ---- Scene data ----
const scene = ref<(SceneDTO & { default_timeout?: number }) | null>(null)

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
    }
  } catch { /* ignore */ }
}

async function saveSceneTimeout() {
  if (!scene.value || !canWriteScene.value) return
  const sceneId = route.params.id as string
  try {
    await updateScene({
      id: sceneId,
      default_timeout: scene.value.default_timeout,
    })
    showToast('超时设置已保存', 'success')
  } catch {
    showToast('保存失败', 'error')
  }
}

// ---- Navigation ----
const activeTab = ref('basic')

const navTabs = [
  {
    key: 'basic',
    label: '基本信息',
    icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>',
  },
  {
    key: 'variables',
    label: '场景变量',
    icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>',
  },
  {
    key: 'datasource',
    label: '数据源 CSV',
    icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="9" y1="21" x2="9" y2="9"/></svg>',
  },
]

// ---- Variables ----
const varEntries = ref<{ key: string; value: string }[]>([])

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

// ---- Data sources ----
const dataSources = ref<DataSourceDTO[]>([])
const dsUploading = ref(false)

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
  } catch { /* ignore */ }
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
  } catch {
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

// ---- CSV Inline Editor ----
const showDsPreview = ref(false)
const dsPreview = ref<DataSourcePreviewDTO | null>(null)
const dsEditColumns = ref<string[]>([])
const dsEditRows = ref<Record<string, string>[]>([])
const dsSaving = ref(false)

// CSV pagination
const dsPage = ref(1)
const dsPageSize = ref(50)
const totalDsPages = computed(() => Math.max(1, Math.ceil(dsEditRows.value.length / dsPageSize.value)))
const pagedRows = computed(() => {
  const start = (dsPage.value - 1) * dsPageSize.value
  return dsEditRows.value.slice(start, start + dsPageSize.value)
})

async function handleDsPreview(ds: DataSourceDTO) {
  try {
    const resp = await previewDataSource(ds.id)
    if (resp.code === 0) {
      dsPreview.value = resp.data
      dsEditColumns.value = [...(resp.data.columns || [])]
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

// ---- Confirm Dialog ----
const showConfirm = ref(false)
const confirmMessage = ref('')
const pendingDeleteAction = ref<'datasource' | 'csv-row' | 'csv-column' | 'var'>('datasource')
const pendingDeleteParam = ref<string | number>('')

async function confirmDeleteAction() {
  const action = pendingDeleteAction.value
  const param = pendingDeleteParam.value
  showConfirm.value = false

  if (action === 'datasource') {
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

  pendingDeleteAction.value = 'datasource'
  pendingDeleteParam.value = ''
}

// ---- CSV Add Column Modal ----
const showAddColumnModal = ref(false)
const newColumnName = ref('')

// ---- Toast ----
const toastMsg = ref('')
const toastType = ref('info')

function showToast(msg: string, type = 'info') {
  toastMsg.value = msg
  toastType.value = type
  setTimeout(() => { toastMsg.value = '' }, 3000)
}

// ---- Helpers ----
function formatTime(t: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function goBack() {
  const id = route.params.id as string
  router.push(`/scenes/${id}`)
}

// ---- Init ----
onMounted(() => {
  fetchScene()
  fetchDataSources()
})
</script>

<style scoped>
/* Page structure */
.settings-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - var(--header-height, 52px));
  overflow: hidden;
  background: var(--bg-primary);
  animation: pageEnter 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
@keyframes pageEnter {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Header */
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 24px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-primary);
  flex-shrink: 0;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
}
.btn-back {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 12px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-back:hover {
  color: var(--text-primary);
  border-color: var(--text-tertiary);
  background: var(--bg-hover);
}
.page-title {
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.3px;
  color: var(--text-primary);
  margin: 0;
}
.scene-name-tag {
  font-size: 11px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 100px;
  background: rgba(0, 229, 255, 0.08);
  color: var(--accent-primary);
  border: 1px solid rgba(0, 229, 255, 0.15);
}

/* Two-column layout */
.page-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.settings-nav {
  width: 200px;
  flex-shrink: 0;
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-primary);
  padding: 16px 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 18px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  cursor: pointer;
  border-left: 3px solid transparent;
  transition: all 0.15s ease;
}
.nav-item:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
.nav-item.active {
  color: var(--accent-primary);
  border-left-color: var(--accent-primary);
  background: rgba(0, 229, 255, 0.04);
  font-weight: 600;
}
.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

/* Content area */
.settings-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 20px 28px;
}
.section-header {
  margin-bottom: 18px;
}
.section-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
}
.section-title::before {
  content: '';
  width: 3px;
  height: 16px;
  border-radius: 2px;
  background: var(--accent-primary);
  flex-shrink: 0;
}
.section-desc {
  font-size: 12px;
  color: var(--text-tertiary);
  margin-top: 4px;
  line-height: 1.5;
}

/* Card */
.card {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  padding: 0;
  margin-bottom: 14px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
  overflow: hidden;
}

/* Info grid (basic info) - flat table style, single column */
.info-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0;
}
.info-item {
  display: flex;
  align-items: stretch;
  border-bottom: 1px solid var(--border-secondary);
}
.info-item:last-child {
  border-bottom: none;
}
.info-label {
  display: flex;
  align-items: center;
  width: 140px;
  flex-shrink: 0;
  padding: 14px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-secondary);
  white-space: nowrap;
  letter-spacing: 0.01em;
}
.info-value {
  display: flex;
  align-items: center;
  flex: 1;
  padding: 14px 20px;
  font-size: 13px;
  color: var(--text-primary);
  font-weight: 400;
  text-align: left;
  word-break: break-all;
}
.info-value.status-badge {
  flex: none;
  padding: 2px 8px;
  margin: 12px 20px;
}
.info-item:has(.status-badge) {
  align-items: center;
}
.info-item:has(.status-badge) .status-badge {
  margin-left: 20px;
}
.info-item:has(.timeout-input) {
  align-items: center;
}
.timeout-input {
  width: 100px;
  height: 30px;
  padding: 0 8px;
  margin-left: 8px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  font-size: 13px;
  background: var(--bg-input);
  color: var(--text-primary);
  outline: none;
  text-align: right;
  transition: all 0.2s;
}
.timeout-input:focus {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(0, 229, 255, 0.08);
}
.timeout-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Status badge */
.status-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 999px;
  white-space: nowrap;
  letter-spacing: 0.04em;
  border: 1px solid transparent;
}
.status-badge.draft { background: var(--bg-tertiary); color: var(--text-secondary); border-color: var(--border-primary); }
.status-badge.ready { background: rgba(74, 222, 128, 0.1); color: var(--accent-success); border-color: rgba(74, 222, 128, 0.2); }
.status-badge.running { background: rgba(0, 229, 255, 0.08); color: var(--accent-primary); border-color: rgba(0, 229, 255, 0.15); }
.status-badge.completed { background: rgba(74, 222, 128, 0.1); color: var(--accent-success); border-color: rgba(74, 222, 128, 0.2); }

/* Variable table */
.var-table-header {
  display: grid;
  grid-template-columns: 160px 28px 1fr 36px;
  gap: 6px;
  padding: 9px 14px;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-primary);
}
.var-row {
  display: grid;
  grid-template-columns: 160px 28px 1fr 36px;
  gap: 6px;
  align-items: center;
  padding: 5px 14px;
  border-bottom: 1px solid var(--border-secondary);
  transition: background 0.12s;
}
.var-row:hover {
  background: var(--bg-hover);
}
.var-input {
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  font-size: 13px;
  background: var(--bg-input);
  color: var(--text-primary);
  outline: none;
  transition: all 0.2s;
  font-family: var(--font-mono);
}
.var-input:focus {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(0, 229, 255, 0.08);
}
.var-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.var-input::placeholder {
  color: var(--text-tertiary);
  font-size: 12px;
}
.var-eq {
  color: var(--text-tertiary);
  font-size: 14px;
  text-align: center;
  user-select: none;
}
.var-empty {
  padding: 28px;
  text-align: center;
  color: var(--text-tertiary);
  font-size: 13px;
}
.var-footer {
  padding: 10px 14px;
}
.btn-add-var {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: 7px 14px;
  border: 1px dashed var(--accent-primary);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--accent-primary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-add-var:hover {
  background: rgba(0, 229, 255, 0.06);
}

/* Datasource cards */
.ds-toolbar {
  margin-bottom: 14px;
}
.btn-upload {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border: 1px solid var(--accent-primary);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--accent-primary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-upload:hover {
  background: rgba(0, 229, 255, 0.06);
}
.ds-empty {
  padding: 40px;
  text-align: center;
  color: var(--text-tertiary);
  font-size: 13px;
  background: var(--bg-card);
  border: 1px dashed var(--border-secondary);
  border-radius: var(--radius-lg);
}
.ds-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 14px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: var(--bg-secondary);
  cursor: pointer;
  transition: all 0.2s;
  margin-bottom: 8px;
}
.ds-card:hover {
  border-color: var(--accent-primary);
  background: var(--bg-hover);
}
.ds-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  background: rgba(0, 229, 255, 0.06);
  color: var(--accent-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.ds-info {
  flex: 1;
  min-width: 0;
}
.ds-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}
.ds-meta {
  font-size: 11px;
  color: var(--text-tertiary);
  margin-top: 2px;
}
.btn-del-ds {
  padding: 4px;
  border-radius: var(--radius-sm);
  color: var(--text-tertiary);
  transition: all 0.15s;
}
.btn-del-ds:hover {
  color: #fff;
  background: var(--accent-danger);
}

/* CSV inline editor */
.csv-editor-wrapper {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
  margin-top: 14px;
}
.csv-editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-primary);
}
.csv-editor-header h4 {
  font-size: 14px;
  font-weight: 700;
  margin: 0;
  color: var(--text-primary);
}
.csv-editor-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.csv-meta {
  font-size: 12px;
  color: var(--text-tertiary);
  white-space: nowrap;
  margin-right: 4px;
}
.csv-editor-body {
  overflow: auto;
  max-height: 380px;
}
.csv-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  font-size: 13px;
  table-layout: fixed;
}
.csv-table th {
  text-align: left;
  padding: 8px 12px;
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-primary);
  font-weight: 600;
  font-size: 11px;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.4px;
  position: sticky;
  top: 0;
  z-index: 2;
  white-space: nowrap;
  vertical-align: middle;
}
.csv-table td {
  padding: 0 12px;
  border-bottom: 1px solid var(--border-secondary);
  height: 32px;
  color: var(--text-primary);
  vertical-align: middle;
}
.csv-table tbody tr {
  transition: background 0.12s;
}
.csv-table tbody tr:hover {
  background: var(--bg-hover);
}

.col-num {
  width: 44px;
  text-align: center;
  color: var(--text-tertiary);
  font-size: 11px;
  user-select: none;
}
.col-action {
  width: 36px;
  text-align: center;
}
.col-editable {
  position: relative;
}
.col-name-input {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-weight: 600;
  font-size: 12px;
  width: calc(100% - 20px);
  padding: 2px 4px;
  outline: none;
  border-radius: 2px;
}
.col-name-input:focus {
  background: rgba(0, 229, 255, 0.06);
}
.col-name-input:disabled {
  opacity: 0.5;
}
.col-del-btn {
  font-size: 10px;
  padding: 2px 5px;
  opacity: 0.3;
  cursor: pointer;
  transition: all 0.15s;
  vertical-align: middle;
  border: 1px solid transparent;
  background: none;
  color: var(--text-tertiary);
  border-radius: var(--radius-sm);
}
.col-del-btn:hover {
  opacity: 1;
  color: #fff;
  background: var(--accent-danger);
  border-color: var(--accent-danger);
}

.cell-input {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 13px;
  width: 100%;
  padding: 6px 4px;
  outline: none;
  border-radius: var(--radius-sm);
  transition: all 0.12s;
  font-family: var(--font-mono);
  box-sizing: border-box;
}
.cell-input:focus {
  background: rgba(0, 229, 255, 0.06);
  box-shadow: 0 0 0 2px rgba(0, 229, 255, 0.1) inset;
}
.cell-input:disabled {
  opacity: 0.5;
}
.row-del-btn {
  font-size: 11px;
  padding: 2px 6px;
  opacity: 0.3;
  transition: all 0.15s;
  color: var(--text-tertiary);
  border: 1px solid transparent;
  background: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
}
.row-del-btn:hover {
  opacity: 1;
  color: #fff;
  background: var(--accent-danger);
  border-color: var(--accent-danger);
}

.csv-footer-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px 16px;
  border-top: 1px solid var(--border-primary);
  background: var(--bg-secondary);
}
.csv-pagination {
  display: flex;
  align-items: center;
  gap: 6px;
}
.page-info {
  font-size: 12px;
  color: var(--text-secondary);
  min-width: 50px;
  text-align: center;
  font-weight: 600;
}
.page-size-select {
  padding: 4px 8px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 12px;
  outline: none;
  cursor: pointer;
}
.page-size-select:focus {
  border-color: var(--accent-primary);
}

/* Button system */
.btn-sm {
  padding: 5px 10px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.btn-sm:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn-sm:hover {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  background: rgba(0, 229, 255, 0.04);
}
.btn-primary {
  padding: 7px 16px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--accent-primary);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-primary:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.btn-primary:hover:not(:disabled) {
  opacity: 0.88;
  box-shadow: 0 2px 8px rgba(0, 229, 255, 0.25);
}
.btn-secondary {
  padding: 7px 16px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-secondary:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

/* Icon button */
.btn-icon {
  border: none;
  background: transparent;
  cursor: pointer;
  padding: 4px;
  border-radius: var(--radius-sm);
  transition: all 0.2s;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.btn-del-var {
  background: none;
  border: 1px solid transparent;
  color: var(--text-tertiary);
  cursor: pointer;
  font-size: 12px;
  padding: 3px 8px;
  border-radius: var(--radius-md);
  transition: all 0.2s;
  flex-shrink: 0;
}
.btn-del-var:hover {
  color: #fff;
  background: var(--accent-danger);
  border-color: var(--accent-danger);
}

/* Modal overlay */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
.modal {
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: 28px;
  width: 480px;
  max-width: 90vw;
  max-height: 80vh;
  overflow-y: auto;
  animation: modalScaleIn 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
@keyframes modalScaleIn {
  from { opacity: 0; transform: scale(0.96) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}
.modal h3 {
  font-size: 17px;
  font-weight: 700;
  margin: 0 0 20px;
  color: var(--text-primary);
}
.form-group {
  margin-bottom: 16px;
}
.form-group label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 5px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}
.form-group input {
  width: 100%;
  padding: 0 12px;
  height: 38px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  font-family: inherit;
  box-sizing: border-box;
  transition: all 0.2s;
}
.form-group input:focus {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(0, 229, 255, 0.08);
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 24px;
}

/* Confirm dialog */
.confirm-dialog {
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: 32px;
  width: 380px;
  text-align: center;
  animation: modalScaleIn 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.confirm-icon {
  width: 52px;
  height: 52px;
  margin: 0 auto 18px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(239, 68, 68, 0.1);
  color: var(--accent-danger);
}
.confirm-icon svg {
  width: 26px;
  height: 26px;
}
.confirm-title {
  font-size: 17px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 8px;
}
.confirm-msg {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0 0 28px;
  line-height: 1.6;
}
.confirm-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
}
.btn-cancel {
  padding: 9px 24px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-cancel:hover {
  background: var(--bg-hover);
}
.btn-danger-confirm {
  padding: 9px 24px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--accent-danger);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-danger-confirm:hover {
  opacity: 0.88;
  box-shadow: 0 2px 8px rgba(239, 68, 68, 0.25);
}

/* Toast */
.toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  padding: 12px 24px;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 600;
  z-index: 200;
  animation: toastSlideIn 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  color: #fff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}
.toast.info { background: var(--accent-primary); }
.toast.error { background: var(--accent-danger); }
.toast.success { background: var(--accent-success); }
[data-theme='dark'] .toast.success { background: #58a6ff; }
@keyframes toastSlideIn {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

/* Responsive */
@media (max-width: 768px) {
  .page-body { flex-direction: column; }
  .settings-nav {
    width: 100%;
    flex-direction: row;
    border-right: none;
    border-bottom: 1px solid var(--border-primary);
    padding: 0;
    overflow-x: auto;
  }
  .nav-item {
    border-left: none;
    border-bottom: 3px solid transparent;
    white-space: nowrap;
  }
  .nav-item.active {
    border-left-color: transparent;
    border-bottom-color: var(--accent-primary);
  }
  .settings-content { padding: 16px; }
}
</style>
