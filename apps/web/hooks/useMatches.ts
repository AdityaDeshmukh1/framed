'use client'

/**
 * useMatches — fetches stored match list for the current user via GET /api/matches/[userId].
 * Uses TanStack Query for caching and background refetches.
 */

import { useQuery } from '@tanstack/react-query'
import { fetchMatchList } from '../lib/api'
import { MOCK_USER_ID } from '../lib/constants'
import type { MatchListItem } from '../lib/types'

interface UseMatchesResult {
  matches: MatchListItem[]
  isLoading: boolean
  error: Error | null
  refetch: () => void
}

export function useMatches(userId: string = MOCK_USER_ID): UseMatchesResult {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['matches', userId],
    queryFn: () => fetchMatchList(userId),
    staleTime: 5 * 60 * 1000,
  })

  return {
    matches: data ?? [],
    isLoading,
    error: error as Error | null,
    refetch,
  }
}
