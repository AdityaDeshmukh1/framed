/**
 * Rating-quiz engine — adaptive preference elicitation over film embeddings.
 *
 * The model: a Gaussian process over the films' MiniLM embeddings (the same
 * ones the backend uses). Every answer updates a guess of how much the user
 * would like *every* film in the pool, plus how sure we are.
 *
 * Picking the next film: ask about films the user has probably seen, whose
 * reaction we can least predict:
 *
 *   score = P(seen) × (uncertainty + bonus for a language not yet asked)
 *           × variety (penalises films like the last few cards)
 *
 * uncertainty mixes raw model variance (what we haven't asked about) with how
 * close P(like) is to a coin-flip (depends on what you liked/disliked).
 * So loving Inception makes Interstellar boring (we can already guess), while
 * films half-way between your likes and dislikes become the best questions.
 *
 * The model learns how you differ from the crowd: each answer is stored as
 * (your rating − the film's average rating on MovieLens). Loving a film
 * everyone loves teaches it nothing, so such films are rarely worth asking,
 * and films people split on (high rating spread) get a bonus.
 *
 * P(seen) starts from MovieLens rating counts and learns from your "haven't
 * seen" answers on similar films: skip three Korean films and Korean sinks.
 *
 * The pool comes from GET /quiz/pool (services/api cmd/catalogue picks it).
 *
 * Everything is pure and synchronous; with ≤ 60 answers and ~400 films a
 * full re-score is well under a few ms.
 */

/* ── Data ────────────────────────────────────────────────────────────────── */

/** One quiz card as served by GET /quiz/pool. */
export interface SeedFilm {
  tmdbId: number
  title: string
  year: number
  directors: string[]
  genres: string[]
  language: string
  posterPath: string
  /** MovieLens crowd stats: mean rating, its spread, number of ratings */
  ratingMean: number
  ratingStd: number
  ratingCount: number
  embedding: number[]
}

export interface PoolFilm extends SeedFilm {
  /** unit-length raw embedding (what the backend stores) */
  raw: Float64Array
  /** pool-mean-centred, unit-length: used for similarity inside the quiz */
  centred: Float64Array
  /** prior probability a typical user has seen it, from rating count */
  seenPrior: number
  /** how much people disagree about it, relative to the pool (0.7–1.4) */
  divisive: number
}

export type Pool = { films: PoolFilm[]; byId: Map<number, PoolFilm>; bigLanguages: Set<string> }

export function buildPool(seed: SeedFilm[]): Pool {
  const dim = seed[0]?.embedding.length ?? 0
  const raws = seed.map((f) => normalize(Float64Array.from(f.embedding)))
  const mean = new Float64Array(dim)
  for (const r of raws) for (let i = 0; i < dim; i++) mean[i] += r[i] / raws.length
  const meanStd = seed.reduce((a, f) => a + f.ratingStd, 0) / Math.max(seed.length, 1)
  const films = seed.map((f, idx) => {
    const centred = new Float64Array(dim)
    for (let i = 0; i < dim; i++) centred[i] = raws[idx][i] - mean[i]
    return {
      ...f,
      raw: raws[idx],
      centred: normalize(centred),
      // ~4k MovieLens ratings → 20%, ~100k → 90%
      seenPrior: clamp(0.2 + (0.7 * (Math.log10(Math.max(f.ratingCount, 1)) - 3.6)) / 1.4, 0.15, 0.9),
      divisive: clamp(f.ratingStd / meanStd, 0.7, 1.4),
    }
  })
  const counts = new Map<string, number>()
  for (const f of seed) counts.set(f.language, (counts.get(f.language) ?? 0) + 1)
  const bigLanguages = new Set([...counts].filter(([, n]) => n >= NOVEL_MIN_FILMS).map(([l]) => l))
  return { films, byId: new Map(films.map((f) => [f.tmdbId, f])), bigLanguages }
}

export const LANGUAGE_NAMES: Record<string, string> = {
  en: 'English', hi: 'Hindi', ta: 'Tamil', te: 'Telugu', ml: 'Malayalam', kn: 'Kannada', bn: 'Bengali',
  mr: 'Marathi', ko: 'Korean', ja: 'Japanese', fr: 'French', es: 'Spanish', it: 'Italian', de: 'German',
  zh: 'Chinese', cn: 'Cantonese', pt: 'Portuguese', sv: 'Swedish', da: 'Danish', fa: 'Persian',
}

