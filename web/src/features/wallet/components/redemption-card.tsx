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
import { ExternalLink, Gift, Loader2, ShoppingCart } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TitledCard } from '@/components/ui/titled-card'

interface RedemptionCardProps {
  enabled: boolean
  // Admin-configured storefront (System Settings -> TopUpLink); opened in a
  // new tab because third-party shops break inside an iframe.
  purchaseLink?: string
  code: string
  onCodeChange: (code: string) => void
  onRedeem: () => void
  redeeming: boolean
}

export function RedemptionCard(props: RedemptionCardProps) {
  const { t } = useTranslation()

  return (
    <TitledCard
      title={t('Redemption Code')}
      description={t(
        'Buy a redemption code, then redeem it here to add balance'
      )}
      icon={<Gift className='h-4 w-4' />}
      iconTone='warning'
      disableHoverEffect
      className='border-warning/40'
    >
      {props.enabled ? (
        <div className='grid gap-4 md:grid-cols-2'>
          {props.purchaseLink && (
            <div className='bg-muted/40 flex flex-col gap-3 rounded-lg border p-4'>
              <p className='text-sm font-medium'>
                {t('Step 1: No code yet? Buy one')}
              </p>
              <Button
                className='w-full gap-2'
                render={
                  <a
                    href={props.purchaseLink}
                    target='_blank'
                    rel='noopener noreferrer'
                  />
                }
              >
                <ShoppingCart className='h-4 w-4' />
                {t('Buy Redemption Code')}
                <ExternalLink className='h-3.5 w-3.5' />
              </Button>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Opens the shop in a new tab. Come back here to redeem after purchase.'
                )}
              </p>
            </div>
          )}
          <form
            className='bg-muted/40 flex flex-col gap-3 rounded-lg border p-4'
            onSubmit={(e) => {
              e.preventDefault()
              props.onRedeem()
            }}
          >
            <Label htmlFor='redemption-code' className='text-sm font-medium'>
              {props.purchaseLink
                ? t('Step 2: Enter your code to redeem')
                : t('Enter your code to redeem')}
            </Label>
            <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
              <Input
                id='redemption-code'
                value={props.code}
                onChange={(e) => props.onCodeChange(e.target.value)}
                placeholder={t('Enter your redemption code')}
                className='h-9 min-w-0'
              />
              <Button
                type='submit'
                disabled={props.redeeming}
                className='h-9 px-4'
              >
                {props.redeeming && (
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                )}
                {t('Redeem')}
              </Button>
            </div>
          </form>
        </div>
      ) : (
        <Alert>
          <AlertDescription>
            {t(
              'Redemption codes are disabled until the administrator confirms compliance terms.'
            )}
          </AlertDescription>
        </Alert>
      )}
    </TitledCard>
  )
}
