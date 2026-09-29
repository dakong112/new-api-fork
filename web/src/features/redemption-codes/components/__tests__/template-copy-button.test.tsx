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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import type { Redemption } from '../../types'
import { RedemptionTemplateCopyButton } from '../redemption-template-copy-button'

const redemption: Redemption = {
  id: 1,
  user_id: 1,
  name: 'promo',
  key: 'abcd1234abcd1234abcd1234abcd1234',
  status: 1,
  quota: 500000,
  created_time: 0,
  redeemed_time: 0,
  expired_time: 0,
  used_user_id: 0,
}

function renderButton(template: string) {
  localStorage.clear()
  vi.spyOn(api, 'get').mockImplementation(async () => ({
    data: { success: true, data: { redemption_copy_template: template } },
  }))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <RedemptionTemplateCopyButton redemption={redemption} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

describe('RedemptionTemplateCopyButton', () => {
  it('stays hidden when no template is configured', async () => {
    renderButton('   ')

    await waitFor(() => expect(api.get).toHaveBeenCalled())
    expect(
      screen.queryByRole('button', { name: 'Copy with template' })
    ).toBeNull()
  })

  it('copies the code filled into the template', async () => {
    const user = userEvent.setup()
    renderButton('Code: {code} / {name}')

    await user.click(
      await screen.findByRole('button', { name: 'Copy with template' })
    )

    expect(await navigator.clipboard.readText()).toBe(
      'Code: abcd1234abcd1234abcd1234abcd1234 / promo'
    )
  })
})
