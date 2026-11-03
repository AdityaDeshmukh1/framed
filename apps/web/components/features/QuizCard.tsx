'use client'

/**
 * QuizCard — a draggable film card for the rating quiz.
 * Drag right = loved, left = not for me. The parent handles buttons/keys.
 */

import { motion, useMotionValue, useTransform } from 'framer-motion'
import type { QuizFilm } from '../../lib/quiz/films'
import { FACET_LABELS } from '../../lib/quiz/films'
import { POSTER_COLORS, POSTER_COLOR_SEQUENCE } from '../../lib/constants'
import type { PosterVariant } from '../../lib/types'

const SWIPE_THRESHOLD = 110

interface Props {
  film: QuizFilm
  onSwipe: (dir: 'left' | 'right') => void
}

export function posterColor(id: string): PosterVariant {
  let h = 0
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0
  return POSTER_COLOR_SEQUENCE[h % POSTER_COLOR_SEQUENCE.length] as PosterVariant
}

export function QuizCard({ film, onSwipe }: Props) {
  const x = useMotionValue(0)
  const rotate = useTransform(x, [-200, 200], [-12, 12])
  const lovedOpacity = useTransform(x, [20, SWIPE_THRESHOLD], [0, 1])
  const nopeOpacity = useTransform(x, [-SWIPE_THRESHOLD, -20], [1, 0])
  const { bg, text } = POSTER_COLORS[posterColor(film.id)]

  return (
    <motion.div
      className="absolute inset-0 cursor-grab active:cursor-grabbing touch-none select-none"
      style={{ x, rotate }}
      drag="x"
      dragSnapToOrigin
      dragElastic={0.9}
      onDragEnd={(_, info) => {
        if (info.offset.x > SWIPE_THRESHOLD) onSwipe('right')
        else if (info.offset.x < -SWIPE_THRESHOLD) onSwipe('left')
      }}
    >
      <div
        className="relative h-full rounded-3xl overflow-hidden flex flex-col justify-between p-6 shadow-xl"
        style={{ background: bg, color: text }}
      >
        <div className="flex items-start justify-between">
          <span className="font-mono text-sm font-semibold opacity-70 tabular-nums">{film.year}</span>
          <div className="flex flex-col gap-2" aria-hidden="true">
            {[...Array(6)].map((_, i) => (
              <div key={i} className="w-2 h-2 rounded-full" style={{ background: text, opacity: 0.15 }} />
            ))}
          </div>
        </div>

        <div className="flex flex-col gap-3">
          <h2 className="font-serif italic text-4xl leading-[1.05] font-semibold">{film.title}</h2>
          <p className="font-sans text-sm opacity-75">{film.director}</p>
          <div className="flex flex-wrap gap-1.5">
            {film.facets.map((f) => (
              <span
                key={f}
                className="font-sans text-[10px] uppercase tracking-widest font-semibold px-2 py-1 rounded-full"
                style={{ background: 'rgba(0,0,0,0.15)' }}
              >
                {FACET_LABELS[f]}
              </span>
            ))}
          </div>
        </div>

        {/* Drag feedback stamps */}
        <motion.div
          className="absolute top-8 left-6 -rotate-12 border-4 rounded-lg px-3 py-1 font-sans font-bold text-2xl tracking-widest"
          style={{ opacity: lovedOpacity, borderColor: 'white', color: 'white', background: 'var(--teal)' }}
        >
          LOVED
        </motion.div>
        <motion.div
          className="absolute top-8 right-6 rotate-12 border-4 rounded-lg px-3 py-1 font-sans font-bold text-2xl tracking-widest"
          style={{ opacity: nopeOpacity, borderColor: 'white', color: 'white', background: 'var(--ink-deep)' }}
        >
          NOPE
        </motion.div>
      </div>
    </motion.div>
  )
}
