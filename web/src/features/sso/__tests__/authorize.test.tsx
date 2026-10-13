/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { createSSOCode } from '@/features/sso/api'
import { SSOAuthorizePage } from '@/routes/_authenticated/sso/authorize'

vi.mock('@/features/sso/api', () => ({ createSSOCode: vi.fn() }))
vi.mock('@/lib/handle-server-error', () => ({ handleServerError: vi.fn() }))

const FAILED = 'Application authorization failed'

function stubLocation(search: string) {
  const location = { search, href: `http://localhost/sso/authorize${search}` }
  vi.stubGlobal('location', location)
  return location
}

beforeEach(() => {
  vi.mocked(createSSOCode).mockReset()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('SSO authorize page', () => {
  it('shows failure without calling the API when state is missing', async () => {
    stubLocation('?client_id=app&redirect_uri=https%3A%2F%2Fa.example%2Fcb')

    render(<SSOAuthorizePage />)

    expect(await screen.findByText(FAILED)).toBeInTheDocument()
    expect(createSSOCode).not.toHaveBeenCalled()
  })

  it('does not navigate to a non-http redirect URL returned by the API', async () => {
    const location = stubLocation('?client_id=app&redirect_uri=x&state=s')
    vi.mocked(createSSOCode).mockResolvedValue('javascript:alert(1)')

    render(<SSOAuthorizePage />)

    expect(await screen.findByText(FAILED)).toBeInTheDocument()
    expect(location.href).not.toContain('javascript:')
  })

  it('requests one code with the query params and navigates to the returned URL', async () => {
    const location = stubLocation(
      '?client_id=app&redirect_uri=https%3A%2F%2Fa.example%2Fcb&state=1234e5'
    )
    vi.mocked(createSSOCode).mockResolvedValue('https://a.example/cb?code=c')

    render(<SSOAuthorizePage />)

    await waitFor(() =>
      expect(location.href).toBe('https://a.example/cb?code=c')
    )
    expect(createSSOCode).toHaveBeenCalledTimes(1)
    expect(createSSOCode).toHaveBeenCalledWith({
      client_id: 'app',
      redirect_uri: 'https://a.example/cb',
      state: '1234e5',
    })
  })
})
