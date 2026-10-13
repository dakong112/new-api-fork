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
import { useNavigate } from '@tanstack/react-router'
import type { AxiosRequestConfig } from 'axios'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'
import { useAuthStore } from '@/stores/auth-store'

import { AuthLayout } from '../auth-layout'
import { useAuthRedirect } from '../hooks/use-auth-redirect'
import { sanitizeAuthRedirect } from '../lib/auth-redirect'
import type { LoginResponse } from '../types'

type TicketRequestConfig = AxiosRequestConfig & {
  skipBusinessError?: boolean
  skipAuthRefresh?: boolean
}

type TicketPreview = {
  username: string
  display_name: string
  client_name: string
}

type TicketState =
  | { status: 'loading' }
  | { status: 'ready'; preview: TicketPreview }
  | { status: 'redeeming'; preview: TicketPreview }
  | { status: 'error' }

const DEFAULT_TARGET = '/wallet'
const requestConfig: TicketRequestConfig = {
  skipBusinessError: true,
  skipAuthRefresh: true,
}

export function SsoTicket() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { handleLoginResult } = useAuthRedirect()
  const currentUser = useAuthStore((state) => state.auth.user)
  const [state, setState] = useState<TicketState>({ status: 'loading' })
  const ticket = useRef('')
  const target = useRef(DEFAULT_TARGET)
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return
    started.current = true
    // The ticket arrives in the fragment so it never reaches a server log or
    // a Referer header; read it once and remove it from the address bar.
    const params = new URLSearchParams(window.location.hash.slice(1))
    ticket.current = params.get('ticket') ?? ''
    target.current =
      sanitizeAuthRedirect(params.get('to'), window.location.origin) ??
      DEFAULT_TARGET
    window.history.replaceState(
      window.history.state,
      '',
      window.location.pathname + window.location.search
    )
    if (!ticket.current) {
      setState({ status: 'error' })
      return
    }
    api
      .post<{ success: boolean; data?: TicketPreview }>(
        '/api/sso/ticket/preview',
        { ticket: ticket.current },
        requestConfig
      )
      .then((response) => {
        const preview = response.data?.data
        setState(
          response.data?.success && preview
            ? { status: 'ready', preview }
            : { status: 'error' }
        )
      })
      .catch(() => setState({ status: 'error' }))
  }, [])

  const confirm = async (preview: TicketPreview) => {
    setState({ status: 'redeeming', preview })
    try {
      const response = await api.post<LoginResponse>(
        '/api/sso/ticket/redeem',
        { ticket: ticket.current },
        requestConfig
      )
      if (!response.data?.success) throw response.data
      await handleLoginResult(response.data.data, target.current)
    } catch (error: unknown) {
      handleServerError(AuthOperationError.from(error, t('Login failed')))
      setState({ status: 'error' })
    }
  }

  const goToSignIn = () => void navigate({ to: '/sign-in', replace: true })

  let content = <LoadingState message={t('Checking your sign-in link...')} />
  if (state.status === 'error') {
    content = (
      <ErrorState
        title={t('This sign-in link is invalid or has expired')}
        description={t(
          'Open the app again to get a new link, or sign in directly.'
        )}
        action={<Button onClick={goToSignIn}>{t('Go to sign in')}</Button>}
      />
    )
  }
  if (state.status === 'ready' || state.status === 'redeeming') {
    const preview = state.preview
    const name = preview.display_name || preview.username
    content = (
      <div className='w-full space-y-6'>
        <div className='space-y-3'>
          <h2 className='text-2xl font-semibold tracking-tight'>
            {t('Confirm sign-in')}
          </h2>
          <p className='text-muted-foreground text-sm sm:text-base'>
            {t(
              'You are about to sign in to the account center as {{name}} (from {{client}}).',
              { name, client: preview.client_name }
            )}
          </p>
          {currentUser && currentUser.username !== preview.username && (
            <p className='text-sm font-medium'>
              {t(
                'Currently signed in as {{current}}; confirming will switch to {{name}}.',
                {
                  current: currentUser.display_name || currentUser.username,
                  name,
                }
              )}
            </p>
          )}
          <p className='text-muted-foreground text-sm'>
            {t('If you did not start this, cancel.')}
          </p>
        </div>
        <div className='flex gap-3'>
          <Button
            onClick={() => void confirm(preview)}
            disabled={state.status === 'redeeming'}
          >
            {t('Confirm sign-in')}
          </Button>
          <Button
            variant='outline'
            onClick={() => void navigate({ to: '/', replace: true })}
            disabled={state.status === 'redeeming'}
          >
            {t('Cancel')}
          </Button>
        </div>
      </div>
    )
  }

  return <AuthLayout>{content}</AuthLayout>
}
