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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { useStatus } from '@/hooks/use-status'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

import { fillRedemptionTemplate } from '../lib'
import type { Redemption } from '../types'

/**
 * Copies a code filled into the admin's template (System Settings > Billing).
 * Renders nothing until a template is configured.
 */
export function RedemptionTemplateCopyButton(props: {
  redemption: Redemption
}) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const template = String(status?.redemption_copy_template ?? '').trim()
  if (!template) return null

  const text = fillRedemptionTemplate(template, {
    code: props.redemption.key,
    name: props.redemption.name,
    quota: formatQuota(props.redemption.quota),
    expires:
      props.redemption.expired_time === 0
        ? t('Never')
        : formatTimestampToDate(props.redemption.expired_time),
  })

  return (
    <CopyButton
      value={text}
      size='sm'
      variant='outline'
      className='h-7 gap-1 px-2 text-xs'
      iconClassName='size-3.5'
      tooltip={t('Copy with template')}
      aria-label={t('Copy with template')}
    >
      {t('Template')}
    </CopyButton>
  )
}
