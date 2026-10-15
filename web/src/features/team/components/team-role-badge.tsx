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

import { StatusBadge } from '@/components/status-badge'

import type { TeamRole } from '../api'

export function TeamRoleBadge(props: { role: TeamRole }) {
  const { t } = useTranslation()
  if (props.role === 'owner') {
    return <StatusBadge label={t('Owner')} variant='purple' size='sm' />
  }
  if (props.role === 'admin') {
    return <StatusBadge label={t('Admin')} variant='info' size='sm' />
  }
  return <StatusBadge label={t('Member')} variant='neutral' size='sm' />
}
