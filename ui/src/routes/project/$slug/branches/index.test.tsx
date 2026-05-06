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
    useCreateBranch: jest.fn(),
    useSeedBranches: jest.fn(),
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

import { useBranches, useBranchDoctorRung, useDeferDoctorCheck, useMarkBranchEOL, useDeleteBranch, useUpdateBranch, useCreateBranch, useSeedBranches } from '../../../../api'

const mockedUseBranches = jest.mocked(useBranches)
const mockedUseBranchDoctorRung = jest.mocked(useBranchDoctorRung)
const mockedUseDeferDoctorCheck = jest.mocked(useDeferDoctorCheck)
const mockedUseMarkBranchEOL = jest.mocked(useMarkBranchEOL)
const mockedUseDeleteBranch = jest.mocked(useDeleteBranch)
const mockedUseUpdateBranch = jest.mocked(useUpdateBranch)
const mockedUseCreateBranch = jest.mocked(useCreateBranch)
const mockedUseSeedBranches = jest.mocked(useSeedBranches)

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
  mockedUseCreateBranch.mockReturnValue({ mutate: jest.fn(), isPending: false, isError: false } as any)
  mockedUseSeedBranches.mockReturnValue({ mutate: jest.fn(), isPending: false, isError: false } as any)
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

describe('AddBranchDialog (TK-1683)', () => {
  beforeEach(() => {
    mockedUseBranchDoctorRung.mockReturnValue({ data: makeRung(), isLoading: false } as any)
    mockedUseDeleteBranch.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'dev', role: 'dev' })],
      isLoading: false,
    } as any)
  })

  test('dialog renders with name, role, and source fields', async () => {
    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('add-branch-button'))

    expect(screen.getByTestId('add-branch-name')).toBeInTheDocument()
    expect(screen.getByTestId('add-branch-role')).toBeInTheDocument()
    expect(screen.getByTestId('add-branch-source')).toBeInTheDocument()
    expect(screen.getByTestId('add-branch-submit')).toBeInTheDocument()
  })

  test('valid submit calls mutate with slug, name, role, and source', async () => {
    const mutate = jest.fn((_vars: any, opts: any) => opts?.onSuccess?.())
    mockedUseCreateBranch.mockReturnValue({ mutate, isPending: false, isError: false } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('add-branch-button'))

    const nameInput = screen.getByTestId('add-branch-name').querySelector('input') as HTMLInputElement
    fe.change(nameInput, { target: { value: 'feature-x' } })

    fe.click(screen.getByTestId('add-branch-submit'))

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ slug: 'test-project', name: 'feature-x', role: 'dev' }),
      expect.any(Object),
    )
  })

  test('server error displays inline', async () => {
    mockedUseCreateBranch.mockReturnValue({ mutate: jest.fn(), isPending: false, isError: true, error: new Error('name taken') } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('add-branch-button'))

    expect(screen.getByTestId('add-branch-error')).toHaveTextContent('name taken')
  })
})

describe('CutReleaseDialog (TK-1684)', () => {
  beforeEach(() => {
    mockedUseBranchDoctorRung.mockReturnValue({ data: makeRung(), isLoading: false } as any)
    mockedUseDeleteBranch.mockReturnValue({ mutate: jest.fn(), isPending: false } as any)
    mockedUseBranches.mockReturnValue({
      data: [makeBranch({ id: 1, name: 'dev', role: 'dev' })],
      isLoading: false,
    } as any)
  })

  test('dialog renders with name, semver, and source fields', async () => {
    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('cut-release-button'))

    expect(screen.getByTestId('cut-release-name')).toBeInTheDocument()
    expect(screen.getByTestId('cut-release-semver')).toBeInTheDocument()
    expect(screen.getByTestId('cut-release-source')).toBeInTheDocument()
    expect(screen.getByTestId('cut-release-submit')).toBeInTheDocument()
  })

  test('valid submit calls mutate with role named-release', async () => {
    const mutate = jest.fn((_vars: any, opts: any) =>
      opts?.onSuccess?.({ id: 1, name: 'v1.2.x', backport_tasks_created: 3, created_at: '', status: 'active' }),
    )
    mockedUseCreateBranch.mockReturnValue({ mutate, isPending: false, isError: false } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('cut-release-button'))

    const nameInput = screen.getByTestId('cut-release-name').querySelector('input') as HTMLInputElement
    fe.change(nameInput, { target: { value: 'v1.2.x' } })

    const semverInput = screen.getByTestId('cut-release-semver').querySelector('input') as HTMLInputElement
    fe.change(semverInput, { target: { value: '1.2.0' } })

    const sourceSelect = screen.getByTestId('cut-release-source').querySelector('input') as HTMLInputElement
    fe.change(sourceSelect, { target: { value: 'dev' } })

    fe.click(screen.getByTestId('cut-release-submit'))

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ slug: 'test-project', name: 'v1.2.x', semver: '1.2.0', source_branch_name: 'dev', role: 'named-release' }),
      expect.any(Object),
    )
  })

  test('success shows toast with correct backport count', async () => {
    const mutate = jest.fn((_vars: any, opts: any) =>
      opts?.onSuccess?.({ id: 1, name: 'v1.2.x', backport_tasks_created: 5, created_at: '', status: 'active' }),
    )
    mockedUseCreateBranch.mockReturnValue({ mutate, isPending: false, isError: false } as any)

    const { fireEvent: fe, act } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('cut-release-button'))

    const nameInput = screen.getByTestId('cut-release-name').querySelector('input') as HTMLInputElement
    fe.change(nameInput, { target: { value: 'v1.2.x' } })

    const semverInput = screen.getByTestId('cut-release-semver').querySelector('input') as HTMLInputElement
    fe.change(semverInput, { target: { value: '1.2.0' } })

    const sourceSelect = screen.getByTestId('cut-release-source').querySelector('input') as HTMLInputElement
    fe.change(sourceSelect, { target: { value: 'dev' } })

    act(() => {
      fe.click(screen.getByTestId('cut-release-submit'))
    })

    expect(screen.getByText('Release cut. Backport tasks created: 5')).toBeInTheDocument()
  })

  test('server error displays inline', async () => {
    mockedUseCreateBranch.mockReturnValue({ mutate: jest.fn(), isPending: false, isError: true, error: new Error('semver conflict') } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('cut-release-button'))

    expect(screen.getByTestId('cut-release-error')).toHaveTextContent('semver conflict')
  })
})

