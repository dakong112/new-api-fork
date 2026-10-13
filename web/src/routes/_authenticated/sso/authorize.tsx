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
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { createSSOCode } from '@/features/sso/api'
import { handleServerError } from '@/lib/handle-server-error'

export const Route = createFileRoute('/_authenticated/sso/authorize')({
  component: SSOAuthorizePage,
})

export function SSOAuthorizePage() {
  const { t } = useTranslation()
  const started = useRef(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    if (started.current) return
    started.current = true
    // Read the raw query: the router's search parser would turn a numeric
    // looking `state` (e.g. "1234e5") into a number and corrupt it.
    const query = new URLSearchParams(window.location.search)
    const clientId = query.get('client_id')
    const redirectUri = query.get('redirect_uri')
    const state = query.get('state')
    if (!clientId || !redirectUri || !state) {
      setFailed(true)
      return
    }
    createSSOCode({
      client_id: clientId,
      redirect_uri: redirectUri,
      state,
    })
      .then((url) => {
        // The server only returns URLs built from a registered redirect_uri.
        if (!/^https?:\/\//i.test(url)) throw new Error('Invalid redirect URL')
        window.location.href = url
      })
      .catch((error: unknown) => {
        setFailed(true)
        handleServerError(error, t('Application authorization failed'))
      })
  }, [t])

  if (failed) {
    return <ErrorState title={t('Application authorization failed')} />
  }
  return <LoadingState message={t('Signing you in to the application...')} />
}
