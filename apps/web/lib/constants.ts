/**
 * Application-wide constants for Framed.
 * MOCK_USER_ID replaces real auth until Clerk is wired up.
 */

/** Stub user ID — replace with Clerk user ID when auth is added */
export const MOCK_USER_ID = '00000000-0000-0000-0000-000000000001'

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

/** Mock soul portrait data for development */
export const MOCK_SOUL_PORTRAIT = {
  portrait: `You are drawn to the spaces between certainties — the long takes, the silences, the films that trust you enough not to explain themselves. Your watchlist reads like a map of emotional archaeology, each entry a site where feeling was once excavated and left exposed. You have seen enough cinema to know that the best films are not about what they are about. There is a recurring quality in what you choose: restraint, longing, the terrible beauty of things that cannot last.`,
  director_note: `A filmmaker of the interior. Influenced by Tarkovsky's patience, Wong Kar-wai's texture, and Agnès Varda's tenderness. Will be found in the small hours watching something subtitled and devastating.`,
}

/** Mock user profile data for development */
export const MOCK_USER = {
  id: MOCK_USER_ID,
  letterboxd_handle: 'adityad',
  display_name: 'Aditya',
  age: 28,
  location: 'Mumbai',
  films_watched: 847,
  obscurity_score: 72,
  avg_rating: 3.8,
  favourite_era: '1990s',
  now_showing: 'Chungking Express',
  intentions: ['meaningful connection', 'film nights', 'slow burn'],
  prompts: [
    {
      film_title: 'In the Mood for Love',
      prompt: 'The film that made me feel seen',
      answer: 'Longing without resolution. The corridor. The slow motion. It understood something I had never put into words.',
    },
    {
      film_title: 'Stalker',
      prompt: 'A film I return to every year',
      answer: 'Something different reveals itself each time. I think that is what the Zone is — the film itself.',
    },
  ],
}

/** Mock matches for development */
export const MOCK_MATCHES = [
  {
    id: 'match-001',
    userA: MOCK_USER_ID,
    userB: 'user-maya',
    score: 91,
    firstDateFilmId: 'film-chungking',
    why_text: 'You both orbit the same emotional frequencies — the bittersweet, the overheard, the almost. Wong Kar-wai is shared grammar. The disagreement on Lynch is the productive kind.',
    other_user: {
      display_name: 'Maya',
      age: 26,
      location: 'Delhi',
      letterboxd_handle: 'mayacinema',
      films_watched: 712,
    },
    shared_films: [
      { id: 'f1', title: 'In the Mood for Love', year: 2000 },
      { id: 'f2', title: 'Chungking Express', year: 1994 },
      { id: 'f3', title: 'Happy Together', year: 1997 },
    ],
    disagreement_film: {
      id: 'f4',
      title: 'Mulholland Drive',
      year: 2001,
      your_rating: 4.5,
      their_rating: 2.5,
    },
    first_date_film: { id: 'f2', title: 'Chungking Express', year: 1994 },
  },
  {
    id: 'match-002',
    userA: MOCK_USER_ID,
    userB: 'user-arjun',
    score: 84,
    firstDateFilmId: 'film-3women',
    why_text: 'A shared obsession with slow cinema and a healthy distrust of easy answers. You would argue about Bergman and enjoy every second of it.',
    other_user: {
      display_name: 'Arjun',
      age: 30,
      location: 'Bangalore',
      letterboxd_handle: 'arjunwatches',
      films_watched: 1203,
    },
    shared_films: [
      { id: 'f5', title: 'Persona', year: 1966 },
      { id: 'f6', title: '3 Women', year: 1977 },
      { id: 'f7', title: 'Wild Strawberries', year: 1957 },
    ],
    disagreement_film: {
      id: 'f8',
      title: 'The Tree of Life',
      year: 2011,
      your_rating: 2.5,
      their_rating: 5.0,
    },
    first_date_film: { id: 'f6', title: '3 Women', year: 1977 },
  },
  {
    id: 'match-003',
    userA: MOCK_USER_ID,
    userB: 'user-priya',
    score: 78,
    firstDateFilmId: 'film-amelie',
    why_text: 'The French New Wave runs through you both like a shared mother tongue. There is warmth here, and a mutual suspicion of cynicism.',
    other_user: {
      display_name: 'Priya',
      age: 27,
      location: 'Pune',
      letterboxd_handle: 'priyaonfilm',
      films_watched: 534,
    },
    shared_films: [
      { id: 'f9', title: 'Amélie', year: 2001 },
      { id: 'f10', title: 'Breathless', year: 1960 },
      { id: 'f11', title: 'The 400 Blows', year: 1959 },
    ],
    disagreement_film: {
      id: 'f12',
      title: 'Jeanne Dielman',
      year: 1975,
      your_rating: 4.0,
      their_rating: 2.0,
    },
    first_date_film: { id: 'f9', title: 'Amélie', year: 2001 },
  },
]

/** Mock soul siblings for development */
export const MOCK_SIBLINGS = [
  {
    id: 'sib-001',
    display_name: 'Rohan',
    age: 29,
    location: 'Chennai',
    letterboxd_handle: 'rohancinema',
    films_watched: 923,
    score: 94,
    shared_trait: 'Both devotees of the long take',
  },
  {
    id: 'sib-002',
    display_name: 'Kavya',
    age: 25,
    location: 'Hyderabad',
    letterboxd_handle: 'kavyafilm',
    films_watched: 645,
    score: 89,
    shared_trait: 'Mutual horror of happy endings',
  },
  {
    id: 'sib-003',
    display_name: 'Sid',
    age: 32,
    location: 'Mumbai',
    letterboxd_handle: 'sidwatches',
    films_watched: 1547,
    score: 86,
    shared_trait: 'Shared obsession: 1970s American cinema',
  },
]

/** Mock chat messages */
export const MOCK_MESSAGES = [
  {
    id: 'msg-001',
    match_id: 'match-001',
    sender_id: 'user-maya',
    content: 'The first-date film suggestion is perfect. I have been meaning to rewatch it for ages.',
    sent_at: '2024-01-15T14:23:00Z',
  },
  {
    id: 'msg-002',
    match_id: 'match-001',
    sender_id: MOCK_USER_ID,
    content: 'Right? The alley scene. Every time. I actually watched it alone on a Tuesday and cried.',
    sent_at: '2024-01-15T14:31:00Z',
  },
  {
    id: 'msg-003',
    match_id: 'match-001',
    sender_id: 'user-maya',
    content: 'A Tuesday. That is exactly the correct day for that film.',
    sent_at: '2024-01-15T14:35:00Z',
  },
  {
    id: 'msg-004',
    match_id: 'match-001',
    sender_id: MOCK_USER_ID,
    content: 'I knew you would understand. So — streaming or projector?',
    sent_at: '2024-01-15T14:40:00Z',
  },
]
