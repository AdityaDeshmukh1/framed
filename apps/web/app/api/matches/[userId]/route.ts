/**
 * GET /api/matches/[userId] — fetches stored matches for a user with other-user info.
 * Returns: { matches: MatchListItem[], count: number }
 */

import { API_BASE_URL } from '../../../../lib/constants'

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ userId: string }> }
): Promise<Response> {
  const { userId } = await params

  const res = await fetch(`${API_BASE_URL}/matches/${userId}`)
  const data = await res.json()
  return Response.json(data, { status: res.status })
}
