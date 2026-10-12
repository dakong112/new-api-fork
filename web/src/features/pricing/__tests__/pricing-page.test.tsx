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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { Pricing } from '../index'

vi.mock('@/components/layout', () => ({
  PublicLayout: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
}))
const search = vi.hoisted(() => ({ value: {} as Record<string, string> }))
vi.mock('@tanstack/react-router', () => ({ useSearch: () => search.value }))
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

const pricing = {
  success: true,
  data: [
    {
      id: 1,
      model_name: 'deepseek-v4',
      vendor_id: 1,
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['ds'],
    },
    {
      id: 2,
      model_name: 'glm-5',
      vendor_id: 2,
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['zp'],
    },
  ],
  vendors: [
    { id: 1, name: 'DeepSeek' },
    { id: 2, name: 'Zhipu' },
  ],
  group_ratio: { ds: 1, zp: 1 },
  usable_group: {
    ds: { desc: 'ds', ratio: 1 },
    zp: { desc: 'zp', ratio: 1 },
  },
  supported_endpoint: {},
  auto_groups: [],
}

function renderPricing() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(['pricing'], pricing, { updatedAt: freshAt })
  queryClient.setQueryData(['status'], {}, { updatedAt: freshAt })
  render(
    <QueryClientProvider client={queryClient}>
      <Pricing />
    </QueryClientProvider>
  )
}

afterEach(() => {
  search.value = {}
  vi.restoreAllMocks()
})

describe('pricing page', () => {
  it('narrows the group filter to groups with the selected vendor', async () => {
    // Anything not seeded in the query cache must not reach the network.
    vi.spyOn(api, 'get').mockRejectedValue(new Error('unexpected request'))
    const user = userEvent.setup()
    renderPricing()
    const sidebar = screen.getByRole('complementary')
    expect(
      within(sidebar).getByRole('button', { name: /^zp/ })
    ).toBeInTheDocument()

    const vendorSection = within(sidebar)
      .getByRole('button', { name: 'All Vendors', expanded: true })
      .closest<HTMLElement>('[data-slot="collapsible"]')
    expect(vendorSection).not.toBeNull()
    await user.click(
      within(vendorSection as HTMLElement).getByRole('button', {
        name: /DeepSeek/,
      })
    )

    expect(
      within(sidebar).getByRole('button', { name: /^ds/ })
    ).toBeInTheDocument()
    expect(screen.queryAllByRole('button', { name: /^zp/ })).toHaveLength(0)
  })

  it('ignores an unknown vendor from the URL when listing groups', () => {
    vi.spyOn(api, 'get').mockRejectedValue(new Error('unexpected request'))
    search.value = { vendor: 'Gone' }
    renderPricing()
    const sidebar = screen.getByRole('complementary')

    expect(
      within(sidebar).getByRole('button', { name: /^ds/ })
    ).toBeInTheDocument()
    expect(
      within(sidebar).getByRole('button', { name: /^zp/ })
    ).toBeInTheDocument()
  })
})
