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

import { GroupVendorFilter } from '../group-vendor-filter'

const vendors = [{ name: 'DeepSeek', icon: 'DeepSeek.Color' }, { name: '' }]

function setup(value: string | null = null) {
  const onChange = vi.fn()
  render(
    <GroupVendorFilter vendors={vendors} value={value} onChange={onChange} />
  )
  return { onChange, user: userEvent.setup() }
}

describe('GroupVendorFilter', () => {
  it('reports the vendor name when a vendor is clicked', async () => {
    const { onChange, user } = setup()
    await user.click(screen.getByRole('button', { name: 'DeepSeek' }))
    expect(onChange).toHaveBeenCalledWith('DeepSeek')
  })

  it('reports an empty name when Other is clicked', async () => {
    const { onChange, user } = setup()
    await user.click(screen.getByRole('button', { name: 'Other' }))
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('reports null when All is clicked after a vendor was selected', async () => {
    const { onChange, user } = setup('DeepSeek')
    await user.click(screen.getByRole('button', { name: 'All' }))
    expect(onChange).toHaveBeenCalledWith(null)
  })

  it('does not report anything when the selected item is clicked again', async () => {
    const { onChange, user } = setup('DeepSeek')
    await user.click(screen.getByRole('button', { name: 'DeepSeek' }))
    expect(onChange).not.toHaveBeenCalled()
  })

  it('marks only the selected vendor as pressed', () => {
    setup('DeepSeek')
    expect(screen.getByRole('button', { name: 'DeepSeek' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('moves focus to the next item with the right arrow key', async () => {
    const { user } = setup()
    screen.getByRole('button', { name: 'All' }).focus()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('button', { name: 'DeepSeek' })).toHaveFocus()
  })

  it('renders nothing when fewer than two vendors exist', () => {
    const { container } = render(
      <GroupVendorFilter
        vendors={[vendors[0]]}
        value={null}
        onChange={vi.fn()}
      />
    )
    expect(container).toBeEmptyDOMElement()
  })
})
