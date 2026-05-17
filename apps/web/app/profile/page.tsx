/**
 * Profile page — the user's own Cinematic Soul card.
 * Shows soul portrait, opening titles, stats, film-anchored prompts,
 * "now showing", and intentions. Server component using mock data.
 */

import { PhoneShell } from '../../components/layout/PhoneShell'
import { AppBar } from '../../components/layout/AppBar'
import { BottomNav } from '../../components/layout/BottomNav'
import { SoulPortrait } from '../../components/features/SoulPortrait'
import { PosterGrid } from '../../components/features/PosterGrid'
import { RuleDouble, RuleThin, DiamondDivider } from '../../components/ui/Dividers'
import { MOCK_USER, MOCK_SOUL_PORTRAIT, POSTER_COLOR_SEQUENCE } from '../../lib/constants'

/** A single stat cell in the 2×4 stats grid */
function StatCell({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex flex-col gap-0.5 py-3">
      <span
        className="font-mono text-lg font-semibold leading-none"
        style={{ color: 'var(--coral)' }}
      >
        {value}
      </span>
      <span
        className="font-sans text-[11px] uppercase tracking-wider opacity-55"
        style={{ color: 'var(--ink)' }}
      >
        {label}
      </span>
    </div>
  )
}

function EditIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3l3 3L6 15H3v-3L12 3Z" />
    </svg>
  )
}

export default function ProfilePage() {
  const user = MOCK_USER
  const portrait = MOCK_SOUL_PORTRAIT

  // Build opening titles list with colours cycling through the palette
  const openingTitles = user.prompts?.map((p, i) => ({
    title: p.film_title,
    year: undefined as number | undefined,
    color: POSTER_COLOR_SEQUENCE[i % POSTER_COLOR_SEQUENCE.length],
  })) ?? []

  // Pad to 4 entries for the grid
  while (openingTitles.length < 4) {
    const idx = openingTitles.length
    openingTitles.push({
      title: '—',
      year: undefined,
      color: POSTER_COLOR_SEQUENCE[idx % POSTER_COLOR_SEQUENCE.length],
    })
  }

  return (
    <PhoneShell hasBottomNav>
      {/* App bar: back on the left, edit on the right */}
      <AppBar
        showLogo
        rightAction={
          <button
            type="button"
            className="p-2 rounded-full hover:bg-black/5 transition-colors"
            aria-label="Edit profile"
            style={{ color: 'var(--ink)', opacity: 0.6 }}
          >
            <EditIcon />
          </button>
        }
      />

      <div className="flex flex-col px-5 pt-5 pb-6 gap-6">

        {/* ── Chapter card ─── */}
        <div className="flex flex-col gap-1">
          <h1
            className="font-serif italic text-[38px] leading-none tracking-tight"
            style={{ color: 'var(--ink-deep)' }}
          >
            {user.display_name}
          </h1>
          <p className="font-sans text-sm opacity-55" style={{ color: 'var(--ink)' }}>
            {user.age} · {user.location}
          </p>
          <p className="font-mono text-[11px] opacity-40 mt-0.5" style={{ color: 'var(--ink)' }}>
            @{user.letterboxd_handle}
          </p>
        </div>

        <RuleDouble />

        {/* ── Cinematic Soul ─── */}
        <SoulPortrait
          portrait={portrait.portrait}
          directorNote={portrait.director_note}
          showDividers={false}
        />

        <RuleDouble />

        {/* ── Opening Titles ─── */}
        <div className="flex flex-col gap-3">
          <p className="chapter-kicker">Opening Titles</p>
          <PosterGrid
            films={openingTitles.slice(0, 4).map((f, i) => ({
              id: String(i),
              title: f.title,
              color: f.color,
            }))}
          />
        </div>

        <RuleThin />

        {/* ── Stats ─── */}
        <div className="flex flex-col gap-2">
          <p className="chapter-kicker">The numbers</p>
          <div
            className="grid grid-cols-2 divide-x divide-y rounded-2xl overflow-hidden"
            style={{ borderColor: 'rgba(74,27,12,0.1)' }}
          >
            <StatCell label="Films watched" value={user.films_watched?.toLocaleString() ?? '—'} />
            <StatCell label="Obscurity score" value={`${user.obscurity_score ?? '—'}%`} />
            <StatCell label="Avg rating" value={`${user.avg_rating ?? '—'} ★`} />
            <StatCell label="Favourite era" value={user.favourite_era ?? '—'} />
          </div>
        </div>

        <RuleThin />

        {/* ── Film-anchored prompts ─── */}
        {user.prompts && user.prompts.length > 0 && (
          <div className="flex flex-col gap-5">
            <p className="chapter-kicker">In their own words</p>
            {user.prompts.map((prompt) => (
              <div key={prompt.film_title} className="flex flex-col gap-2">
                <p
                  className="font-sans text-xs font-semibold uppercase tracking-wider opacity-50"
                  style={{ color: 'var(--ink)' }}
                >
                  {prompt.prompt}
                </p>
                <p
                  className="font-serif italic text-base leading-relaxed"
                  style={{ color: 'var(--ink-deep)' }}
                >
                  "{prompt.answer}"
                </p>
                <p className="chapter-kicker" style={{ opacity: 0.5 }}>
                  — {prompt.film_title}
                </p>
              </div>
            ))}
          </div>
        )}

        <RuleThin />

        {/* ── Now showing ─── */}
        {user.now_showing && (
          <div
            className="rounded-2xl p-4 flex items-center gap-3"
            style={{ background: 'var(--coral-deep)' }}
          >
            {/* Pulsing reel dot */}
            <span
              className="w-2 h-2 rounded-full flex-shrink-0"
              style={{
                background: 'var(--coral)',
                animation: 'pulse 2s ease-in-out infinite',
              }}
              aria-hidden="true"
            />
            <div className="flex flex-col gap-0.5">
              <p
                className="font-sans text-[10px] uppercase tracking-widest opacity-50"
                style={{ color: 'var(--cream)' }}
              >
                Now showing
              </p>
              <p
                className="font-serif italic text-base"
                style={{ color: 'var(--cream)' }}
              >
                {user.now_showing}
              </p>
            </div>
          </div>
        )}

        {/* ── Intentions ─── */}
        {user.intentions && user.intentions.length > 0 && (
          <div className="flex flex-col gap-3">
            <p className="chapter-kicker">Looking for</p>
            <div className="flex flex-col gap-2">
              {user.intentions.map((intention) => (
                <div key={intention} className="flex items-center gap-3">
                  <div
                    className="w-4 h-4 rounded-sm flex-shrink-0 flex items-center justify-center"
                    style={{ background: 'var(--teal)' }}
                    aria-hidden="true"
                  >
                    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="white" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M2 5l2.5 2.5L8 3" />
                    </svg>
                  </div>
                  <span className="font-sans text-sm capitalize" style={{ color: 'var(--ink)' }}>
                    {intention}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}

        <DiamondDivider />

        {/* ── Signature footer ─── */}
        <p
          className="text-center font-serif italic text-sm"
          style={{ color: 'var(--ink)', opacity: 0.35 }}
        >
          — end of reel —
        </p>
      </div>

      <BottomNav activeTab="portrait" />
    </PhoneShell>
  )
}
