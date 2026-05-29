/**
 * Landing page — dark coral-deep background with film strip, value props, and CTA.
 * Server component — no interactivity needed at this level.
 */

import Link from 'next/link'
import { StatusBar } from '../components/layout/StatusBar'
import { RuleDouble, DiamondDivider } from '../components/ui/Dividers'

const FILM_STRIP = [
  { color: 'var(--coral)', title: 'In the Mood for Love', year: '2000' },
  { color: 'var(--teal)', title: 'Stalker', year: '1979' },
  { color: 'var(--amber)', title: 'Jeanne Dielman', year: '1975' },
  { color: 'var(--blue)', title: 'Chungking Express', year: '1994' },
  { color: 'var(--coral-darker)', title: 'Persona', year: '1966' },
]

const VALUE_PROPS = [
  {
    icon: '🎬',
    title: 'Letterboxd import',
    body: 'We read your public watchlist and ratings to build your cinematic fingerprint.',
  },
  {
    icon: '✦',
    title: 'Soul portrait',
    body: 'An AI-written portrait of who you are, written in the language of the films you love.',
  },
  {
    icon: '◈',
    title: 'Film-taste matching',
    body: 'Matched on how you watch — not just what you watch. Shared obsessions. Productive disagreements.',
  },
]

export default function HomePage() {
  return (
    <div
      className="min-h-screen flex justify-center"
      style={{ background: 'var(--coral-deep)' }}
    >
      <div
        className="w-full max-w-[390px] min-h-screen flex flex-col"
        style={{ background: 'var(--coral-deep)' }}
      >
        {/* Status bar */}
        <StatusBar theme="dark" />

        {/* Double rule */}
        <div className="px-5 pt-2">
          <RuleDouble />
        </div>

        {/* Hero */}
        <div className="px-6 pt-8 pb-6 flex flex-col gap-2">
          <p
            className="font-sans text-xs font-semibold uppercase tracking-[0.2em]"
            style={{ color: 'var(--coral)', opacity: 0.85 }}
          >
            A film about you
          </p>
          <h1
            className="font-serif italic text-[52px] leading-none tracking-tight"
            style={{ color: 'var(--cream)' }}
          >
            Framed
          </h1>
          <p
            className="font-sans text-sm leading-relaxed mt-1"
            style={{ color: 'var(--cream)', opacity: 0.65 }}
          >
            Your cinema. Your character. Your match.
          </p>
        </div>

        {/* Film strip */}
        <div className="px-6 pb-8">
          <div className="flex gap-2">
            {FILM_STRIP.map((film) => (
              <div
                key={film.title}
                className="flex-1 rounded-xl flex flex-col justify-between p-2"
                style={{
                  aspectRatio: '2/3',
                  background: film.color,
                }}
                aria-hidden="true"
              >
                <span
                  className="font-mono text-[9px] font-semibold tabular-nums"
                  style={{ color: 'rgba(255,255,255,0.65)' }}
                >
                  {film.year}
                </span>
                <span
                  className="font-serif italic text-[10px] leading-tight"
                  style={{ color: 'rgba(255,255,255,0.9)' }}
                >
                  {film.title}
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* Double rule */}
        <div className="px-5 pb-6">
          <RuleDouble />
        </div>

        {/* Value props */}
        <div className="px-6 pb-8 flex flex-col gap-5">
          {VALUE_PROPS.map((prop) => (
            <div key={prop.title} className="flex items-start gap-4">
              <span className="text-2xl flex-shrink-0 mt-0.5">{prop.icon}</span>
              <div className="flex flex-col gap-1">
                <p
                  className="font-sans text-sm font-semibold"
                  style={{ color: 'var(--cream)' }}
                >
                  {prop.title}
                </p>
                <p
                  className="font-sans text-xs leading-relaxed"
                  style={{ color: 'var(--cream)', opacity: 0.55 }}
                >
                  {prop.body}
                </p>
              </div>
            </div>
          ))}
        </div>

        {/* Diamond divider */}
        <div className="px-6 pb-8">
          <DiamondDivider />
        </div>

        {/* CTAs */}
        <div className="px-6 pb-12 flex flex-col gap-4 mt-auto">
          <Link
            href="/onboarding"
            className="block w-full py-4 px-6 rounded-full text-center font-sans font-semibold text-base tracking-wide transition-opacity hover:opacity-90 active:scale-[0.97]"
            style={{ background: 'var(--coral)', color: 'white' }}
          >
            Begin your screening
          </Link>
          <Link
            href="/discover"
            className="block text-center font-sans text-sm font-medium py-2 transition-opacity hover:opacity-70"
            style={{ color: 'var(--cream)', opacity: 0.6 }}
          >
            I already have an account
          </Link>
        </div>
      </div>
    </div>
  )
}
