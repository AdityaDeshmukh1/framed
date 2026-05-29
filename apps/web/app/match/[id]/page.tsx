'use client'

/**
 * Match detail page — chemistry report between two users.
 * Fetches real data from GET /api/match/[matchId]?user_id=...
 */

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { motion } from 'framer-motion'
import { PhoneShell } from '../../../components/layout/PhoneShell'
import { AppBar } from '../../../components/layout/AppBar'
import { FilmPoster } from '../../../components/features/FilmPoster'
import { RuleDouble, RuleThin, DiamondDivider } from '../../../components/ui/Dividers'
import { Button } from '../../../components/ui/Button'
import { Badge } from '../../../components/ui/Badge'
import { fetchMatch } from '../../../lib/api'
import { POSTER_COLOR_SEQUENCE } from '../../../lib/constants'
import type { MatchDetail, RatedFilm } from '../../../lib/types'
import type { PosterVariant } from '../../../lib/types'

export default function MatchDetailPage() {
  const params  = useParams()
  const router  = useRouter()
  const matchId = params.id as string

  const [match,     setMatch]     = useState<MatchDetail | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error,     setError]     = useState<string | null>(null)

  useEffect(() => {
    console.log(`[framed:match] loading detail  match=${matchId}`)
    fetchMatch(matchId)
      .then((data) => {
        console.log(`[framed:match] loaded  score=${data.score}%  shared=${data.shared_films.length}  disagreements=${data.disagreement_films.length}`)
        setMatch(data)
      })
      .catch((err) => {
        console.error('[framed:match] fetch failed:', err)
        setError('Could not load match details.')
      })
      .finally(() => setIsLoading(false))
  }, [matchId])

  if (isLoading) {
    return (
      <PhoneShell>
        <AppBar showBack backHref="/discover" />
        <div className="flex flex-col gap-4 px-5 pt-8">
          {[80, 120, 160, 80].map((h, i) => (
            <div key={i} className="rounded-2xl animate-pulse" style={{ height: h, background: 'var(--cream-2)' }} />
          ))}
        </div>
      </PhoneShell>
    )
  }

  if (error || !match) {
    return (
      <PhoneShell>
        <AppBar showBack backHref="/discover" />
        <div className="flex flex-col items-center gap-4 px-5 pt-12">
          <p className="font-serif italic text-base text-center" style={{ color: 'var(--ink)', opacity: 0.5 }}>
            {error ?? 'Match not found.'}
          </p>
          <Button variant="ghost" onClick={() => router.push('/discover')}>Back to discover</Button>
        </div>
      </PhoneShell>
    )
  }

  const { other_user, score, why_text, shared_films, disagreement_films, first_date_film } = match
  const displayName = other_user.letterboxd_handle
  const location    = [other_user.location_city, other_user.location_country].filter(Boolean).join(', ')
  const disagreement: RatedFilm | undefined = disagreement_films[0]

  return (
    <PhoneShell>
      <AppBar showBack title={`@${displayName}`} backHref="/discover" />

      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: 'easeOut' }}
        className="flex flex-col px-5 pt-5 pb-10 gap-6"
      >
        {/* ── Chapter header ── */}
        <div className="flex flex-col gap-2">
          <RuleDouble />
          <div className="flex items-start justify-between">
            <h1 className="font-serif italic text-[28px] leading-tight" style={{ color: 'var(--ink-deep)' }}>
              You × @{displayName}
            </h1>
            <Badge variant="score">{score}%</Badge>
          </div>
          {location ? (
            <p className="font-sans text-xs opacity-50" style={{ color: 'var(--ink)' }}>{location}</p>
          ) : null}
          <p className="font-sans text-xs opacity-40" style={{ color: 'var(--ink)' }}>alignment score</p>
          <RuleDouble />
        </div>

        {/* ── Why you'd click ── */}
        {why_text && (
          <div className="flex flex-col gap-2">
            <p className="chapter-kicker">Why you'd click</p>
            <p className="font-serif italic text-base leading-relaxed" style={{ color: 'var(--ink-deep)' }}>
              {why_text}
            </p>
          </div>
        )}

        {shared_films.length > 0 && <RuleThin />}

        {/* ── Films you both adore ── */}
        {shared_films.length > 0 && (
          <div className="flex flex-col gap-3">
            <p className="chapter-kicker">Films you both adore</p>
            <div className="flex gap-2">
              {shared_films.slice(0, 3).map((film, i) => (
                <FilmPoster
                  key={film.id}
                  title={film.title}
                  year={film.year}
                  color={POSTER_COLOR_SEQUENCE[i % POSTER_COLOR_SEQUENCE.length] as PosterVariant}
                  className="flex-1"
                />
              ))}
            </div>
          </div>
        )}

        {disagreement && <RuleThin />}

        {/* ── Productive disagreement ── */}
        {disagreement && (
          <div className="flex flex-col gap-3">
            <p className="chapter-kicker">The productive disagreement</p>
            <div className="flex items-center gap-4">
              <FilmPoster title={disagreement.title} year={disagreement.year} color="coral-deep" small />
              <div className="flex flex-col gap-2 flex-1">
                <p className="font-serif italic text-base leading-snug" style={{ color: 'var(--ink-deep)' }}>
                  {disagreement.title}
                </p>
                <div className="flex items-center gap-3">
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-xs opacity-40" style={{ color: 'var(--ink)' }}>You</span>
                    <span className="font-mono text-sm font-semibold" style={{ color: 'var(--coral)' }}>
                      {disagreement.your_rating} ★
                    </span>
                  </div>
                  <div className="w-px h-8 opacity-20" style={{ background: 'var(--ink)' }} />
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-xs opacity-40" style={{ color: 'var(--ink)' }}>@{displayName}</span>
                    <span className="font-mono text-sm font-semibold" style={{ color: 'var(--teal)' }}>
                      {disagreement.their_rating} ★
                    </span>
                  </div>
                </div>
                <p className="font-sans text-xs leading-relaxed opacity-55" style={{ color: 'var(--ink)' }}>
                  Good disagreements make better conversations.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* ── First date film ── */}
        {first_date_film && (
          <div className="rounded-2xl p-5 flex flex-col items-center gap-2 text-center" style={{ background: 'var(--amber)' }}>
            <p className="font-sans text-[10px] font-semibold uppercase tracking-[0.2em]" style={{ color: 'var(--amber-deep)', opacity: 0.7 }}>
              First date film
            </p>
            <p className="font-serif italic text-2xl leading-snug" style={{ color: 'var(--coral-deep)' }}>
              {first_date_film.title}
            </p>
            {first_date_film.year && (
              <p className="font-mono text-sm" style={{ color: 'var(--coral-deep)', opacity: 0.55 }}>
                {first_date_film.year}
              </p>
            )}
            <p className="font-sans text-xs leading-relaxed mt-1" style={{ color: 'var(--coral-deep)', opacity: 0.65 }}>
              Chosen where your tastes converge.
            </p>
          </div>
        )}

        <DiamondDivider />

        {/* ── Actions ── */}
        <div className="flex flex-col gap-3">
          <Link href={`/chat/${matchId}`}>
            <Button variant="primary" fullWidth>Say hello</Button>
          </Link>
          <Link href="/discover">
            <Button variant="ghost" fullWidth>Pass</Button>
          </Link>
        </div>
      </motion.div>
    </PhoneShell>
  )
}
