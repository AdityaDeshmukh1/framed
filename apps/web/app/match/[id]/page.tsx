/**
 * Match detail page — the chemistry report between two users.
 * Shows alignment score, why-text blurb, shared films, a productive disagreement,
 * and the computed first-date film recommendation.
 *
 * params.id is the match ID used to look up mock data (real: DB query).
 */

import Link from 'next/link'
import { notFound } from 'next/navigation'
import { PhoneShell } from '../../../components/layout/PhoneShell'
import { AppBar } from '../../../components/layout/AppBar'
import { FilmPoster } from '../../../components/features/FilmPoster'
import { RuleDouble, RuleThin, DiamondDivider } from '../../../components/ui/Dividers'
import { Button } from '../../../components/ui/Button'
import { Badge } from '../../../components/ui/Badge'
import { MOCK_MATCHES, POSTER_COLOR_SEQUENCE } from '../../../lib/constants'
import type { PosterVariant } from '../../../lib/types'

interface Props {
  params: Promise<{ id: string }>
}

export default async function MatchDetailPage({ params }: Props) {
  const { id } = await params

  const match = MOCK_MATCHES.find((m) => m.id === id)
  if (!match) notFound()

  const { other_user, score, why_text, shared_films, disagreement_film, first_date_film } = match

  return (
    <PhoneShell>
      <AppBar showBack title={`${other_user.display_name}`} backHref="/discover" />

      <div className="flex flex-col px-5 pt-5 pb-10 gap-6">

        {/* ── Chapter header ─── */}
        <div className="flex flex-col gap-2">
          <RuleDouble />
          <div className="flex items-start justify-between">
            <h1
              className="font-serif italic text-[28px] leading-tight"
              style={{ color: 'var(--ink-deep)' }}
            >
              You × {other_user.display_name}
            </h1>
            <Badge variant="score">{score}%</Badge>
          </div>
          <p
            className="font-sans text-xs opacity-50"
            style={{ color: 'var(--ink)' }}
          >
            alignment score
          </p>
          <RuleDouble />
        </div>

        {/* ── Why you'd click ─── */}
        {why_text && (
          <div className="flex flex-col gap-2">
            <p className="chapter-kicker">Why you'd click</p>
            <p
              className="font-serif italic text-base leading-relaxed"
              style={{ color: 'var(--ink-deep)' }}
            >
              {why_text}
            </p>
          </div>
        )}

        <RuleThin />

        {/* ── Films you both adore ─── */}
        {shared_films && shared_films.length > 0 && (
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

        <RuleThin />

        {/* ── Productive disagreement ─── */}
        {disagreement_film && (
          <div className="flex flex-col gap-3">
            <p className="chapter-kicker">The productive disagreement</p>
            <div className="flex items-center gap-4">
              <FilmPoster
                title={disagreement_film.title}
                year={disagreement_film.year}
                color="coral-deep"
                small
              />
              <div className="flex flex-col gap-2 flex-1">
                <p
                  className="font-serif italic text-base leading-snug"
                  style={{ color: 'var(--ink-deep)' }}
                >
                  {disagreement_film.title}
                </p>
                <div className="flex items-center gap-3">
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-xs opacity-40" style={{ color: 'var(--ink)' }}>
                      You
                    </span>
                    <span
                      className="font-mono text-sm font-semibold"
                      style={{ color: 'var(--coral)' }}
                    >
                      {disagreement_film.your_rating} ★
                    </span>
                  </div>
                  <div
                    className="w-px h-8 opacity-20"
                    style={{ background: 'var(--ink)' }}
                  />
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-xs opacity-40" style={{ color: 'var(--ink)' }}>
                      {other_user.display_name}
                    </span>
                    <span
                      className="font-mono text-sm font-semibold"
                      style={{ color: 'var(--teal)' }}
                    >
                      {disagreement_film.their_rating} ★
                    </span>
                  </div>
                </div>
                <p
                  className="font-sans text-xs leading-relaxed opacity-55"
                  style={{ color: 'var(--ink)' }}
                >
                  Good disagreements make better conversations.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* ── First date film ─── */}
        {first_date_film && (
          <div
            className="rounded-2xl p-5 flex flex-col items-center gap-2 text-center"
            style={{ background: 'var(--amber)' }}
          >
            <p
              className="font-sans text-[10px] font-semibold uppercase tracking-[0.2em]"
              style={{ color: 'var(--amber-deep)', opacity: 0.7 }}
            >
              First date film
            </p>
            <p
              className="font-serif italic text-2xl leading-snug"
              style={{ color: 'var(--coral-deep)' }}
            >
              {first_date_film.title}
            </p>
            {first_date_film.year && (
              <p
                className="font-mono text-sm"
                style={{ color: 'var(--coral-deep)', opacity: 0.55 }}
              >
                {first_date_film.year}
              </p>
            )}
            <p
              className="font-sans text-xs leading-relaxed mt-1"
              style={{ color: 'var(--coral-deep)', opacity: 0.65 }}
            >
              Neither of you has seen it. Chosen from where your tastes meet.
            </p>
          </div>
        )}

        <DiamondDivider />

        {/* ── Actions ─── */}
        <div className="flex flex-col gap-3">
          <Link href={`/chat/${id}`}>
            <Button variant="primary" fullWidth>
              Say hello
            </Button>
          </Link>
          <Link href="/discover">
            <Button variant="ghost" fullWidth>
              Pass
            </Button>
          </Link>
        </div>
      </div>
    </PhoneShell>
  )
}
