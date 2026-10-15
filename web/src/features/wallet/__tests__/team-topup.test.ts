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
import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { requestPayment } from '../api'
import { usePayment } from '../hooks/use-payment'

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  requestPayment: vi.fn(async () => ({ success: false, message: 'stop' })),
}))

afterEach(() => {
  vi.mocked(requestPayment).mockClear()
})

describe('team top-up', () => {
  it('an epay request for a team carries its team_id', async () => {
    const { result } = renderHook(() => usePayment(7))
    await act(() => result.current.processPayment(10, 'alipay'))
    expect(requestPayment).toHaveBeenCalledWith({
      amount: 10,
      payment_method: 'alipay',
      team_id: 7,
    })
  })

  it('a personal epay request carries no team', async () => {
    const { result } = renderHook(() => usePayment())
    await act(() => result.current.processPayment(10, 'alipay'))
    expect(vi.mocked(requestPayment).mock.calls[0][0].team_id).toBeUndefined()
  })
})
