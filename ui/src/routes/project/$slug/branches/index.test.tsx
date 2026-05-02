import { render, screen, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../../../../theme'
import type { VersionBranchItem, BranchDoctorRung } from '../../../../api'

jest.mock('../../../../api', () => {
  const actual = jest.requireActual('../../../../api')
  return {
    ...actual,
    useBranches: jest.fn(),
    useBranchDoctorRung: jest.fn(),
    useDeferDoctorCheck: jest.fn(),
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

import { useBranches, useBranchDoctorRung, useDeferDoctorCheck } from '../../../../api'

const mockedUseBranches = jest.mocked(useBranches)
const mockedUseBranchDoctorRung = jest.mocked(useBranchDoctorRung)
const mockedUseDeferDoctorCheck = jest.mocked(useDeferDoctorCheck)

function makeBranch(overrides: Partial<VersionBranchItem> = {}): VersionBranchItem {
  return {
    id: 1,
    name: 'dev',
    role: 'dev',
    type: 'dev',
    status: 'active',
    semver: '',
    source_branch_name: '',
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as VersionBranchItem
}

function makeRung(overrides: Partial<BranchDoctorRung> = {}): BranchDoctorRung {
  return {
    status: 'pass',
    current_rung: 3,
    message: 'service at rung 3',
    classification: 'service',
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  mockedUseDeferDoctorCheck.mockReturnValue({
    mutate: jest.fn(),
    isPending: false,
  } as any)
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

describe('Branches UI', () => {
  test('renders branch cards with correct role chip text', async () => {
    mockedUseBranches.mockReturnValue({
      data: [
        makeBranch({ id: 1, name: 'dev', role: 'dev' }),
        makeBranch({ id: 2, name: 'main', role: 'rolling-release', source_branch_name: 'dev' }),
      ],
      isLoading: false,
    } as any)
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung(),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getAllByText('dev').length).toBeGreaterThan(0)
    expect(screen.getAllByText('main').length).toBeGreaterThan(0)
    expect(screen.getAllByText('rolling-release').length).toBeGreaterThan(0)
  })

  test('renders pass rung banner as success alert', async () => {
    mockedUseBranches.mockReturnValue({ data: [], isLoading: false } as any)
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ status: 'pass', current_rung: 3, message: 'service at rung 3' }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByRole('alert')).toHaveAttribute('class', expect.stringContaining('MuiAlert'))
    expect(screen.getByText('service at rung 3')).toBeInTheDocument()
  })

  test('renders fail rung banner with defer button', async () => {
    mockedUseBranches.mockReturnValue({ data: [], isLoading: false } as any)
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ status: 'fail', current_rung: 1, message: 'service at rung 1 — no dev branch row', proposal: 'Advance to rung 3...' }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByText('service at rung 1 — no dev branch row')).toBeInTheDocument()
    expect(screen.getByText('Advance to rung 3...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Defer' })).toBeInTheDocument()
  })

  test('empty state renders when no branches', async () => {
    mockedUseBranches.mockReturnValue({ data: [], isLoading: false } as any)
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung(),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('empty-state')).toBeInTheDocument()
  })
})
