/** Backend API base URL */
export const API_BASE_URL = 'http://localhost:8081'

/** Poster color palette — cycle through for grids */
export const POSTER_COLORS = {
  coral: { bg: 'var(--coral)', text: 'white' },
  teal: { bg: 'var(--teal)', text: 'white' },
  amber: { bg: 'var(--amber)', text: 'var(--ink-deep)' },
  blue: { bg: 'var(--blue)', text: 'white' },
  'coral-deep': { bg: 'var(--coral-deep)', text: 'var(--cream)' },
} as const

/** Ordered poster color sequence for cycling */
export const POSTER_COLOR_SEQUENCE = [
  'coral',
  'teal',
  'amber',
  'blue',
  'coral-deep',
] as const

/** Pipeline stages shown during portrait loading */
export const PIPELINE_STAGES = [
  {
    key: 'scraping',
    label: 'Scanning Letterboxd',
    romanNumeral: 'I',
    completedStatuses: ['enriching', 'embedding', 'complete'],
  },
  {
    key: 'enriching',
    label: 'Enriching metadata',
    romanNumeral: 'II',
    completedStatuses: ['embedding', 'complete'],
  },
  {
    key: 'embedding',
    label: 'Computing embeddings',
    romanNumeral: 'III',
    completedStatuses: ['complete'],
  },
  {
    key: 'writing',
    label: 'Writing your soul',
    romanNumeral: 'IV',
    completedStatuses: [] as string[],
  },
] as const

