/**
 * Typed fetch wrappers for all Framed API routes.
 * All functions call internal /api/* routes which proxy to the Go backend.
 */

import type { ScrapeJob, Match, SoulPortrait } from './types'

/** Base fetch wrapper with JSON serialization */
async function apiFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })

  if (!res.ok) {
    const text = await res.text().catch(() => 'Unknown error')
    throw new Error(`API error ${res.status}: ${text}`)
  }

  return res.json() as Promise<T>
}

/** Start the onboarding pipeline for a Letterboxd handle */
export async function startOnboarding(
  userId: string,
  letterboxdHandle: string
): Promise<{ job_id: string }> {
  return apiFetch('/api/onboard', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, letterboxd_handle: letterboxdHandle }),
  })
}

/** Poll the status of a scrape/enrich job */
export async function fetchJobStatus(jobId: string): Promise<ScrapeJob> {
  return apiFetch(`/api/status/${jobId}`)
}

/** Fetch match candidates for the current user */
export async function fetchMatches(userId: string): Promise<Match[]> {
  return apiFetch('/api/match', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId }),
  })
}

/** Fetch or generate the soul portrait for a user */
export async function fetchSoulPortrait(userId: string): Promise<SoulPortrait> {
  return apiFetch(`/api/soul/${userId}`)
}
