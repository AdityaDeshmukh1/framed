'use client'

import { useQuery } from '@tanstack/react-query'
import { fetchSoul } from '../lib/api'
import type { SoulPortraitResponse } from '../lib/types'

interface UseSoulResult {
  soul: SoulPortraitResponse | null
  isLoading: boolean
  error: Error | null
}

export function useSoul(): UseSoulResult {
  const { data, isLoading, error } = useQuery({
    queryKey: ['soul'],
    queryFn: () => fetchSoul(),
    staleTime: 30 * 60 * 1000,
    retry: false,
  })

  return {
    soul: data ?? null,
    isLoading,
    error: error as Error | null,
  }
}
