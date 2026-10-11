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

import { buildRetryProcess } from '../retry-attempts'

const policy: PolicyEvent[] = [
  {
    attempt: 1,
    channel_id: 45,
    group: 'g',
    elapsed_ms: 4,
    decision: {
      action: 'attempt',
      reason: 'channel_selected',
      source: 'routing',
    },
  },
  {
    attempt: 1,
    channel_id: 45,
    status: 524,
    error_code: 'bad_response_status_code',
    error_source: 'upstream',
    message: 'origin timeout',
    elapsed_ms: 125127,
    decision: {
      action: 'failure',
      reason: 'upstream_failure',
      source: 'upstream',
    },
  },
  {
    attempt: 1,
    channel_id: 45,
    status: 524,
    elapsed_ms: 125127,
    decision: {
      action: 'retry',
      reason: 'network_error_retry',
      source: 'group',
    },
  },
  {
    attempt: 2,
    channel_id: 42,
    group: 'g',
    elapsed_ms: 125130,
    decision: {
      action: 'attempt',
      reason: 'channel_selected',
      source: 'routing',
    },
  },
  {
    attempt: 2,
    channel_id: 42,
    elapsed_ms: 139000,
    decision: {
      action: 'success',
      reason: 'request_completed',
      source: 'upstream',
    },
  },
]
const errorLog = {
  id: 1,
  type: 5,
  channel: 45,
  channel_name: '美西',
  quota: 0,
  use_time: 125,
  other: JSON.stringify({ admin_info: { request_policy: policy.slice(0, 3) } }),
}
const consumeLog = {
  id: 2,
  type: 2,
  channel: 42,
  channel_name: '备用',
  quota: 716,
  use_time: 139,
  other: JSON.stringify({ admin_info: { request_policy: policy } }),
}

describe('buildRetryProcess', () => {
  it('returns null when the request had a single attempt', () => {
    const single = {
      ...consumeLog,
      other: JSON.stringify({
        admin_info: { request_policy: [policy[3], policy[4]] },
      }),
    }
    expect(buildRetryProcess(single, [single])).toBeNull()
  })

  it('lists each attempt with its channel, outcome, duration and charge', () => {
    const process = buildRetryProcess(consumeLog, [errorLog, consumeLog])
    expect(process?.attempts).toEqual([
      expect.objectContaining({
        attempt: 1,
        channelId: 45,
        channelName: '美西',
        succeeded: false,
        status: 524,
        message: 'origin timeout',
        durationMs: 125123,
        quota: 0,
        upstreamMayCharge: true,
      }),
      expect.objectContaining({
        attempt: 2,
        channelId: 42,
        channelName: '备用',
        succeeded: true,
        durationMs: 13870,
        quota: 716,
        upstreamMayCharge: false,
      }),
    ])
  })

  it('sums the outcome from the final record', () => {
    const process = buildRetryProcess(consumeLog, [errorLog, consumeLog])
    expect(process?.summary).toEqual({
      succeeded: true,
      attemptCount: 2,
      totalSeconds: 139,
      totalQuota: 716,
    })
  })

  it('completes an intermediate failure from the final record and points to it', () => {
    const process = buildRetryProcess(errorLog, [errorLog, consumeLog])
    expect(process?.currentAttempt).toBe(1)
    expect(process?.finalLogId).toBe(2)
    expect(process?.summary.succeeded).toBe(true)
  })

  it('reports a final failure with nothing charged', () => {
    const failed = [
      ...policy.slice(0, 3),
      { ...policy[3] },
      {
        attempt: 2,
        channel_id: 42,
        status: 502,
        elapsed_ms: 130000,
        decision: {
          action: 'failure',
          reason: 'upstream_failure',
          source: 'upstream',
        },
      },
      {
        attempt: 2,
        channel_id: 42,
        status: 502,
        elapsed_ms: 130000,
        decision: {
          action: 'stop',
          reason: 'attempt_budget_exhausted',
          source: 'global',
        },
      },
    ]
    const finalError = {
      id: 3,
      type: 5,
      channel: 42,
      channel_name: '备用',
      quota: 0,
      use_time: 130,
      other: JSON.stringify({ admin_info: { request_policy: failed } }),
    }
    const process = buildRetryProcess(finalError, [errorLog, finalError])
    expect(process?.summary).toEqual({
      succeeded: false,
      attemptCount: 2,
      totalSeconds: 130,
      totalQuota: 0,
    })
    expect(process?.finalLogId).toBe(3)
  })

  it('leaves the channel name empty when no sibling names it', () => {
    const process = buildRetryProcess(consumeLog, [consumeLog])
    expect(process?.attempts[0].channelName).toBe('')
  })

  it('marks a billed stream that broke mid-way as succeeded and interrupted', () => {
    const events = [
      policy[0],
      policy[1],
      policy[2],
      policy[3],
      {
        attempt: 2,
        channel_id: 42,
        elapsed_ms: 130000,
        decision: {
          action: 'stop',
          reason: 'stream_not_successful',
          source: 'upstream',
        },
      },
    ]
    const streamLog = {
      ...consumeLog,
      other: JSON.stringify({ admin_info: { request_policy: events } }),
    }
    const process = buildRetryProcess(streamLog, [errorLog, streamLog])
    expect(process?.attempts[1]).toEqual(
      expect.objectContaining({
        succeeded: true,
        streamInterrupted: true,
        quota: 716,
        durationMs: 4870,
      })
    )
    expect(process?.summary.succeeded).toBe(true)
  })

  it('reports a null duration for an attempt without an end event', () => {
    const events = [policy[0], policy[1], policy[2], policy[3]]
    const log = {
      ...consumeLog,
      other: JSON.stringify({ admin_info: { request_policy: events } }),
    }
    expect(buildRetryProcess(log, [log])?.attempts[1].durationMs).toBeNull()
  })

  it('keeps the outcome unknown while the final record is not loaded', () => {
    const longer = {
      ...errorLog,
      id: 9,
      other: JSON.stringify({
        admin_info: { request_policy: policy.slice(0, 4) },
      }),
    }
    const process = buildRetryProcess(longer, [])
    expect(process?.attempts.map((a) => a.attempt)).toEqual([1, 2])
    expect(process?.finalLogId).toBeNull()
    expect(process?.summary).toEqual({
      succeeded: null,
      attemptCount: 2,
      totalSeconds: null,
      totalQuota: null,
    })
  })

  it('returns null for a single-attempt record with nothing else loaded', () => {
    expect(buildRetryProcess(errorLog, [])).toBeNull()
  })

  it('ignores a malformed other field', () => {
    const broken = { ...consumeLog, other: '{not json' }
    expect(buildRetryProcess(broken, [broken])).toBeNull()
  })
})