export type PosterSize = 'w185' | 'w342' | 'w500' | 'w780'
export const posterUrl = (path: string, size: PosterSize = 'w500') => `https://image.tmdb.org/t/p/${size}${path}`
/** srcSet for a full-size card: w500 on standard screens, w780 on high-density ones. */
export const posterSrcSet = (path: string) => `${posterUrl(path, 'w500')} 500w, ${posterUrl(path, 'w780')} 780w`
export const POSTER_SIZES = '(max-width: 480px) 92vw, 420px'

/* ── State ───────────────────────────────────────────────────────────────── */

export type Answer = 'favourite' | 'loved' | 'fine' | 'nope' | 'unseen'
export interface Response {
  id: number
  answer: Answer
  /** this card was a follow-up to the previous answer (no follow-up chains) */
  followUp?: boolean
}
export interface QuizState {
  favourites: number[]
  responses: Response[]
}

/** Answers on the Letterboxd 0.5–5 scale (what the backend would store). */
export const ANSWER_RATING: Record<Exclude<Answer, 'unseen'>, number> = { favourite: 5, loved: 4.5, fine: 3, nope: 1.5 }
const FAVOURITE_RATING = 5

/**
 * Onboarding stops at TARGET_RATED: enough for "Strong / Good" match labels
 * (match scores move ±29 points at 10 films). The profile keeps sharpening
 * from ratings collected later, in Discover.
 */
export const TARGET_RATED = 10
export const MIN_RATED = 6
export const MAX_SHOWN = 20


export const ratedCount = (s: QuizState) => s.responses.filter((r) => r.answer !== 'unseen').length

/* ── Model ───────────────────────────────────────────────────────────────── */

// Kernel = how much liking film A tells us about film B. Tuned by simulating
// three synthetic users against the real pool (apps/web/scripts/quiz-sim.py):
//   local:    near-identical films (Totoro ↔ Spirited Away)
//   linear:   broad direction in embedding space (dark crime vs. feel-good)
//   language: the embeddings are built from English TMDB text and carry no
//             language/era signal, so "loves Hindi cinema" is added here
//   era:      decade taste
//   bias:     how generous this user is overall
const LOCAL = 0.8
const LENGTH = 0.25
const LINEAR = 3.0
const LANGUAGE = 0.6
const ERA = 0.3
const BIAS = 0.3
const NOISE = 0.45 // answer noise
const FAV_NOISE = 0.15 // favourites are deliberate, so trust them more
const PRIOR_VAR = LOCAL + LINEAR + LANGUAGE + ERA + BIAS

/** Weight on "we know nothing here" vs "could go either way". */
const SPREAD_WEIGHT = 0.7
/** Bonus for a language not shown yet: makes sure we probe each one once. */
const NOVEL_LANGUAGE = 0.6
/** Only languages with at least this many pool films get probed. */
const NOVEL_MIN_FILMS = 8
/**
 * Bonus for films we think you'll like. Pure uncertainty sampling learns fast
 * but feels deaf: love one Hindi film and it never shows another. This keeps
 * following your likes, to tell apart *which* ones you love.
 */
const LIKE_BONUS = 0.15
/**
 * Variety. Order barely changes how accurate the profile ends up (simulated on
 * real MovieLens users), so we're free to optimise for a quiz that doesn't
 * feel samey. A card is penalised for resembling the last few cards, and
 * more so for sharing their main genre or decade.
 */
const RECENT = 4
const SIMILAR_PENALTY = 0.8 // × max(0, similarity to a recent card)
const GENRE_FATIGUE = 0.6 // × per recent card with the same main genre
const DECADE_FATIGUE = 0.9 // × per recent card from the same decade
/** Follow-ups ("you loved X, does this land too?"): at most this many, spaced out. */
const MAX_FOLLOW_UPS = 2
const FOLLOW_UP_GAP = 3

const sim = (a: Float64Array, b: Float64Array) => {
  let d = 0
  for (let i = 0; i < a.length; i++) d += a[i] * b[i]
  return d
}
const kernel = (a: PoolFilm, b: PoolFilm) => {
  const s = sim(a.centred, b.centred)
  return (
    LOCAL * Math.exp((s - 1) / LENGTH) +
    LINEAR * s +
    (a.language === b.language ? LANGUAGE : 0) +
    ERA * Math.exp(-Math.abs(a.year - b.year) / 15) +
    BIAS
  )
}

