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

import { Dialog } from '@/components/dialog'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

/**
 * "How to choose a group": the administrator's guide (Markdown in the site
 * settings), or a built-in one while it is empty.
 */
export function GroupGuideDialog() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const guide = (status?.api_keys_group_guide as string | undefined)?.trim()

  return (
    <Dialog
      title={t('How to choose a group')}
      trigger={
        <Button
          variant='link'
          size='xs'
          className='text-muted-foreground hover:text-foreground h-auto px-0'
        >
          <CircleHelp aria-hidden='true' />
          {t('How to choose a group?')}
        </Button>
      }
    >
      {guide ? <RichContent content={guide} /> : <DefaultGroupGuide />}
    </Dialog>
  )
}

function DefaultGroupGuide() {
  const { t } = useTranslation()
  return (
    <div className='space-y-4 text-sm'>
      <p>
        {t(
          'The group decides which models a key can call and the price ratio it pays. Choose a group that serves every model you plan to use.'
        )}
      </p>
      <section className='space-y-2'>
        <h3 className='font-medium'>{t('Steps')}</h3>
        <ol className='list-decimal space-y-1.5 ps-5'>
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
      </section>
      <section className='space-y-2'>
        <h3 className='font-medium'>{t('Tips')}</h3>
        <ul className='list-disc space-y-1.5 ps-5'>
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
          <li>
            {t('Need a model no group serves? Contact the administrator.')}
          </li>
        </ul>
      </section>
    </div>
  )
}
