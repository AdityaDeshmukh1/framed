"""Offline tuning harness for the quiz engine (lib/quiz/engine.ts).

Simulates three synthetic users against the real film pool and reports, per
kernel/scoring setting: cards shown, AUC of predicted ratings on films not
asked, and precision of the top-6 picks. Mirrors the TS engine minus the
follow-up rule and like-bonus. Usage (needs numpy):

    python apps/web/scripts/quiz-sim.py apps/web/public/quiz/films.json
"""
import json, sys, numpy as np
from math import erf, sqrt, log2
f = json.load(open(sys.argv[1] if len(sys.argv) > 1 else 'films.json'))
E = np.array([x['embedding'] for x in f]); E /= np.linalg.norm(E, axis=1, keepdims=True)
C = E - E.mean(0); C /= np.linalg.norm(C, axis=1, keepdims=True); SIM = C @ C.T
prior = np.clip((np.log10(np.maximum([x['voteCount'] for x in f], 1)) - 2.3) / 2.3, 0.05, 0.92)
N = len(f)
VC = np.array([x['voteCount'] for x in f], float)
def within_lang_prior():
    langs = [x['language'] for x in f]; out = np.zeros(N)
    for l in set(langs):
        idx = [i for i in range(N) if langs[i] == l]
        if len(idx) < 5: out[idx] = prior[idx]; continue
        r = np.argsort(np.argsort(-VC[idx]))  # 0 = most voted in that language
        out[idx] = 0.92 - 0.6 * r / (len(idx) - 1)
    return out
PRIOR_GLOBAL = prior.copy()
LANG = np.array([x['language'] for x in f]); SAME = (LANG[:, None] == LANG[None, :]).astype(float)
YR = np.array([x['year'] for x in f], float); ERA = np.exp(-np.abs(YR[:, None]-YR[None, :])/15)

def persona(name):
    def p1(x):  # Bollywood + Nolan, hates horror, no foreign arthouse
        seen = x['language'] == 'hi' or (x['language'] == 'en' and x['voteCount'] > 8000)
        if x['language'] == 'hi' or 'Christopher Nolan' in x['directors']: r = 4.5
        elif 'Horror' in x['genres']: r = 1.5
        else: r = 3
        return seen, r
    def p2(x):  # cinephile
        seen = x['language'] not in ('hi', 'ta', 'te', 'ml')
        if x['language'] in ('ja', 'ko', 'fr') or ('Drama' in x['genres'] and x['year'] < 1985): r = 4.5
        elif x['language'] == 'en' and ('Action' in x['genres'] or 'Adventure' in x['genres']): r = 1.5
        else: r = 3
        return seen, r
    def p3(x):  # horror + animation
        seen = x['voteCount'] > 3000
        if 'Horror' in x['genres'] or 'Animation' in x['genres']: r = 4.5
        elif 'Romance' in x['genres'] or ('Drama' in x['genres'] and 'Thriller' not in x['genres']): r = 1.5
        else: r = 3
        return seen, r
    return {'bolly': p1, 'cine': p2, 'horror': p3}[name]

def normcdf(z): return 0.5 * (1 + erf(z / sqrt(2)))
def H(p): return 0 if p <= 0 or p >= 1 else -(p*log2(p) + (1-p)*log2(1-p))

