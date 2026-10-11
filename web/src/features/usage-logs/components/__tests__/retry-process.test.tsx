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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import type { PolicyEvent } from '@/features/system-settings/request-policies/api'
import { formatLogQuota } from '@/lib/format'

import type { RetryAttempt, RetryProcess } from '../../lib/retry-attempts'
import { RetryProcessView } from '../dialogs/retry-process'

const timeout: RetryAttempt = {
  attempt: 1,
  channelId: 45,
  channelName: '美西',
  group: 'g',
  succeeded: false,
  status: 524,
  errorCode: 'bad_response_status_code',
  message: 'origin timeout',
  durationMs: 125123,
  quota: 0,
  upstreamMayCharge: true,
}
const success: RetryAttempt = {
  attempt: 2,
  channelId: 42,
  channelName: '备用',
  group: 'g',
  succeeded: true,
  durationMs: 13870,
  quota: 716,
  upstreamMayCharge: false,
}
const succeeded: RetryProcess = {
  attempts: [timeout, success],
  summary: {
    succeeded: true,
    attemptCount: 2,
    totalSeconds: 139,
    totalQuota: 716,
  },
  currentAttempt: null,
  finalLogId: 2,
  events: [],
}
const events: PolicyEvent[] = [
  {
    attempt: 1,
    channel_id: 45,
    elapsed_ms: 4,
    decision: {
      action: 'attempt',
      reason: 'channel_selected',
      source: 'routing',
    },
  },
]

function renderProcess(process: RetryProcess, onViewLog = vi.fn()) {
  render(
    <RetryProcessView process={process} events={events} onViewLog={onViewLog} />
  )
  return onViewLog
}

