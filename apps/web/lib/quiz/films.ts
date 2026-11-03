/**
 * Mock film pool for the rating quiz.
 *
 * In the real version this comes from the backend: a curated seed set of
 * well-known films with embeddings, so picks can be spread across the
 * embedding space. Here each film carries hand-written facet tags and a rough
 * popularity (how likely a typical user is to have seen it), which is enough
 * to prototype the adaptive picking.
 */

export type Facet =
  | 'arthouse'
  | 'blockbuster'
  | 'indian'
  | 'world'
  | 'classic'
  | 'horror'
  | 'romance'
  | 'comedy'
  | 'crime'
  | 'scifi'
  | 'animation'
  | 'drama'

export const FACET_LABELS: Record<Facet, string> = {
  arthouse: 'Arthouse',
  blockbuster: 'Big-screen spectacle',
  indian: 'Indian cinema',
  world: 'World cinema',
  classic: 'Classics',
  horror: 'Horror',
  romance: 'Romance',
  comedy: 'Comedy',
  crime: 'Crime',
  scifi: 'Sci-fi',
  animation: 'Animation',
  drama: 'Drama',
}

export const ALL_FACETS = Object.keys(FACET_LABELS) as Facet[]

export interface QuizFilm {
  id: string
  title: string
  year: number
  director: string
  facets: Facet[]
  /** 0–1: how likely a typical user has seen it */
  popularity: number
}

type Row = [string, number, string, Facet[], number]

