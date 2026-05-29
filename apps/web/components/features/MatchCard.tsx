/**
 * MatchCard — a card in the discover feed showing a potential match.
 * Server component — link navigation handled by Next.js Link.
 */

import Link from 'next/link'
import { Badge } from '../ui/Badge'
import { FilmPoster } from './FilmPoster'
import { RuleThin } from '../ui/Dividers'
import type { PosterVariant } from '../../lib/types'
import { POSTER_COLOR_SEQUENCE } from '../../lib/constants'

interface SharedFilm {
  id: string
  title: string
  year?: number
}

interface OtherUser {
  display_name: string
  age?: number
  location?: string
  letterboxd_handle: string
  films_watched?: number
}

/** Props for the MatchCard component */
interface Props {
  /** Match identifier — used to build the link href */
  id: string
  /** Compatibility score 0–100 */
  score: number
  /** Brief prose explaining why you match */
  why_text?: string
  /** Up to 3 shared films */
  shared_films?: SharedFilm[]
  /** The other user's public info */
  other_user: OtherUser
}

export function MatchCard({ id, score, why_text, shared_films = [], other_user }: Props) {
  return (
    <Link
      href={`/match/${id}`}
      className="block rounded-2xl overflow-hidden"
      style={{ background: 'var(--cream-2)' }}
    >
      {/* Header row */}
      <div className="flex items-start justify-between p-4 pb-3">
        <div className="flex flex-col gap-0.5">
          <span
            className="font-serif italic text-xl"
            style={{ color: 'var(--ink-deep)' }}
          >
            {other_user.display_name}
          </span>
          {other_user.age && other_user.location && (
            <span
              className="font-sans text-xs opacity-60"
              style={{ color: 'var(--ink)' }}
            >
              {other_user.age} · {other_user.location}
            </span>
          )}
          <span
            className="font-mono text-[11px] opacity-45 mt-0.5"
            style={{ color: 'var(--ink)' }}
          >
            @{other_user.letterboxd_handle}
          </span>
        </div>

        {/* Score badge */}
        <Badge variant="score">{score}%</Badge>
      </div>

      {/* Shared films strip */}
      {shared_films.length > 0 && (
        <div className="px-4 pb-3">
          <div className="flex gap-2">
            {shared_films.slice(0, 3).map((film, i) => (
              <FilmPoster
                key={film.id}
                title={film.title}
                year={film.year}
                color={POSTER_COLOR_SEQUENCE[i % POSTER_COLOR_SEQUENCE.length] as PosterVariant}
                small
                className="flex-1"
              />
            ))}
          </div>
        </div>
      )}

      <RuleThin className="mx-4" />

      {/* Why text */}
      {why_text && (
        <p
          className="px-4 py-3 font-serif italic text-sm leading-relaxed opacity-80"
          style={{ color: 'var(--ink)' }}
        >
          {why_text}
        </p>
      )}

      {/* Footer */}
      <div
        className="px-4 py-3 flex items-center justify-between"
        style={{ borderTop: '1px solid rgba(74,27,12,0.08)' }}
      >
        <span
          className="font-sans text-xs font-semibold uppercase tracking-widest"
          style={{ color: 'var(--coral)', opacity: 0.8 }}
        >
          View chemistry →
        </span>
        {other_user.films_watched && (
          <span
            className="font-mono text-[10px] opacity-40"
            style={{ color: 'var(--ink)' }}
          >
            {other_user.films_watched} films
          </span>
        )}
      </div>
    </Link>
  )
}