describe('retry process', () => {
  test('succeeded request shows the success outcome', () => {
    renderProcess(succeeded)

    expect(screen.getByRole('status')).toHaveTextContent('Succeeded in the end')
  })

  test('summary shows the attempt count and total time', () => {
    renderProcess(succeeded)

    const summary = screen.getByRole('status')
    expect(summary).toHaveTextContent('2 attempts')
    expect(summary).toHaveTextContent('Total time 2m 19s')
  })

  test('succeeded request shows the charged amount', () => {
    renderProcess(succeeded)

    expect(screen.getByRole('status')).toHaveTextContent(
      `Charged ${formatLogQuota(716)}`
    )
  })

  test('failed attempt shows its channel and raw status', () => {
    renderProcess(succeeded)

    const first = screen.getAllByRole('listitem')[0]
    expect(first).toHaveTextContent('#45 美西')
    expect(first).toHaveTextContent('524 bad_response_status_code')
  })

  test('status 524 is labelled as an upstream timeout', () => {
    renderProcess(succeeded)

    expect(screen.getAllByRole('listitem')[0]).toHaveTextContent(
      'Upstream timeout'
    )
  })

  test('status 504 is labelled as a gateway timeout', () => {
    renderProcess({ ...succeeded, attempts: [{ ...timeout, status: 504 }] })

    expect(screen.getByRole('listitem')).toHaveTextContent('Gateway timeout')
  })

  test('other 5xx status is labelled as an upstream error', () => {
    renderProcess({ ...succeeded, attempts: [{ ...timeout, status: 502 }] })

    expect(screen.getByRole('listitem')).toHaveTextContent('Upstream error')
  })

  test('4xx status is labelled as a rejected request', () => {
    renderProcess({ ...succeeded, attempts: [{ ...timeout, status: 429 }] })

    expect(screen.getByRole('listitem')).toHaveTextContent('Request rejected')
  })

  test('timeout attempt warns that the upstream may have billed it', () => {
    renderProcess(succeeded)

    expect(screen.getAllByRole('listitem')[0]).toHaveTextContent(
      'Timed out after reaching the upstream, which may have billed it'
    )
  })

  test('succeeded attempt shows its channel and success', () => {
    renderProcess(succeeded)

    const second = screen.getAllByRole('listitem')[1]
    expect(second).toHaveTextContent('#42 备用')
    expect(second).toHaveTextContent('Success')
  })

  test('very long channel name is rendered in full', () => {
    const longName = 'x'.repeat(200)
    renderProcess({
      ...succeeded,
      attempts: [{ ...timeout, channelName: longName }],
    })

    expect(screen.getByText(`#45 ${longName}`)).toBeInTheDocument()
  })

  test('error summary starts collapsed', () => {
    renderProcess(succeeded)

    expect(
      within(screen.getAllByRole('listitem')[0]).getByRole('button', {
        name: 'Expand',
      })
    ).toHaveAttribute('aria-expanded', 'false')
  })

  test('clicking the error summary toggle shows the full message', async () => {
    renderProcess(succeeded)
    const item = screen.getAllByRole('listitem')[0]

    await userEvent.click(within(item).getByRole('button', { name: 'Expand' }))

    expect(
      within(item).getByRole('button', { name: 'Collapse' })
    ).toHaveAttribute('aria-expanded', 'true')
    expect(within(item).getByText('origin timeout')).not.toHaveClass('truncate')
  })

  test('intermediate record shows which channel then succeeded', () => {
    renderProcess({ ...succeeded, currentAttempt: 1 })

    expect(
      screen.getByText(
        'This is attempt 1 of the request. It then succeeded on #42.'
      )
    ).toBeInTheDocument()
  })

  test('intermediate record without a succeeded attempt omits the channel', () => {
    renderProcess({ ...succeeded, attempts: [timeout], currentAttempt: 1 })

    expect(
      screen.getByText('This is attempt 1 of the request. It then succeeded.')
    ).toBeInTheDocument()
  })

  test('intermediate record of a failed request says it failed after retries', () => {
    renderProcess({
      ...succeeded,
      summary: { ...succeeded.summary, succeeded: false },
      currentAttempt: 1,
    })

    expect(
      screen.getByText(
        'This is attempt 1 of the request. It failed after all retries.'
      )
    ).toBeInTheDocument()
  })

  test('final record shows no banner', () => {
    renderProcess(succeeded)

    expect(
      screen.queryByRole('button', { name: 'View final record' })
    ).toBeNull()
  })

  test('opened attempt is marked as the current step', () => {
    renderProcess({ ...succeeded, currentAttempt: 1 })

    expect(screen.getAllByRole('listitem')[0]).toHaveAttribute(
      'aria-current',
      'step'
    )
  })

  test('view final record button opens the final log', async () => {
    const onViewLog = renderProcess({ ...succeeded, currentAttempt: 1 })

    await userEvent.click(
      screen.getByRole('button', { name: 'View final record' })
    )

    expect(onViewLog).toHaveBeenCalledWith(2)
  })

  test('failed request shows the failure and that nothing was charged', () => {
    renderProcess({
      ...succeeded,
      attempts: [timeout, { ...timeout, attempt: 2 }],
      summary: { ...succeeded.summary, succeeded: false, totalQuota: 0 },
    })

    const summary = screen.getByRole('status')
    expect(summary).toHaveTextContent('Failed in the end')
    expect(summary).toHaveTextContent('Not charged (fully refunded)')
  })

  test('routing decision details start collapsed', () => {
    renderProcess(succeeded)

    expect(
      screen.getByRole('button', { name: 'Routing decision details' })
    ).toHaveAttribute('aria-expanded', 'false')
  })

  test('clicking routing decision details shows the decision record', async () => {
    renderProcess(succeeded)

    await userEvent.click(
      screen.getByRole('button', { name: 'Routing decision details' })
    )

    expect(
      screen.getByRole('list', { name: 'Request decision flow' })
    ).toBeInTheDocument()
  })

  test('unknown final outcome shows that the result is unknown', () => {
    renderProcess({
      ...succeeded,
      summary: {
        succeeded: null,
        attemptCount: 2,
        totalSeconds: null,
        totalQuota: null,
      },
      finalLogId: null,
    })

    expect(screen.getByRole('status')).toHaveTextContent('Final result unknown')
  })

  test('intermediate record without a final record says it was not found', () => {
    renderProcess({ ...succeeded, currentAttempt: 1, finalLogId: null })

    expect(
      screen.getByText('The final record of this request was not found')
    ).toBeInTheDocument()
  })

  test('stream interrupted attempt shows the partial charge note', () => {
    renderProcess({
      ...succeeded,
      attempts: [timeout, { ...success, streamInterrupted: true }],
    })

    expect(screen.getAllByRole('listitem')[1]).toHaveTextContent(
      'Stream interrupted midway; charged for the output delivered'
    )
  })

  test('attempt without a known duration shows a dash', () => {
    renderProcess({
      ...succeeded,
      attempts: [{ ...timeout, durationMs: null }, success],
    })

    expect(screen.getAllByRole('listitem')[0]).toHaveTextContent('Duration —')
  })
})
