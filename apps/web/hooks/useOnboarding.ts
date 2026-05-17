'use client'

/**
 * useOnboarding — mutation to start the onboarding pipeline and poll job status.
 * Returns the onboard mutate function, the current job state, and a polling status.
 */

import { useMutation, useQuery } from '@tanstack/react-query'
import { startOnboarding, fetchJobStatus } from '../lib/api'
import type { ScrapeJob } from '../lib/types'
import { useState } from 'react'

interface OnboardingState {
  /** Call this to kick off the pipeline */
  onboard: (userId: string, handle: string) => void
  /** Whether the initial onboard POST is in flight */
  isLoading: boolean
  /** The job ID returned by the POST, if available */
  jobId: string | null
  /** The live job status (polled every 2s after jobId is set) */
  job: ScrapeJob | null
  /** Any error that occurred during onboard or polling */
  error: Error | null
}

export function useOnboarding(): OnboardingState {
  const [jobId, setJobId] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: ({ userId, handle }: { userId: string; handle: string }) =>
      startOnboarding(userId, handle),
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
    onboard: (userId, handle) => mutation.mutate({ userId, handle }),
    isLoading: mutation.isPending,
    jobId,
    job: job ?? null,
    error: mutation.error ?? null,
  }
}
