import { describe, test, expect, afterEach, vi, beforeEach } from 'vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../../../../theme'
import type { BlockerQuestionItem } from '../../../../api'

vi.mock('../../../../api', async () => {
  const actual = await vi.importActual<typeof import('../../../../api')>('../../../../api')
  return {
    ...actual,
    useBlockerQuestions: vi.fn(),
    useAnswerBlockerQuestion: vi.fn(),
  }
})

vi.mock('@tanstack/react-router', () => ({
  createFileRoute: () => {
    const fn = (opts: any) => {
      const route = { ...opts, useParams: () => ({ slug: 'test-project', component: 'server' }) }
      return route
    }
    return fn
  },
}))

import { useBlockerQuestions, useAnswerBlockerQuestion } from '../../../../api'

const mockedUseBlockerQuestions = vi.mocked(useBlockerQuestions)
const mockedUseAnswerBlockerQuestion = vi.mocked(useAnswerBlockerQuestion)

function makeBQ(overrides: Partial<BlockerQuestionItem> = {}): BlockerQuestionItem {
  return {
    id: 1,
    target_type: 'issue',
    target_id: 'IS-1',
    context: 'Which database?',
    choices: [],
    answer: '',
    answered_by: '',
    status: 'pending',
    created_at: '2026-01-01T00:00:00Z',
    answered_at: '',
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

async function renderPage() {
  const mod = await import('./blocker-questions')
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

describe('Blocker Questions UI', () => {
  test('spec 5: answering a pending question marks it answered', async () => {
    const mutateAsyncMock = vi.fn().mockResolvedValue({})
    mockedUseBlockerQuestions.mockReturnValue({
      data: { questions: [makeBQ({ id: 10, context: 'Should we migrate?' })], total: 1 },
      isLoading: false,
    } as any)
    mockedUseAnswerBlockerQuestion.mockReturnValue({
      mutateAsync: mutateAsyncMock,
      isPending: false,
      isError: false,
      error: null,
    } as any)

    await renderPage()

    expect(screen.getByText('Should we migrate?')).toBeInTheDocument()
    const input = screen.getByLabelText('Answer')
    fireEvent.change(input, { target: { value: 'Yes, migrate now' } })
    fireEvent.click(screen.getByRole('button', { name: 'Answer' }))

    await waitFor(() => {
      expect(mutateAsyncMock).toHaveBeenCalledWith({
        slug: 'test-project',
        id: 10,
        answer: 'Yes, migrate now',
      })
    })
  })

  test('spec 6: toggling between Pending and All changes filter', async () => {
    mockedUseBlockerQuestions.mockReturnValue({
      data: { questions: [], total: 0 },
      isLoading: false,
    } as any)
    mockedUseAnswerBlockerQuestion.mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    } as any)

    await renderPage()

    expect(mockedUseBlockerQuestions).toHaveBeenCalledWith('test-project', 'pending')

    fireEvent.click(screen.getByRole('button', { name: 'All' }))

    await waitFor(() => {
      expect(mockedUseBlockerQuestions).toHaveBeenCalledWith('test-project', undefined)
    })
  })

  test('spec 7: choices appear as chips', async () => {
    mockedUseBlockerQuestions.mockReturnValue({
      data: { questions: [makeBQ({ choices: ['postgres', 'sqlite', 'mysql'] })], total: 1 },
      isLoading: false,
    } as any)
    mockedUseAnswerBlockerQuestion.mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    } as any)

    await renderPage()

    expect(screen.getByText('postgres')).toBeInTheDocument()
    expect(screen.getByText('sqlite')).toBeInTheDocument()
    expect(screen.getByText('mysql')).toBeInTheDocument()
  })

  test('spec 8: answered question shows answer and attribution', async () => {
    mockedUseBlockerQuestions.mockReturnValue({
      data: { questions: [
        makeBQ({
          status: 'answered',
          answer: 'Use postgres for everything',
          answered_by: 'tech-lead',
        }),
      ], total: 1 },
      isLoading: false,
    } as any)
    mockedUseAnswerBlockerQuestion.mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    } as any)

    await renderPage()

    expect(screen.getByText('Use postgres for everything')).toBeInTheDocument()
    expect(screen.getByText('— tech-lead')).toBeInTheDocument()
  })
})