interface Obs {
  film: PoolFilm
  y: number // your rating − the crowd's average
  noise: number
}

function observations(pool: Pool, s: QuizState): Obs[] {
  const obs: Obs[] = []
  for (const id of s.favourites) {
    const film = pool.byId.get(id)
    if (film) obs.push({ film, y: FAVOURITE_RATING - film.ratingMean, noise: FAV_NOISE })
  }
  for (const r of s.responses) {
    const film = pool.byId.get(r.id)
    if (film && r.answer !== 'unseen')
      obs.push({ film, y: ANSWER_RATING[r.answer] - film.ratingMean, noise: r.answer === 'favourite' ? FAV_NOISE : NOISE })
  }
  return obs
}

export interface Prediction {
  film: PoolFilm
  /** predicted rating − the crowd's average */
  mean: number
  variance: number
  pLike: number
  pSeen: number
  uncertainty: number
  /** bonus for a language we haven't asked about yet */
  novel: number
  score: number
  /** set when this card follows up on the previous loved/disliked film */
  followUpOf?: { title: string; answer: Answer }
}

export interface Model {
  predictions: Prediction[] // every unused film, best question first
  /** 0–1: how much of the prior uncertainty the answers have removed */
  confidence: number
  obs: Obs[]
}

export function fitModel(pool: Pool, s: QuizState): Model {
  const obs = observations(pool, s)
  const n = obs.length
  const used = new Set([...s.favourites, ...s.responses.map((r) => r.id)])

  // GP posterior: alpha = (K + N)⁻¹ y via Cholesky.
  const K = new Float64Array(n * n)
  for (let i = 0; i < n; i++)
    for (let j = 0; j <= i; j++) {
      const k = kernel(obs[i].film, obs[j].film) + (i === j ? obs[i].noise : 0)
      K[i * n + j] = k
      K[j * n + i] = k
    }
  const L = cholesky(K, n)
  const alpha = cholSolve(L, n, Float64Array.from(obs.map((o) => o.y)))

  // Seen model: similarity-weighted share of "seen" among everything shown.
  const shown = [
    ...s.favourites.map((id) => ({ film: pool.byId.get(id), seen: 1 })),
    ...s.responses.map((r) => ({ film: pool.byId.get(r.id), seen: r.answer === 'unseen' ? 0 : 1 })),
  ].filter((x): x is { film: PoolFilm; seen: number } => !!x.film)
  const shownLanguages = new Set(shown.map((x) => x.film.language))
  const recent = s.responses
    .slice(-RECENT)
    .map((r) => pool.byId.get(r.id))
    .filter((f): f is PoolFilm => !!f)

  let varSum = 0
  let varWeight = 0
  const predictions: Prediction[] = []
  for (const film of pool.films) {
    const kx = new Float64Array(n)
    for (let i = 0; i < n; i++) kx[i] = kernel(film, obs[i].film)
    let mean = 0
    for (let i = 0; i < n; i++) mean += kx[i] * alpha[i]
    const v = forwardSub(L, n, kx)
    let variance = PRIOR_VAR
    for (let i = 0; i < n; i++) variance -= v[i] * v[i]
    variance = Math.max(variance, 1e-6)

    // Model confidence over the films people actually know.
    varSum += film.seenPrior * (variance / PRIOR_VAR)
    varWeight += film.seenPrior
    if (used.has(film.tmdbId)) continue

    let wSeen = 0
    let wTotal = 0
    for (const x of shown) {
      // "Haven't seen" spreads to similar films, and strongly within a language.
      const w = Math.exp((sim(film.centred, x.film.centred) - 1) / 0.3) * (film.language === x.film.language ? 4 : 1)
      wSeen += w * x.seen
      wTotal += w
    }
    const PRIOR_STRENGTH = 1.5
    const pSeen = (wSeen + PRIOR_STRENGTH * film.seenPrior) / (wTotal + PRIOR_STRENGTH)

    // "Loved" = predicted rating of 4+ (crowd average + your deviation).
    const pLike = normCdf((film.ratingMean + mean - 4) / Math.sqrt(variance + NOISE))
    const coinFlip = entropy(pLike) // 1 when 50/50
    const spread = Math.sqrt(variance / PRIOR_VAR) // 1 when we know nothing
    const uncertainty = (1 - SPREAD_WEIGHT) * coinFlip + SPREAD_WEIGHT * spread
    const novel = shownLanguages.has(film.language) || !pool.bigLanguages.has(film.language) ? 0 : NOVEL_LANGUAGE
    const likeBonus = n > 0 ? LIKE_BONUS * pLike : 0
    let closest = 0
    let sameGenre = 0
    let sameDecade = 0
    for (const r of recent) {
      closest = Math.max(closest, sim(film.centred, r.centred))
      if (r.genres[0] && r.genres[0] === film.genres[0]) sameGenre++
      if (Math.floor(r.year / 10) === Math.floor(film.year / 10)) sameDecade++
    }
    const variety =
      Math.max(0.05, 1 - SIMILAR_PENALTY * closest) * GENRE_FATIGUE ** sameGenre * DECADE_FATIGUE ** sameDecade
    const score = pSeen * (uncertainty + novel + likeBonus) * film.divisive * variety
    predictions.push({ film, mean, variance, pLike, pSeen, uncertainty, novel, score })
  }
  predictions.sort((a, b) => b.score - a.score)

  // Follow-up: right after a strong reaction, show the closest film to it.
  // Loved → "does this land too?" (which of these do you *really* love);
  // nope → "is this different?" (the film, or the whole kind of film?).
  const last = s.responses.at(-1)
  const lastFilm = last && pool.byId.get(last.id)
  const strong = last && (last.answer === 'loved' || last.answer === 'favourite' || last.answer === 'nope')
  const followUps = s.responses.filter((r) => r.followUp).length
  const lastFollowUp = s.responses.findLastIndex((r) => r.followUp)
  const spaced = lastFollowUp < 0 || s.responses.length - lastFollowUp > FOLLOW_UP_GAP
  if (last && lastFilm && strong && !last.followUp && followUps < MAX_FOLLOW_UPS && spaced) {
    let best = -1
    let bestScore = -Infinity
    predictions.forEach((p, i) => {
      const closeness = (kernel(lastFilm, p.film) - BIAS) / (PRIOR_VAR - BIAS)
      const sc = closeness * p.pSeen * (0.5 + p.uncertainty)
      if (sc > bestScore) {
        bestScore = sc
        best = i
      }
    })
    if (best >= 0) {
      const [p] = predictions.splice(best, 1)
      predictions.unshift({ ...p, followUpOf: { title: lastFilm.title, answer: last.answer } })
    }
  }
  return { predictions, confidence: 1 - varSum / varWeight, obs }
}

