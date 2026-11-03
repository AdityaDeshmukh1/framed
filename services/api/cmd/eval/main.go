// Command eval measures taste-vector variants on real MovieLens users.
//
//	go run ./cmd/eval -movielens ../../data/movielens/ml-32m
//
// For a deterministic sample of MovieLens users (restricted to films in our
// catalogue), each variant (embedding text version × weighting) is scored on:
//
//	capture   per-user AUC: a vector built from 70% of your ratings should
//	          score your held-out loved films above your held-out disliked ones
//	fidelity  Spearman correlation, over user pairs, between our cosine and how
//	          similarly the two actually rated the films they share (Pearson
//	          on co-rated films, ≥20 in common)
//	thin      fidelity when one side has only 10 ratings (a quiz-sized profile)
//	spread    std and 5–95% range of pairwise cosines: bigger = users look less
//	          alike, so match scores mean more
//
// Caveat: film_stats includes these users' own ratings, a slight leak in
// favour of the crowd baseline (each user is one rating among thousands).
package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/framed-app/api/internal/embedding"
	"github.com/framed-app/api/internal/taste"
	"github.com/jackc/pgx/v5/pgxpool"
)

type film struct {
	tmdbID    int
	emb       map[int][]float32 // by text version
	crowdMean float64
	count     int
}

type rating struct {
	film int // index into films
	r    float64
}

type user struct{ ratings []rating } // sorted by film

type variant struct {
	name    string
	version int
	opts    func(center []float32) taste.Options
}

var variants = []variant{
	{"v1 text · v1 weights (production today)", 1, func([]float32) taste.Options { return taste.V1 }},
	{"v2 text · v1 weights", 2, func([]float32) taste.Options { return taste.V1 }},
	{"v2 text · crowd baseline", 2, func([]float32) taste.Options { return taste.Options{Baseline: taste.Crowd} }},
	{"v2 text · crowd · centred", 2, func(c []float32) taste.Options { return taste.Options{Baseline: taste.Crowd, Center: c} }},
	{"v2 text · crowd · centred · signed", 2, func(c []float32) taste.Options {
		return taste.Options{Baseline: taste.Crowd, Center: c, Signed: true}
	}},
	{"v2 text · user mean · centred · signed", 2, func(c []float32) taste.Options {
		return taste.Options{Baseline: taste.UserMean, Center: c, Signed: true}
	}},
	{"v1 text · user mean · centred · signed", 1, func(c []float32) taste.Options {
		return taste.Options{Baseline: taste.UserMean, Center: c, Signed: true}
	}},
	{"v2 text · user mean · centred", 2, func(c []float32) taste.Options {
		return taste.Options{Baseline: taste.UserMean, Center: c}
	}},
}

func main() {
	mlDir := flag.String("movielens", "../../data/movielens/ml-32m", "MovieLens directory")
	every := flag.Int("every", 40, "sample users whose id is divisible by this")
	minRatings := flag.Int("min", 40, "minimum catalogue ratings per sampled user")
	pairs := flag.Int("pairs", 40000, "user pairs to sample for fidelity")
	cacheDir := flag.String("cache", "../../data/eval", "where to cache v1 embeddings")
	flag.Parse()
	ctx := context.Background()
	rng := rand.New(rand.NewSource(1))

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	films, byTMDB := loadCatalogue(ctx, pool)
	ensureV1(ctx, pool, films, *cacheDir)
	users := loadUsers(*mlDir, byTMDB, *every, *minRatings)
	log.Printf("[eval] %d films, %d users (%d ratings)", len(films), len(users), countRatings(users))

	pairList := samplePairs(users, *pairs, rng)
	log.Printf("[eval] %d user pairs with ≥20 co-rated films", len(pairList))

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "variant\tcapture AUC\tfidelity ρ\tthin ρ\tcos mean\tcos std\tcos 5–95%\t")
	for _, v := range variants {
		center := popularityMean(films, v.version)
		opts := v.opts(center)
		capture := heldOutAUC(users, films, v.version, opts, rng)
		full := make([][]float32, len(users))
		thin := make([][]float32, len(users))
		for i, u := range users {
			full[i] = vectorFor(u.ratings, films, v.version, opts)
			thin[i] = vectorFor(sampleN(u.ratings, 10, rand.New(rand.NewSource(int64(i)))), films, v.version, opts)
		}
		var pred, predThin, truth, cosAll []float64
		for _, p := range pairList {
			pred = append(pred, dot(full[p.a], full[p.b]))
			predThin = append(predThin, dot(thin[p.a], full[p.b]))
			truth = append(truth, p.agreement)
			cosAll = append(cosAll, dot(full[p.a], full[p.b]))
		}
		mean, std := meanStd(cosAll)
		sort.Float64s(cosAll)
		fmt.Fprintf(tw, "%s\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\t%.2f – %.2f\t\n", v.name, capture, spearman(pred, truth), spearman(predThin, truth),
			mean, std, cosAll[len(cosAll)*5/100], cosAll[len(cosAll)*95/100])
		tw.Flush()
	}
}

