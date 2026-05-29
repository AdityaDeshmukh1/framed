import { auth } from '@clerk/nextjs/server'
import { API_BASE_URL } from '../../../lib/constants'

export async function POST(request: Request): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()

  const body = await request.json()
  const res = await fetch(`${API_BASE_URL}/onboard`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify(body),
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
