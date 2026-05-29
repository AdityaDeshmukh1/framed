'use client'

import { useMutation, useQuery } from '@tanstack/react-query'
import { startOnboarding, fetchJobStatus } from '../lib/api'
import type { ScrapeJob } from '../lib/types'
import { useState } from 'react'

interface OnboardingState {
  onboard: (handle: string) => void
  isLoading: boolean
  jobId: string | null
  job: ScrapeJob | null
  error: Error | null
}

export function useOnboarding(): OnboardingState {
  const [jobId, setJobId] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: (handle: string) => startOnboarding(handle),
    onSuccess: (data) => {
      setJobId(data.job_id)
    },
  })

  const { data: job } = useQuery({
    queryKey: ['job', jobId],
    queryFn: () => fetchJobStatus(jobId!),
    enabled: jobId !== null,
    refetchInterval: (query) => {
      const status = query.state.data?.status
      if (status === 'complete' || status === 'failed') return false
      return 2000
    },
  })

  return {
    onboard: (handle) => mutation.mutate(handle),
    isLoading: mutation.isPending,
    jobId,
    job: job ?? null,
    error: mutation.error ?? null,
  }
}
