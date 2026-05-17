/**
 * Shared TypeScript types for Framed — mirrors Go backend models.
 * These types are used across API routes, hooks, and components.
 */

/** Represents a scrape/enrich job returned by the onboard endpoint */
export interface ScrapeJob {
  id: string
  status: 'queued' | 'scraping' | 'enriching' | 'embedding' | 'complete' | 'failed'
  films_found: number
  films_processed: number
  started_at: string | null
  completed_at: string | null
}

/** A match between two users with a compatibility score */
export interface Match {
  id: string
  userA: string
  userB: string
  score: number
  firstDateFilmId: string
  why_text?: string
}

/** A film in the system */
export interface Film {
  id: string
  title: string
  year: number
  tmdb_id?: number
  poster_url?: string
  director?: string
  genre?: string[]
  keywords?: string[]
}

/** A user rating of a film */
export interface UserRating {
  film_id: string
  user_id: string
  rating: number | null
  watched_date: string | null
  liked: boolean
}

/** A user profile */
export interface UserProfile {
  id: string
  letterboxd_handle: string
  display_name?: string
  age?: number
  location?: string
  bio?: string
  intentions?: string[]
  films_watched?: number
  obscurity_score?: number
  avg_rating?: number
  favourite_era?: string
  now_showing?: string
  prompts?: FilmPrompt[]
}

/** A film-anchored prompt on a profile */
export interface FilmPrompt {
  film_title: string
  prompt: string
  answer: string
}

/** The generated soul portrait for a user */
export interface SoulPortrait {
  user_id: string
  portrait: string
  director_note?: string
  opening_titles?: Film[]
}

/** A chat message */
export interface ChatMessage {
  id: string
  match_id: string
  sender_id: string
  content: string
  sent_at: string
}

/** Film poster color variants — cycle through these for poster grids */
export type PosterVariant = 'coral' | 'teal' | 'amber' | 'blue' | 'coral-deep'

/** Pipeline stage for the portrait loading screen */
export interface PipelineStage {
  key: string
  label: string
  romanNumeral: string
  completedStatus: ScrapeJob['status'][]
}
