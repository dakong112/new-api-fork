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
import type { PolicyEvent } from '@/features/system-settings/request-policies/api'

/** Fields of a usage log row this module reads. */
export type RetryLogSource = {
  id: number
  type: number
  channel: number
  channel_name?: string | null
  quota: number
  use_time: number
  other: string
}

export type RetryAttempt = {
  attempt: number
  channelId: number
  channelName: string
  group: string
  succeeded: boolean
  status?: number
  errorCode?: string
  message?: string
  durationMs: number
  quota: number
  /** Timeouts reach the upstream before failing, so it may have billed. */
  upstreamMayCharge: boolean
}

export type RetryProcess = {
  attempts: RetryAttempt[]
  summary: {
    succeeded: boolean
    attemptCount: number
    totalSeconds: number
    totalQuota: number
  }
  /** Attempt number of the opened record; null when it is the final one. */
  currentAttempt: number | null
  /** Log id holding the final outcome, or null when it isn't loaded. */
  finalLogId: number | null
}

const TIMEOUT_STATUSES = new Set([504, 524])
const LOG_TYPE_CONSUME = 2

function policyOf(log: RetryLogSource): PolicyEvent[] {
  try {
    const other = JSON.parse(log.other || '{}') as {
      admin_info?: { request_policy?: PolicyEvent[] }
    }
    return other.admin_info?.request_policy ?? []
  } catch {
    return []
  }
}

/**
 * Rebuilds every attempt of one request from the routing events, which the
 * record holding the final outcome carries in full. Returns null for a
 * request that never retried, so its details stay as they are.
 */
export function buildRetryProcess(
  opened: RetryLogSource,
  siblings: RetryLogSource[]
): RetryProcess | null {
  const rows = siblings.some((row) => row.id === opened.id)
    ? siblings
    : [opened, ...siblings]
  const final = rows.reduce((best, row) =>
    policyOf(row).length > policyOf(best).length ? row : best
  )
  const events = policyOf(final)
  const attemptNumbers = [...new Set(events.map((e) => e.attempt))].filter(
    (n) => n > 0
  )
  if (attemptNumbers.length < 2) return null

  const names = new Map(
    rows.map((row) => [row.channel, row.channel_name ?? ''])
  )
  const succeeded = final.type === LOG_TYPE_CONSUME
  const attempts = attemptNumbers.map((attempt): RetryAttempt => {
    const own = events.filter((e) => e.attempt === attempt)
    const start = own.find((e) => e.decision.action === 'attempt')
    const failure = own.find((e) => e.decision.action === 'failure')
    const success = own.find((e) => e.decision.action === 'success')
    const end = failure ?? success
    const channelId = start?.channel_id ?? end?.channel_id ?? 0
    const isSuccess = success !== undefined
    return {
      attempt,
      channelId,
      channelName: names.get(channelId) ?? '',
      group: start?.group ?? '',
      succeeded: isSuccess,
      status: failure?.status,
      errorCode: failure?.error_code,
      message: failure?.message,
      durationMs: Math.max(
        0,
        (end?.elapsed_ms ?? 0) - (start?.elapsed_ms ?? 0)
      ),
      quota: isSuccess ? final.quota : 0,
      upstreamMayCharge:
        failure !== undefined && TIMEOUT_STATUSES.has(failure.status ?? 0),
    }
  })

  const openedEvents = policyOf(opened)
  const openedIsFinal = opened.id === final.id
  return {
    attempts,
    summary: {
      succeeded,
      attemptCount: attempts.length,
      totalSeconds: final.use_time,
      totalQuota: succeeded ? final.quota : 0,
    },
    currentAttempt: openedIsFinal
      ? null
      : (openedEvents.at(-1)?.attempt ?? null),
    finalLogId: final.id,
  }
}
