/**
 * GET  /api/soul/[userId] — fetch existing soul portrait (404 if not yet generated).
 * POST /api/soul/[userId] — generate + store portrait via Claude (takes ~5s).
 */

import { API_BASE_URL } from '../../../../lib/constants'

type RouteContext = { params: Promise<{ userId: string }> }

export async function GET(_request: Request, { params }: RouteContext): Promise<Response> {
  const { userId } = await params
  const res = await fetch(`${API_BASE_URL}/soul/${userId}`)
  const data = await res.json()
  return Response.json(data, { status: res.status })
}

export async function POST(_request: Request, { params }: RouteContext): Promise<Response> {
  const { userId } = await params
  const res = await fetch(`${API_BASE_URL}/soul/${userId}`, { method: 'POST' })
  const data = await res.json()
  return Response.json(data, { status: res.status })
}