export function isDone(model: Model, s: QuizState, target = TARGET_RATED) {
  return model.predictions.length === 0 || s.responses.length >= MAX_SHOWN + (target - TARGET_RATED) * 2 || ratedCount(s) >= target
}

export function strengthLabel(confidence: number): 'Sketch' | 'Outline' | 'Portrait' {
  return confidence < 0.15 ? 'Sketch' : confidence < 0.28 ? 'Outline' : 'Portrait'
}

/* ── "Why this film?" ───────────────────────────────────────────────────── */

export function explain(model: Model, p: Prediction): string {
  if (model.obs.length === 0) return 'Starting with something most people have seen.'
  if (p.followUpOf) {
    return p.followUpOf.answer !== 'nope'
      ? `You loved ${p.followUpOf.title}. Does this one land too?`
      : `${p.followUpOf.title} wasn't for you. Is this different?`
  }
  if (p.novel > 0) {
    const lang = LANGUAGE_NAMES[p.film.language] ?? 'this language'
    return `Your first ${lang} film here. Is ${lang} cinema your thing?`
  }
  let best: Obs | null = null
  let bestSim = -Infinity
  for (const o of model.obs) {
    const s = sim(p.film.centred, o.film.centred)
    if (s > bestSim) {
      bestSim = s
      best = o
    }
  }
  if (best && bestSim > 0.22) {
    const t = best.film.title
    if (best.y >= 0.8) return p.pLike > 0.6 ? `You loved ${t}. Does this land too?` : `Close to ${t}, which you loved, but we can't call this one.`
    if (best.y <= -0.8) return `${t} wasn't for you. Is this different?`
    return `You were lukewarm on ${t}. Tie-breaker.`
  }
  if (p.film.divisive > 1.2) return 'People are split on this one. Which side are you on?'
  const askedGenres = new Set(model.obs.flatMap((o) => o.film.genres))
  const fresh = p.film.genres.find((g) => !askedGenres.has(g))
  if (fresh) return `New territory: we don't know your take on ${fresh.toLowerCase()} yet.`
  if (p.pLike > 0.65) return `We think you'll love this. ${Math.round(p.pLike * 100)}% sure.`
  if (p.pLike < 0.3) return "Our guess: not your thing. Prove us wrong?"
  const lines = ['Could go either way for you.', "We can't call this one.", 'Honestly, no idea what you’ll say.']
  return lines[p.film.tmdbId % lines.length]
}

