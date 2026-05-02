import { render, screen, cleanup, within } from '@testing-library/react'
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
}))

import { useEnvironments, useRequestEnvironmentTodo } from '../../../../api'

const mockedUseEnvironments = jest.mocked(useEnvironments)
const mockedUseRequestEnvironmentTodo = jest.mocked(useRequestEnvironmentTodo)

function makeEnv(overrides: Partial<EnvironmentItem> = {}): EnvironmentItem {
  return {
    id: 1,
    name: 'staging',
    url: 'https://staging.example.com',
    release_branch: 'release/v1.x',
    current_build_sha: 'abc12345def67890',
    current_build_branch: 'release/v1.x',
    deployed_at: new Date(Date.now() - 3600_000).toISOString(),
    created_at: '2026-01-01T00:00:00Z',
    drift_count: 0,
    drift_oldest_age_secs: 0,
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  mockedUseRequestEnvironmentTodo.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
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

describe('ReleaseIndexGroupsByVersion (spec 155)', () => {
  test('groups environments under their version branch header', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging', release_branch: 'release/v1.x', current_build_sha: 'aaaa1111', current_build_branch: 'release/v1.x', url: 'https://staging.example.com' }),
        makeEnv({ id: 2, name: 'production', release_branch: 'release/v2.x', current_build_sha: 'bbbb2222', current_build_branch: 'release/v2.x', url: 'https://prod.example.com' }),
      ],
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('version-group-release/v1.x')).toBeInTheDocument()
    expect(screen.getByTestId('version-group-release/v2.x')).toBeInTheDocument()
  })

  test('shows short SHA for each environment', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging', release_branch: 'release/v1.x', current_build_sha: 'abc12345def67890' }),
      ],
      isLoading: false,
    } as any)

    await renderPage()

    const shaCell = screen.getByTestId('env-sha-staging')
    expect(shaCell).toHaveTextContent('abc12345')
  })

  test('shows build branch for each environment', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging', release_branch: 'release/v1.x', current_build_branch: 'release/v1.x' }),
      ],
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('env-branch-staging')).toHaveTextContent('release/v1.x')
  })

  test('shows deployed-at for each environment', async () => {
    const deployedAt = new Date(Date.now() - 2 * 3600_000).toISOString()
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging', release_branch: 'release/v1.x', deployed_at: deployedAt }),
      ],
      isLoading: false,
    } as any)

    await renderPage()

    const cell = screen.getByTestId('env-deployed-at-staging')
    expect(cell).not.toHaveTextContent('—')
    expect(cell).toHaveTextContent('ago')
  })

  test('shows URL for each environment', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging', release_branch: 'release/v1.x', url: 'https://staging.example.com' }),
      ],
      isLoading: false,
    } as any)

    await renderPage()

    const urlCell = screen.getByTestId('env-url-staging')
    expect(urlCell).toHaveTextContent('https://staging.example.com')
  })

  test('environments without release_branch grouped under (no version branch)', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'sandbox', release_branch: '' }),
      ],
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('version-group-(no version branch)')).toBeInTheDocument()
  })

  test('two envs on same branch appear under same group header', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging-us', release_branch: 'release/v1.x' }),
        makeEnv({ id: 2, name: 'staging-eu', release_branch: 'release/v1.x' }),
      ],
      isLoading: false,
    } as any)

    await renderPage()

    const headers = screen.getAllByTestId('version-group-release/v1.x')
    expect(headers).toHaveLength(1)
    expect(screen.getByTestId('env-row-staging-us')).toBeInTheDocument()
    expect(screen.getByTestId('env-row-staging-eu')).toBeInTheDocument()
  })
})

describe('ReleaseIndexMobileCardPattern (spec 161)', () => {
  function renderAtMobile() {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 375, writable: true })
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: (query: string) => ({
        matches: query.includes('max-width') && /max-width:[^,]*[6-9]\d{2}px/.test(query),
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        dispatchEvent: jest.fn(),
      }),
    })
    return renderPage()
  }

  test('spec 161: env card has header containing version label and drift indicator', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({
          id: 1,
          name: 'staging',
          release_branch: 'release/v1.x',
          drift_count: 3,
          drift_oldest_age_secs: Math.floor(Date.now() / 1000) - 86400,
        }),
      ],
      isLoading: false,
    } as any)

    await renderAtMobile()

    const header = screen.getByTestId('env-card-header-staging')
    expect(header).toBeInTheDocument()
    expect(header).toHaveTextContent('release/v1.x')
    expect(header).toHaveTextContent('3 behind')
  })

  test('spec 161: env card has content containing environment name and URL', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({
          id: 1,
          name: 'staging',
          release_branch: 'release/v1.x',
          url: 'https://staging.example.com',
        }),
      ],
      isLoading: false,
    } as any)

    await renderAtMobile()

    const content = screen.getByTestId('env-card-content-staging')
    expect(content).toBeInTheDocument()
    expect(content).toHaveTextContent('staging')
    expect(content).toHaveTextContent('https://staging.example.com')
  })

  test('spec 161: env card has footer containing action buttons', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging', release_branch: 'release/v1.x' }),
      ],
      isLoading: false,
    } as any)

    await renderAtMobile()

    const footer = screen.getByTestId('env-card-footer-staging')
    expect(footer).toBeInTheDocument()
    const shipBtn = within(footer).getByText('Ship')
    expect(shipBtn).toBeInTheDocument()
    expect(shipBtn.closest('button')).not.toBeNull()
  })

  test('spec 161: header/content/footer regions are all present per env row', async () => {
    mockedUseEnvironments.mockReturnValue({
      data: [
        makeEnv({ id: 1, name: 'staging', release_branch: 'release/v1.x' }),
        makeEnv({ id: 2, name: 'production', release_branch: 'release/v1.x' }),
      ],
      isLoading: false,
    } as any)

    await renderAtMobile()

    for (const name of ['staging', 'production']) {
      expect(screen.getByTestId(`env-card-header-${name}`)).toBeInTheDocument()
      expect(screen.getByTestId(`env-card-content-${name}`)).toBeInTheDocument()
      expect(screen.getByTestId(`env-card-footer-${name}`)).toBeInTheDocument()
    }
  })
})
