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

describe('GroupVendorFilter', () => {
  it('selects a vendor and reports it, with All clearing to null', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <GroupVendorFilter vendors={vendors} value={null} onChange={onChange} />
    )
    expect(screen.getByRole('radio', { name: 'All' })).toBeChecked()
    await user.click(screen.getByRole('radio', { name: 'DeepSeek' }))
    expect(onChange).toHaveBeenLastCalledWith('DeepSeek')
    await user.click(screen.getByRole('radio', { name: 'Other' }))
    expect(onChange).toHaveBeenLastCalledWith('')
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
