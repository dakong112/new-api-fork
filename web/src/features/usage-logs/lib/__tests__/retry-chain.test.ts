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

import type { PolicyEvent } from '@/features/system-settings/request-policies/api'

import { describeRetryChain } from '../retry-chain'

const failure = (channel: number, status: number): PolicyEvent => ({
  attempt: 1,
  channel_id: channel,
  status,
  elapsed_ms: 0,
  decision: {
    action: 'failure',
    reason: 'upstream_failure',
    source: 'upstream',
  },
})
const retry = (reason: string): PolicyEvent => ({
  attempt: 1,
  elapsed_ms: 0,
  decision: { action: 'retry', reason, source: 'group' },
})

describe('describeRetryChain', () => {
  it('labels each failed hop with its status and marks the successful hop', () => {
    const chain = describeRetryChain(
      ['1', '3'],
      [failure(1, 524), retry('network_error_retry')],
      true
    )
    expect(chain).toEqual({ label: '#1 (524) → #3 ✓', networkRetry: true })
  })

  it('keeps the final status when every hop failed', () => {
    const chain = describeRetryChain(
      [1, 3],
      [failure(1, 502), retry('network_error_retry'), failure(3, 502)],
      false
    )
    expect(chain.label).toBe('#1 (502) → #3 (502)')
  })

  it('does not flag retries caused by other statuses as network retries', () => {
    const chain = describeRetryChain(
      ['1', '2'],
      [failure(1, 429), retry('retry_status_matched')],
      true
    )
    expect(chain).toEqual({ label: '#1 (429) → #2 ✓', networkRetry: false })
  })

  it('falls back to bare channel ids for logs without policy events', () => {
    expect(describeRetryChain(['1', '2'], undefined, true).label).toBe(
      '#1 → #2 ✓'
    )
  })
})
