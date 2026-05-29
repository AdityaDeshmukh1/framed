import { auth } from '@clerk/nextjs/server'
import { API_BASE_URL } from '../../../lib/constants'

export async function POST(_request: Request): Promise<Response> {
  const { getToken } = await auth()
  const token = await getToken()

  const res = await fetch(`${API_BASE_URL}/match`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  })

  const data = await res.json()
  return Response.json(data, { status: res.status })
}
