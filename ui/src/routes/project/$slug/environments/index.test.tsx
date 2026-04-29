import { render, screen, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../../../../theme'
import type { EnvironmentItem } from '../../../../api'

jest.mock('../../../../api', () => {
  const actual = jest.requireActual('../../../../api')
  return {
    ...actual,
    useEnvironments: jest.fn(),
    useRequestEnvironmentTodo: jest.fn(),
    useCreateEnvironment: jest.fn(),
    useUpdateEnvironment: jest.fn(),
    useDeleteEnvironment: jest.fn(),
  }
})

jest.mock('@tanstack/react-router', () => ({
  createFileRoute: () => {
    const fn = (opts: any) => {
      const route = { ...opts, useParams: () => ({ slug: 'test-project' }) }
      return route
    }
    return fn
  },
  Link: (props: any) => <a href={props.to}>{props.children}</a>,
}))

import {
  useEnvironments,
  useRequestEnvironmentTodo,
  useCreateEnvironment,
  useUpdateEnvironment,
  useDeleteEnvironment,
} from '../../../../api'

const mockedUseEnvironments = jest.mocked(useEnvironments)
const mockedUseRequestEnvironmentTodo = jest.mocked(useRequestEnvironmentTodo)
const mockedUseCreateEnvironment = jest.mocked(useCreateEnvironment)
const mockedUseUpdateEnvironment = jest.mocked(useUpdateEnvironment)
const mockedUseDeleteEnvironment = jest.mocked(useDeleteEnvironment)

function makeEnv(overrides: Partial<EnvironmentItem> = {}): EnvironmentItem {
  return {
    id: 1,
    name: 'staging',
    url: '',
    release_branch: 'release/staging',
    current_build_sha: '',
    current_build_branch: '',
    deployed_at: '',
    created_at: '2026-01-01T00:00:00Z',
    drift_count: 0,
    drift_oldest_age_secs: 0,
    ...overrides,
  }
}

function mockMutations() {
  const noop = { mutate: jest.fn(), isPending: false }
  mockedUseRequestEnvironmentTodo.mockReturnValue(noop as any)
  mockedUseCreateEnvironment.mockReturnValue(noop as any)
  mockedUseUpdateEnvironment.mockReturnValue(noop as any)
  mockedUseDeleteEnvironment.mockReturnValue(noop as any)
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  mockMutations()
})

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

async function renderPage() {
  const mod = await import('./index')
  const PageComponent = (mod as any).Route.component
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <PageComponent />
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('Environment dashboard drift display (spec 156)', () => {
  test('shows drift count chip when release branch is behind dev', async () => {
    const oldestAt = Math.floor(Date.now() / 1000) - 3 * 86400 // 3 days ago
    mockedUseEnvironments.mockReturnValue({
      data: [makeEnv({ drift_count: 5, drift_oldest_age_secs: oldestAt })],
      isLoading: false,
    } as any)

    await renderPage()

    const chip = screen.getByTestId('drift-chip')
    expect(chip).toBeInTheDocument()
    expect(chip).toHaveTextContent('5 behind')
  })

  test('shows dash when drift is zero (zero-drift case)', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [makeEnv({ drift_count: 0, drift_oldest_age_secs: 0 })],
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.queryByTestId('drift-chip')).not.toBeInTheDocument()
    expect(screen.getByTestId('drift-none')).toBeInTheDocument()
  })

  test('environment without release_branch shows no drift chip', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [makeEnv({ release_branch: '', drift_count: 0, drift_oldest_age_secs: 0 })],
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.queryByTestId('drift-chip')).not.toBeInTheDocument()
  })
})
