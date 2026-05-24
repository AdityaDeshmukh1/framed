/**
 * GET /api/match/[matchId]?user_id=... — fetches full match detail.
 * user_id query param identifies the requester (to orient "your" vs "their" ratings).
 * Returns: MatchDetail object with shared_films and disagreement_films.
 */

import { API_BASE_URL } from '../../../../lib/constants'

export async function GET(
  request: Request,
  { params }: { params: Promise<{ matchId: string }> }
): Promise<Response> {
  const { matchId } = await params
  const { searchParams } = new URL(request.url)
  const userId = searchParams.get('user_id') ?? ''

  const res = await fetch(`${API_BASE_URL}/match/${matchId}?user_id=${userId}`)
  const data = await res.json()
  return Response.json(data, { status: res.status })
}
