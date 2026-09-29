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
import {
  useState,
  type ChangeEvent,
  type FocusEvent,
  type InputHTMLAttributes,
  type MouseEvent as ReactMouseEvent,
} from 'react'

import { Input } from '@/components/ui/input'

function formatNumberDraft(value: number | string | undefined): string {
  if (value === '' || value === undefined) return ''
  if (typeof value === 'number') {
    return Number.isFinite(value) ? String(value) : '0'
  }
  return value
}

function parseNumberDraft(
  value: string,
  emptyValue: number,
  integer: boolean
): number {
  if (value.trim() === '') return emptyValue
  const next = Number(value)
  if (!Number.isFinite(next)) return emptyValue
  return integer ? Math.trunc(next) : next
}

function isZeroDraft(value: string): boolean {
  return value.trim() !== '' && Number(value) === 0
}

type DraftNumberInputProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'type' | 'value' | 'onChange'
> & {
  value: number | string | undefined
  onValueChange: (next: number) => void
  selectZeroOnFocus?: boolean
  /** Value reported while the field is empty, and restored on blur. */
  emptyValue?: number
  /** Drop the fractional part, like parseInt. */
  integer?: boolean
}

/**
 * Number input that keeps the typed text while focused, so clearing the field
 * shows it empty instead of snapping back to 0. Use it instead of an Input
 * whose onChange coerces `''` to a number.
 */
export function DraftNumberInput({
  value,
  onValueChange,
  selectZeroOnFocus = true,
  emptyValue = 0,
  integer = false,
  onBlur,
  onFocus,
  onMouseUp,
  ...props
}: DraftNumberInputProps) {
  // The draft only exists while editing; otherwise the field shows the form
  // value directly, so async resets (loaded records) appear immediately.
  const [draft, setDraft] = useState<string | null>(null)

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextDraft = event.target.value
    setDraft(nextDraft)
    onValueChange(parseNumberDraft(nextDraft, emptyValue, integer))
  }

  const handleFocus = (event: FocusEvent<HTMLInputElement>) => {
    setDraft(formatNumberDraft(value))
    onFocus?.(event)
    if (selectZeroOnFocus && isZeroDraft(event.currentTarget.value)) {
      event.currentTarget.select()
    }
  }

  const handleMouseUp = (event: ReactMouseEvent<HTMLInputElement>) => {
    onMouseUp?.(event)
    if (selectZeroOnFocus && isZeroDraft(event.currentTarget.value)) {
      event.preventDefault()
      event.currentTarget.select()
    }
  }

  const handleBlur = (event: FocusEvent<HTMLInputElement>) => {
    const normalized = parseNumberDraft(
      event.currentTarget.value,
      emptyValue,
      integer
    )
    setDraft(null)
    onValueChange(normalized)
    onBlur?.(event)
  }

  return (
    <Input
      {...props}
      type='number'
      value={draft ?? formatNumberDraft(value)}
      onChange={handleChange}
      onFocus={handleFocus}
      onMouseUp={handleMouseUp}
      onBlur={handleBlur}
    />
  )
}
