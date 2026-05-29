# Sprint — Beta Release Readiness

## Blockers (must fix before any users touch it)

- [ ] **ML service dependency** — `handleEmbedFilm` calls `http://localhost:8001/embed`; if it's down, the whole pipeline silently stalls at `enriching`. Either self-host the embedding service or document the startup requirement clearly.
- [ ] **Taste vector normalization** — the weighted average of film embeddings produces a near-zero vector when low-weight films cancel high-weight ones. Normalize the final vector (L2 norm) before writing to `taste_vectors`, otherwise cosine similarity scores come out ~1% and matches are meaningless.
- [ ] **Pipeline smoke test** — run one full onboarding end-to-end (Letterboxd handle → `complete` status → soul portrait → match score) and confirm `status` reaches `complete` only after the taste vector row is written.

## MVP gaps (required for a coherent beta experience)

- [ ] **Match like / pass** — `user_a_action` / `user_b_action` columns exist on `matches`; wire up POST `/match/:id/like` and `/match/:id/pass` endpoints + swipe/button UI on the Discover card.
- [ ] **Mutual match notification** — when both users like, surface it somewhere (badge on Discover, or a simple toast).
- [ ] **Re-onboarding** — if a user wants to change their Letterboxd handle, there's no path to do so. Add a "re-run" trigger in profile settings (just re-POST `/onboard` with a new handle).
- [ ] **Empty states** — Discover and Profile show blank screens for new users. Add a friendly prompt to complete onboarding if no portrait/matches exist yet.

## Polish (nice to have before launch)

- [ ] Match score display — `1%` looks broken; show only after normalization fix, or hide until then.
- [ ] Error states on portrait generation — surface a retry button if `/soul` returns non-ok.
- [ ] Loading skeletons on Discover cards while matches are fetching.
- [ ] `@username` on profile falls back to empty string if Clerk has no username — show email prefix instead.
- [ ] Confirm Clerk sign-up flow works on mobile (test on actual device, not just browser DevTools).

## Out of scope for this sprint

- Soul Siblings tab (backend not wired)
- Chat / messaging (backend not wired)
- Film stats ("Films watched", "Obscurity score" cells on Profile)
