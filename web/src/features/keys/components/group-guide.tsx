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
import { CircleHelp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { RichContent } from '@/components/rich-content'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { useStatus } from '@/hooks/use-status'

/**
 * "How to choose a group?", shown right under the group field so users read
 * it before picking: the administrator's guide (Markdown in the site
 * settings), or a built-in one while it is empty.
 */
export function GroupGuide() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const guide = (status?.api_keys_group_guide as string | undefined)?.trim()

  return (
    // Static guidance, not an alert to announce.
    <Alert role='note' className='bg-muted/30 py-2.5'>
      <CircleHelp aria-hidden='true' />
      <AlertTitle className='text-xs'>{t('How to choose a group?')}</AlertTitle>
      <AlertDescription className='text-xs [&_p:not(:last-child)]:mb-1.5'>
        {guide ? <RichContent content={guide} /> : <DefaultGroupGuide />}
      </AlertDescription>
    </Alert>
  )
}

function DefaultGroupGuide() {
  const { t } = useTranslation()
  return (
    <div className='space-y-1.5'>
      <p>
        {t(
          'The group decides which models a key can call and the price ratio it pays. Choose a group that serves every model you plan to use.'
        )}
      </p>
      <ol className='list-decimal space-y-0.5 ps-4'>
        <li>
          {t(
            'Open the group list and point at a group, or move to it with the arrow keys; the models it serves appear beside the list.'
          )}
        </li>
        <li>
          {t(
            'Pick a group that lists the models you need. The ratio next to a group multiplies the model price.'
          )}
        </li>
        <li>
          {t(
            'After saving, click a model name in the key list to copy it and paste it into your client instead of typing it.'
          )}
        </li>
      </ol>
      <p className='font-medium'>{t('Tips')}</p>
      <ul className='list-disc space-y-0.5 ps-4'>
        <li>
          {t(
            'Auto tries the Auto groups in order for each request, so the key keeps working when one group is busy.'
          )}
        </li>
        <li>
          {t(
            'If your client fetches no models, check that the key’s group serves the model you need.'
          )}
        </li>
        <li>{t('Need a model no group serves? Contact the administrator.')}</li>
      </ul>
    </div>
  )
}
