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
  id: string
  email: string
  nickname: string
  role_id: string
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
  role_id: string
}

export interface UpdateUserRequest {
  id: string
  nickname?: string
  role_id?: string
  status?: string
}

export interface RoleDTO {
  id: string
  name: string
  description: string
  is_builtin: boolean
  permissions: PermissionDTO[]
  created_at: string
  updated_at: string
}

export interface PermissionDTO {
  id: string
  resource: string
  action: string
  description: string
}

export interface SceneDTO {
  id: string
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
  id: string
  scene_id: string
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
  id: string
  scene_id: string
  run_id: string
  status: string
  error?: string
  spans: SpanDTO[]
  started_at: string
  finished_at: string
  duration_ns: number
}

export interface SpanDTO {
  id: string
  trace_id: string
  node_id: string
  status: string
  error?: string
  input?: string
  output?: string
  started_at: string
  finished_at: string
  duration_ns: number
}

export interface ReportDTO {
  id: string
  scene_id: string
  run_id: string
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

export interface SceneStatusDTO {
  scene_id: string
  run_id: string
  status: string
  workers: number
  total_reqs: number
  success_reqs: number
  failed_reqs: number
  duration_seconds: number
}

export interface StartSceneRequest {
  scene_id: string
  workers?: number
  run_mode?: string
  count?: number
  duration?: number
  timeout?: number
  variables?: Record<string, string>
}
