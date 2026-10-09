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
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import type { Channel } from '../../types'
import { ChannelCard } from '../channel-card'
import { ChannelsProvider } from '../channels-provider'

function CardFixture(props: { channel: Partial<Channel> }) {
  const table = useReactTable({
    data: [{ id: 1, group: 'default', ...props.channel } as Channel],
    columns: [],
    getCoreRowModel: getCoreRowModel(),
  })
  return <ChannelCard row={table.getRowModel().rows[0]} isSelected={false} />
}

function renderCard(channel: Partial<Channel>) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <ChannelsProvider>
        <CardFixture channel={channel} />
      </ChannelsProvider>
    </QueryClientProvider>
  )
}

describe('ChannelCard', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows the configured ratio next to each group', async () => {
    vi.spyOn(api, 'get').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: url === '/api/group/ratios' ? { default: 1, vip: 1.5 } : {},
      },
    }))
    renderCard({ group: 'default,vip,unknown' })

    expect(await screen.findByText('1.5x')).toBeVisible()
    expect(screen.getByText('1x')).toBeVisible()
    expect(screen.getByText('unknown')).toBeVisible()
    expect(screen.getAllByText(/^[\d.]+x$/)).toHaveLength(2)
  })

  it('links the base URL in a new tab', () => {
    renderCard({ base_url: 'https://api.example.com', models: 'gpt-4o' })

    const link = screen.getByRole('link', { name: 'https://api.example.com' })
    expect(link).toHaveAttribute('href', 'https://api.example.com')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('shows Default when the channel uses the provider default address', () => {
    renderCard({ base_url: '', models: '' })

    expect(screen.queryByRole('link')).toBeNull()
    expect(screen.getByText('Default')).toBeVisible()
  })

  it('lists models inline and every model on hover', async () => {
    const user = userEvent.setup()
    renderCard({ models: 'gpt-4o, claude-sonnet,deepseek-chat' })

    const inline = screen.getByText('gpt-4o, claude-sonnet, deepseek-chat')
    await user.hover(inline)

    expect(await screen.findByText('3 models')).toBeVisible()
  })
})
