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

export type RetryChain = {
  /** e.g. "#1 (524) → #3 ✓"; the last hop has no status when it succeeded. */
  label: string
  /** True when a group network retry (502/504/524) moved the request. */
  networkRetry: boolean
}

/**
 * Joins the tried channels with the upstream status each one failed with,
 * taken from the request policy failure events.
 */
export function describeRetryChain(
  useChannel: Array<string | number>,
  events: PolicyEvent[] | undefined,
  succeeded: boolean
): RetryChain {
  const failures = (events ?? []).filter(
    (event) => event.decision?.action === 'failure' && event.status
  )
  const hops = useChannel.map((channel, index) => {
    const failure = failures.find(
      (event) => String(event.channel_id) === String(channel)
    )
    if (failure) return `#${channel} (${failure.status})`
    const isLast = index === useChannel.length - 1
    return isLast && succeeded ? `#${channel} ✓` : `#${channel}`
  })
  return {
    label: hops.join(' → '),
    networkRetry: (events ?? []).some(
      (event) => event.decision?.reason === 'network_error_retry'
    ),
  }
}
