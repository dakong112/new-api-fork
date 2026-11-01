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
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'

import { Button } from '@/components/ui/button'

import { DataTableToolbar } from '../toolbar'

function Fixture() {
  const table = useReactTable({
    data: [{ status: 'enabled' }],
    columns: [{ accessorKey: 'status' }],
    getCoreRowModel: getCoreRowModel(),
  })
  return (
    <DataTableToolbar
      table={table}
      filters={[
        {
          columnId: 'status',
          title: 'Status',
          options: [{ value: 'enabled', label: 'Enabled' }],
        },
      ]}
      preActions={<Button>Refresh</Button>}
      inlineContent={<p>Contact support</p>}
    />
  )
}

it('renders inline content between the filters and the action buttons', () => {
  render(<Fixture />)

  const notice = screen.getByText('Contact support')
  const filter = screen.getByRole('button', { name: /^Status/ })
  const action = screen.getByRole('button', { name: 'Refresh' })
  expect(filter.compareDocumentPosition(notice)).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING
  )
  expect(notice.compareDocumentPosition(action)).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING
  )
})