describe('Empty-state seed preview (TK-1674)', () => {
  beforeEach(() => {
    mockedUseBranches.mockReturnValue({ data: [], isLoading: false } as any)
  })

  test('library classification previews single main branch', async () => {
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'library', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('seed-preview-main')).toBeInTheDocument()
    expect(screen.queryByTestId('seed-preview-dev')).not.toBeInTheDocument()
    expect(screen.getByTestId('seed-branches-button')).toBeInTheDocument()
  })

  test('tool classification previews single main branch', async () => {
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'tool', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('seed-preview-main')).toBeInTheDocument()
    expect(screen.queryByTestId('seed-preview-dev')).not.toBeInTheDocument()
  })

  test('site classification previews single main branch', async () => {
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'site', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('seed-preview-main')).toBeInTheDocument()
    expect(screen.queryByTestId('seed-preview-dev')).not.toBeInTheDocument()
  })

  test('service classification previews dev + main', async () => {
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'service', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('seed-preview-dev')).toBeInTheDocument()
    expect(screen.getByTestId('seed-preview-main')).toBeInTheDocument()
  })

  test('saas classification previews dev + main', async () => {
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'saas', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('seed-preview-dev')).toBeInTheDocument()
    expect(screen.getByTestId('seed-preview-main')).toBeInTheDocument()
  })

  test('unknown classification falls back to manual help text without seed button', async () => {
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: '', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    expect(screen.queryByTestId('seed-branches-button')).not.toBeInTheDocument()
    expect(screen.queryByTestId('seed-preview-main')).not.toBeInTheDocument()
  })

  test('clicking Seed Branches calls mutate with slug', async () => {
    const mutate = jest.fn()
    mockedUseSeedBranches.mockReturnValue({ mutate, isPending: false, isError: false } as any)
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'service', current_rung: 1 }),
      isLoading: false,
    } as any)

    const { fireEvent: fe } = await import('@testing-library/react')
    await renderPage()

    fe.click(screen.getByTestId('seed-branches-button'))

    expect(mutate).toHaveBeenCalledWith({ slug: 'test-project' })
  })

  test('seed error renders inline', async () => {
    mockedUseSeedBranches.mockReturnValue({ mutate: jest.fn(), isPending: false, isError: true, error: new Error('seed failed: db down') } as any)
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'service', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    expect(screen.getByTestId('seed-branches-error')).toHaveTextContent('seed failed: db down')
  })

  test('button shows pending label while seeding', async () => {
    mockedUseSeedBranches.mockReturnValue({ mutate: jest.fn(), isPending: true, isError: false } as any)
    mockedUseBranchDoctorRung.mockReturnValue({
      data: makeRung({ classification: 'service', current_rung: 1 }),
      isLoading: false,
    } as any)

    await renderPage()

    const btn = screen.getByTestId('seed-branches-button') as HTMLButtonElement
    expect(btn).toBeDisabled()
    expect(btn).toHaveTextContent(/Seeding/)
  })
})