def run(pname, L, SIG, BIAS, LIN, ML=0.0, ME=0.0, MIX=0.0, SEENLANG=0.0, WC=0.6, PEXP=1.5, NFAV=2, NOVEL=0.0, NOISE=0.45, target=0.62, maxr=35, verbose=False, mode='gp'):
    truth = [persona(pname)(x) for x in f]
    prior = (1-MIX)*PRIOR_GLOBAL + MIX*within_lang_prior()
    K = lambda a, b: SIG*np.exp((SIM[a, b]-1)/L) + LIN*SIM[a, b] + BIAS
    PV = SIG + LIN + BIAS + ML + ME
    KM = lambda a, b: ML*SAME[a, b] + ME*ERA[a, b]
    obs, y, shown, seenv = [], [], [], []
    favs = [i for i in np.argsort(-prior) if truth[i][0] and truth[i][1] == 4.5][:NFAV]
    for i in favs: obs.append(i); y.append(2.0); shown.append(i); seenv.append(1)
    noise = [0.15]*len(favs)
    used = set(favs); log = []
    while True:
        n = len(obs); o = np.array(obs, dtype=int)
        if n == 0:
            mu = np.zeros(N); var = np.full(N, PV)
        else:
          Kn = SIG*np.exp((SIM[np.ix_(o, o)]-1)/L) + LIN*SIM[np.ix_(o, o)] + BIAS + KM(*np.ix_(o, o)) + np.diag(noise)
          Lc = np.linalg.cholesky(Kn); alpha = np.linalg.solve(Kn, np.array(y))
          kx = SIG*np.exp((SIM[:, o]-1)/L) + LIN*SIM[:, o] + BIAS + KM(slice(None), o)
          mu = kx @ alpha; v = np.linalg.solve(Lc, kx.T); var = np.maximum(PV - (v*v).sum(0), 1e-6)
        conf = 1 - (prior * var/PV).sum()/prior.sum()
        rated = len(obs) - len(favs)
        if rated >= maxr or len(shown) - len(favs) >= 60 or (rated >= 8 and conf >= target): break
        sh = np.array(shown, dtype=int); w = np.exp((SIM[:, sh]-1)/0.3) * (1 + SEENLANG*SAME[:, sh])
        pseen = (w @ np.array(seenv) + 1.5*prior) / (w.sum(1) + 1.5)
        plike = np.array([normcdf((m-0.5)/sqrt(vv+NOISE)) for m, vv in zip(mu, var)])
        unc = WC*np.array([H(p) for p in plike]) + (1-WC)*np.sqrt(var/PV)
        asked = set(LANG[sh])
        novel = np.array([0.0 if l in asked else NOVEL for l in LANG])
        score = pseen**PEXP * (unc + novel)
        if mode == 'random': score = prior * np.random.default_rng(len(shown)).random(N)
        score[list(used)] = -1
        i = int(np.argmax(score)); used.add(i); shown.append(i)
        s, r = truth[i]; seenv.append(1 if s else 0)
        if s: obs.append(i); y.append(r-3); noise.append(NOISE)
        if verbose: log.append(f"{len(shown)-len(favs):>2} {'seen' if s else '----'} {r if s else '':>3} {f[i]['title'][:34]:34} {f[i]['language']} plike={plike[i]:.2f} pseen={pseen[i]:.2f}")
    # evaluate on films the persona has seen and we didn't ask about
    ev = [i for i in range(N) if i not in used and truth[i][0]]
    tr = np.array([truth[i][1] for i in ev]); m = mu[ev]
    pos, neg = m[tr == 4.5], m[tr < 4.5]
    auc = (pos[:, None] > neg[None, :]).mean() if len(pos) and len(neg) else float('nan')
    top = np.array(ev)[np.argsort(-m)[:6]]
    prec = np.mean([truth[i][1] == 4.5 for i in top])
    if verbose: print('\n'.join(log))
    return dict(cards=len(shown)-len(favs), rated=len(obs)-len(favs), auc=round(float(auc), 3), top6=round(float(prec), 2), conf=round(float(conf), 2))

if __name__ == '__main__':
    base = dict(L=0.25, SIG=0.8, BIAS=0.3, MIX=0.5, SEENLANG=3, WC=0.3, PEXP=1.0, ME=0.3, target=0.35)
    for LIN, ML, NOVEL in [(3.0,1.0,0.0),(3.0,1.0,0.3),(3.0,1.0,0.6),(1.5,1.5,0.3),(1.5,1.5,0.6),(3.0,2.0,0.4)]:
        for nf in (2, 0):
            r = {p: run(p, LIN=LIN, ML=ML, NOVEL=NOVEL, NFAV=nf, **base) for p in ['bolly', 'cine', 'horror']}
            print(f"lin={LIN} lang={ML} novel={NOVEL} favs={nf}", {p: (v['cards'], v['auc'], v['top6']) for p, v in r.items()})
