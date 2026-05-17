'use client'

/**
 * useSoul — fetches or generates the soul portrait for a given user.
 * Polls the status endpoint then fetches the portrait once complete.
 */

import { useQuery } from '@tanstack/react-query'
import { fetchSoulPortrait } from '../lib/api'
import { MOCK_USER_ID } from '../lib/constants'
import type { SoulPortrait } from '../lib/types'

interface UseSoulResult {
  /** The soul portrait data, or null while loading */
  soul: SoulPortrait | null
  /** Whether the query is in its initial loading state */
  isLoading: boolean
  /** Any error that occurred */
  error: Error | null
}

export function useSoul(userId: string = MOCK_USER_ID): UseSoulResult {
  const { data, isLoading, error } = useQuery({
    queryKey: ['soul', userId],
    queryFn: () => fetchSoulPortrait(userId),
    staleTime: 30 * 60 * 1000, // 30 minutes — portrait is stable
  })

  return {
    soul: data ?? null,
    isLoading,
    error: error as Error | null,
  }
}
