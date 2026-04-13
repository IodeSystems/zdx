// API layer for zdx server.
// Types generated from /openapi.json via openapi-typescript. Run bin/api-types to regenerate.
// Pattern: use client from ./client for new hooks; migrate legacy hooks incrementally.

export type {
  OKResponse,
  ProjectResp,
  IssueResp,
  IssueWorkResp,
  TaskResp,
  FeatureResp,
  SpecResp,
  SoloItem,
  CreateProjectRequest,
  CreateIssueRequest,
  UpdateIssueRequest,
  CloseIssueRequest,
  AppendIssueWorkRequest,
  CreateTaskRequest,
  UpdateTaskStatusRequest,
  MarkTaskDoneRequest,
  MarkTaskUndoneRequest,
  CreateFeatureRequest,
  UpdateFeatureFieldRequest,
  AddSpecRequest,
} from './generated-types'

import type {
  OKResponse,
  ProjectResp,
  IssueResp,
  TaskResp,
  FeatureResp,
  SoloItem,
  CreateProjectRequest,
  CreateIssueRequest,
  UpdateIssueRequest,
  CloseIssueRequest,
  AppendIssueWorkRequest,
  CreateTaskRequest,
  UpdateTaskStatusRequest,
  MarkTaskDoneRequest,
  MarkTaskUndoneRequest,
  CreateFeatureRequest,
  UpdateFeatureFieldRequest,
  AddSpecRequest,
} from './generated-types'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { client, getToken, setToken, clearToken } from './client'
export { getToken, setToken, clearToken }
import type { components } from '../api.gen'

// Re-export new generated types for incremental migration
export type { components }
export type IssueItem = components['schemas']['IssueItem']

export type MeItem = { id: number; email: string; name: string; role: string }

// ── fetch utility ─────────────────────────────────────────────────────────────

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

export const useLogin = () => {
  const qc = useQueryClient()
  return useMutation<{ token: string; email: string; role: string }, Error, { email: string; password: string }>({
    mutationFn: async (body) => {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).title ?? 'Login failed')
      return res.json()
    },
    onSuccess: ({ token }) => {
      setToken(token)
      qc.invalidateQueries({ queryKey: ['me'] })
    },
  })
}

export const useRegister = () => {
  const qc = useQueryClient()
  return useMutation<{ token: string; email: string; role: string }, Error, { email: string; name: string; password: string; invite_code: string }>({
    mutationFn: async (body) => {
      const res = await fetch('/api/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).title ?? 'Registration failed')
      return res.json()
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

const json = (body: unknown) => ({
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})

const jsonPut = (body: unknown) => ({
  method: 'PUT',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})

// ── projects ──────────────────────────────────────────────────────────────────

export const useProjects = () =>
  useQuery<ProjectResp[]>({
    queryKey: ['projects'],
    queryFn: () => apiFetch<{ projects: ProjectResp[] }>('/api/projects').then(r => r.projects ?? []),
  })

export const useCreateProject = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, CreateProjectRequest>({
    mutationFn: (body) => apiFetch('/api/project', json(body)),
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
  useQuery<IssueResp>({
    queryKey: ['issue', slug, id],
    queryFn: () => apiFetch(`/api/dx/issue?slug=${encodeURIComponent(slug)}&id=${encodeURIComponent(id)}`),
    enabled: !!slug && !!id,
  })

export const useCreateIssue = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, CreateIssueRequest>({
    mutationFn: (body) => apiFetch('/api/dx/issue', json(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['issues', v.slug] }),
  })
}

export const useUpdateIssue = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, UpdateIssueRequest>({
    mutationFn: (body) => apiFetch('/api/dx/issue', jsonPut(body)),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['issues', v.slug] })
      qc.invalidateQueries({ queryKey: ['issue', v.slug, v.id] })
    },
  })
}

export const useCloseIssue = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, CloseIssueRequest>({
    mutationFn: (body) => apiFetch('/api/dx/issue/close', json(body)),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['issues', v.slug] })
      qc.invalidateQueries({ queryKey: ['issue', v.slug, v.id] })
    },
  })
}

export const useAppendIssueWork = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, AppendIssueWorkRequest>({
    mutationFn: (body) => apiFetch('/api/dx/issue/work', json(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['issue', v.slug, v.id] }),
  })
}

// ── tasks ─────────────────────────────────────────────────────────────────────

export const useTasks = (slug: string, opts?: { feature?: string; issue?: string }) =>
  useQuery<TaskResp[]>({
    queryKey: ['tasks', slug, opts],
    queryFn: () => {
      const params = new URLSearchParams({ slug })
      if (opts?.feature) params.set('feature', opts.feature)
      if (opts?.issue) params.set('issue', opts.issue)
      return apiFetch<{ tasks: TaskResp[] }>(`/api/tasks?${params}`).then(r => r.tasks ?? [])
    },
    enabled: !!slug,
  })

export const useCreateTask = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, CreateTaskRequest>({
    mutationFn: (body) => apiFetch('/api/dx/task', json(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
  })
}

export const useUpdateTaskStatus = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, UpdateTaskStatusRequest>({
    mutationFn: (body) => apiFetch('/api/dx/task/status', jsonPut(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
  })
}

export const useMarkTaskDone = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, MarkTaskDoneRequest>({
    mutationFn: (body) => apiFetch('/api/dx/task/done', json(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
  })
}

export const useMarkTaskUndone = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, MarkTaskUndoneRequest>({
    mutationFn: (body) => apiFetch('/api/dx/task/undone', json(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['tasks', v.slug] }),
  })
}

// ── features ──────────────────────────────────────────────────────────────────

export const useFeatures = (slug: string) =>
  useQuery<FeatureResp[]>({
    queryKey: ['features', slug],
    queryFn: () => apiFetch<{ features: FeatureResp[] }>(`/api/features?slug=${encodeURIComponent(slug)}`).then(r => r.features ?? []),
    enabled: !!slug,
  })

export const useFeature = (slug: string, name: string) =>
  useQuery<FeatureResp>({
    queryKey: ['feature', slug, name],
    queryFn: () => apiFetch(`/api/dx/feature?slug=${encodeURIComponent(slug)}&name=${encodeURIComponent(name)}`),
    enabled: !!slug && !!name,
  })

export const useCreateFeature = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, CreateFeatureRequest>({
    mutationFn: (body) => apiFetch('/api/dx/feature', json(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['features', v.slug] }),
  })
}

export const useUpdateFeatureField = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, UpdateFeatureFieldRequest>({
    mutationFn: (body) => apiFetch('/api/dx/feature/field', jsonPut(body)),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['features', v.slug] })
      qc.invalidateQueries({ queryKey: ['feature', v.slug, v.feature] })
    },
  })
}

export const useAddSpec = () => {
  const qc = useQueryClient()
  return useMutation<OKResponse, Error, AddSpecRequest>({
    mutationFn: (body) => apiFetch('/api/dx/spec', json(body)),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ['feature', v.slug, v.feature] }),
  })
}

// ── solo ──────────────────────────────────────────────────────────────────────

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
