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

import { Dialog } from '@/components/dialog'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'
import { useNotifications } from '@/hooks/use-notifications'
import { useNotificationStore } from '@/stores/notification-store'

/**
 * Pops the system notice once per content revision. "Close" marks it read
 * (same as opening the bell); "Close Today" snoozes it until tomorrow.
 */
export function NoticeDialog() {
  const { t } = useTranslation()
  const { notice, unreadNoticeCount } = useNotifications()
  const markNoticeRead = useNotificationStore((s) => s.markNoticeRead)
  const setClosedUntilDate = useNotificationStore((s) => s.setClosedUntilDate)
  const closedToday = useNotificationStore((s) => s.isNoticeClosed(notice))

  const open = unreadNoticeCount > 0 && !closedToday

  const handleClose = () => markNoticeRead(notice)
  const handleCloseToday = () =>
    setClosedUntilDate(new Date().toDateString(), notice)

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) handleClose()
      }}
      title={t('System Notice')}
      contentClassName='max-h-[85svh] sm:max-w-lg'
      footer={
        <>
          <Button variant='outline' onClick={handleCloseToday}>
            {t('Close Today')}
          </Button>
          <Button onClick={handleClose}>{t('Close')}</Button>
        </>
      }
    >
      <RichContent breaks content={notice} />
    </Dialog>
  )
}
