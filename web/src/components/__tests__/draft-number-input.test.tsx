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
import { useState } from 'react'
import { describe, expect, it } from 'vitest'

import { DraftNumberInput } from '../draft-number-input'

function Fixture(props: {
  initial: number
  emptyValue?: number
  integer?: boolean
}) {
  const [value, setValue] = useState(props.initial)
  return (
    <>
      <DraftNumberInput
        aria-label='amount'
        value={value}
        onValueChange={setValue}
        emptyValue={props.emptyValue}
        integer={props.integer}
      />
      <output aria-label='form value'>{value}</output>
    </>
  )
}

describe('DraftNumberInput', () => {
  it('stays empty after clearing so a new number can be typed', async () => {
    render(<Fixture initial={13} />)
    const input = screen.getByLabelText('amount')

    await userEvent.clear(input)
    expect(input).toHaveProperty('value', '')

    await userEvent.type(input, '5')
    expect(input).toHaveProperty('value', '5')
    expect(screen.getByLabelText('form value').textContent).toBe('5')
  })

  it('restores the empty value on blur when left blank', async () => {
    render(<Fixture initial={13} emptyValue={1} />)
    const input = screen.getByLabelText('amount')

    await userEvent.clear(input)
    expect(screen.getByLabelText('form value').textContent).toBe('1')
    await userEvent.tab()

    expect(input).toHaveProperty('value', '1')
  })

  it('drops the fraction for integer fields', async () => {
    render(<Fixture initial={0} integer />)
    const input = screen.getByLabelText('amount')

    await userEvent.type(input, '2.7')
    await userEvent.tab()

    expect(screen.getByLabelText('form value').textContent).toBe('2')
    expect(input).toHaveProperty('value', '2')
  })

  it('shows a value that arrives later, like a loaded record', () => {
    const view = render(
      <DraftNumberInput
        aria-label='amount'
        value={10}
        onValueChange={() => {}}
      />
    )

    view.rerender(
      <DraftNumberInput
        aria-label='amount'
        value={1}
        onValueChange={() => {}}
      />
    )

    expect(screen.getByLabelText('amount')).toHaveProperty('value', '1')
  })
})
