import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'
import type { GoalItem } from '../api'

jest.mock('../api', () => {
  const actual = jest.requireActual('../api')
  return {
    ...actual,
    useGoals: jest.fn(),
    useCreateGoal: jest.fn(),
    useUpdateGoal: jest.fn(),
    useDeleteGoal: jest.fn(),
  }
})

import {
  useGoals,
  useCreateGoal,
  useUpdateGoal,
  useDeleteGoal,
} from '../api'
import { GoalsTab } from './GoalsTab'

const mockedUseGoals = jest.mocked(useGoals)
const mockedUseCreateGoal = jest.mocked(useCreateGoal)
const mockedUseUpdateGoal = jest.mocked(useUpdateGoal)
const mockedUseDeleteGoal = jest.mocked(useDeleteGoal)

function makeGoal(overrides: Partial<GoalItem> = {}): GoalItem {
  return {
    id: 1,
    title: 'Test Goal',
    description: 'Test goal description',
    priority: 2,
    status: 'active',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

let queryClient: QueryClient

function stubMutation() {
  return { mutateAsync: jest.fn().mockResolvedValue({}) } as any
}

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  mockedUseCreateGoal.mockReturnValue(stubMutation())
  mockedUseUpdateGoal.mockReturnValue(stubMutation())
  mockedUseDeleteGoal.mockReturnValue(stubMutation())
})

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

function renderTab(slug = 'test-project') {
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <GoalsTab slug={slug} />
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('GoalsTab', () => {
  test('spec 55: goals section renders items as cards under Goals heading', () => {
    const goals = [
      makeGoal({ id: 1, title: 'Increase retention', description: 'Keep users engaged', priority: 1, status: 'active' }),
      makeGoal({ id: 2, title: 'Reduce churn', description: '', priority: 2, status: 'paused' }),
    ]
    mockedUseGoals.mockReturnValue({ data: goals, isLoading: false } as any)

    renderTab()

    expect(screen.getByText(/Goals \(2\)/i)).toBeInTheDocument()
    expect(screen.getByText('Increase retention')).toBeInTheDocument()
    expect(screen.getByText('Keep users engaged')).toBeInTheDocument()
    expect(screen.getByText('Reduce churn')).toBeInTheDocument()
  })

  test('spec 55: create goal — Add opens form; Save calls useCreateGoal().mutateAsync', async () => {
    const createMutate = jest.fn().mockResolvedValue({})
    mockedUseCreateGoal.mockReturnValue({ mutateAsync: createMutate } as any)
    mockedUseGoals.mockReturnValue({ data: [], isLoading: false } as any)

    renderTab('my-project')

    fireEvent.click(screen.getByRole('button', { name: /add/i }))

    expect(screen.getByText('Add Goal')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled()

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Ship it' } })
    expect(screen.getByRole('button', { name: /save/i })).not.toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(createMutate).toHaveBeenCalledWith({
        slug: 'my-project',
        title: 'Ship it',
        description: '',
        priority: 1,
        status: 'active',
      })
    })
    await waitFor(() => expect(screen.queryByText('Add Goal')).not.toBeInTheDocument())
  })

  test('spec 55: edit goal — Edit icon opens dialog pre-populated; Save calls useUpdateGoal().mutateAsync', async () => {
    const updateMutate = jest.fn().mockResolvedValue({})
    mockedUseUpdateGoal.mockReturnValue({ mutateAsync: updateMutate } as any)
    const goal = makeGoal({ id: 42, title: 'Old Title', description: 'Old desc', priority: 3, status: 'paused' })
    mockedUseGoals.mockReturnValue({ data: [goal], isLoading: false } as any)

    renderTab()

    // buttons order with 1 goal: Add, Edit, Delete
    const buttons = screen.getAllByRole('button')
    fireEvent.click(buttons[1]) // Edit icon

    expect(screen.getByText('Edit Goal')).toBeInTheDocument()
    expect(screen.getByLabelText('Title')).toHaveValue('Old Title')
    expect(screen.getByLabelText('Description')).toHaveValue('Old desc')

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'New Title' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(updateMutate).toHaveBeenCalledWith({
        id: 42,
        title: 'New Title',
        description: 'Old desc',
        priority: 3,
        status: 'paused',
      })
    })
  })

  test('spec 55: delete goal — Delete icon calls useDeleteGoal().mutateAsync with goal id', () => {
    const deleteMutate = jest.fn().mockResolvedValue({})
    mockedUseDeleteGoal.mockReturnValue({ mutateAsync: deleteMutate } as any)
    const goal = makeGoal({ id: 7, title: 'Goal to delete' })
    mockedUseGoals.mockReturnValue({ data: [goal], isLoading: false } as any)

    renderTab()

    // buttons order with 1 goal: Add, Edit, Delete
    const buttons = screen.getAllByRole('button')
    fireEvent.click(buttons[2]) // Delete icon

    expect(deleteMutate).toHaveBeenCalledWith(7)
  })
})
