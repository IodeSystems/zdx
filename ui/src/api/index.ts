// API layer for zdx server.
// Types from /openapi.json via openapi-typescript. Run bin/api-types to regenerate.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { client, getToken, setToken, clearToken } from './client'
export { getToken, setToken, clearToken }
import type { components } from '../api.gen'

export type { components }
export type IssueItem = components['schemas']['IssueItem']
export type IssueWorkItem = components['schemas']['IssueWorkItem']
export type TaskItem = components['schemas']['TaskItem']
export type FeatureItem = components['schemas']['FeatureItem']
export type SpecItem = components['schemas']['SpecItem']
export type ProjectItem = components['schemas']['ProjectItem']
export type MeItem = components['schemas']['MeItem']
export type OKBody = components['schemas']['OKBody']
export type ShowIssueResponse = components['schemas']['Show-issueResponse']

// SoloItem is not in the OpenAPI spec (undocumented endpoint).
export interface SoloItem {
  id: string
  kind: string
  summary: string
  issue?: string
  task?: string
  feature?: string
}

// ── fetch utility (used for undocumented endpoints) ────────────────────────────

export async function apiFetch<T = unknown>(url: string, init?: RequestInit): Promise<T> {
  const token = getToken()
  const headers = new Headers(init?.headers)
  if (token) headers.set('X-Api-Key', token)
  const res = await fetch(url, { ...init, headers })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} ${text}`)
  }
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
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects'] }),
  })
}

// ── issues ────────────────────────────────────────────────────────────────────

export const useIssues = (slug: string) =>
  useQuery<IssueItem[]>({
    queryKey: ['issues', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/todo/issue/list', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return data?.issues ?? []
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

export const useUpdateIssue = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, components['schemas']['Update-issueRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/todo/issue/update', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['issues'] })
      qc.invalidateQueries({ queryKey: ['issue'] })
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

export const useAppendIssueWork = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, components['schemas']['Append-issue-workRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/issue-work', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['issue'] }),
  })
}

// ── tasks ─────────────────────────────────────────────────────────────────────

export const useTasks = (slug: string, opts?: { feature?: string; issue?: string }) =>
  useQuery<TaskItem[]>({
    queryKey: ['tasks', slug, opts],
    queryFn: async () => {
      if (opts?.feature) {
        const { data, error } = await client.GET('/api/tasks-by-feature', { params: { query: { slug, feature: opts.feature } } })
        if (error) throw new Error(JSON.stringify(error))
        return data?.tasks ?? []
      }
      if (opts?.issue) {
        const { data, error } = await client.GET('/api/dx/todo/issue/tasks', { params: { query: { slug, issue_id: opts.issue } } })
        if (error) throw new Error(JSON.stringify(error))
        return data?.tasks ?? []
      }
      const { data, error } = await client.GET('/api/tasks', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return data?.tasks ?? []
    },
    enabled: !!slug,
  })

export const useCreateTask = () => {
  const qc = useQueryClient()
  return useMutation<TaskItem, Error, components['schemas']['Add-taskRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/todo/tech/add', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
  })
}

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

export const useMarkTaskDone = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, components['schemas']['Mark-task-doneRequest'] & { slug: string }>({
    mutationFn: async ({ id, test_plan, test_refs }) => {
      const { data, error } = await client.POST('/api/dx/todo/dev/done', { body: { id, test_plan, test_refs } })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
  })
}

export const useMarkTaskUndone = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, components['schemas']['Mark-task-undoneRequest'] & { slug: string }>({
    mutationFn: async ({ id }) => {
      const { data, error } = await client.POST('/api/dx/todo/dev/undone', { body: { id } })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
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

export const useCreateFeature = () => {
  const qc = useQueryClient()
  return useMutation<FeatureItem, Error, components['schemas']['Upsert-featureRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/feature', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['features', v.slug] }),
  })
}

export const useUpdateFeatureField = () => {
  const qc = useQueryClient()
  return useMutation<OKBody, Error, components['schemas']['Set-feature-fieldRequest']>({
    mutationFn: async (body) => {
      const { data, error } = await client.POST('/api/dx/features/field', { body })
      if (error) throw new Error(JSON.stringify(error))
      return data!
    },
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['features', v.slug] })
      qc.invalidateQueries({ queryKey: ['feature', v.slug, v.feature] })
    },
  })
}

// ── solo (undocumented endpoint) ──────────────────────────────────────────────

// ── errors & slow queries ────────────────────────────────────────────────────

export type ErrorReportItem = components['schemas']['ErrorReportItem']
export type SlowQueryItem = components['schemas']['SlowQueryItem']

export const useErrors = (slug: string) =>
  useQuery<ErrorReportItem[]>({
    queryKey: ['errors', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/errors', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return data?.errors ?? []
    },
    enabled: !!slug,
  })

export const useSlowQueries = (slug: string) =>
  useQuery<SlowQueryItem[]>({
    queryKey: ['slow-queries', slug],
    queryFn: async () => {
      const { data, error } = await client.GET('/api/dx/slow-queries', { params: { query: { slug } } })
      if (error) throw new Error(JSON.stringify(error))
      return data?.queries ?? []
    },
    enabled: !!slug,
  })

// ── solo (undocumented endpoint) ──────────────────────────────────────────────

export const useSolo = (slug: string, issueFilter?: string) =>
  useQuery<SoloItem[]>({
    queryKey: ['solo', slug, issueFilter],
    queryFn: () => {
      const params = new URLSearchParams({ slug })
      if (issueFilter) params.set('issue', issueFilter)
      return apiFetch(`/api/dx/solo?${params}`)
    },
    enabled: !!slug,
  })

// ── comments ──────────────────────────────────────────────────────────────────

export interface CommentItem {
  id: number
  target_type: string
  target_id: string
  author: string
  body: string
  created_at: string
}

export const useComments = (slug: string, targetType: string, targetId: string) =>
  useQuery<CommentItem[]>({
    queryKey: ['comments', slug, targetType, targetId],
    queryFn: async () => {
      const res = await apiFetch<{ comments: CommentItem[] }>(
        `/api/dx/comment/list?slug=${encodeURIComponent(slug)}&target_type=${targetType}&target_id=${encodeURIComponent(targetId)}`
      )
      return res.comments ?? []
    },
    enabled: !!slug && !!targetType && !!targetId,
  })

export const useAddComment = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (p: { slug: string; target_type: string; target_id: string; body: string }) =>
      apiFetch<CommentItem>('/api/dx/comment/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(p),
      }),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['comments', vars.slug, vars.target_type, vars.target_id] })
    },
  })
}

// ── revisions ─────────────────────────────────────────────────────────────────

export interface RevisionItem {
  id: number
  target_type: string
  target_id: string
  field: string
  old_val: string
  new_val: string
  agent: string
  created_at: string
}

export const useRevisions = (slug: string, targetType: string, targetId: string) =>
  useQuery<RevisionItem[]>({
    queryKey: ['revisions', slug, targetType, targetId],
    queryFn: async () => {
      const res = await apiFetch<{ revisions: RevisionItem[] }>(
        `/api/dx/revisions?slug=${encodeURIComponent(slug)}&target_type=${targetType}&target_id=${encodeURIComponent(targetId)}`
      )
      return res.revisions ?? []
    },
    enabled: !!slug && !!targetType && !!targetId,
  })
