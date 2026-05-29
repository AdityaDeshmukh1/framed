/**
 * PosterGrid — 2×2 grid of FilmPoster components.
 * Cycles through the design palette automatically.
 * Server component.
 */

import { FilmPoster } from './FilmPoster'
import type { PosterVariant } from '../../lib/types'
import { POSTER_COLOR_SEQUENCE } from '../../lib/constants'

interface Film {
  id: string
  title: string
  year?: number
}

/** Props for the PosterGrid component */
interface Props {
  /** Up to 4 films to display */
  films: Film[]
  /** Additional CSS class names */
  className?: string
}

export function PosterGrid({ films, className = '' }: Props) {
  const displayFilms = films.slice(0, 4)

  return (
    <div className={`grid grid-cols-2 gap-2 ${className}`}>
      {displayFilms.map((film, i) => (
        <FilmPoster
          key={film.id}
          title={film.title}
          year={film.year}
          color={POSTER_COLOR_SEQUENCE[i % POSTER_COLOR_SEQUENCE.length] as PosterVariant}
        />
      ))}
    </div>
  )
}
