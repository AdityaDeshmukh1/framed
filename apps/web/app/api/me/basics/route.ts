import { auth } from '@clerk/nextjs/server'
import { API_BASE_URL } from '../../../../lib/constants'

export async function PUT(request: Request): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()
  const res = await fetch(`${API_BASE_URL}/me/basics`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(await request.json()),
  })
  if (res.status === 204) return new Response(null, { status: 204 })
  return Response.json(await res.json(), { status: res.status })
}
