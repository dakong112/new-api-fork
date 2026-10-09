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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { DashboardFilters } from '../../types'
import { ActiveFilterChips } from '../active-filter-chips'

const filters: DashboardFilters = {
  time_granularity: 'day',
  model_name: 'gpt-4o',
  group: 'vip',
  token_id: 7,
  token_name: 'my key (#7)',
}

describe('ActiveFilterChips', () => {
  it('renders nothing when no condition is applied', () => {
    const { container } = render(
      <ActiveFilterChips
        filters={{ time_granularity: 'day' }}
        onChange={() => {}}
      />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('removes a single condition and keeps the rest', async () => {
    const onChange = vi.fn()
    render(<ActiveFilterChips filters={filters} onChange={onChange} />)
    expect(screen.getByText('API Key: my key (#7)')).toBeVisible()

    await userEvent.click(
      screen.getByRole('button', { name: 'Remove filter Group: vip' })
    )

    expect(onChange).toHaveBeenCalledWith({ ...filters, group: '' })
  })

  it('clears every condition but keeps the time settings', async () => {
    const onChange = vi.fn()
    render(<ActiveFilterChips filters={filters} onChange={onChange} />)

    await userEvent.click(screen.getByRole('button', { name: 'Clear all' }))

    const next = onChange.mock.calls[0][0] as DashboardFilters
    expect(next.time_granularity).toBe('day')
    expect(next.model_name).toBe('')
    expect(next.group).toBe('')
    expect(next.token_id).toBeUndefined()
  })
})
