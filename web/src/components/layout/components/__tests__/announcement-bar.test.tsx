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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { toPlainSummary } from '../../lib/announcement-bar'
import { AnnouncementBar, HeaderContactLinks } from '../announcement-bar'

const statusMock = vi.hoisted(() => ({
  status: {} as Record<string, unknown>,
}))
const copyMock = vi.hoisted(() => vi.fn(async () => true))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: statusMock.status, loading: false }),
}))
vi.mock('@/lib/copy-to-clipboard', () => ({ copyToClipboard: copyMock }))

function renderBar(notice = '**Maintenance** on [Friday](https://x.y)') {
  const onOpen = vi.fn()
  const view = render(<AnnouncementBar notice={notice} onOpenNotice={onOpen} />)
  return { onOpen, ...view }
}

describe('toPlainSummary', () => {
  it('strips markdown syntax and keeps link text on one line', () => {
    expect(
      toPlainSummary('# Title\n\n**bold** [docs](https://a.b) ![i](x.png)')
    ).toBe('Title bold docs')
  })
})

describe('AnnouncementBar', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders nothing when the system notice is empty', () => {
    const { container } = renderBar('  ')
    expect(container.firstChild).toBeNull()
  })

  it('scrolls the system notice as plain text and opens it on click', () => {
    const { onOpen } = renderBar()

    // Only the first copy is reachable; the loop duplicate is aria-hidden.
    const item = screen.getByRole('button', { name: 'Maintenance on Friday' })
    fireEvent.click(item)
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('stays hidden after close until a different notice is published', () => {
    const view = renderBar('Maintenance on Friday')
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(view.container.firstChild).toBeNull()

    view.unmount()
    expect(renderBar('Maintenance on Friday').container.firstChild).toBeNull()

    const { container } = renderBar('Maintenance moved to Saturday')
    expect(container.firstChild).not.toBeNull()
  })
})

describe('HeaderContactLinks', () => {
  beforeEach(() => {
    statusMock.status = {}
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('opens contacts with a link in a new tab and copies contacts without one', () => {
    statusMock.status = {
      contact_links_enabled: true,
      contact_links: [
        {
          icon: 'qq',
          label: 'QQ Group',
          value: '123456',
          url: 'https://qm.qq.com/q/a',
        },
        { icon: 'email', label: 'Email', value: 'a@b.c' },
      ],
    }
    render(<HeaderContactLinks />)

    const link = screen.getByRole('button', { name: 'QQ Group 123456' })
    expect(link.getAttribute('href')).toBe('https://qm.qq.com/q/a')
    expect(link.getAttribute('target')).toBe('_blank')

    fireEvent.click(screen.getByRole('button', { name: 'Email a@b.c' }))
    expect(copyMock).toHaveBeenCalledWith('a@b.c')
  })

  it('renders nothing when the admin disables contacts', () => {
    statusMock.status = {
      contact_links_enabled: false,
      contact_links: [{ icon: 'qq', label: 'QQ Group', value: '1' }],
    }
    const { container } = render(<HeaderContactLinks />)
    expect(container.firstChild).toBeNull()
  })
})
