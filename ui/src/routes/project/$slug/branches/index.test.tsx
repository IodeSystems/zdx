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
    useMarkBranchEOL: jest.fn(),
    useDeleteBranch: jest.fn(),
    useUpdateBranch: jest.fn(),
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

import { useBranches, useBranchDoctorRung, useDeferDoctorCheck, useMarkBranchEOL, useDeleteBranch, useUpdateBranch } from '../../../../api'

const mockedUseBranches = jest.mocked(useBranches)
const mockedUseBranchDoctorRung = jest.mocked(useBranchDoctorRung)
const mockedUseDeferDoctorCheck = jest.mocked(useDeferDoctorCheck)
const mockedUseMarkBranchEOL = jest.mocked(useMarkBranchEOL)
const mockedUseDeleteBranch = jest.mocked(useDeleteBranch)
const mockedUseUpdateBranch = jest.mocked(useUpdateBranch)

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
  mockedUseDeferDoctorCheck.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
  mockedUseMarkBranchEOL.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
  mockedUseDeleteBranch.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
  mockedUseUpdateBranch.mockReturnValue({ mutate: jest.fn(), isPending: false, isError: false } as any)
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

describe('BranchCard EOL toggle (TK-1679)', () => {
  beforeEach(() => {
    mockedUseBranchDoctorRung.mockReturnValue({ data: makeRung(), isLoading: false } as any)
    mockedUseDeleteBranch.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
  })

  test('EOL branch renders card with reduced opacity', async () => {
    mockedUseMarkBranchEOL.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'v1.0.x', role: 'named-release', status: 'eol' })],
      isLoading: false,
    } as any)

    await renderPage()

    const card = document.getElementById('branch-v1.0.x')
    expect(card).not.toBeNull()
    expect(card).toHaveAttribute('data-eol', 'true')
  })

  test('clicking EOL toggle calls mark.mutate with slug and name', async () => {
    const mutate = jest.fn()
    mockedUseMarkBranchEOL.mockReturnValue({ mutate, isPending: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'v1.0.x', role: 'named-release', status: 'active' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-eol-v1.0.x'))

    expect(mutate).toHaveBeenCalledWith({ slug: 'test-project', name: 'v1.0.x' })
  })
})

describe('BranchCard delete (TK-1680)', () => {
  beforeEach(() => {
    mockedUseBranchDoctorRung.mockReturnValue({ data: makeRung(), isLoading: false } as any)
  })

  test('clicking Delete shows inline confirm without calling mutate', async () => {
    const mutate = jest.fn()
    mockedUseDeleteBranch.mockReturnValue({ mutate, isPending: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'feature-x', role: 'named-release' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-delete-feature-x'))

    expect(screen.getByTestId('branch-confirm-delete-feature-x')).toBeInTheDocument()
    expect(screen.getByTestId('branch-cancel-delete-feature-x')).toBeInTheDocument()
    expect(screen.queryByTestId('branch-delete-feature-x')).not.toBeInTheDocument()
    expect(mutate).not.toHaveBeenCalled()
  })

  test('confirming delete calls mutate with slug and name then hides confirm', async () => {
    const mutate = jest.fn((_vars: any, opts: any) => opts?.onSettled?.())
    mockedUseDeleteBranch.mockReturnValue({ mutate, isPending: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'feature-x', role: 'named-release' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-delete-feature-x'))
    fe.click(screen.getByTestId('branch-confirm-delete-feature-x'))

    expect(mutate).toHaveBeenCalledWith(
      { slug: 'test-project', name: 'feature-x' },
      expect.objectContaining({ onError: expect.any(Function), onSettled: expect.any(Function) }),
    )
    expect(screen.queryByTestId('branch-confirm-delete-feature-x')).not.toBeInTheDocument()
    expect(screen.getByTestId('branch-delete-feature-x')).toBeInTheDocument()
  })

  test('409 with blocking env names shows toast with Cannot delete message', async () => {
    let capturedOnError: ((e: Error) => void) | undefined
    const mutate = jest.fn((_vars: any, opts: any) => {
      capturedOnError = opts?.onError
      opts?.onSettled?.()
    })
    mockedUseDeleteBranch.mockReturnValue({ mutate, isPending: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'feature-x', role: 'named-release' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe, act } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-delete-feature-x'))
    fe.click(screen.getByTestId('branch-confirm-delete-feature-x'))

    act(() => {
      capturedOnError?.(new Error('blocked by environments: staging, prod'))
    })

    expect(screen.getByText('Cannot delete: blocked by environments: staging, prod')).toBeInTheDocument()
  })
})

describe('EditBranchDialog (TK-1685)', () => {
  beforeEach(() => {
    mockedUseBranchDoctorRung.mockReturnValue({ data: makeRung(), isLoading: false } as any)
    mockedUseDeleteBranch.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
  })

  test('edit button opens dialog pre-filled with branch values', async () => {
    mockedUseUpdateBranch.mockReturnValue({ mutate: jest.fn(), isPending: false, isError: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'dev', role: 'dev', source_branch_name: 'main' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-edit-dev'))

    expect(screen.getByTestId('edit-branch-role')).toBeInTheDocument()
    expect(screen.getByTestId('edit-branch-source')).toBeInTheDocument()
    expect(screen.getByTestId('edit-branch-submit')).toBeInTheDocument()
  })

  test('submit sends only changed fields', async () => {
    const mutate = jest.fn((_vars: any, opts: any) => opts?.onSuccess?.())
    mockedUseUpdateBranch.mockReturnValue({ mutate, isPending: false, isError: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'dev', role: 'dev', source_branch_name: '' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-edit-dev'))
    fe.click(screen.getByTestId('edit-branch-submit'))

    expect(mutate).toHaveBeenCalledWith(
      { slug: 'test-project', name: 'dev' },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    )
  })

  test('submit sends role when changed', async () => {
    const mutate = jest.fn((_vars: any, opts: any) => opts?.onSuccess?.())
    mockedUseUpdateBranch.mockReturnValue({ mutate, isPending: false, isError: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [
        makeBranch({ id: 1, name: 'dev', role: 'dev', source_branch_name: '' }),
        makeBranch({ id: 2, name: 'main', role: 'rolling-release', source_branch_name: '' }),
      ],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-edit-dev'))

    const roleSelect = screen.getByTestId('edit-branch-role').querySelector('input') as HTMLInputElement
    fe.change(roleSelect, { target: { value: 'named-release' } })

    fe.click(screen.getByTestId('edit-branch-submit'))

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ role: 'named-release' }),
      expect.any(Object),
    )
  })

  test('source_branch_name is null when cleared', async () => {
    const mutate = jest.fn()
    mockedUseUpdateBranch.mockReturnValue({ mutate, isPending: false, isError: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'dev', role: 'dev', source_branch_name: 'main' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-edit-dev'))

    const sourceSelect = screen.getByTestId('edit-branch-source').querySelector('input') as HTMLInputElement
    fe.change(sourceSelect, { target: { value: '' } })

    fe.click(screen.getByTestId('edit-branch-submit'))

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ source_branch_name: null }),
      expect.any(Object),
    )
  })

  test('server error displays inline', async () => {
    const mutate = jest.fn()
    mockedUseUpdateBranch.mockReturnValue({ mutate, isPending: false, isError: true, error: new Error('invalid role') } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'dev', role: 'dev', source_branch_name: '' })],
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('branch-edit-dev'))

    expect(screen.getByTestId('edit-branch-error')).toHaveTextContent('invalid role')
  })
})
