'use client'

/**
 * QuizCard — a draggable poster card for the rating quiz.
 * Drag right = loved, left = not for me, up = it's fine, double-tap = all-time
 * favourite. The parent handles buttons and keys.
 */

import { useRef, useState } from 'react'
import { AnimatePresence, motion, useMotionValue, useTransform, type MotionValue } from 'framer-motion'
import { Star } from 'lucide-react'
import { POSTER_SIZES, posterSrcSet, posterUrl, type PoolFilm } from '../../lib/quiz/engine'

const SWIPE = 110

interface Props {
  film: PoolFilm
  reason: string
  onSwipe: (dir: 'left' | 'right' | 'up') => void
  onFavourite: () => void
}

const DOUBLE_TAP_MS = 300

export function QuizCard({ film, reason, onSwipe, onFavourite }: Props) {
  const lastTap = useRef(0)
  const [burst, setBurst] = useState(false)
  const x = useMotionValue(0)
  const y = useMotionValue(0)
  const rotate = useTransform(x, [-200, 200], [-12, 12])
  const loved = useTransform(x, [20, SWIPE], [0, 1])
  const nope = useTransform(x, [-SWIPE, -20], [1, 0])
  const fine = useTransform(y, [-SWIPE, -20], [1, 0])

  return (
    <motion.div
      className="absolute inset-0 cursor-grab active:cursor-grabbing touch-none select-none"
      style={{ x, y, rotate }}
      drag
      dragSnapToOrigin
      dragElastic={0.9}
      onTap={() => {
        const now = Date.now()
        if (now - lastTap.current < DOUBLE_TAP_MS) {
          lastTap.current = 0
          setBurst(true)
          setTimeout(onFavourite, 280)
        } else {
          lastTap.current = now
        }
      }}
      onDragEnd={(_, info) => {
        const { x: dx, y: dy } = info.offset
        if (dy < -SWIPE && Math.abs(dy) > Math.abs(dx)) onSwipe('up')
        else if (dx > SWIPE) onSwipe('right')
        else if (dx < -SWIPE) onSwipe('left')
      }}
    >
      <div className="relative h-full rounded-3xl overflow-hidden shadow-xl" style={{ background: 'var(--ink-deep)' }}>
        {/* eslint-disable-next-line @next/next/no-img-element -- mock: TMDB CDN directly */}
        <img
          src={posterUrl(film.posterPath)}
          srcSet={posterSrcSet(film.posterPath)}
          sizes={POSTER_SIZES}
          alt={film.title}
          draggable={false}
          className="absolute inset-0 w-full h-full object-cover pointer-events-none"
        />
        <div
          className="absolute inset-0 pointer-events-none"
          style={{ background: 'linear-gradient(to bottom, transparent 62%, rgba(26,24,23,0.9) 100%)' }}
        />

        {/* Why this film */}
        <div className="absolute top-4 inset-x-4">
          <p className="font-sans text-xs font-medium leading-snug px-3 py-2 rounded-xl backdrop-blur-sm" style={{ background: 'rgba(250,238,218,0.88)', color: 'var(--ink-deep)' }}>
            {reason}
          </p>
        </div>

        <div className="absolute bottom-0 inset-x-0 px-4 pb-4 flex flex-col gap-1" style={{ color: 'var(--cream)' }}>
          <h2 className="font-serif italic text-xl leading-tight font-semibold">{film.title}</h2>
          <p className="font-sans text-xs opacity-80">
            {film.year}
            {film.directors[0] ? ` · ${film.directors.slice(0, 2).join(', ')}` : ''}
          </p>
          <div className="flex flex-wrap gap-1.5 pt-1">
            {film.genres.slice(0, 3).map((g) => (
              <span key={g} className="font-sans text-[10px] uppercase tracking-widest font-semibold px-2 py-1 rounded-full" style={{ background: 'rgba(250,238,218,0.15)' }}>
                {g}
              </span>
            ))}
          </div>
        </div>

        <AnimatePresence>
          {burst && (
            <motion.div
              className="absolute inset-0 flex items-center justify-center pointer-events-none"
              initial={{ scale: 0.4, opacity: 0 }}
              animate={{ scale: 1.1, opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ type: 'spring', stiffness: 400, damping: 15 }}
            >
              <Star size={110} fill="var(--coral)" color="white" strokeWidth={1.5} />
            </motion.div>
          )}
        </AnimatePresence>

        <Stamp opacity={loved} className="left-5 -rotate-12" bg="var(--teal)" label="LOVED" />
        <Stamp opacity={nope} className="right-5 rotate-12" bg="var(--ink-deep)" label="NOPE" />
        <Stamp opacity={fine} className="left-1/2 -translate-x-1/2" bg="var(--amber)" label="FINE" color="var(--ink-deep)" />
      </div>
    </motion.div>
  )
}

function Stamp({
  opacity,
  className,
  bg,
  label,
  color = 'white',
}: {
  opacity: MotionValue<number>
  className: string
  bg: string
  label: string
  color?: string
}) {
  return (
    <motion.div
      className={`absolute top-24 border-4 rounded-lg px-3 py-1 font-sans font-bold text-2xl tracking-widest ${className}`}
      style={{ opacity, borderColor: 'white', color, background: bg }}
    >
      {label}
    </motion.div>
  )
}
