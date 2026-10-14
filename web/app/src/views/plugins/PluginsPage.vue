<template>
  <div class="plugins-page">
    <div class="page-header">
      <h2>SO 插件管理</h2>
      <button class="btn-primary" @click="showUpload = true">+ 上传插件</button>
    </div>

    <div class="table-wrapper">
      <table class="data-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>版本</th>
            <th>文件路径</th>
            <th>状态</th>
            <th>配置</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="plugins.length === 0"><td colspan="7" class="empty">暂无插件</td></tr>
          <tr v-for="p in plugins" :key="p.id">
            <td class="mono">{{ p.name }}</td>
            <td>{{ p.version }}</td>
            <td class="mono path">{{ p.file_path }}</td>
            <td><span :class="['status-badge', p.status]">{{ statusLabel(p.status) }}</span></td>
            <td>
              <button v-if="p.config" class="btn-sm" @click="viewConfig(p)">查看</button>
              <span v-else class="text-muted">-</span>
            </td>
            <td>{{ formatTime(p.created_at) }}</td>
            <td class="actions">
              <button class="btn-sm" :class="p.status === 'enabled' ? 'warn' : 'ok'" @click="toggleStatus(p)">
                {{ p.status === 'enabled' ? '禁用' : '启用' }}
              </button>
              <button class="btn-sm" @click="editConfig(p)">配置</button>
              <button class="btn-sm danger" @click="handleDelete(p)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Upload Modal -->
    <div v-if="showUpload" class="modal-overlay" @click.self="showUpload = false">
      <div class="modal">
        <h3>上传 SO 插件</h3>
        <div class="form-group">
          <label>名称 <span class="required">*</span></label>
          <input v-model="uploadForm.name" placeholder="例如: aes" />
        </div>
        <div class="form-group">
          <label>版本 <span class="required">*</span></label>
          <input v-model="uploadForm.version" placeholder="例如: 1.0.0" />
        </div>
        <div class="form-group">
          <label>插件文件 <span class="required">*</span></label>
          <div class="file-upload-wrapper">
            <input
              ref="fileInputRef"
              type="file"
              accept="*"
              class="file-input-hidden"
              @change="handleFileSelect"
            />
            <button class="btn-file-select" @click="fileInputRef?.click()">
              {{ selectedFile ? selectedFile.name : '选择 .so 文件' }}
            </button>
            <span v-if="selectedFile" class="file-info">{{ formatFileSize(selectedFile.size) }}</span>
            <button v-if="selectedFile" class="btn-file-clear" @click="clearFile">✕</button>
          </div>
        </div>
        <div class="form-group">
          <label>配置 (JSON，可选)</label>
          <textarea v-model="uploadForm.config" rows="4" placeholder='{"key": "value"}'></textarea>
        </div>
        <div class="form-error-slot">
          <div v-if="uploadError" class="form-error">{{ uploadError }}</div>
        </div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="closeUpload">取消</button>
          <button class="btn-primary" :disabled="uploading" @click="handleUpload">
            {{ uploading ? '上传中...' : '上传' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Config Modal -->
    <div v-if="showConfig" class="modal-overlay" @click.self="showConfig = false">
      <div class="modal">
        <h3>插件配置 - {{ configTarget?.name }}@{{ configTarget?.version }}</h3>
        <div class="form-group">
          <label>配置 (JSON)</label>
          <textarea v-model="configForm.config" rows="10" placeholder="{}"></textarea>
        </div>
        <div v-if="configError" class="form-error">{{ configError }}</div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showConfig = false">取消</button>
          <button class="btn-primary" :disabled="savingConfig" @click="handleSaveConfig">
            {{ savingConfig ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- View Config Modal (readonly) -->
    <div v-if="showViewConfig" class="modal-overlay" @click.self="showViewConfig = false">
      <div class="modal">
        <h3>插件配置 - {{ viewConfigTarget?.name }}@{{ viewConfigTarget?.version }}</h3>
        <pre class="config-preview">{{ viewConfigTarget?.config }}</pre>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showViewConfig = false">关闭</button>
        </div>
      </div>
    </div>

    <!-- Status Toggle Confirm -->
    <div v-if="showStatusConfirm" class="modal-overlay" @click.self="showStatusConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
        </div>
        <h3 class="confirm-title">确认{{ statusTarget?.status === 'enabled' ? '停用' : '启用' }}</h3>
        <p class="confirm-msg">确定要{{ statusTarget?.status === 'enabled' ? '停用' : '启用' }}插件「{{ statusTarget?.name }}@{{ statusTarget?.version }}」吗？</p>
        <div class="confirm-actions">
          <button class="btn-cancel" @click="showStatusConfirm = false">取消</button>
          <button class="btn-danger-confirm" @click="confirmToggleStatus">确认</button>
        </div>
      </div>
    </div>

    <!-- Delete Confirm -->
    <div v-if="showDeleteConfirm" class="modal-overlay" @click.self="showDeleteConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
        </div>
        <h3 class="confirm-title">确认删除</h3>
        <p class="confirm-msg">确定要删除插件「{{ deleteTarget?.name }}@{{ deleteTarget?.version }}」吗？此操作不可恢复。</p>
        <div class="confirm-actions">
          <button class="btn-cancel" @click="showDeleteConfirm = false">取消</button>
          <button class="btn-danger-confirm" @click="confirmDelete">确认删除</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { listSOPlugins, createSOPlugin, updateSOPluginStatus, updateSOPluginConfig, deleteSOPlugin, uploadSOPluginFile } from '@/api/so-plugin'
import type { SOPluginDTO } from '@/types'

const plugins = ref<SOPluginDTO[]>([])

// --- Upload ---
const showUpload = ref(false)
const uploadForm = reactive({ name: '', version: '', file_path: '', config: '' })
const uploadError = ref('')
const uploading = ref(false)
const selectedFile = ref<File | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)

function handleFileSelect(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    const file = input.files[0]
    // Validate file extension since accept attribute is broadened for macOS compatibility.
    if (!file.name.toLowerCase().endsWith('.so')) {
      uploadError.value = '只能上传 .so 文件'
      input.value = ''
      return
    }
    selectedFile.value = file
    uploadForm.file_path = '' // will be set after upload
    uploadError.value = ''
  }
}

function clearFile() {
  selectedFile.value = null
  uploadForm.file_path = ''
  if (fileInputRef.value) fileInputRef.value.value = ''
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function closeUpload() {
  showUpload.value = false
  uploadForm.name = ''
  uploadForm.version = ''
  uploadForm.file_path = ''
  uploadForm.config = ''
  uploadError.value = ''
  selectedFile.value = null
  if (fileInputRef.value) fileInputRef.value.value = ''
}

// Watch showUpload to reset file input when modal opens.
// This ensures the file picker works correctly on first open after page load.
watch(showUpload, (val) => {
  if (val && fileInputRef.value) {
    fileInputRef.value.value = ''
  }
})

async function handleUpload() {
  uploadError.value = ''
  if (!uploadForm.name || !uploadForm.version) {
    uploadError.value = '名称和版本为必填项'
    return
  }
  if (!selectedFile.value && !uploadForm.file_path) {
    uploadError.value = '请选择 .so 文件或填写文件路径'
    return
  }
  uploading.value = true
  try {
    // Step 1: Upload file if selected.
    if (selectedFile.value) {
      try {
        const result = await uploadSOPluginFile(selectedFile.value)
        uploadForm.file_path = result.file_path
      } catch (e: any) {
        uploadError.value = e.response?.data?.message || e.message || '文件上传失败'
        return
      }
    }
    // Step 2: Create plugin record.
    const resp = await createSOPlugin({
      name: uploadForm.name,
      version: uploadForm.version,
      file_path: uploadForm.file_path,
      config: uploadForm.config || undefined,
    })
    if (resp.code !== 0) {
      uploadError.value = resp.message || '创建插件失败'
      return
    }
    // Refresh list first, then close modal to avoid UI flicker.
    await fetchPlugins()
    closeUpload()
  } catch (e: any) {
    uploadError.value = e.message || '上传失败'
  } finally {
    uploading.value = false
  }
}

// --- Status Toggle ---
const showStatusConfirm = ref(false)
const statusTarget = ref<SOPluginDTO | null>(null)

function toggleStatus(p: SOPluginDTO) {
  statusTarget.value = p
  showStatusConfirm.value = true
}

async function confirmToggleStatus() {
  const p = statusTarget.value!
  const newStatus = p.status === 'enabled' ? 'disabled' : 'enabled'
  showStatusConfirm.value = false
  statusTarget.value = null
  try {
    const resp = await updateSOPluginStatus(p.id, newStatus)
    if (resp.code === 0) {
      fetchPlugins()
    }
  } catch { /* ignore */ }
}

// --- Config Edit ---
const showConfig = ref(false)
const configTarget = ref<SOPluginDTO | null>(null)
const configForm = reactive({ config: '' })
const configError = ref('')
const savingConfig = ref(false)

function editConfig(p: SOPluginDTO) {
  configTarget.value = p
  configForm.config = p.config || '{}'
  configError.value = ''
  showConfig.value = true
}

async function handleSaveConfig() {
  const p = configTarget.value!
  configError.value = ''
  savingConfig.value = true
  try {
    const resp = await updateSOPluginConfig(p.id, configForm.config)
    if (resp.code !== 0) {
      configError.value = resp.message || '保存失败'
      return
    }
    showConfig.value = false
    fetchPlugins()
  } catch (e: any) {
    configError.value = e.message || '保存失败'
  } finally {
    savingConfig.value = false
  }
}

// --- View Config (readonly) ---
const showViewConfig = ref(false)
const viewConfigTarget = ref<SOPluginDTO | null>(null)

function viewConfig(p: SOPluginDTO) {
  viewConfigTarget.value = p
  showViewConfig.value = true
}

// --- Delete ---
const showDeleteConfirm = ref(false)
const deleteTarget = ref<SOPluginDTO | null>(null)

function handleDelete(p: SOPluginDTO) {
  deleteTarget.value = p
  showDeleteConfirm.value = true
}

async function confirmDelete() {
  const p = deleteTarget.value!
  showDeleteConfirm.value = false
  deleteTarget.value = null
  try {
    const resp = await deleteSOPlugin(p.id)
    if (resp.code === 0) {
      fetchPlugins()
    }
  } catch { /* ignore */ }
}

// --- Data ---
async function fetchPlugins() {
  try {
    const resp = await listSOPlugins({ limit: 100 })
    if (resp.code === 0) plugins.value = resp.data.items || []
  } catch { /* ignore */ }
}

function statusLabel(status: string): string {
  const map: Record<string, string> = { enabled: '已启用', disabled: '已禁用' }
  return map[status] || status
}

function formatTime(t?: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  const padMs = (n: number) => String(n).padStart(3, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${padMs(d.getMilliseconds())}`
}

onMounted(() => {
  fetchPlugins()
})
</script>

<style scoped>
.plugins-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; justify-content: space-between; align-items: center; }
.page-header h2 { font-size: 18px; font-weight: 600; }

.btn-sm {
  padding: 4px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm);
  background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer;
  transition: all 0.15s ease;
}
.btn-sm.danger { color: var(--accent-danger); border-color: var(--accent-danger); }
.btn-sm.danger:hover { background: var(--accent-danger); color: #fff; }
.btn-sm.warn { color: #e7a23b; border-color: #e7a23b; }
.btn-sm.warn:hover { background: #e7a23b; color: #fff; }
.btn-sm.ok { color: var(--accent-success); border-color: var(--accent-success); }
.btn-sm.ok:hover { background: var(--accent-success); color: #fff; }

.form-error {
  font-size: 12px; color: var(--accent-danger, #e74c3c);
  background: rgba(248,81,73,0.1); padding: 6px 10px;
  border-radius: var(--radius-sm); margin-bottom: 8px;
}
/* Reserve fixed height for error area to prevent dialog height jitter */
.form-error-slot { min-height: 32px; margin-bottom: 8px; }
.required { color: var(--accent-danger); }
.text-muted { color: var(--text-tertiary); font-size: 12px; }

.table-wrapper {
  background: var(--bg-card); border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md); overflow: auto;
}
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td {
  padding: 10px 14px; text-align: left; font-size: 13px;
  border-bottom: 1px solid var(--border-secondary);
}
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: var(--font-mono); font-size: 12px; }
.path { max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.actions { display: flex; gap: 6px; }

.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.enabled { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.disabled { background: rgba(139,148,158,0.15); color: var(--text-tertiary); }

.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.5);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal {
  background: var(--bg-card); border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg); padding: 24px; width: 480px; max-height: 80vh; overflow-y: auto;
}
.modal h3 { font-size: 16px; margin-bottom: 16px; }
.modal pre {
  background: var(--bg-tertiary); padding: 12px; border-radius: var(--radius-sm);
  font-size: 12px; line-height: 1.5; max-height: 300px; overflow: auto;
  white-space: pre-wrap; word-break: break-all;
}
.form-group { margin-bottom: 14px; }
.form-group label { display: block; font-size: 13px; color: var(--text-secondary); margin-bottom: 4px; }
.form-group input, .form-group textarea {
  width: 100%; padding: 8px 10px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm); background: var(--bg-input);
  color: var(--text-primary); font-size: 13px; outline: none; box-sizing: border-box;
}
.form-group textarea { resize: vertical; font-family: var(--font-mono); }
.form-group input:focus, .form-group textarea:focus { border-color: var(--accent-primary); }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }

/* File upload */
.file-upload-wrapper { display: flex; align-items: center; gap: 8px; }
.file-input-hidden { display: none; }
.btn-file-select {
  padding: 8px 14px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm);
  background: var(--bg-tertiary); color: var(--text-primary); font-size: 13px;
  cursor: pointer; white-space: nowrap; transition: all 0.15s ease;
}
.btn-file-select:hover { background: var(--bg-hover); border-color: var(--accent-primary); }
.file-info { font-size: 12px; color: var(--text-secondary); }
.btn-file-clear {
  padding: 4px 8px; border: none; border-radius: var(--radius-sm);
  background: transparent; color: var(--accent-danger); font-size: 14px;
  cursor: pointer; line-height: 1;
}
.btn-file-clear:hover { background: rgba(248,81,73,0.1); }
.config-preview { background: var(--bg-tertiary); padding: 12px; border-radius: var(--radius-sm); font-size: 12px; line-height: 1.5; max-height: 300px; overflow: auto; white-space: pre-wrap; word-break: break-all; }

.confirm-dialog {
  background: var(--bg-card); border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg); padding: 28px; width: 380px;
  text-align: center; box-shadow: 0 20px 60px rgba(0,0,0,0.3), 0 0 1px rgba(0,0,0,0.15);
}
.confirm-icon {
  width: 48px; height: 48px; margin: 0 auto 16px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: rgba(248,81,73,0.12); color: var(--accent-danger);
}
.confirm-title { font-size: 16px; font-weight: 600; color: var(--text-primary); margin: 0 0 8px; }
.confirm-msg { font-size: 13px; color: var(--text-secondary); margin: 0 0 24px; line-height: 1.5; }
.confirm-actions { display: flex; justify-content: center; gap: 10px; }
.btn-cancel {
  padding: 8px 20px; border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  background: var(--bg-tertiary); color: var(--text-primary); font-size: 13px; cursor: pointer;
}
.btn-cancel:hover { background: var(--bg-hover); }
.btn-danger-confirm {
  padding: 8px 20px; border: none; border-radius: var(--radius-md);
  background: var(--accent-danger); color: #fff; font-size: 13px; cursor: pointer;
}
.btn-danger-confirm:hover { opacity: 0.88; }
</style>