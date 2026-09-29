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
import { describe, expect, it } from 'vitest'

import { fillRedemptionTemplate } from '../utils'

const values = {
  code: 'abcd1234',
  name: 'promo',
  quota: '$2.00',
  expires: 'Never',
}

describe('fillRedemptionTemplate', () => {
  it('fills every placeholder, including repeats, across lines', () => {
    const template =
      '兑换码：{code}\n面额：{quota}\n有效期：{expires}\n{name} / {code}'

    expect(fillRedemptionTemplate(template, values)).toBe(
      '兑换码：abcd1234\n面额：$2.00\n有效期：Never\npromo / abcd1234'
    )
  })

  it('leaves unknown placeholders as typed', () => {
    expect(fillRedemptionTemplate('{code} {site}', values)).toBe(
      'abcd1234 {site}'
    )
  })
})
