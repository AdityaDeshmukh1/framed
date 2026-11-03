/**
 * Rating-quiz engine — pure functions, no React.
 *
 * Picking the next film: we want films the user has probably seen (otherwise
 * the card is wasted) from parts of taste space we know least about. So each
 * candidate scores
 *
 *   P(seen) × information
 *
 * where P(seen) is the film's popularity adjusted by how often this user has
 * said "haven't seen" to films with the same facets, and information is
 * higher for facets with few ratings so far. The real version will do the
 * same thing over embeddings instead of hand-written facets.
 */

import { ALL_FACETS, QUIZ_FILMS, type Facet, type QuizFilm } from './films'

export type Answer = 'loved' | 'fine' | 'nope' | 'unseen'

export interface Response {
  filmId: string
  answer: Answer
}

export interface QuizState {
  favourites: string[]
  responses: Response[]
}

/** Stop asking once this many seen films are rated. */
export const TARGET_RATED = 20
/** Allow finishing early from this many. */
export const MIN_RATED = 8
/** Hard cap on cards shown, including "haven't seen". */
export const MAX_SHOWN = 45

/** Ratings sent to the backend, on Letterboxd's 0.5–5 scale. */
const ANSWER_RATING: Record<Exclude<Answer, 'unseen'>, number> = {
  loved: 4.5,
  fine: 3,
  nope: 1.5,
}

export const filmById = new Map(QUIZ_FILMS.map((f) => [f.id, f]))

function facetStats(state: QuizState) {
  const rated: Record<string, number> = {}
  const shown: Record<string, number> = {}
  const seen: Record<string, number> = {}
  const bump = (m: Record<string, number>, f: Facet, by = 1) => (m[f] = (m[f] ?? 0) + by)

  for (const id of state.favourites) {
    for (const f of filmById.get(id)?.facets ?? []) bump(rated, f)
  }
  for (const r of state.responses) {
    for (const f of filmById.get(r.filmId)?.facets ?? []) {
      bump(shown, f)
      if (r.answer !== 'unseen') {
        bump(seen, f)
        bump(rated, f)
      }
    }
  }
  return { rated, shown, seen }
}

export function ratedCount(state: QuizState) {
  return state.responses.filter((r) => r.answer !== 'unseen').length
}

export function isDone(state: QuizState) {
  return ratedCount(state) >= TARGET_RATED || state.responses.length >= MAX_SHOWN || nextFilm(state) === null
}

export function nextFilm(state: QuizState): QuizFilm | null {
  const used = new Set([...state.favourites, ...state.responses.map((r) => r.filmId)])
  const { rated, shown, seen } = facetStats(state)

  let best: QuizFilm | null = null
  let bestScore = -Infinity
  for (const film of QUIZ_FILMS) {
    if (used.has(film.id)) continue
    // Per facet, the share of films the user has seen (weak prior of 2/3).
    // Take the lowest: one "never watches arthouse" facet sinks the film.
    const seenRate = Math.min(...film.facets.map((f) => ((seen[f] ?? 0) + 1) / ((shown[f] ?? 0) + 1.5)))
    const pSeen = film.popularity * seenRate
    const info = film.facets.reduce((s, f) => s + 1 / (1 + (rated[f] ?? 0)), 0) / film.facets.length
    // Deterministic per-film jitter so ties don't always resolve the same way.
    const jitter = 1 + 0.08 * ((hash(film.id) % 100) / 100)
    const score = Math.pow(pSeen, 1.5) * info * jitter
    if (score > bestScore) {
      bestScore = score
      best = film
    }
  }
  return best
}

export interface FacetAffinity {
  facet: Facet
  /** −1 (avoids) … +1 (loves), relative to the user's own average */
  affinity: number
  /** how many rated films back this up */
  evidence: number
}

export interface TasteProfile {
  strength: number
  strengthLabel: 'Sketch' | 'Outline' | 'Portrait'
  facets: FacetAffinity[]
  rated: number
  unseen: number
}

export function buildProfile(state: QuizState): TasteProfile {
  const points: { film: QuizFilm; value: number; weight: number }[] = []
  for (const id of state.favourites) {
    const film = filmById.get(id)
    if (film) points.push({ film, value: 5, weight: 2 })
  }
  for (const r of state.responses) {
    const film = filmById.get(r.filmId)
    if (film && r.answer !== 'unseen') points.push({ film, value: ANSWER_RATING[r.answer], weight: 1 })
  }

  const totalW = points.reduce((s, p) => s + p.weight, 0)
  const mean = totalW ? points.reduce((s, p) => s + p.value * p.weight, 0) / totalW : 3

  const sum: Partial<Record<Facet, number>> = {}
  const w: Partial<Record<Facet, number>> = {}
  const n: Partial<Record<Facet, number>> = {}
  for (const p of points) {
    for (const f of p.film.facets) {
      sum[f] = (sum[f] ?? 0) + (p.value - mean) * p.weight
      w[f] = (w[f] ?? 0) + p.weight
      n[f] = (n[f] ?? 0) + 1
    }
  }

  const facets = ALL_FACETS.filter((f) => n[f])
    .map((f) => ({
      facet: f,
      // Shrink towards 0 with little evidence; ±2 stars from the mean is "max".
      affinity: Math.max(-1, Math.min(1, (sum[f]! / (w[f]! + 1)) / 2)),
      evidence: n[f]!,
    }))
    .sort((a, b) => b.affinity - a.affinity)

  const rated = ratedCount(state)
  const signal = state.favourites.length * 3 + rated
  const coverage = facets.length / ALL_FACETS.length
  const strength = Math.min(1, signal / 32) * (0.6 + 0.4 * coverage)

  return {
    strength,
    strengthLabel: strength < 0.35 ? 'Sketch' : strength < 0.7 ? 'Outline' : 'Portrait',
    facets,
    rated,
    unseen: state.responses.length - rated,
  }
}

/**
 * What the quiz would POST to the backend. Same shape as importer.Film, so a
 * `quiz` Importer can feed the existing pipeline unchanged.
 */
export function toImportPayload(state: QuizState) {
  const film = (id: string) => filmById.get(id)!
  return {
    source: 'quiz',
    films: [
      ...state.favourites.map((id) => ({
        title: film(id).title,
        year: film(id).year,
        rating: 5,
        liked: true,
        favourite: true,
      })),
      ...state.responses
        .filter((r) => r.answer !== 'unseen')
        .map((r) => ({
          title: film(r.filmId).title,
          year: film(r.filmId).year,
          rating: ANSWER_RATING[r.answer as Exclude<Answer, 'unseen'>],
          liked: r.answer === 'loved',
        })),
    ],
  }
}

function hash(s: string) {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return h
}
