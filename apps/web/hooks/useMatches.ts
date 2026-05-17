'use client'

/**
 * useMatches — fetches match candidates for the current user via POST /api/match.
 * Uses TanStack Query for caching and background refetches.
 */

import { useQuery } from '@tanstack/react-query'
import { fetchMatches } from '../lib/api'
import { MOCK_USER_ID } from '../lib/constants'
import type { Match } from '../lib/types'

interface UseMatchesResult {
  /** The list of matches, or an empty array while loading */
  matches: Match[]
  /** Whether the query is in its initial loading state */
  isLoading: boolean
  /** Any error that occurred */
  error: Error | null
  /** Manual refetch trigger */
  refetch: () => void
}

export function useMatches(userId: string = MOCK_USER_ID): UseMatchesResult {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['matches', userId],
    queryFn: () => fetchMatches(userId),
    staleTime: 5 * 60 * 1000, // 5 minutes
  })

  return {
    matches: data ?? [],
    isLoading,
    error: error as Error | null,
    refetch,
  }
}
