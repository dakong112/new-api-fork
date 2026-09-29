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
import { expect, test, vi } from 'vitest'

import { RedemptionCard } from '../redemption-card'

const SHOP = 'https://shop.example.com/x'

function renderCard(overrides: Partial<Parameters<typeof RedemptionCard>[0]>) {
  const props = {
    enabled: true,
    purchaseLink: SHOP,
    code: '',
    onCodeChange: vi.fn(),
    onRedeem: vi.fn(),
    redeeming: false,
    ...overrides,
  }
  render(<RedemptionCard {...props} />)
  return props
}

test('purchase link set: buy button opens the shop in a new tab', () => {
  renderCard({})

  const buy = screen.getByRole('button', { name: /Buy Redemption Code/ })
  expect(buy.getAttribute('href')).toBe(SHOP)
  expect(buy.getAttribute('target')).toBe('_blank')
  expect(buy.getAttribute('rel')).toContain('noopener')
})

test('no purchase link: only the redeem form is shown', () => {
  renderCard({ purchaseLink: undefined })

  expect(screen.queryByText('Buy Redemption Code')).toBeNull()
  expect(screen.getByLabelText('Enter your code to redeem')).toBeTruthy()
})

test('pressing Enter in the code field triggers redeem', async () => {
  const props = renderCard({ code: 'ABC' })

  await userEvent.type(screen.getByRole('textbox'), '{Enter}')

  expect(props.onRedeem).toHaveBeenCalledTimes(1)
})

test('redemption disabled: shows compliance notice instead of the form', () => {
  renderCard({ enabled: false })

  expect(screen.queryByRole('textbox')).toBeNull()
  expect(screen.queryByText('Buy Redemption Code')).toBeNull()
})