const ROWS: Row[] = [
  ['The Dark Knight', 2008, 'Christopher Nolan', ['blockbuster', 'crime'], 0.95],
  ['Inception', 2010, 'Christopher Nolan', ['blockbuster', 'scifi'], 0.95],
  ['Interstellar', 2014, 'Christopher Nolan', ['blockbuster', 'scifi', 'drama'], 0.93],
  ['Titanic', 1997, 'James Cameron', ['blockbuster', 'romance'], 0.93],
  ['3 Idiots', 2009, 'Rajkumar Hirani', ['indian', 'comedy', 'drama'], 0.9],
  ['Dilwale Dulhania Le Jayenge', 1995, 'Aditya Chopra', ['indian', 'romance', 'classic'], 0.85],
  ['Spirited Away', 2001, 'Hayao Miyazaki', ['animation', 'world'], 0.8],
  ['Parasite', 2019, 'Bong Joon-ho', ['world', 'crime', 'arthouse'], 0.82],
  ['Pulp Fiction', 1994, 'Quentin Tarantino', ['crime', 'comedy'], 0.85],
  ['The Godfather', 1972, 'Francis Ford Coppola', ['crime', 'classic', 'drama'], 0.82],
  ['Get Out', 2017, 'Jordan Peele', ['horror'], 0.75],
  ['Hereditary', 2018, 'Ari Aster', ['horror', 'arthouse'], 0.6],
  ['La La Land', 2016, 'Damien Chazelle', ['romance', 'drama'], 0.8],
  ['Before Sunrise', 1995, 'Richard Linklater', ['romance', 'arthouse'], 0.55],
  ['In the Mood for Love', 2000, 'Wong Kar-wai', ['romance', 'world', 'arthouse'], 0.45],
  ['Mad Max: Fury Road', 2015, 'George Miller', ['blockbuster', 'scifi'], 0.78],
  ['Everything Everywhere All at Once', 2022, 'Daniels', ['scifi', 'comedy', 'arthouse'], 0.75],
  ['Gangs of Wasseypur', 2012, 'Anurag Kashyap', ['indian', 'crime'], 0.7],
  ['Dangal', 2016, 'Nitesh Tiwari', ['indian', 'drama'], 0.78],
  ['Zindagi Na Milegi Dobara', 2011, 'Zoya Akhtar', ['indian', 'comedy', 'romance'], 0.78],
  ['Lagaan', 2001, 'Ashutosh Gowariker', ['indian', 'classic', 'drama'], 0.7],
  ['Sholay', 1975, 'Ramesh Sippy', ['indian', 'classic'], 0.65],
  ['Pather Panchali', 1955, 'Satyajit Ray', ['indian', 'world', 'classic', 'arthouse'], 0.3],
  ['Seven Samurai', 1954, 'Akira Kurosawa', ['world', 'classic'], 0.35],
  ['2001: A Space Odyssey', 1968, 'Stanley Kubrick', ['scifi', 'classic', 'arthouse'], 0.55],
  ['Blade Runner 2049', 2017, 'Denis Villeneuve', ['scifi', 'arthouse'], 0.65],
  ['The Shining', 1980, 'Stanley Kubrick', ['horror', 'classic'], 0.65],
  ['The Grand Budapest Hotel', 2014, 'Wes Anderson', ['comedy', 'arthouse'], 0.7],
  ['Superbad', 2007, 'Greg Mottola', ['comedy'], 0.6],
  ['The Notebook', 2004, 'Nick Cassavetes', ['romance'], 0.7],
  ['Toy Story', 1995, 'John Lasseter', ['animation', 'comedy'], 0.9],
  ['Spider-Man: Into the Spider-Verse', 2018, 'Persichetti, Ramsey, Rothman', ['animation', 'blockbuster'], 0.78],
  ['Avengers: Endgame', 2019, 'Russo brothers', ['blockbuster'], 0.9],
  ['Amélie', 2001, 'Jean-Pierre Jeunet', ['world', 'romance', 'comedy'], 0.55],
  ['Portrait of a Lady on Fire', 2019, 'Céline Sciamma', ['world', 'romance', 'arthouse'], 0.4],
  ['Oldboy', 2003, 'Park Chan-wook', ['world', 'crime'], 0.5],
  ['Drive', 2011, 'Nicolas Winding Refn', ['crime', 'arthouse'], 0.6],
  ['Whiplash', 2014, 'Damien Chazelle', ['drama'], 0.75],
  ['Moonlight', 2016, 'Barry Jenkins', ['drama', 'arthouse'], 0.55],
  ['The Social Network', 2010, 'David Fincher', ['drama'], 0.75],
  ['Andhadhun', 2018, 'Sriram Raghavan', ['indian', 'crime', 'comedy'], 0.65],
  ['Tumbbad', 2018, 'Rahi Anil Barve', ['indian', 'horror'], 0.5],
  ['Stree', 2018, 'Amar Kaushik', ['indian', 'horror', 'comedy'], 0.6],
  ['Kabhi Khushi Kabhie Gham', 2001, 'Karan Johar', ['indian', 'romance', 'drama'], 0.8],
  ['RRR', 2022, 'S. S. Rajamouli', ['indian', 'blockbuster'], 0.72],
  ['Baahubali: The Beginning', 2015, 'S. S. Rajamouli', ['indian', 'blockbuster'], 0.7],
  ['Jurassic Park', 1993, 'Steven Spielberg', ['blockbuster', 'scifi', 'classic'], 0.88],
  ['Back to the Future', 1985, 'Robert Zemeckis', ['scifi', 'comedy', 'classic'], 0.8],
  ['Casablanca', 1942, 'Michael Curtiz', ['romance', 'classic'], 0.45],
  ['Psycho', 1960, 'Alfred Hitchcock', ['horror', 'classic'], 0.55],
  ['Midsommar', 2019, 'Ari Aster', ['horror', 'arthouse'], 0.6],
  ['The Conjuring', 2013, 'James Wan', ['horror'], 0.72],
  ['Your Name', 2016, 'Makoto Shinkai', ['animation', 'romance', 'world'], 0.6],
  ['Taare Zameen Par', 2007, 'Aamir Khan', ['indian', 'drama'], 0.75],
  ['The Lunchbox', 2013, 'Ritesh Batra', ['indian', 'romance', 'arthouse'], 0.45],
  ['Mulholland Drive', 2001, 'David Lynch', ['arthouse', 'crime'], 0.4],
  ['Persona', 1966, 'Ingmar Bergman', ['arthouse', 'classic', 'world'], 0.2],
  ['The Hangover', 2009, 'Todd Phillips', ['comedy'], 0.78],
  ['Barbie', 2023, 'Greta Gerwig', ['comedy', 'blockbuster'], 0.75],
  ['Oppenheimer', 2023, 'Christopher Nolan', ['drama', 'blockbuster'], 0.82],
  ['Dune', 2021, 'Denis Villeneuve', ['scifi', 'blockbuster'], 0.75],
  ['Ratatouille', 2007, 'Brad Bird', ['animation', 'comedy'], 0.82],
]

const slug = (s: string) =>
  s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')

export const QUIZ_FILMS: QuizFilm[] = ROWS.map(([title, year, director, facets, popularity]) => ({
  id: `${slug(title)}-${year}`,
  title,
  year,
  director,
  facets,
  popularity,
}))
