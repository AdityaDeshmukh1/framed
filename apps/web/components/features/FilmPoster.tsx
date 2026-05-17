/**
 * FilmPoster — a coloured block representing a film.
 * Aspect ratio 2:3 (portrait), shows title at bottom and year at top.
 * Server component — purely presentational.
 */

import type { PosterVariant } from '../../lib/types'
import { POSTER_COLORS } from '../../lib/constants'

/** Props for the FilmPoster component */
interface Props {
  /** Film title */
  title: string
  /** Release year */
  year?: number
  /** Colour variant from the design palette */
  color: PosterVariant
  /** Additional CSS class names */
  className?: string
  /** Optional size override for very small contexts */
  small?: boolean
}

export function FilmPoster({ title, year, color, className = '', small = false }: Props) {
  const { bg, text } = POSTER_COLORS[color]

  return (
    <div
      className={`relative rounded-xl overflow-hidden flex flex-col justify-between ${small ? 'p-2' : 'p-3'} ${className}`}
      style={{
        aspectRatio: '2/3',
        background: bg,
        color: text,
      }}
    >
      {/* Year top-left */}
      {year && (
        <span
          className={`font-mono font-semibold ${small ? 'text-[9px]' : 'text-xs'} opacity-70 tabular-nums`}
        >
          {year}
        </span>
      )}

      {/* Decorative film strip holes — purely visual */}
      <div className="absolute top-0 right-0 flex flex-col gap-1.5 py-2 pr-1" aria-hidden="true">
        {[...Array(4)].map((_, i) => (
          <div
            key={i}
            className="w-1.5 h-1.5 rounded-full"
            style={{ background: text, opacity: 0.15 }}
          />
        ))}
      </div>

      {/* Title bottom-left */}
      <span
        className={`font-serif italic leading-tight ${small ? 'text-[10px]' : 'text-sm'} font-semibold`}
        style={{ color: text }}
      >
        {title}
      </span>
    </div>
  )
}
