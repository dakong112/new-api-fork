import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore, type AuthBundle } from '@/stores/auth-store'

import { SsoTicket } from '..'

const bundle: AuthBundle = {
  access_token: 'ticket-login',
  token_type: 'Bearer',
  access_expires_at: 9999999999,
  user: { id: 7, username: 'alice', role: 1 },
  session: {
    sid: 'ticket-session',
    current: true,
    login_method: 'sso_ticket',
    ip: '',
    user_agent: '',
    created_at: 1,
    last_active_at: 1,
    expires_at: 9999999999,
  },
}

const preview = {
  data: {
    success: true,
    data: { username: 'alice', display_name: 'Alice', client_name: 'Studio' },
  },
}

function renderTicketPage(hash: string) {
  useAuthStore.getState().auth.reset('complete')
  window.history.replaceState(null, '', `/sso/ticket${hash}`)
  const root = createRootRoute({ component: Outlet })
  const page = (path: string, label: string) =>
    createRoute({
      getParentRoute: () => root,
      path,
      component: () => <div>{label}</div>,
    })
  const router = createRouter({
    routeTree: root.addChildren([
      createRoute({
        getParentRoute: () => root,
        path: '/sso/ticket',
        component: SsoTicket,
      }),
      page('/wallet', 'Wallet destination'),
      page('/pricing', 'Pricing destination'),
      page('/otp', 'Otp page'),
      page('/sign-in', 'Sign-in page'),
    ]),
    history: createMemoryHistory({ initialEntries: ['/sso/ticket'] }),
  })
  const view = render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return { router, ...view }
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset('idle')
})

it('shows an error without calling the server when the link has no ticket', async () => {
  const post = vi.spyOn(api, 'post')
  renderTicketPage('')
  expect(
    await screen.findByText('This sign-in link is invalid or has expired')
  ).toBeVisible()
  expect(post).not.toHaveBeenCalled()
})

it('removes the ticket from the address bar and offers sign-in when the preview fails', async () => {
  const post = vi.spyOn(api, 'post').mockRejectedValue(new Error('400'))
  const { router } = renderTicketPage('#ticket=secret-ticket')
  expect(
    await screen.findByText('This sign-in link is invalid or has expired')
  ).toBeVisible()
  expect(window.location.hash).toBe('')
  expect(post).toHaveBeenCalledWith(
    '/api/sso/ticket/preview',
    { ticket: 'secret-ticket' },
    expect.anything()
  )
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Go to sign in' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/sign-in'))
})

it('signs in and opens the requested in-site page only after confirmation', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValueOnce(preview)
    .mockResolvedValueOnce({ data: { success: true, data: bundle } })
  const { router } = renderTicketPage('#ticket=t1&to=%2Fpricing')
  const confirm = await screen.findByRole('button', { name: 'Confirm sign-in' })
  expect(post).toHaveBeenCalledTimes(1)
  expect(useAuthStore.getState().auth.user).toBeNull()
  await userEvent.setup().click(confirm)
  await waitFor(() => expect(router.state.location.pathname).toBe('/pricing'))
  expect(post).toHaveBeenLastCalledWith(
    '/api/sso/ticket/redeem',
    { ticket: 't1' },
    expect.anything()
  )
  expect(useAuthStore.getState().auth.user?.id).toBe(7)
})

it('ignores an off-site destination and falls back to the wallet', async () => {
  vi.spyOn(api, 'post')
    .mockResolvedValueOnce(preview)
    .mockResolvedValueOnce({ data: { success: true, data: bundle } })
  const { router } = renderTicketPage(
    `#ticket=t1&to=${encodeURIComponent('https://evil.example.com/x')}`
  )
  await userEvent
    .setup()
    .click(await screen.findByRole('button', { name: 'Confirm sign-in' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/wallet'))
})

it('moves to the verification step instead of signing in when a challenge is returned', async () => {
  vi.spyOn(api, 'post')
    .mockResolvedValueOnce(preview)
    .mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          require_verification: true,
          flow_token: 'challenge',
          expires_at: Math.floor(Date.now() / 1000) + 300,
          methods: [{ method: '2fa', available: true }],
        },
      },
    })
  const { router } = renderTicketPage('#ticket=t1')
  await userEvent
    .setup()
    .click(await screen.findByRole('button', { name: 'Confirm sign-in' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/otp'))
  expect(useAuthStore.getState().auth.user).toBeNull()
  expect(useAuthStore.getState().auth.pendingLoginVerification).not.toBeNull()
})
