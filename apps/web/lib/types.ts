/**
 * Shared TypeScript types for Framed — mirrors Go backend response shapes.
 */

export interface ScrapeJob {
  id: string
  status: 'queued' | 'scraping' | 'enriching' | 'embedding' | 'complete' | 'failed'
  films_found: number
  films_processed: number
  started_at: string | null
  completed_at: string | null
}

export interface FilmSummary {
  id: string
  title: string
  year: number
}

/** A film with both users' ratings — used in match detail shared/disagreement lists */
export interface RatedFilm extends FilmSummary {
  your_rating: number
  their_rating: number
}

/** The other user as returned by GET /matches and GET /match */
export interface OtherUser {
  id: string
  /** what to call them: their chosen name, else their Letterboxd handle */
  display_name: string
  letterboxd_handle: string
  age: number
  location_city: string
  location_country: string
}

/** One item from GET /matches/:user_id */
export interface MatchListItem {
  id: string
  score: number
  why_text: string
  status: string
  other_user: OtherUser
  first_date_film: FilmSummary | null
}

/** Full match detail from GET /match/:matchId */
export interface MatchDetail extends MatchListItem {
  shared_films: RatedFilm[]
  disagreement_films: RatedFilm[]
}

/** Response from GET /soul/:user_id */
export interface SoulPortraitResponse {
  portrait: string
  summary_line: string
  computed_at: string
}

export type PosterVariant = 'coral' | 'teal' | 'amber' | 'blue' | 'coral-deep'

/** A chat message */
export interface ChatMessage {
  id: string
  match_id: string
  sender_id: string
  content: string
  sent_at: string
}
