export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

export interface Pagination {
  total: number
  offset: number
  limit: number
}

export interface ListResponse<T> {
  items: T[]
  pagination: Pagination
}

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  token: string
  user: UserDTO
}

export interface UserDTO {
  id: number
  email: string
  nickname: string
  role_id: number
  role_name: string
  status: string
  last_login_at?: string
  created_at: string
  updated_at: string
}

export interface CreateUserRequest {
  email: string
  password: string
  nickname: string
  role_id: number
}

export interface UpdateUserRequest {
  id: number
  nickname?: string
  role_id?: number
  status?: string
}

export interface RoleDTO {
  id: number
  name: string
  description: string
  is_builtin: boolean
  permissions: PermissionDTO[]
  created_at: string
  updated_at: string
}

export interface PermissionDTO {
  id: number
  resource: string
  action: string
  description: string
}

export interface SceneDTO {
  id: number
  name: string
  description: string
  dag_json: string
  variables: string
  plugins: string
  status: string
  created_at: string
  updated_at: string
}

export interface CreateSceneRequest {
  name: string
  description?: string
  dag_json?: string
  status?: string
}

export interface RunRecordDTO {
  id: number
  scene_id: number
  status: string
  worker_count: number
  run_mode: string
  duration: number
  total_reqs: number
  success_reqs: number
  failed_reqs: number
  avg_latency: number
  p50_latency: number
  p95_latency: number
  p99_latency: number
  error_msg: string
  started_at?: string
  finished_at?: string
  created_at: string
  updated_at: string
}

export interface TraceDTO {
  id: number
  scene_id: number
  run_id: number
  status: string
  error?: string
  spans: SpanDTO[]
  started_at: string
  finished_at: string
  duration_ns: number
}

export interface SpanDTO {
  id: number
  trace_id: number
  node_id: number
  status: string
  error?: string
  input?: string
  output?: string
  started_at: string
  finished_at: string
  duration_ns: number
}

export interface ReportDTO {
  id: number
  scene_id: number
  run_id: number
  status: string
  summary: string
  detail: string
  started_at?: string
  finished_at?: string
  created_at: string
  updated_at: string
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export interface ResetPasswordRequest {
  user_id: number
  new_password: string
}

export interface NodeDTO {
  id: number
  scene_id: number
  name: string
  type: string
  config: string
  position: string
  loop_count: number
  created_at: string
  updated_at: string
}

export interface EdgeDTO {
  id: number
  scene_id: number
  from_node: number
  to_node: number
  condition: string
  priority: number
  created_at: string
  updated_at: string
}

export interface AddNodeRequest {
  scene_id: number
  name: string
  type: string
  config?: string
  position?: string
  loop_count?: number
}

export interface UpdateNodeRequest {
  id: number
  name?: string
  type?: string
  config?: string
  position?: string
  loop_count?: number
}

export interface AddEdgeRequest {
  scene_id: number
  from_node: number
  to_node: number
  condition?: string
  priority?: number
}

export interface VariableDTO {
  id: number
  scene_id: number
  scope: string
  key: string
  value: string
  created_at: string
  updated_at: string
}

export interface HTTPNodeConfig {
  url: string
  method: string
  headers?: Record<string, string>
  body?: string
  timeout?: number
  expect_status?: number
  extract?: Record<string, string>
}

export interface SceneStatusDTO {
  scene_id: number
  run_id: number
  status: string
  workers: number
  total_reqs: number
  success_reqs: number
  failed_reqs: number
  duration_seconds: number
}

export interface StartSceneRequest {
  scene_id: number
  workers?: number
  run_mode?: string
  count?: number
  duration?: number
  timeout?: number
  variables?: Record<string, string>
}

export interface DashboardOverviewDTO {
  total_reqs: number
  success_reqs: number
  failed_reqs: number
  p50_latency: number
  p95_latency: number
  p99_latency: number
  avg_latency: number
  running: number
  recent_runs: RunRecordDTO[]
  node_metrics: NodeMetricDTO[]
  time_series?: TimeSeriesDTO
}

export interface NodeMetricDTO {
  name: string
  type: string
  total_reqs: number
  success_reqs: number
  p50_latency: number
  p95_latency: number
  p99_latency: number
  avg_latency: number
}

export interface TimeSeriesDTO {
  timestamps: string[]
  qps: number[]
  p50: number[]
  p95: number[]
  p99: number[]
  error_rate: number[]
}
