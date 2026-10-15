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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { DissolveTeamAction } from '../dissolve-team-action'

const { navigate } = vi.hoisted(() => ({ navigate: vi.fn() }))
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

function mockVerification(proofToken: string) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        scope: 'team.dissolve',
        methods: [{ method: '2fa', available: true }],
        oauth_providers: [],
        password_encryption_enabled: false,
      },
    },
  })
  return vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: {
        proof_token: proofToken,
        method: '2fa',
        scope: 'team.dissolve',
        expires_at: Math.floor(Date.now() / 1000) + 60,
      },
    },
  })
}

function renderAction() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DissolveTeamAction teamId={7} teamName='Studio' balance={500} />
    </QueryClientProvider>
  )
}

async function confirmAndVerify() {
  await userEvent.click(screen.getByRole('button', { name: 'Dissolve team' }))
  const dialog = await screen.findByRole('alertdialog')
  await userEvent.click(
    within(dialog).getByRole('button', { name: 'Dissolve team' })
  )
  await userEvent.type(
    await screen.findByLabelText('Authenticator code or backup code'),
    '123456'
  )
  await userEvent.click(screen.getByRole('button', { name: 'Verify' }))
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  navigate.mockReset()
})

it('dissolving sends the request only with a proof bound to the team, then leaves the page', async () => {
  const post = mockVerification('dissolve-proof')
  const del = vi
    .spyOn(api, 'delete')
    .mockResolvedValue({ data: { success: true, data: { quota: 500 } } })
  renderAction()

  await confirmAndVerify()

  await waitFor(() =>
    expect(del).toHaveBeenCalledWith(
      '/api/team/7',
      expect.objectContaining({
        headers: { 'X-Security-Proof': 'dissolve-proof' },
        singleUseAuthorization: true,
      })
    )
  )
  expect(post).toHaveBeenCalledWith(
    '/api/verify',
    expect.objectContaining({
      scope: 'team.dissolve',
      context: { team_id: 7 },
    }),
    expect.anything()
  )
  await waitFor(() => expect(navigate).toHaveBeenCalledWith({ to: '/team' }))
})

it('a refused dissolve keeps the user on the team page', async () => {
  mockVerification('dissolve-proof')
  vi.spyOn(api, 'delete').mockResolvedValue({
    data: { success: false, message: 'Team not found' },
  })
  renderAction()

  await confirmAndVerify()

  await waitFor(() => expect(api.delete).toHaveBeenCalled())
  expect(navigate).not.toHaveBeenCalled()
})
