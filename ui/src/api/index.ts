// API layer for zdx server.
// Types from /openapi.json via openapi-typescript. Dev dx-server regenerates on startup when the spec hash changes.

import { useQuery, useInfiniteQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { client, getToken, setToken, clearToken } from './client'
export { getToken, setToken, clearToken }
import type { components } from '../api.gen'

export type { components }
export type IssueItem = components['schemas']['IssueItem']
export type ShowIssueResponse = components['schemas']['Show-issueResponse']
export type IssueWorkItem = components['schemas']['IssueWorkItem']
export type TaskItem = components['schemas']['TaskItem']
export type FeatureItem = components['schemas']['FeatureItem']
export type SpecItem = components['schemas']['SpecItem']
export type ProjectItem = components['schemas']['ProjectItem']
export type MeItem = components['schemas']['MeItem']
export type OKBody = components['schemas']['OKBody']
export type FocusItem = components['schemas']['FocusItem']

export interface SoloItem {
  id: number
  text: string
  title?: string
  description?: string
  key: string
  kind: string
  target_type: string
  target_id: string
  issue_ref: string
  priority: number
  blocked: boolean
  persona: string
  instructions?: string
  claimed_by?: string
  claimed_at?: string
  created_at: string
  resolved_at?: string
  status: string
}

export interface EvaluateDiff {
  added: SoloItem[]
  removed: SoloItem[]
  changed: { before: SoloItem; after: SoloItem }[]
  unchanged: SoloItem[]
}

// ── fetch utility (used for undocumented endpoints) ────────────────────────────
// Retained only for endpoints served by raw http.Mux handlers (e.g.
// /api/dx/demos) that are not registered with huma and therefore have no
// openapi entry. All other callers should use the typed client from ./client.
// See IS-275 for the tracker.

async function apiFetch<T = unknown>(url: string, init?: RequestInit): Promise<T> { // raw-ok
  const token = getToken()
  const headers = new Headers(init?.headers)
  if (token) headers.set('X-Api-Key', token)
  const res = await fetch(url, { ...init, headers })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} ${text}`)
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') return undefined as T
  return res.json() as Promise<T>
}

// ── auth ──────────────────────────────────────────────────────────────────────

export const useMe = () =>
  useQuery<MeItem | null>({
    queryKey: ['me'],
    queryFn: async () => {
      if (!getToken()) return null
      const res = await fetch('/api/me', { headers: { 'X-Api-Key': getToken()! } })
      if (!res.ok) return null
      return res.json() as Promise<MeItem>
    },
    retry: false,
    staleTime: 60_000,
  })

export const useZdxConfig = () =>
  useQuery({
    queryKey: ['config'],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/config')
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    staleTime: 300_000,
  })

export const useLogin = () => {
  const qc = useQueryClient()
  return useMutation<components['schemas']['Auth-loginResponse'], Error, components['schemas']['Auth-loginRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/auth/login', { body })
      if (error) throw new Error((error as { title?: string }).title ?? 'Login failed')
      return data!
    },
    onSuccess: ({ token }) => {
      setToken(token)
      qc.invalidateQueries({ queryKey: ['me'] })
    },
  })
}

export const useRegister = () => {
  const qc = useQueryClient()
  return useMutation<components['schemas']['Auth-registerResponse'], Error, components['schemas']['Auth-registerRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/auth/register', { body })
      if (error) throw new Error((error as { title?: string }).title ?? 'Registration failed')
      return data!
    },
    onSuccess: ({ token }) => {
      setToken(token)
      qc.invalidateQueries({ queryKey: ['me'] })
    },
  })
}

export const useTokenLogin = () => {
  const qc = useQueryClient()
  return useMutation<MeItem, Error, string>({
    mutationFn: async (token) => {
      const res = await fetch('/api/me', { headers: { 'X-Api-Key': token } })
      if (!res.ok) throw new Error('Invalid token')
      return res.json()
    },
    onSuccess: (_, token) => {
      setToken(token)
      qc.invalidateQueries({ queryKey: ['me'] })
    },
  })
}

export const useLogout = () => {
  const qc = useQueryClient()
  return () => {
    clearToken()
    qc.setQueryData(['me'], null)
    qc.clear()
  }
}


// ── projects ──────────────────────────────────────────────────────────────────

export const useProjects = () =>
  useQuery<ProjectItem[]>({
    queryKey: ['projects'],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/projects')
      if (error) throw new Error(JSON.stringify(error))
      return data?.projects ?? []
    },
  })

export const useCreateProject = () => {
  const qc = useQueryClient()
  return useMutation<ProjectItem, Error, components['schemas']['Create-projectRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/project', { body })
      if (error) {
        const e = error as { detail?: string; title?: string }
        throw new Error(e.detail ?? e.title ?? JSON.stringify(error))
      }
      return data!
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects'] })
      qc.invalidateQueries({ queryKey: ['admin-stats'] })
    },
  })
}

// ── issues ────────────────────────────────────────────────────────────────────

export const useIssues = (slug: string, limit?: number, offset?: number) =>
  useQuery<{ issues: IssueItem[]; total: number }>({
    queryKey: ['issues', slug, limit, offset],
    queryFn: async () => {
      const params: Record<string, string> = { slug }
      if (limit != null) params.limit = String(limit)
      if (offset != null) params.offset = String(offset)
      const { data, error } = await client.GET('/api/dx/todo/issue/list', { params: { query: params as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { issues: data?.issues ?? [], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useIssue = (slug: string, id: string) =>
  useQuery<ShowIssueResponse>({
    queryKey: ['issue', slug, id],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/todo/issue/show', { params: { query: { slug, id } } })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    enabled: !!slug && !!id,
  })

export const useCreateIssue = () => {
  const qc = useQueryClient()
  return useMutation<IssueItem, Error, components['schemas']['Add-issueRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/todo/issue/add', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['issues', v.slug] }),
  })
}

export const useUploadFile = () => {
  return useMutation<{ id: number; url: string }, Error, File>({
    mutationFn: async (file) => {
      const fd = new FormData()
      fd.append('file', file)
      const token = getToken()
      const res = await fetch('/api/upload', {
        method: 'POST',
        headers: token ? { 'X-Api-Key': token } : {},
        body: fd,
      })
      if (!res.ok) throw new Error(`Upload failed: ${res.status}`)
      return res.json()
    },
  })
}

export const useCloseIssue = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, components['schemas']['Close-issueRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/todo/issue/close', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['issues', v.slug] })
      qc.invalidateQueries({ queryKey: ['issue'] })
    },
  })
}

export const useReadyIssue = () => {
  const qc = useQueryClient()
  return useMutation<components['schemas']['Ready-issueResponse'], Error, components['schemas']['Ready-issueRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/todo/issue/ready', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['issues', v.slug] })
      qc.invalidateQueries({ queryKey: ['issue'] })
    },
  })
}

export const useEditIssue = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, components['schemas']['Edit-issueRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/todo/issue/edit', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['issues', v.slug] })
      qc.invalidateQueries({ queryKey: ['issue'] })
    },
  })
}

// ── tasks ─────────────────────────────────────────────────────────────────────

export const useTasks = (slug: string, opts?: { feature?: string; issue?: string; status?: string; search?: string }, limit?: number, offset?: number) =>
  useQuery<{ tasks: TaskItem[]; total: number }>({
    queryKey: ['tasks', slug, opts, limit, offset],
    queryFn: async () => {
      const pageParams = {} as Record<string, string>
      if (limit != null) pageParams.limit = String(limit)
      if (offset != null) pageParams.offset = String(offset)
      if (opts?.status) pageParams.status = opts.status
      if (opts?.search) pageParams.search = opts.search
      if (opts?.feature) {
        const { data, error } = await client.GET('/api/tasks-by-feature', { params: { query: { slug, feature: opts.feature, ...pageParams } as any } })
        if (error) throw new Error(JSON.stringify(error))
        return { tasks: data?.tasks ?? [], total: (data as any)?.total ?? 0 }
      }
      if (opts?.issue) {
        const { data, error } = await client.GET('/api/dx/todo/issue/tasks', { params: { query: { slug, issue_id: opts.issue, ...pageParams } as any } })
        if (error) throw new Error(JSON.stringify(error))
        return { tasks: data?.tasks ?? [], total: (data as any)?.total ?? 0 }
      }
      const { data, error } = await client.GET('/api/tasks', { params: { query: { slug, ...pageParams } as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { tasks: data?.tasks ?? [], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
    placeholderData: keepPreviousData,
  })

export const useTask = (slug: string, id: string) =>
  useQuery<TaskItem>({
    queryKey: ['task', slug, id],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/task', { params: { query: { slug, id } } })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    enabled: !!slug && !!id,
  })

// Includes slug for cache invalidation only — not sent to server.
type UpdateTaskStatusInput = components['schemas']['Update-task-statusRequest'] & { slug: string }

export const useUpdateTaskStatus = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, UpdateTaskStatusInput>({
    mutationFn: async ({ id, status, reason }) => {
      const { data, error } = await client.PUT('/api/task-status', { body: { id, status, reason } })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
  })
}

export const useReadyTask = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, { slug: string; id: number }>({
    mutationFn: async ({ id }) => {
      const { data, error } = await client.POST('/api/dx/todo/task/ready', { body: { id } })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['tasks', v.slug] })
      qc.invalidateQueries({ queryKey: ['task', v.slug] })
    },
  })
}

// Release (unclaim) a task. Empty agent_id triggers an admin release that
// clears the claim regardless of which agent originally took it.
export const useReleaseTask = () => {
  const qc = useQueryClient()
  return useMutation<void, Error, { slug: string; taskId: string }>({
    mutationFn: async ({ taskId }) => {
      const { error } = await client.POST('/api/tasks/{id}/release', {
        params: { path: { id: taskId } },
        body: { agent_id: '' },
      })
      if (error) throw new Error(JSON.stringify(error))
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['task', v.slug] })
      qc.invalidateQueries({ queryKey: ['tasks', v.slug] })
    },
  })
}

// ── features ──────────────────────────────────────────────────────────────────

export const useFeatures = (slug: string) =>
  useQuery<FeatureItem[]>({
    queryKey: ['features', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/features', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return data?.features ?? []
    },
    enabled: !!slug,
  })

export const useFeature = (slug: string, name: string) =>
  useQuery<FeatureItem | null>({
    queryKey: ['feature', slug, name],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/feature', { params: { query: { slug, name } } })
      if (error) throw new Error(JSON.stringify(error))
      return data ?? null
    },
    enabled: !!slug && !!name,
  })

// ── solo (undocumented endpoint) ──────────────────────────────────────────────

// ── errors & slow queries ────────────────────────────────────────────────────

export type ErrorReportItem = components['schemas']['ErrorReportItem']
export const useErrors = (slug: string, limit?: number, offset?: number) =>
  useQuery<{ errors: ErrorReportItem[]; total: number }>({
    queryKey: ['errors', slug, limit, offset],
    queryFn: async () => {
      const params: Record<string, string> = { slug }
      if (limit != null) params.limit = String(limit)
      if (offset != null) params.offset = String(offset)
      const { data, error } = await client.GET('/api/dx/errors', { params: { query: params as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { errors: data?.errors ?? [], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useError = (id: string, slug?: string) =>
  useQuery<ErrorReportItem>({
    queryKey: ['error', id, slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/errors/{id}', {
        params: { path: { id: Number(id) }, query: slug ? { slug } : {} },
      })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    enabled: !!id,
  })

export const useClearErrors = (slug: string) => {
  const qc = useQueryClient()
  return useMutation<void, Error, void>({
    mutationFn: async () => {
      const { error } = await client.DELETE('/api/dx/errors', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['errors', slug] }) },
  })
}

export const useReportError = (slug: string) => {
  const qc = useQueryClient()
  return useMutation<void, Error, { source: string; errorName: string }>({
    mutationFn: async ({ source, errorName }) => {
      const { error } = await client.POST('/api/dx/errors/report', {
        body: {
          slug,
          source,
          endpoint: window.location.href,
          error_name: errorName,
          stack_trace: `Simulated ${source} error for flow testing`,
        },
      })
      if (error) throw new Error(JSON.stringify(error))
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['errors', slug] }) },
  })
}

export interface WorklogEntry {
  issue_id: string
  issue_title: string
  agent: string
  note: string
  created_at: string
}

export const useWorklog = (slug: string, limit?: number, offset?: number) =>
  useQuery<{ entries: WorklogEntry[]; total: number }>({
    queryKey: ['worklog', slug, limit, offset],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      const { data, error } = await client.GET('/api/dx/worklog', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { entries: (data as any)?.entries ?? [], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

// ── solo queue ────────────────────────────────────────────────────────────────

export const useSolo = (slug: string, issueFilter?: string) =>
  useQuery<SoloItem[]>({
    queryKey: ['solo', slug, issueFilter],
    queryFn: async () => {
      const query: Record<string, string> = { slug }
      if (issueFilter) query.issue = issueFilter
      const { data, error } = await client.GET('/api/dx/solo', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as SoloItem[]
    },
    enabled: !!slug,
  })

export interface TodoSessionItem {
  id: number
  session_id: string
  issue_id?: string
  title?: string
  alias?: string
  header?: string
  summary?: string
  status?: string
  created_at: string
  updated_at: string
  closed_at?: string
}

export interface TodoDetailResponse {
  todo: SoloItem
  reservations: ReservationItem[]
  sessions: TodoSessionItem[]
}

export const useTodoDetail = (slug: string, key: string) =>
  useQuery<TodoDetailResponse>({
    queryKey: ['todo-detail', slug, key],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/projects/{slug}/todos/{key}' as any, {
        params: { path: { slug, key } } as any,
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as TodoDetailResponse
    },
    enabled: !!slug && !!key,
  })

export const useEvaluateSolo = () => {
  return useMutation<EvaluateDiff, Error, { slug: string; issue?: string }>({
    mutationFn: async ({ slug, issue }) => {
      const { data, error } = await client.POST('/api/dx/solo/evaluate', {
        body: { slug, issue: issue ?? '' },
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as EvaluateDiff
    },
  })
}

export interface ActiveClaimsResponse {
  todos: {
    id: number
    text: string
    key: string
    kind: string
    issue_ref: string
    claimed_by: string
    claimed_at: string
    lease_expires_at?: string
  }[]
  tasks: {
    id: string
    title: string
    text: string
    issue: string
    task_group: string
    claimed_by: string
    claimed_at: string
    lease_expires_at: string
  }[]
}

export const useActiveClaims = (slug: string) =>
  useQuery<ActiveClaimsResponse>({
    queryKey: ['active-claims', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/solo/claims' as any, {
        params: { query: { slug } as any },
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as ActiveClaimsResponse
    },
    enabled: !!slug,
    refetchInterval: 30_000,
  })

export const useApplySolo = () => {
  const qc = useQueryClient()
  return useMutation<{ ok: boolean }, Error, { slug: string; items: SoloItem[] }>({
    mutationFn: async ({ slug, items }) => {
      const { data, error } = await client.POST('/api/dx/solo/apply', {
        body: { slug, items } as any,
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as { ok: boolean }
    },
    onSuccess: (_, { slug }) => {
      qc.invalidateQueries({ queryKey: ['solo', slug] })
    },
  })
}

// ── comments ──────────────────────────────────────────────────────────────────

export interface CommentItem {
  id: number
  target_type: string
  target_id: string
  author: string
  author_alias?: string
  body: string
  created_at: string
  parent_id?: number | null
  unread?: boolean
}

export const useComments = (slug: string, targetType: string, targetId: string, limit?: number, offset?: number) => {
  const { data: me } = useMe()
  const role = me ? `web:${me.id}` : ''
  return useQuery<{ comments: CommentItem[]; total: number }>({
    queryKey: ['comments', slug, targetType, targetId, role, limit, offset],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug, target_type: targetType, target_id: targetId, role }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      const { data, error } = await client.GET('/api/dx/comment/list', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { comments: (data as any)?.comments ?? [], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug && !!targetType && !!targetId && !!role,
  })
}

export const useAddComment = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (p: { slug: string; target_type: string; target_id: string; body: string }) => {
      const { data, error } = await client.POST('/api/dx/comment/add', { body: p as any })
      if (error) throw new Error(JSON.stringify(error))
      return data as CommentItem
    },
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['comments', vars.slug, vars.target_type, vars.target_id] })
    },
  })
}

export const useMarkCommentsRead = () => {
  const qc = useQueryClient()
  const { data: me } = useMe()
  return useMutation({
    mutationFn: async (p: { slug: string; target_type: string; target_id: string }) => {
      if (!me) throw new Error('not authenticated')
      const role = `web:${me.id}`
      const { data, error } = await client.POST('/api/dx/comment/mark-read', { body: { ...p, role } as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as { ok: boolean }
    },
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['comments', vars.slug, vars.target_type, vars.target_id] })
      qc.invalidateQueries({ queryKey: ['notifications', 'unread-count', vars.slug] })
    },
  })
}

export const useMyComments = (slug: string, limit?: number, offset?: number) =>
  useQuery<{ comments: CommentItem[]; total: number }>({
    queryKey: ['comments', 'mine', slug, limit, offset],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      const { data, error } = await client.GET('/api/dx/comment/mine', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { comments: (data as any)?.comments ?? [], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

// ── notifications ────────────────────────────────────────────────────────────

export const useUnreadCount = (slug: string) =>
  useQuery<number>({
    queryKey: ['notifications', 'unread-count', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/notifications/unread-count', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as any)?.count ?? 0
    },
    enabled: !!slug,
    refetchInterval: 60_000,
  })

export interface UnreadThread {
  target_type: string
  target_id: string
  unread_count: number
  last_unread_at: string
}

export const useUnreadThreads = (slug: string) =>
  useQuery<UnreadThread[]>({
    queryKey: ['notifications', 'unread-threads', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/notifications/unread-threads' as any, { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as any)?.threads ?? []
    },
    enabled: !!slug,
  })

export const useDismissAllNotifications = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (slug: string) => {
      const { data, error } = await client.POST('/api/dx/notifications/dismiss-all' as any, { body: { slug } as any })
      if (error) throw new Error(JSON.stringify(error))
      return data
    },
    onSuccess: (_data, slug) => {
      qc.invalidateQueries({ queryKey: ['notifications', 'unread-count', slug] })
      qc.invalidateQueries({ queryKey: ['notifications', 'unread-threads', slug] })
    },
  })
}

// ── unified history (status + field revisions) ──────────────────────────────

export interface HistoryEvent {
  kind: 'status' | 'field'
  id: number
  created_at: string
  from_status?: string
  to_status?: string
  field?: string
  old_val?: string
  new_val?: string
  agent_id: string
  session_id: string
  user_id: string
}

export const useHistory = (targetType: string, targetId: string) =>
  useQuery<{ events: HistoryEvent[] }>({
    queryKey: ['history', targetType, targetId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/history', {
        params: { query: { target_type: targetType, target_id: targetId } as any },
      })
      if (error) throw new Error(JSON.stringify(error))
      return { events: ((data as any)?.events ?? []) as HistoryEvent[] }
    },
    enabled: !!targetType && !!targetId,
  })

// ── timed ─────────────────────────────────────────────────────────────────────

export interface TimedItem {
  id: number
  name: string
  duration_ms: number
  count: number
  total_ms: number
  avg_ms: number
  source: string
  context_json: Record<string, string>
  created_at: string
}

export interface TimedGroupedItem {
  group_value: string
  entry_count: number
  max_ms: number
  sum_total_ms: number
  sum_count: number
  avg_ms: number
}

export const useTimed = (slug: string, limit?: number, offset?: number, tagFilter?: Record<string, string>) =>
  useQuery<{ items: TimedItem[]; total: number }>({
    queryKey: ['timed', slug, limit, offset, tagFilter],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      if (tagFilter && Object.keys(tagFilter).length > 0) query.tag_filter = JSON.stringify(tagFilter)
      const { data, error } = await client.GET('/api/dx/timed', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { items: ((data as any)?.items ?? []) as TimedItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useTimedGrouped = (slug: string, groupBy: string, tagFilter?: Record<string, string>) =>
  useQuery<{ items: TimedGroupedItem[] }>({
    queryKey: ['timed-grouped', slug, groupBy, tagFilter],
    queryFn: async () => {
      const query: Record<string, string> = { slug, group_by: groupBy }
      if (tagFilter && Object.keys(tagFilter).length > 0) query.tag_filter = JSON.stringify(tagFilter)
      const { data, error } = await client.GET('/api/dx/timed/grouped', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { items: ((data as any)?.items ?? []) as TimedGroupedItem[] }
    },
    enabled: !!slug && !!groupBy,
  })

export const useTimedTagKeys = (slug: string) =>
  useQuery<{ keys: string[] }>({
    queryKey: ['timed-tag-keys', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/timed/tags/keys', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return { keys: ((data as any)?.keys ?? []) as string[] }
    },
    enabled: !!slug,
  })

export const useTimedTagValues = (slug: string, key: string) =>
  useQuery<{ values: string[] }>({
    queryKey: ['timed-tag-values', slug, key],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/timed/tags/values', { params: { query: { slug, key } } })
      if (error) throw new Error(JSON.stringify(error))
      return { values: ((data as any)?.values ?? []) as string[] }
    },
    enabled: !!slug && !!key,
  })

// ── counters ──────────────────────────────────────────────────────────────────

export interface CountedItem {
  id: number
  name: string
  value: number
  count: number
  total_value: number
  avg_value: number
  source: string
  context_json: Record<string, string>
  created_at: string
}

export interface CountedGroupedItem {
  group_value: string
  entry_count: number
  max_value: number
  sum_total_value: number
  sum_count: number
  avg_value: number
}

export const useCounted = (slug: string, limit?: number, offset?: number, tagFilter?: Record<string, string>) =>
  useQuery<{ items: CountedItem[]; total: number }>({
    queryKey: ['counted', slug, limit, offset, tagFilter],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      if (tagFilter && Object.keys(tagFilter).length > 0) query.tag_filter = JSON.stringify(tagFilter)
      const { data, error } = await client.GET('/api/dx/counted', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { items: ((data as any)?.items ?? []) as CountedItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useCountedGrouped = (slug: string, groupBy: string, tagFilter?: Record<string, string>) =>
  useQuery<{ items: CountedGroupedItem[] }>({
    queryKey: ['counted-grouped', slug, groupBy, tagFilter],
    queryFn: async () => {
      const query: Record<string, string> = { slug, group_by: groupBy }
      if (tagFilter && Object.keys(tagFilter).length > 0) query.tag_filter = JSON.stringify(tagFilter)
      const { data, error } = await client.GET('/api/dx/counted/grouped', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { items: ((data as any)?.items ?? []) as CountedGroupedItem[] }
    },
    enabled: !!slug && !!groupBy,
  })

export const useCountedTagKeys = (slug: string) =>
  useQuery<{ keys: string[] }>({
    queryKey: ['counted-tag-keys', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/counted/tags/keys', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return { keys: ((data as any)?.keys ?? []) as string[] }
    },
    enabled: !!slug,
  })

export const useCountedTagValues = (slug: string, key: string) =>
  useQuery<{ values: string[] }>({
    queryKey: ['counted-tag-values', slug, key],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/counted/tags/values', { params: { query: { slug, key } } })
      if (error) throw new Error(JSON.stringify(error))
      return { values: ((data as any)?.values ?? []) as string[] }
    },
    enabled: !!slug && !!key,
  })

// ── log events ──────────────────────────────────────────────────────────────────

export interface LogEventItem {
  id: number
  level: string
  message: string
  source: string
  component: string
  environment: string
  context_json: Record<string, string>
  created_at: string
}

export interface LogEventGroupedItem {
  group_value: string
  entry_count: number
  first_seen: string
  last_seen: string
}

export const useLogEvents = (slug: string, limit?: number, offset?: number, tagFilter?: Record<string, string>) =>
  useQuery<{ items: LogEventItem[]; total: number }>({
    queryKey: ['log-events', slug, limit, offset, tagFilter],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      if (tagFilter && Object.keys(tagFilter).length > 0) query.tag_filter = JSON.stringify(tagFilter)
      const { data, error } = await client.GET('/api/dx/log-events', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { items: ((data as any)?.items ?? []) as LogEventItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useLogEventsGrouped = (slug: string, groupBy: string, tagFilter?: Record<string, string>) =>
  useQuery<{ items: LogEventGroupedItem[] }>({
    queryKey: ['log-events-grouped', slug, groupBy, tagFilter],
    queryFn: async () => {
      const query: Record<string, string> = { slug, group_by: groupBy }
      if (tagFilter && Object.keys(tagFilter).length > 0) query.tag_filter = JSON.stringify(tagFilter)
      const { data, error } = await client.GET('/api/dx/log-events/grouped', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { items: ((data as any)?.items ?? []) as LogEventGroupedItem[] }
    },
    enabled: !!slug && !!groupBy,
  })

export const useLogEventsTagKeys = (slug: string) =>
  useQuery<{ keys: string[] }>({
    queryKey: ['log-events-tag-keys', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/log-events/tags/keys', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return { keys: ((data as any)?.keys ?? []) as string[] }
    },
    enabled: !!slug,
  })

export const useLogEventsTagValues = (slug: string, key: string) =>
  useQuery<{ values: string[] }>({
    queryKey: ['log-events-tag-values', slug, key],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/log-events/tags/values', { params: { query: { slug, key } } })
      if (error) throw new Error(JSON.stringify(error))
      return { values: ((data as any)?.values ?? []) as string[] }
    },
    enabled: !!slug && !!key,
  })

// ── Project git config (admin) ────────────────────────────────────────────────

export interface GitConfig {
  git_url: string
  git_branch: string
  git_token?: string
}

export const useProjectGitConfig = (slug: string) =>
  useQuery<GitConfig>({
    queryKey: ['project-git-config', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/admin/project-git-config', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as GitConfig
    },
    enabled: !!slug,
  })

export const useSetProjectGitConfig = () => {
  const qc = useQueryClient()
  return useMutation<GitConfig, Error, { slug: string } & GitConfig>({
    mutationFn: async (body) => {
      const { data, error } = await client.PUT('/api/admin/project-git-config', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as GitConfig
    },
    onSuccess: (_, vars) => { qc.invalidateQueries({ queryKey: ['project-git-config', vars.slug] }) },
  })
}

export interface GitConfigTestResult {
  ok: boolean
  message: string
}

export const useTestProjectGitConfig = () =>
  useMutation<GitConfigTestResult, Error, { slug: string } & GitConfig>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/admin/project-git-config/test', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as GitConfigTestResult
    },
  })

// ── Project stage (admin) ────────────────────────────────────────────────────

export const useSetProjectStage = () => {
  const qc = useQueryClient()
  return useMutation<{ stage: string }, Error, { slug: string; stage: string }>({
    mutationFn: async (body) => {
      const { data, error } = await client.PUT('/api/admin/project-stage', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as { stage: string }
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['projects'] }) },
  })
}

// ── LLM configs (admin, multi-row) ────────────────────────────────────────

export type LLMConfigBody = components['schemas']['LLMConfigBody']

export interface LLMConfigTestResult {
  ok: boolean
  message: string
}

export const useLLMConfigs = () =>
  useQuery<LLMConfigBody[]>({
    queryKey: ['llm-configs'],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/admin/llm-configs')
      if (error) throw new Error(JSON.stringify(error))
      return (data?.configs ?? []) as LLMConfigBody[]
    },
  })

export const useCreateLLMConfig = () => {
  const qc = useQueryClient()
  return useMutation<LLMConfigBody, Error, LLMConfigBody>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/admin/llm-configs', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data as LLMConfigBody
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['llm-configs'] }) },
  })
}

export const useUpdateLLMConfig = () => {
  const qc = useQueryClient()
  return useMutation<LLMConfigBody, Error, { id: number; body: LLMConfigBody }>({
    mutationFn: async ({ id, body }) => {
      const { data, error } = await client.PUT('/api/admin/llm-configs/{id}', {
        params: { path: { id } },
        body,
      })
      if (error) throw new Error(JSON.stringify(error))
      return data as LLMConfigBody
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['llm-configs'] }) },
  })
}

export const useDeleteLLMConfig = () => {
  const qc = useQueryClient()
  return useMutation<void, Error, number>({
    mutationFn: async (id) => {
      const { error } = await client.DELETE('/api/admin/llm-configs/{id}', {
        params: { path: { id } },
      })
      if (error) throw new Error(JSON.stringify(error))
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['llm-configs'] }) },
  })
}

export const useReorderLLMConfigs = () => {
  const qc = useQueryClient()
  return useMutation<LLMConfigBody[], Error, number[]>({
    mutationFn: async (ids) => {
      const { data, error } = await client.POST('/api/admin/llm-configs/reorder', {
        body: { ids },
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data?.configs ?? []) as LLMConfigBody[]
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['llm-configs'] }) },
  })
}

export const useTestLLMConfig = () =>
  useMutation<LLMConfigTestResult, Error, { id: number; body: LLMConfigBody }>({
    mutationFn: async ({ id, body }) => {
      const { data, error } = await client.POST('/api/admin/llm-configs/{id}/test', {
        params: { path: { id } },
        body,
      })
      if (error) throw new Error(JSON.stringify(error))
      return data as LLMConfigTestResult
    },
  })

export const useLLMModels = (id: number, url: string, enabled: boolean) =>
  useQuery<string[]>({
    queryKey: ['llm-models', id, url],
    enabled: enabled && !!url,
    queryFn: async () => {
      const { data, error } = await client.GET('/api/admin/llm-configs/{id}/models', {
        params: { path: { id }, query: { url } },
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data?.models ?? []) as string[]
    },
  })

// ── Admin stats ──────────────────────────────────────────────────────────────

export interface AdminStats {
  user_count: number
  invite_count: number
  project_count: number
}

export const useAdminStats = () =>
  useQuery<AdminStats>({
    queryKey: ['admin-stats'],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/admin/stats')
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as AdminStats
    },
  })

// ── Users (admin) ────────────────────────────────────────────────────────────

export interface AdminUserItem {
  id: number
  email: string
  name: string
  role: string
  created_at: string
}

export const useAdminUsers = (q?: string) =>
  useQuery<AdminUserItem[]>({
    queryKey: ['admin-users', q ?? ''],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/admin/users', { params: { query: q ? { q } : {} } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.users ?? []) as AdminUserItem[]
    },
  })

export const useUpdateUserRole = () => {
  const qc = useQueryClient()
  return useMutation<void, Error, { id: number; role: string }>({
    mutationFn: async ({ id, role }) => {
      const { error } = await client.PUT('/api/admin/users/{id}/role', {
        params: { path: { id } },
        body: { role },
      })
      if (error) throw new Error(JSON.stringify(error))
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin-users'] }) },
  })
}

// ── Invites (admin) ──────────────────────────────────────────────────────────

export interface InviteItem {
  id: number
  email: string
  token: string
  invited_by: number
  expires_at: string
  used_at?: string
  created_at: string
}

export const useInvites = () =>
  useQuery<InviteItem[]>({
    queryKey: ['invites'],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/admin/invites')
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.invites ?? []) as InviteItem[]
    },
  })

export const useCreateInvite = () => {
  const qc = useQueryClient()
  return useMutation<InviteItem, Error, { email: string }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/admin/invites', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as InviteItem
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['invites'] }) },
  })
}

export const useDeleteInvite = () => {
  const qc = useQueryClient()
  return useMutation<void, Error, number>({
    mutationFn: async (id) => {
      const { error } = await client.DELETE('/api/admin/invites/{id}', { params: { path: { id } } })
      if (error) throw new Error(JSON.stringify(error))
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['invites'] }) },
  })
}

// ── Admin WebSocket diagnostics ───────────────────────────────────────────────

export interface WSClientItem {
  id: number
  channels: string[]
  user_id: number
  remote_addr: string
  connected_at: string
}

export const useWSClients = (refetchInterval = 2000) =>
  useQuery<WSClientItem[]>({
    queryKey: ['admin-ws-clients'],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/admin/ws/clients')
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.clients ?? []) as WSClientItem[]
    },
    refetchInterval,
  })

export const useWSEcho = () =>
  useMutation<{ sent_at: string }, Error, { channel: string; payload?: string }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/admin/ws/echo', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as { sent_at: string }
    },
  })

// ── Issue similarity ──────────────────────────────────────────────────────────

export interface SimilarIssueItem {
  id: string
  title: string
  context: string
  status: string
  score: number
}

export const useSimilarIssues = () =>
  useMutation<SimilarIssueItem[], Error, { slug: string; text: string; n?: number }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/issues/similar', { body })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.issues ?? []) as SimilarIssueItem[]
    },
  })

export const useSearchIssues = (slug: string, q: string, enabled: boolean) =>
  useQuery<IssueItem[]>({
    queryKey: ['issue-search', slug, q],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/todo/issue/search', { params: { query: { slug, q } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.issues ?? []) as IssueItem[]
    },
    enabled: enabled && !!slug && q.length > 1,
    staleTime: 10_000,
  })

// ── Code refs ─────────────────────────────────────────────────────────────────

export interface CodeRefItem {
  id: number
  file_path: string
  git_hash: string
  line_start: number
  line_end: number
  note: string
  created_at: string
}

export const useIssueCodeRefs = (slug: string, issueId: string) =>
  useQuery<CodeRefItem[]>({
    queryKey: ['code-refs', 'issue', slug, issueId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/code-refs/issue', { params: { query: { slug, issue_id: issueId } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.refs ?? []) as CodeRefItem[]
    },
    enabled: !!slug && !!issueId,
  })

export interface CodeSnippetResult {
  content: string
  start_line: number
  end_line: number
  total_lines: number
  hash_used: string
  truncated: boolean
}

export const useCodeSnippet = (
  slug: string,
  refId: number,
  args: { path: string; hash?: string; start?: number; end?: number; pad?: number },
  enabled: boolean,
) =>
  useQuery<CodeSnippetResult>({
    queryKey: ['code-snippet', slug, refId, args.hash ?? '', args.start ?? 0, args.end ?? 0, args.pad ?? 0],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug, path: args.path }
      if (args.hash) query.hash = args.hash
      if (args.start) query.start = args.start
      if (args.end) query.end = args.end
      if (args.pad !== undefined) query.pad = args.pad
      const { data, error } = await client.GET('/api/dx/code-refs/snippet', { params: { query: query as any } })
      if (error) throw new Error((error as any)?.detail || JSON.stringify(error))
      return data as CodeSnippetResult
    },
    enabled: enabled && !!slug && !!args.path,
    staleTime: 5 * 60_000,
  })

export interface ResolutionCommitItem {
  sha: string
  ord: number
}

export interface ResolutionItem {
  id: string
  issue_id: string
  branch_of_origin: string
  resolved_at: string
  author: string
  source: string
  parent_resolution?: string
  commits: ResolutionCommitItem[]
  reverted: boolean
  reverted_by?: string
}

export const useIssueResolutions = (slug: string, issueId: string, branch?: string) =>
  useQuery<ResolutionItem[]>({
    queryKey: ['issue-resolutions', slug, issueId, branch],
    queryFn: async () => {
      const query: Record<string, string> = { slug, id: issueId }
      if (branch) query.branch = branch
      const { data, error } = await client.GET('/api/dx/todo/issue/resolutions', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.resolutions ?? []) as ResolutionItem[]
    },
    enabled: !!slug && !!issueId,
  })

export const useTaskCodeRefs = (slug: string, taskId: string) =>
  useQuery<CodeRefItem[]>({
    queryKey: ['code-refs', 'task', slug, taskId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/code-refs/task', { params: { query: { slug, task_id: taskId } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.refs ?? []) as CodeRefItem[]
    },
    enabled: !!slug && !!taskId,
  })

export type TaskReviewItem = components['schemas']['TaskReviewItem']

export const useTaskReviews = (slug: string, taskId: string) =>
  useQuery<TaskReviewItem[]>({
    queryKey: ['task-reviews', slug, taskId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/task/review', { params: { query: { slug, task_id: taskId } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.reviews ?? []) as TaskReviewItem[]
    },
    enabled: !!slug && !!taskId,
  })

// ── questions ─────────────────────────────────────────────────────────────────

export interface QuestionItem {
  id: number
  category: string
  question: string
  answer: string
  created_at: string
  updated_at: string
  parent_question_id: number | null
}

export const useQuestion = (slug: string, id: number) =>
  useQuery<QuestionItem>({
    queryKey: ['question', slug, id],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/qa/get', { params: { query: { slug, id } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as QuestionItem
    },
    enabled: !!slug && id > 0,
  })

export const useQuestions = (slug: string, limit?: number, offset?: number) =>
  useQuery<{ questions: QuestionItem[]; total: number }>({
    queryKey: ['questions', slug, limit, offset],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      const { data, error } = await client.GET('/api/dx/qa/list', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { questions: ((data as any)?.questions ?? []) as QuestionItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useCreateQuestion = () => {
  const qc = useQueryClient()
  return useMutation<
    QuestionItem,
    Error,
    { slug: string; category: string; question: string; parent_question_id?: number }
  >({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/qa/add', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as QuestionItem
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['questions', v.slug] }),
  })
}

export interface SimilarQuestionItem {
  id: number
  question: string
  answer: string
  score: number
}

export const useSimilarQuestions = () =>
  useMutation<SimilarQuestionItem[], Error, { slug: string; text: string; n?: number }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/qa/similar', { body })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.questions ?? []) as SimilarQuestionItem[]
    },
  })

export const useChildQuestions = (slug: string, parentQuestionId: number | null) =>
  useQuery<{ questions: QuestionItem[] }>({
    queryKey: ['child-questions', slug, parentQuestionId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/qa/children', {
        params: { query: { slug, parent_question_id: parentQuestionId! } as any },
      })
      if (error) throw new Error(JSON.stringify(error))
      return { questions: ((data as any)?.questions ?? []) as QuestionItem[] }
    },
    enabled: !!slug && parentQuestionId != null,
  })

// ── blocker questions ────────────────────────────────────────────────────────

export interface BlockerQuestionItem {
  id: number
  target_type: string
  target_id: string
  context: string
  choices: string[] | null
  answer: string
  answered_by: string
  status: string
  created_at: string
  answered_at: string
}

export const useBlockerQuestions = (slug: string, status?: string, limit?: number, offset?: number) =>
  useQuery<{ questions: BlockerQuestionItem[]; total: number }>({
    queryKey: ['blocker-questions', slug, status, limit, offset],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (status) query.status = status
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      const { data, error } = await client.GET('/api/dx/blocker-questions/list', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { questions: ((data as any)?.questions ?? []) as BlockerQuestionItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useBlockerQuestionsByTarget = (slug: string, targetType: string, targetId: string) =>
  useQuery<{ questions: BlockerQuestionItem[] }>({
    queryKey: ['blocker-questions-by-target', slug, targetType, targetId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/blocker-questions/by-target', {
        params: { query: { slug, target_type: targetType, target_id: targetId } },
      })
      if (error) throw new Error(JSON.stringify(error))
      return { questions: ((data as any)?.questions ?? []) as BlockerQuestionItem[] }
    },
    enabled: !!slug && !!targetType && !!targetId,
  })

export const useAnswerBlockerQuestion = () => {
  const qc = useQueryClient()
  return useMutation<
    BlockerQuestionItem,
    Error,
    { slug: string; id: number; answer: string; answered_by?: string }
  >({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/blocker-questions/answer', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as BlockerQuestionItem
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['blocker-questions', v.slug] })
      qc.invalidateQueries({ queryKey: ['blocker-questions-by-target', v.slug] })
    },
  })
}

// ── question proposals ──────────────────────────────────────────────────────

export interface QuestionProposalItem {
  id: number
  question_id: number
  question_type: string
  title: string
  context: string
  status: string
  denied_reason: string
  created_issue_id: string
  created_at: string
  updated_at: string
}

export const useQuestionProposals = (slug: string, questionId: number, questionType: string) =>
  useQuery<{ proposals: QuestionProposalItem[] }>({
    queryKey: ['question-proposals', slug, questionId, questionType],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/question-proposals/by-question', {
        params: { query: { slug, question_id: questionId, question_type: questionType } as any },
      })
      if (error) throw new Error(JSON.stringify(error))
      return { proposals: ((data as any)?.proposals ?? []) as QuestionProposalItem[] }
    },
    enabled: !!slug && questionId > 0,
  })

export const useAcceptQuestionProposal = () => {
  const qc = useQueryClient()
  return useMutation<
    { proposal: QuestionProposalItem; issue: { id: string } },
    Error,
    { slug: string; id: number }
  >({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/question-proposals/accept', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as { proposal: QuestionProposalItem; issue: { id: string } }
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['question-proposals', v.slug] })
      qc.invalidateQueries({ queryKey: ['issues', v.slug] })
    },
  })
}

export const useDenyQuestionProposal = () => {
  const qc = useQueryClient()
  return useMutation<
    QuestionProposalItem,
    Error,
    { slug: string; id: number; reason: string }
  >({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/question-proposals/deny', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as QuestionProposalItem
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['question-proposals', v.slug] })
    },
  })
}

// ── Spec tests ───────────────────────────────────────────────────────────

export interface SpecTestItem {
  id: number
  component: string
  name: string
  layer: string
  status: string
}

export const useSpecTests = (specId: number, enabled = true) =>
  useQuery<SpecTestItem[]>({
    queryKey: ['spec-tests', specId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/specs/tests', { params: { query: { spec_id: specId } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.tests ?? []) as SpecTestItem[]
    },
    enabled: enabled && specId > 0,
  })

export interface SpecDemoItem {
  id: number
  type: 'cli' | 'video'
  test_component: string
  test_name: string
  url: string
  name: string
}

export const useSpecDemos = (specId: number, enabled = true) =>
  useQuery<SpecDemoItem[]>({
    queryKey: ['spec-demos', specId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/specs/demos', { params: { query: { spec_id: specId } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.demos ?? []) as SpecDemoItem[]
    },
    enabled: enabled && specId > 0,
  })

export interface SpecIssueItem {
  spec_id: number
  issue_id: string
  title: string
  status: string
}

export interface SpecDetailResult {
  spec: SpecItem
  issues: SpecIssueItem[]
}

export const useSpecDetail = (specId: number, enabled = true) =>
  useQuery<SpecDetailResult | null>({
    queryKey: ['spec-detail', specId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/specs/detail', { params: { query: { spec_id: specId } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as SpecDetailResult
    },
    enabled: enabled && specId > 0,
  })

export const linkSpecIssue = async (specId: number, issueId: string) => {
  const { data, error } = await client.POST('/api/dx/specs/link-issue', { body: { spec_id: specId, issue_id: issueId } })
  if (error) throw new Error(JSON.stringify(error))
  return data
}


export const deferSpec = async (specId: number, reason: string) => {
  const { data, error } = await client.POST('/api/dx/specs/defer', { body: { spec_id: specId, reason } })
  if (error) throw new Error(JSON.stringify(error))
  return data
}

export const undeferSpec = async (specId: number) => {
  const { data, error } = await client.POST('/api/dx/specs/undefer', { body: { spec_id: specId } })
  if (error) throw new Error(JSON.stringify(error))
  return data
}

// ── Demos ────────────────────────────────────────────────────────────────

export interface DemoListItem {
  id: number
  test_id: number
  type: 'cli' | 'video'
  name: string
  test_component: string
  test_name: string
  test_status: string
  test_duration_ms: number
  url: string
  recorded_branch?: string
  recorded_sha?: string
}

export interface CLIDemoStep {
  cmd: string
  args: string[]
  stdout: string
  stderr?: string
  exit_code: number
  duration_ms: number
}

export interface CLIDemoData {
  name: string
  recorded_at: string
  steps: CLIDemoStep[]
}

// /api/dx/demos is served by a raw http.Mux handler (not huma-registered),
// so it has no openapi entry. See IS-275 for follow-up to document these.
// Each item carries an HMAC-signed asset URL the browser can hit without X-Api-Key
// (needed because <video src> can't supply auth headers).
export const useDemos = (slug: string) =>
  useQuery<DemoListItem[]>({
    queryKey: ['demos', slug],
    enabled: !!slug,
    queryFn: async () => {
      const res = await apiFetch<{ demos: DemoListItem[] }>(`/api/dx/demos?slug=${encodeURIComponent(slug)}`) // raw-ok
      return res.demos ?? []
    },
  })

export interface DemoSpec {
  id: number
  description: string
  kind: string
  feature_id: number
}

export interface DemoDetail extends DemoListItem {
  specs: DemoSpec[]
}

export const useDemo = (id: number) =>
  useQuery<DemoDetail>({
    queryKey: ['demo', id],
    enabled: !!id,
    queryFn: async () => {
      return apiFetch<DemoDetail>(`/api/dx/demos/${id}`) // raw-ok
    },
  })

export const useDemoConsoleLog = (id: number) =>
  useQuery<string | null>({
    queryKey: ['demo-console-log', id],
    enabled: !!id,
    queryFn: async () => {
      const token = getToken()
      const headers = new Headers()
      if (token) headers.set('X-Api-Key', token)
      const res = await fetch(`/api/dx/demos/${id}/console-log`, { headers })
      if (res.status === 204 || res.status === 404) return null
      if (!res.ok) return null
      return res.text()
    },
  })

// ── Claude sessions ──────────────────────────────────────────────────────────

export interface ClaudeSessionItem {
  id: number
  issue_id: string
  session_id: string
  title: string
  header: string
  summary: string
  status: string
  lifecycle: string
  event_count: number
  created_at: string
  updated_at: string
  closed_at?: string
  todo_id?: number
  todo_text?: string
  todo_target_type?: string
  todo_target_id?: string
}

export interface ClaudeEventItem {
  id: number
  seq: number
  event_type: string
  event_json: Record<string, unknown>
  created_at: string
  agent_id: string
  agent_type: string
  agent_description: string
  is_sidechain: boolean
}

export const useClaudeSessions = (slug: string, limit?: number, offset?: number) =>
  useQuery<{ sessions: ClaudeSessionItem[]; total: number }>({
    queryKey: ['claude-sessions', slug, limit, offset],
    queryFn: async () => {
      const query: Record<string, string | number> = { slug }
      if (limit != null) query.limit = limit
      if (offset != null) query.offset = offset
      const { data, error } = await client.GET('/api/dx/claude/sessions', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { sessions: ((data as any)?.sessions ?? []) as ClaudeSessionItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export type ReservationItem = {
  id: number
  target_type: string
  target_id: string
  claimed_by: string
  claimed_at: string
  released_at: string
  lease_expires_at: string
  todo_text?: string
  session_id?: number
  session_status?: string
  session_closed_at?: string
  session_header?: string
  session_alias?: string
}

export const useReservationsByIssue = (slug: string, issueId: string) =>
  useQuery<{ reservations: ReservationItem[] }>({
    queryKey: ['reservations-by-issue', slug, issueId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/solo/reservations', {
        params: { query: { slug, issue_id: issueId } as any },
      })
      if (error) throw new Error(JSON.stringify(error))
      return { reservations: ((data as any)?.reservations ?? []) as ReservationItem[] }
    },
    enabled: !!slug && !!issueId,
  })

export const useInfiniteClaudeSessionEvents = (slug: string, sessionId: number | null, pageSize = 200) =>
  useInfiniteQuery<{ events: ClaudeEventItem[]; total: number }>({
    queryKey: ['claude-events-infinite', slug, sessionId, pageSize],
    queryFn: async ({ pageParam }) => {
      const { data, error } = await client.GET('/api/dx/claude/sessions/{sessionId}/events', {
        params: {
          path: { sessionId: sessionId! },
          query: { slug, limit: pageSize, offset: pageParam as number },
        },
      })
      if (error) throw new Error(JSON.stringify(error))
      return {
        events: ((data as any)?.events ?? []) as ClaudeEventItem[],
        total: (data as any)?.total ?? 0,
      }
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, p) => n + p.events.length, 0)
      return loaded < lastPage.total ? loaded : undefined
    },
    enabled: !!slug && sessionId != null,
  })

export interface TokenUsage {
  input_tokens: number
  output_tokens: number
  cache_read_input_tokens: number
  cache_creation_input_tokens: number
}

export const useClaudeSession = (slug: string, sessionId: number | null) =>
  useQuery<ClaudeSessionItem>({
    queryKey: ['claude-session', slug, sessionId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/claude/sessions/{sessionId}', {
        params: { path: { sessionId: sessionId! }, query: { slug } },
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as ClaudeSessionItem
    },
    enabled: !!slug && sessionId != null,
  })

export const useClaudeSessionTokenUsage = (slug: string, sessionId: number | null) =>
  useQuery<TokenUsage>({
    queryKey: ['claude-token-usage', slug, sessionId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/claude/sessions/{sessionId}/token-usage', {
        params: { path: { sessionId: sessionId! }, query: { slug } },
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as TokenUsage
    },
    enabled: !!slug && sessionId != null,
  })

export interface AgentTokenUsage {
  agent_id: string
  agent_type: string
  agent_description: string
  input_tokens: number
  output_tokens: number
  cache_read_input_tokens: number
  cache_creation_input_tokens: number
  event_count: number
}

export const useClaudeSessionTokenUsageByAgent = (slug: string, sessionId: number | null) =>
  useQuery<{ agents: AgentTokenUsage[] }>({
    queryKey: ['claude-token-usage-by-agent', slug, sessionId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/claude/sessions/{sessionId}/token-usage/by-agent', {
        params: { path: { sessionId: sessionId! }, query: { slug } },
      })
      if (error) throw new Error(JSON.stringify(error))
      return { agents: ((data as any)?.agents ?? []) as AgentTokenUsage[] }
    },
    enabled: !!slug && sessionId != null,
  })

export const useChurnSessions = (slug: string, since?: string) =>
  useQuery<{ sessions: ClaudeSessionItem[]; total: number }>({
    queryKey: ['churn-sessions', slug, since],
    queryFn: async () => {
      const query: Record<string, string> = { slug }
      if (since) query.since = since
      const { data, error } = await client.GET('/api/dx/claude/sessions/churns', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { sessions: ((data as any)?.sessions ?? []) as ClaudeSessionItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useExtractPatternFromSession = () =>
  useMutation<PatternItem, Error, { slug: string; sessionId: number }>({
    mutationFn: async ({ slug, sessionId }) => {
      const { data, error } = await client.POST('/api/dx/claude/sessions/{sessionId}/extract-pattern', {
        params: { path: { sessionId }, query: { slug } },
      })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as PatternItem
    },
  })

// ── Goals & Constraints ──────────────────────────────────────────────────

export interface GoalItem {
  id: number
  title: string
  description: string
  priority: number
  status: string
  created_at: string
  updated_at: string
}

export interface ConstraintItem {
  id: number
  title: string
  description: string
  priority: number
  status: string
  created_at: string
  updated_at: string
}

export const useGoals = (slug: string) =>
  useQuery<GoalItem[]>({
    queryKey: ['goals', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/goals', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.goals ?? []) as GoalItem[]
    },
    enabled: !!slug,
  })

export const useCreateGoal = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: { slug: string; title: string; description: string; priority: number; status?: string }) => {
      const { data, error } = await client.POST('/api/goal', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as GoalItem
    },
    onSuccess: (_, vars) => qc.invalidateQueries({ queryKey: ['goals', vars.slug] }),
  })
}

export const useUpdateGoal = (slug: string) => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: { id: number; title: string; description: string; priority: number; status: string }) => {
      const { data, error } = await client.PUT('/api/goal', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as OKBody
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['goals', slug] }),
  })
}

export const useDeleteGoal = (slug: string) => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: number) => {
      const { data, error } = await client.DELETE('/api/goal', { body: { id } as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as OKBody
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['goals', slug] }),
  })
}

export const useConstraints = (slug: string) =>
  useQuery<ConstraintItem[]>({
    queryKey: ['constraints', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/constraints', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.constraints ?? []) as ConstraintItem[]
    },
    enabled: !!slug,
  })

export const useCreateConstraint = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: { slug: string; title: string; description: string; priority: number; status?: string }) => {
      const { data, error } = await client.POST('/api/constraint', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as ConstraintItem
    },
    onSuccess: (_, vars) => qc.invalidateQueries({ queryKey: ['constraints', vars.slug] }),
  })
}

export const useUpdateConstraint = (slug: string) => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: { id: number; title: string; description: string; priority: number; status: string }) => {
      const { data, error } = await client.PUT('/api/constraint', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as OKBody
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['constraints', slug] }),
  })
}

export const useDeleteConstraint = (slug: string) => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: number) => {
      const { data, error } = await client.DELETE('/api/constraint', { body: { id } as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as OKBody
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['constraints', slug] }),
  })
}

// ── Journal ────────────────────────────────────────────────────────────────

export interface JournalEntryItem {
  id: number
  date: string
  baseline: boolean
  tldr: string
  assessment: string
  concerns: string
  next: string
  changelog_json: string
  state_json: string
}

export const useJournalEntries = (slug: string, role: string) =>
  useQuery<JournalEntryItem[]>({
    queryKey: ['journal', slug, role],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/journal/show', { params: { query: { slug, role } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.entries ?? []) as JournalEntryItem[]
    },
    enabled: !!slug && !!role,
  })

export const useJournalEntry = (slug: string, id: string) =>
  useQuery<JournalEntryItem>({
    queryKey: ['journal-entry', slug, id],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/journal/entry', { params: { query: { slug, id: Number(id) } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as any).entry as JournalEntryItem
    },
    enabled: !!slug && !!id,
  })

export const useGenerateJournalEntry = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: { slug: string; role: string }) => {
      const { data, error } = await client.POST('/api/dx/journal/generate', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as { entry: JournalEntryItem }
    },
    onSuccess: (_, vars) => qc.invalidateQueries({ queryKey: ['journal', vars.slug, vars.role] }),
  })
}

// ── focuses ──────────────────────────────────────────────────────────────────

export const useListFocuses = (slug: string) =>
  useQuery<FocusItem[]>({
    queryKey: ['focuses', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/focuses', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.focuses ?? []) as FocusItem[]
    },
    enabled: !!slug,
  })


export const useAddFocusBlocker = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, { slug: string; focus: string; issue: string }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/focuses/block', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as OKBody
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['focuses', v.slug] }),
  })
}

export const useRemoveFocusBlocker = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, { slug: string; focus: string; issue: string }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/focuses/unblock', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as OKBody
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['focuses', v.slug] }),
  })
}

export const useAddFocus = () => {
  const qc = useQueryClient()
  return useMutation<FocusItem, Error, { slug: string; name: string; description: string; priority: number; blockers: string }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/focuses/add', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as FocusItem
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['focuses', v.slug] }),
  })
}

// ── Tests ─────────────────────────────────────────────────────────────────

export interface TestItem {
  id: number
  component: string
  name: string
  layer: string
  status: string
  duration_ms: number
  last_run_at?: string
}

export interface TestHistoryItem {
  id: number
  driver: string
  test_name: string
  status: string
  duration_ms: number
  run_at: string
}

export const useTests = (slug: string, limit?: number, offset?: number) =>
  useQuery<{ tests: TestItem[]; total: number }>({
    queryKey: ['tests', slug, limit, offset],
    queryFn: async () => {
      const params: Record<string, string> = { slug }
      if (limit != null) params.limit = String(limit)
      if (offset != null) params.offset = String(offset)
      const { data, error } = await client.GET('/api/dx/tests', { params: { query: params as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { tests: (data as any)?.tests ?? [], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useTest = (slug: string, testId: number) =>
  useQuery<TestItem>({
    queryKey: ['test', slug, testId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/tests/detail', { params: { query: { slug, test_id: testId } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as TestItem
    },
    enabled: !!slug && testId > 0,
  })

export const useTestCodeRefs = (slug: string, testId: number) =>
  useQuery<CodeRefItem[]>({
    queryKey: ['code-refs', 'test', slug, testId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/code-refs/test', { params: { query: { slug, test_id: testId } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.refs ?? []) as CodeRefItem[]
    },
    enabled: !!slug && testId > 0,
  })

export const useSpecCodeRefs = (slug: string, specId: number) =>
  useQuery<CodeRefItem[]>({
    queryKey: ['code-refs', 'spec', slug, specId],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/code-refs/spec', { params: { query: { slug, spec_id: specId } } })
      if (error) throw new Error(JSON.stringify(error))
      return ((data as any)?.refs ?? []) as CodeRefItem[]
    },
    enabled: !!slug && specId > 0,
  })

export const useTestHistory = (slug: string, testName: string, enabled: boolean) =>
  useQuery<{ history: TestHistoryItem[] }>({
    queryKey: ['test-history', slug, testName],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/tests/history' as any, {
        params: { query: { slug, test_name: testName, limit: '50' } as any },
      })
      if (error) throw new Error(JSON.stringify(error))
      return { history: (data as any)?.history ?? [] }
    },
    enabled: !!slug && !!testName && enabled,
  })

export const useSetFocusStatus = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, { slug: string; focus: string; status: string }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/focuses/status', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as OKBody
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['focuses', v.slug] }),
  })
}

// ── patterns ─────────────────────────────────────────────────────────────────

export interface PatternItem {
  id: number
  name: string
  description: string
  code_refs: { path: string }[]
  created_at: string
  updated_at: string
}

export const useListPatterns = (slug: string, search?: string) =>
  useQuery<{ patterns: PatternItem[]; total: number }>({
    queryKey: ['patterns', slug, search],
    queryFn: async () => {
      const query: Record<string, string> = { slug }
      if (search) query.search = search
      const { data, error } = await client.GET('/api/dx/patterns', { params: { query: query as any } })
      if (error) throw new Error(JSON.stringify(error))
      return { patterns: ((data as any)?.patterns ?? []) as PatternItem[], total: (data as any)?.total ?? 0 }
    },
    enabled: !!slug,
  })

export const useGetPattern = (slug: string, id: number) =>
  useQuery<PatternItem>({
    queryKey: ['pattern', slug, id],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/patterns/get', { params: { query: { slug, id } } })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as PatternItem
    },
    enabled: !!slug && id > 0,
  })

export const useAddPattern = () => {
  const qc = useQueryClient()
  return useMutation<PatternItem, Error, { slug: string; name: string; description: string; code_refs?: { path: string }[] }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/patterns/add', { body: body as any })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as PatternItem
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['patterns', v.slug] }),
  })
}

export const useDeletePattern = () => {
  const qc = useQueryClient()
  return useMutation<{ ok: boolean }, Error, { slug: string; id: number }>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/patterns/delete', { body })
      if (error) throw new Error(JSON.stringify(error))
      return (data as unknown) as { ok: boolean }
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['patterns', v.slug] }),
  })
}

// ── proposals ─────────────────────────────────────────────────────────────────

export type ProposalItem = components['schemas']['ProposalItem']
export type ShowProposalResponse = components['schemas']['Show-proposalResponse']
export type ApproveProposalResponse = components['schemas']['Approve-proposalResponse']

export const useProposals = (slug: string, status?: string) =>
  useQuery<ProposalItem[]>({
    queryKey: ['proposals', slug, status ?? ''],
    queryFn: async () => {
      const query: { slug: string; status?: string } = { slug }
      if (status) query.status = status
      const { data, error } = await client.GET('/api/dx/proposals', { params: { query } })
      if (error) throw new Error(JSON.stringify(error))
      return data?.proposals ?? []
    },
    enabled: !!slug,
  })

export const useProposal = (slug: string, id: number) =>
  useQuery<ShowProposalResponse>({
    queryKey: ['proposal', slug, id],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/proposals/{id}', {
        params: { path: { id }, query: { slug } },
      })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    enabled: !!slug && id > 0,
  })

export const useUpdateProposal = () => {
  const qc = useQueryClient()
  return useMutation<ProposalItem, Error, { slug: string; id: number; title: string; body: string }>({
    mutationFn: async ({ slug, id, title, body }) => {
      const { data, error } = await client.PATCH('/api/dx/proposals/{id}', {
        params: { path: { id } },
        body: { slug, title, body },
      })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['proposal', v.slug, v.id] })
      qc.invalidateQueries({ queryKey: ['proposals', v.slug] })
    },
  })
}

export const useApproveProposal = () => {
  const qc = useQueryClient()
  return useMutation<ApproveProposalResponse, Error, { slug: string; id: number; issue_type?: string; priority?: number }>({
    mutationFn: async ({ slug, id, issue_type, priority }) => {
      const body: { slug: string; issue_type?: string; priority?: number } = { slug }
      if (issue_type) body.issue_type = issue_type
      if (priority) body.priority = priority
      const { data, error } = await client.POST('/api/dx/proposals/{id}/approve', {
        params: { path: { id } },
        body,
      })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['proposal', v.slug, v.id] })
      qc.invalidateQueries({ queryKey: ['proposals', v.slug] })
      qc.invalidateQueries({ queryKey: ['issues', v.slug] })
    },
  })
}

export const useRejectProposal = () => {
  const qc = useQueryClient()
  return useMutation<ProposalItem, Error, { slug: string; id: number; reason?: string }>({
    mutationFn: async ({ slug, id, reason }) => {
      const body: { slug: string; reason?: string } = { slug }
      if (reason) body.reason = reason
      const { data, error } = await client.POST('/api/dx/proposals/{id}/reject', {
        params: { path: { id } },
        body,
      })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['proposal', v.slug, v.id] })
      qc.invalidateQueries({ queryKey: ['proposals', v.slug] })
    },
  })
}

export const useSnoozeProposal = () => {
  const qc = useQueryClient()
  return useMutation<ProposalItem, Error, { slug: string; id: number; snoozed_until: string }>({
    mutationFn: async ({ slug, id, snoozed_until }) => {
      const { data, error } = await client.POST('/api/dx/proposals/{id}/snooze', {
        params: { path: { id } },
        body: { slug, snoozed_until },
      })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['proposal', v.slug, v.id] })
      qc.invalidateQueries({ queryKey: ['proposals', v.slug] })
    },
  })
}
