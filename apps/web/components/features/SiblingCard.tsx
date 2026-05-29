/**
 * SiblingCard — a card in the soul siblings list.
 * Shows similarity score, shared trait, and basic user info.
 * Server component.
 */

import { Badge } from '../ui/Badge'
import { RuleThin } from '../ui/Dividers'

/** Props for the SiblingCard component */
interface Props {
  /** Sibling's display name */
  display_name: string
  /** Optional age */
  age?: number
  /** Optional location */
  location?: string
  /** Letterboxd handle */
  letterboxd_handle: string
  /** Similarity score 0–100 */
  score: number
  /** One-line description of the shared trait */
  shared_trait?: string
  /** Films watched count */
  films_watched?: number
}

export function SiblingCard({
  display_name,
  age,
  location,
  letterboxd_handle,
  score,
  shared_trait,
  films_watched,
}: Props) {
  return (
    <div
      className="rounded-2xl overflow-hidden"
      style={{ background: 'var(--cream-2)' }}
    >
      {/* Header */}
      <div className="flex items-start justify-between p-4 pb-3">
        <div className="flex flex-col gap-0.5">
          <span
            className="font-serif italic text-xl"
            style={{ color: 'var(--ink-deep)' }}
          >
            {display_name}
          </span>
          {age && location && (
            <span
              className="font-sans text-xs opacity-60"
              style={{ color: 'var(--ink)' }}
            >
              {age} · {location}
            </span>
          )}
          <span
            className="font-mono text-[11px] opacity-45 mt-0.5"
            style={{ color: 'var(--ink)' }}
          >
            @{letterboxd_handle}
          </span>
        </div>
        <Badge variant="score">{score}%</Badge>
      </div>

      {shared_trait && (
        <>
          <RuleThin className="mx-4" />
          <div className="px-4 py-3 flex items-center gap-2">
            <span className="text-base" aria-hidden="true">✦</span>
            <p
              className="font-sans text-sm"
              style={{ color: 'var(--ink)', opacity: 0.75 }}
            >
              {shared_trait}
            </p>
          </div>
        </>
      )}

      {/* Footer */}
      <div
        className="px-4 py-2.5 flex items-center justify-between"
        style={{ borderTop: '1px solid rgba(74,27,12,0.08)' }}
      >
        <span
          className="font-sans text-xs font-semibold uppercase tracking-widest"
          style={{ color: 'var(--coral)', opacity: 0.8 }}
        >
          No romance attached
        </span>
        {films_watched && (
          <span
            className="font-mono text-[10px] opacity-40"
            style={{ color: 'var(--ink)' }}
          >
            {films_watched} films
          </span>
        )}
      </div>
    </div>
  )
}
