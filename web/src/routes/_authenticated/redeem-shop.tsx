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
import { ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'

// ponytail: hard-coded shop URL; move to a system setting if a second store appears
const SHOP_URL = 'https://catfk.com/shop/2V0PRESR'

export const Route = createFileRoute('/_authenticated/redeem-shop')({
  component: RouteComponent,
})

function RouteComponent() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Buy Redemption Code')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          render={
            <a href={SHOP_URL} target='_blank' rel='noopener noreferrer' />
          }
        >
          {t('Open in new tab')}
          <ExternalLink />
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {/* ponytail: no sandbox — the shop is a third-party storefront that
            needs its own cookies, scripts and popups to work, and a sandbox
            permissive enough for that (allow-scripts + allow-same-origin)
            grants it nothing less than this. */}
        {/* eslint-disable-next-line react/iframe-missing-sandbox */}
        <iframe
          src={SHOP_URL}
          title={t('Buy Redemption Code')}
          className='h-full w-full rounded-lg border bg-white'
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
