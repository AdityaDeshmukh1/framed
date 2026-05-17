/**
 * GET /api/soul/[userId] — fetches or generates the soul portrait for a user.
 * Returns: SoulPortrait object
 */

import { API_BASE_URL } from '../../../../lib/constants'

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ userId: string }> }
): Promise<Response> {
  const { userId } = await params

  const res = await fetch(`${API_BASE_URL}/soul/${userId}`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
