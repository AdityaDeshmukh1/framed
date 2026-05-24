'use client'

/**
 * useSoul — fetches an existing soul portrait for a user via GET /api/soul/[userId].
 * Uses TanStack Query for caching (portrait is stable, 30-min stale time).
 */

import { useQuery } from '@tanstack/react-query'
import { fetchSoul } from '../lib/api'
import { MOCK_USER_ID } from '../lib/constants'
import type { SoulPortraitResponse } from '../lib/types'

interface UseSoulResult {
  soul: SoulPortraitResponse | null
  isLoading: boolean
  error: Error | null
}

export function useSoul(userId: string = MOCK_USER_ID): UseSoulResult {
  const { data, isLoading, error } = useQuery({
    queryKey: ['soul', userId],
    queryFn: () => fetchSoul(userId),
    staleTime: 30 * 60 * 1000,
    retry: false, // 404 (not yet generated) should not retry
  })

  return {
    soul: data ?? null,
    isLoading,
    error: error as Error | null,
  }
}
