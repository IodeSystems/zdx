import { useState, useCallback } from 'react'

export const PAGE_SIZE = 200

export function useLoadMore() {
  const [offset, setOffset] = useState(0)

  const loadMore = useCallback(() => {
    setOffset((prev) => prev + PAGE_SIZE)
  }, [])

  const reset = useCallback(() => {
    setOffset(0)
  }, [])

  return { offset, loadMore, reset, pageSize: PAGE_SIZE }
}