/* ── data ───────────────────────────────────────────────────────────────── */

func loadCatalogue(ctx context.Context, pool *pgxpool.Pool) ([]*film, map[int]int) {
	rows, err := pool.Query(ctx, `
		SELECT f.tmdb_id, f.embedding::text, s.rating_mean, s.rating_count
		FROM films f JOIN film_stats s ON s.tmdb_id = f.tmdb_id
		WHERE f.embedding IS NOT NULL AND f.embedding_version = 2
		ORDER BY f.tmdb_id`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var films []*film
	byTMDB := map[int]int{}
	for rows.Next() {
		var f film
		var emb string
		if err := rows.Scan(&f.tmdbID, &emb, &f.crowdMean, &f.count); err != nil {
			log.Fatal(err)
		}
		v, err := embedding.ParseVector(emb)
		if err != nil {
			log.Fatal(err)
		}
		f.emb = map[int][]float32{2: v}
		byTMDB[f.tmdbID] = len(films)
		films = append(films, &f)
	}
	if len(films) == 0 {
		log.Fatal("no v2 catalogue films with stats: run cmd/catalogue first")
	}
	return films, byTMDB
}

// ensureV1 embeds every catalogue film with the v1 text, cached on disk.
func ensureV1(ctx context.Context, pool *pgxpool.Pool, films []*film, dir string) {
	path := filepath.Join(dir, "embeddings-v1.gob")
	cache := map[int][]float32{}
	if f, err := os.Open(path); err == nil {
		_ = gob.NewDecoder(f).Decode(&cache)
		f.Close()
	}
	client := embedding.NewClient(envOr("ML_SERVICE_URL", "http://localhost:8001"))
	var added int
	for _, f := range films {
		if v, ok := cache[f.tmdbID]; ok {
			f.emb[1] = v
			continue
		}
		var ef embedding.Film
		if err := pool.QueryRow(ctx, `SELECT title, year, genres, directors, themes, synopsis FROM films WHERE tmdb_id = $1`, f.tmdbID).
			Scan(&ef.Title, &ef.Year, &ef.Genres, &ef.Directors, &ef.Keywords, &ef.Synopsis); err != nil {
			log.Fatal(err)
		}
		v, err := client.Embed(ctx, embedding.Text(ef, 1))
		if err != nil {
			log.Fatal(err)
		}
		cache[f.tmdbID], f.emb[1] = v, v
		if added++; added%500 == 0 {
			log.Printf("[eval] v1 embeddings: %d new", added)
		}
	}
	if added > 0 {
		_ = os.MkdirAll(dir, 0o755)
		f, err := os.Create(path)
		if err == nil {
			_ = gob.NewEncoder(f).Encode(cache)
			f.Close()
		}
	}
}

func loadUsers(dir string, byTMDB map[int]int, every, minRatings int) []user {
	links := map[int]int{} // movieId → film index
	lf, err := os.Open(filepath.Join(dir, "links.csv"))
	if err != nil {
		log.Fatal(err)
	}
	r := csv.NewReader(lf)
	_, _ = r.Read()
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		movie, _ := strconv.Atoi(rec[0])
		tmdb, _ := strconv.Atoi(rec[2])
		if i, ok := byTMDB[tmdb]; ok {
			links[movie] = i
		}
	}
	lf.Close()

	f, err := os.Open(filepath.Join(dir, "ratings.csv"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	byUser := map[int][]rating{}
	sc := bufio.NewScanner(f)
	sc.Scan()
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), ",", 4)
		if len(parts) < 3 {
			continue
		}
		uid, _ := strconv.Atoi(parts[0])
		if uid%every != 0 {
			continue
		}
		movie, _ := strconv.Atoi(parts[1])
		fi, ok := links[movie]
		if !ok {
			continue
		}
		rv, _ := strconv.ParseFloat(parts[2], 64)
		byUser[uid] = append(byUser[uid], rating{film: fi, r: rv})
	}
	ids := make([]int, 0, len(byUser))
	for id, rs := range byUser {
		if len(rs) >= minRatings {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	users := make([]user, len(ids))
	for i, id := range ids {
		rs := byUser[id]
		sort.Slice(rs, func(a, b int) bool { return rs[a].film < rs[b].film })
		users[i] = user{ratings: rs}
	}
	return users
}

/* ── metrics ────────────────────────────────────────────────────────────── */

func vectorFor(rs []rating, films []*film, version int, opts taste.Options) []float32 {
	entries := make([]taste.Entry, len(rs))
	for i, r := range rs {
		rv, cm := r.r, films[r.film].crowdMean
		entries[i] = taste.Entry{Rating: &rv, Embedding: films[r.film].emb[version], CrowdMean: &cm}
	}
	return taste.Vector(entries, opts)
}

// heldOutAUC: for each user, vector from 70% of ratings; do held-out films
// they liked score higher than ones they disliked?
func heldOutAUC(users []user, films []*film, version int, opts taste.Options, rng *rand.Rand) float64 {
	var sum float64
	var n int
	for ui, u := range users {
		r := rand.New(rand.NewSource(int64(ui)))
		perm := r.Perm(len(u.ratings))
		cut := len(perm) * 7 / 10
		train := make([]rating, 0, cut)
		var trainMean float64
		for _, i := range perm[:cut] {
			train = append(train, u.ratings[i])
			trainMean += u.ratings[i].r
		}
		trainMean /= float64(cut)
		vec := vectorFor(train, films, version, opts)
		if vec == nil {
			continue
		}
		var pos, neg []float64
		for _, i := range perm[cut:] {
			rt := u.ratings[i]
			e := films[rt.film].emb[version]
			s := 0.0
			for d := range vec {
				x := e[d]
				if opts.Center != nil {
					x -= opts.Center[d]
				}
				s += float64(vec[d]) * float64(x)
			}
			if rt.r >= trainMean+0.5 {
				pos = append(pos, s)
			} else if rt.r <= trainMean-0.5 {
				neg = append(neg, s)
			}
		}
		if len(pos) < 3 || len(neg) < 3 {
			continue
		}
		sum += auc(pos, neg)
		n++
	}
	_ = rng
	return sum / float64(n)
}

type pair struct {
	a, b      int
	agreement float64
}

// samplePairs draws random user pairs and keeps those with ≥20 co-rated
// films; agreement is the Pearson correlation of their co-rated ratings.
func samplePairs(users []user, want int, rng *rand.Rand) []pair {
	var out []pair
	for tries := 0; len(out) < want && tries < want*30; tries++ {
		a, b := rng.Intn(len(users)), rng.Intn(len(users))
		if a == b {
			continue
		}
		var xs, ys []float64
		ra, rb := users[a].ratings, users[b].ratings
		for i, j := 0, 0; i < len(ra) && j < len(rb); {
			switch {
			case ra[i].film < rb[j].film:
				i++
			case ra[i].film > rb[j].film:
				j++
			default:
				xs, ys = append(xs, ra[i].r), append(ys, rb[j].r)
				i, j = i+1, j+1
			}
		}
		if len(xs) < 20 {
			continue
		}
		if p, ok := pearson(xs, ys); ok {
			out = append(out, pair{a, b, p})
		}
	}
	return out
}

func popularityMean(films []*film, version int) []float32 {
	dim := len(films[0].emb[version])
	acc := make([]float64, dim)
	var w float64
	for _, f := range films {
		fw := float64(f.count)
		for d, x := range f.emb[version] {
			acc[d] += fw * float64(x)
		}
		w += fw
	}
	out := make([]float32, dim)
	for d := range acc {
		out[d] = float32(acc[d] / w)
	}
	return out
}

func sampleN(rs []rating, n int, rng *rand.Rand) []rating {
	if len(rs) <= n {
		return rs
	}
	out := make([]rating, n)
	for i, j := range rng.Perm(len(rs))[:n] {
		out[i] = rs[j]
	}
	return out
}

func auc(pos, neg []float64) float64 {
	var wins float64
	for _, p := range pos {
		for _, q := range neg {
			if p > q {
				wins++
			} else if p == q {
				wins += 0.5
			}
		}
	}
	return wins / float64(len(pos)*len(neg))
}

func pearson(x, y []float64) (float64, bool) {
	mx, sx := meanStd(x)
	my, sy := meanStd(y)
	if sx == 0 || sy == 0 {
		return 0, false
	}
	var c float64
	for i := range x {
		c += (x[i] - mx) * (y[i] - my)
	}
	return c / float64(len(x)) / (sx * sy), true
}

func spearman(x, y []float64) float64 {
	r, _ := pearson(ranks(x), ranks(y))
	return r
}

func ranks(v []float64) []float64 {
	idx := make([]int, len(v))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return v[idx[a]] < v[idx[b]] })
	out := make([]float64, len(v))
	for r, i := range idx {
		out[i] = float64(r)
	}
	return out
}

func meanStd(v []float64) (float64, float64) {
	var s, ss float64
	for _, x := range v {
		s += x
	}
	m := s / float64(len(v))
	for _, x := range v {
		ss += (x - m) * (x - m)
	}
	return m, math.Sqrt(ss / float64(len(v)))
}

func dot(a, b []float32) float64 {
	if a == nil || b == nil {
		return 0
	}
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

func countRatings(us []user) int {
	n := 0
	for _, u := range us {
		n += len(u.ratings)
	}
	return n
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
