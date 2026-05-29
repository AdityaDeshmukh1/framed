'use client'

import { useQuery } from '@tanstack/react-query'
import { fetchMatchList } from '../lib/api'
import type { MatchListItem } from '../lib/types'

interface UseMatchesResult {
  matches: MatchListItem[]
  isLoading: boolean
  error: Error | null
  refetch: () => void
}

export function useMatches(): UseMatchesResult {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['matches'],
    queryFn: () => fetchMatchList(),
    staleTime: 5 * 60 * 1000,
  })

  return {
    matches: data ?? [],
    isLoading,
    error: error as Error | null,
    refetch,
  }
}