/* ── Results ─────────────────────────────────────────────────────────────── */

interface Entry {
  film: PoolFilm
  rating: number
  liked: boolean
}

function entries(pool: Pool, s: QuizState): Entry[] {
  const out: Entry[] = []
  for (const id of s.favourites) {
    const film = pool.byId.get(id)
    if (film) out.push({ film, rating: FAVOURITE_RATING, liked: true })
  }
  for (const r of s.responses) {
    const film = pool.byId.get(r.id)
    if (film && r.answer !== 'unseen')
      out.push({ film, rating: ANSWER_RATING[r.answer], liked: r.answer === 'loved' || r.answer === 'favourite' })
  }
  return out
}

export function genreAffinity(model: Model) {
  const sum = new Map<string, number>()
  const n = new Map<string, number>()
  for (const o of model.obs)
    for (const g of o.film.genres) {
      sum.set(g, (sum.get(g) ?? 0) + o.y)
      n.set(g, (n.get(g) ?? 0) + 1)
    }
  return [...sum.keys()]
    .map((g) => ({ genre: g, affinity: clamp(sum.get(g)! / (n.get(g)! + 1) / 1.5, -1, 1), evidence: n.get(g)! }))
    .sort((a, b) => b.affinity - a.affinity)
}

/** The answers in the shape POST /onboard {source: "quiz"} expects. */
export function toSubmission(pool: Pool, s: QuizState): { tmdb_id: number; rating: number; liked: boolean }[] {
  return entries(pool, s).map((e) => ({ tmdb_id: e.film.tmdbId, rating: e.rating, liked: e.liked }))
}

/* ── Maths helpers ───────────────────────────────────────────────────────── */

function normalize(v: Float64Array) {
  let s = 0
  for (const x of v) s += x * x
  const n = Math.sqrt(s) || 1
  for (let i = 0; i < v.length; i++) v[i] /= n
  return v
}

function clamp(x: number, lo: number, hi: number) {
  return Math.min(hi, Math.max(lo, x))
}

function cholesky(A: Float64Array, n: number) {
  const L = new Float64Array(n * n)
  for (let i = 0; i < n; i++)
    for (let j = 0; j <= i; j++) {
      let s = A[i * n + j]
      for (let k = 0; k < j; k++) s -= L[i * n + k] * L[j * n + k]
      L[i * n + j] = i === j ? Math.sqrt(Math.max(s, 1e-9)) : s / L[j * n + j]
    }
  return L
}

function forwardSub(L: Float64Array, n: number, b: Float64Array) {
  const x = new Float64Array(n)
  for (let i = 0; i < n; i++) {
    let s = b[i]
    for (let k = 0; k < i; k++) s -= L[i * n + k] * x[k]
    x[i] = s / L[i * n + i]
  }
  return x
}

function cholSolve(L: Float64Array, n: number, b: Float64Array) {
  const y = forwardSub(L, n, b)
  const x = new Float64Array(n)
  for (let i = n - 1; i >= 0; i--) {
    let s = y[i]
    for (let k = i + 1; k < n; k++) s -= L[k * n + i] * x[k]
    x[i] = s / L[i * n + i]
  }
  return x
}

function normCdf(z: number) {
  // Abramowitz–Stegun 7.1.26
  const t = 1 / (1 + 0.3275911 * Math.abs(z) / Math.SQRT2)
  const y =
    1 - ((((1.061405429 * t - 1.453152027) * t + 1.421413741) * t - 0.284496736) * t + 0.254829592) * t * Math.exp(-(z * z) / 2)
  return z >= 0 ? (1 + y) / 2 : (1 - y) / 2
}

function entropy(p: number) {
  if (p <= 0 || p >= 1) return 0
  return -(p * Math.log2(p) + (1 - p) * Math.log2(1 - p))
}
