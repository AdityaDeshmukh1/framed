/**
 * Typed fetch wrappers for all Framed API routes.
 * Client-side calls go through /api/* proxy routes (avoids CORS + adds auth).
 * The proxy routes forward Authorization: Bearer tokens to the Go backend.
 *
 * Log format: [framed:<scope>] → METHOD /path  |  ✓ ... → 200  |  ✗ ... → 4xx
 */

import type { ScrapeJob, MatchListItem, MatchDetail, SoulPortraitResponse } from './types'

function log(scope: string, msg: string, data?: unknown) {
  if (data !== undefined) console.log(`[framed:${scope}]`, msg, data)
  else console.log(`[framed:${scope}]`, msg)
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const method = options?.method ?? 'GET'
  log('api', `→ ${method} ${path}`)

  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })

  if (!res.ok) {
    const text = await res.text().catch(() => 'unknown error')
    log('api', `✗ ${method} ${path} → ${res.status}`, text)
    throw new Error(`API ${res.status}: ${text}`)
  }

  log('api', `✓ ${method} ${path} → ${res.status}`)
  return res.json() as Promise<T>
}

// ── Onboarding ───────────────────────────────────────────────────────────────

export async function startOnboarding(letterboxdHandle: string) {
  log('onboard', `starting pipeline  handle=@${letterboxdHandle}`)
  const result = await apiFetch<{ job_id: string; status: string }>('/api/onboard', {
    method: 'POST',
    body: JSON.stringify({ letterboxd_handle: letterboxdHandle }),
  })
  log('onboard', `job queued`, result)
  return result
}

/** One quiz answer, on Letterboxd's 0.5–5 scale. */
export interface QuizRating {
  tmdb_id: number
  rating?: number
  liked?: boolean
}

/** Start the pipeline from quiz answers instead of a Letterboxd handle. */
export async function startQuizOnboarding(ratings: QuizRating[]) {
  log('onboard', `starting pipeline  source=quiz ratings=${ratings.length}`)
  return apiFetch<{ job_id: string; status: string }>('/api/onboard', {
    method: 'POST',
    body: JSON.stringify({ source: 'quiz', ratings }),
  })
}

export async function saveBasics(basics: { name: string; age: number; seeking: string[]; city: string }) {
  const res = await fetch('/api/me/basics', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(basics),
  })
  if (!res.ok) throw new Error(`API ${res.status}: ${await res.text().catch(() => '')}`)
}

export async function fetchJobStatus(jobId: string) {
  return apiFetch<ScrapeJob>(`/api/status/${jobId}`)
}

// ── Soul portrait ────────────────────────────────────────────────────────────

/** Generate a new soul portrait (calls Claude — takes ~5s). */
export async function generateSoul() {
  log('soul', `generating portrait`)
  const result = await apiFetch<{ portrait: string }>('/api/soul', {
    method: 'POST',
  })
  log('soul', `portrait generated  length=${result.portrait.length}`)
  return result
}

/** Fetch an existing soul portrait (404 if not yet generated). */
export async function fetchSoul() {
  log('soul', `fetching portrait`)
  return apiFetch<SoulPortraitResponse>('/api/soul')
}

// ── Matching ─────────────────────────────────────────────────────────────────

/**
 * Compute and persist matches (runs the similarity pipeline).
 * Fire-and-forget — call after soul generation; response is not needed by UI.
 */
export async function computeMatches() {
  log('match', `computing matches`)
  const result = await apiFetch<{ count: number }>('/api/match', {
    method: 'POST',
  })
  log('match', `computed  count=${result.count}`)
  return result
}

/**
 * Fetch stored match list for the discover feed.
 * Score comes back as 0–1 float; converted to 0–100 integer here.
 */
export async function fetchMatchList(): Promise<MatchListItem[]> {
  log('match', `fetching list`)
  const result = await apiFetch<{ matches: MatchListItem[]; count: number }>('/api/matches')
  log('match', `fetched  count=${result.count}`)
  return result.matches.map((m) => ({ ...m, score: Math.round(m.score * 100) }))
}

/**
 * Fetch full match detail including shared films and disagreements.
 */
export async function fetchMatch(matchId: string): Promise<MatchDetail> {
  log('match', `fetching detail  match=${matchId}`)
  const result = await apiFetch<MatchDetail>(`/api/match/${matchId}`)
  return { ...result, score: Math.round(result.score * 100) }
}
