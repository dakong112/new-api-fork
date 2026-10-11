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
import { ArrowDown, ArrowUp, GripVertical, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { LobeIconField } from '@/components/lobe-icon-field'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type VendorRule = { name: string; icon?: string; keywords: string[] }

type VendorRuleRow = {
  _id: string
  name: string
  icon: string
  keywords: string
}

let vendorRuleIdCounter = 0

function createVendorRuleRow(rule: Partial<VendorRule> = {}): VendorRuleRow {
  vendorRuleIdCounter += 1
  return {
    _id: `vr_${vendorRuleIdCounter}`,
    name: rule.name ?? '',
    icon: rule.icon ?? '',
    keywords: Array.isArray(rule.keywords) ? rule.keywords.join(', ') : '',
  }
}

function toVendorRuleRows(value: string): VendorRuleRow[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return []
  }
  if (!Array.isArray(parsed)) return []
  return (parsed as Partial<VendorRule>[]).map((rule) =>
    createVendorRuleRow(rule ?? {})
  )
}

function splitKeywords(value: string) {
  return value
    .split(',')
    .map((keyword) => keyword.trim())
    .filter(Boolean)
}

export function VendorRulesCard(props: { defaultValue: string }) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [baseline, setBaseline] = useState(props.defaultValue)
  const [rows, setRows] = useState(() => toVendorRuleRows(props.defaultValue))
  const [showErrors, setShowErrors] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [draggingRowId, setDraggingRowId] = useState<string | null>(null)
  const [dropTarget, setDropTarget] = useState<{
    id: string
    after: boolean
  } | null>(null)

  // A saved value comes back through system-options; start over from it.
  if (baseline !== props.defaultValue) {
    setBaseline(props.defaultValue)
    setRows(toVendorRuleRows(props.defaultValue))
    setShowErrors(false)
  }

  // Mirrors the server validation in model/vendor_rules.go.
  const nameCounts = new Map<string, number>()
  for (const row of rows) {
    const key = row.name.trim().toLowerCase()
    if (key) nameCounts.set(key, (nameCounts.get(key) ?? 0) + 1)
  }
  const errors = rows.map((row) => {
    const key = row.name.trim().toLowerCase()
    let name: string | undefined
    if (!key) {
      name = t('Vendor name is required')
    } else if ((nameCounts.get(key) ?? 0) > 1) {
      name = t('Vendor name must be unique')
    }
    const keywords =
      splitKeywords(row.keywords).length === 0
        ? t('At least one keyword is required')
        : undefined
    return { name, keywords }
  })

  const updateRow = (id: string, patch: Partial<VendorRuleRow>) =>
    setRows(rows.map((row) => (row._id === id ? { ...row, ...patch } : row)))

  const moveRow = (id: string, offset: -1 | 1) => {
    const from = rows.findIndex((row) => row._id === id)
    const to = from + offset
    if (from < 0 || to < 0 || to >= rows.length) return
    const nextRows = [...rows]
    ;[nextRows[from], nextRows[to]] = [nextRows[to], nextRows[from]]
    setRows(nextRows)
  }

  const dropRow = (sourceId: string, targetId: string, after: boolean) => {
    const moving = rows.find((row) => row._id === sourceId)
    const rest = rows.filter((row) => row._id !== sourceId)
    const targetIndex = rest.findIndex((row) => row._id === targetId)
    if (!moving || targetIndex < 0) return
    const at = targetIndex + (after ? 1 : 0)
    setRows([...rest.slice(0, at), moving, ...rest.slice(at)])
  }

  const save = async () => {
    if (errors.some((error) => error.name || error.keywords)) {
      setShowErrors(true)
      return
    }
    const rules: VendorRule[] = rows.map((row) => ({
      name: row.name.trim(),
      ...(row.icon.trim() ? { icon: row.icon.trim() } : {}),
      keywords: splitKeywords(row.keywords),
    }))
    await updateOption.mutateAsync({
      key: 'VendorRules',
      value: JSON.stringify(rules),
    })
  }

  const restoreDefaults = async () => {
    // The server treats an empty value as "use the built-in rules".
    await updateOption.mutateAsync({ key: 'VendorRules', value: '' })
    setConfirmOpen(false)
  }

  return (
    <SettingsSection title={t('Vendor rules')}>
      <SettingsPageFormActions
        onSave={save}
        onReset={() => setConfirmOpen(true)}
        resetLabel='Restore defaults'
        isSaving={updateOption.isPending}
      />
      <p className='text-muted-foreground text-sm'>
        {t(
          'Decide which vendor a model belongs to by keywords in its name. Matching ignores case and the longest keyword wins. Vendors set on a model in Model management take precedence.'
        )}
      </p>
      <div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => setRows([...rows, createVendorRuleRow()])}
        >
          <Plus data-icon='inline-start' />
          {t('Add vendor')}
        </Button>
      </div>
      <StaticDataTable
        tableClassName='min-w-[860px]'
        tableProps={{ 'aria-label': t('Vendor rules') }}
        data={rows}
        getRowKey={(row) => row._id}
        getRowClassName={(row) =>
          cn(
            'align-top',
            draggingRowId === row._id && 'opacity-50',
            dropTarget?.id === row._id &&
              (dropTarget.after
                ? 'border-b-primary border-b-2'
                : 'border-t-primary border-t-2')
          )
        }
        getRowProps={(row) => ({
          onDragOver: (event) => {
            if (!draggingRowId || draggingRowId === row._id) return
            event.preventDefault()
            event.dataTransfer.dropEffect = 'move'
            const rect = event.currentTarget.getBoundingClientRect()
            const after = event.clientY > rect.top + rect.height / 2
            if (dropTarget?.id !== row._id || dropTarget.after !== after) {
              setDropTarget({ id: row._id, after })
            }
          },
          onDrop: (event) => {
            if (!draggingRowId) return
            event.preventDefault()
            const rect = event.currentTarget.getBoundingClientRect()
            dropRow(
              draggingRowId,
              row._id,
              event.clientY > rect.top + rect.height / 2
            )
            setDraggingRowId(null)
            setDropTarget(null)
          },
        })}
        columns={[
          {
            id: 'drag',
            header: '',
            className: 'w-8',
            cell: (row) => (
              // Native drag from the handle only, so text in the inputs stays
              // selectable; the arrow keys mirror Move up/down.
              <Button
                variant='ghost'
                size='icon-sm'
                className='text-muted-foreground cursor-grab active:cursor-grabbing'
                draggable
                aria-label={t('Drag {{group}} to reorder', {
                  group: row.name.trim(),
                })}
                title={t('Drag {{group}} to reorder', {
                  group: row.name.trim(),
                })}
                onDragStart={(event) => {
                  event.dataTransfer.effectAllowed = 'move'
                  event.dataTransfer.setData('text/plain', row._id)
                  const tableRow = event.currentTarget.closest('tr')
                  if (tableRow) event.dataTransfer.setDragImage(tableRow, 0, 0)
                  setDraggingRowId(row._id)
                }}
                onDragEnd={() => {
                  setDraggingRowId(null)
                  setDropTarget(null)
                }}
                onKeyDown={(event) => {
                  if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
                    event.preventDefault()
                    moveRow(row._id, event.key === 'ArrowUp' ? -1 : 1)
                  }
                }}
              >
                <GripVertical className='h-4 w-4' aria-hidden='true' />
              </Button>
            ),
          },
          {
            id: 'preview',
            header: '',
            className: 'w-8',
            cell: (row) => (
              <span
                className='flex size-8 items-center justify-center'
                aria-hidden='true'
              >
                {getLobeIcon(row.icon, 20)}
              </span>
            ),
          },
          {
            id: 'name',
            header: t('Name'),
            className: 'min-w-36',
            cell: (row, index) => (
              <div className='space-y-1'>
                <Input
                  aria-label={t('Name')}
                  value={row.name}
                  aria-invalid={showErrors && Boolean(errors[index].name)}
                  onChange={(event) =>
                    updateRow(row._id, { name: event.target.value })
                  }
                />
                {showErrors && errors[index].name && (
                  <p className='text-destructive text-xs'>
                    {errors[index].name}
                  </p>
                )}
              </div>
            ),
          },
          {
            id: 'icon',
            header: t('Icon'),
            className: 'min-w-44',
            cell: (row) => (
              <LobeIconField
                compact
                aria-label={t('Icon')}
                value={row.icon}
                onChange={(icon) => updateRow(row._id, { icon })}
              />
            ),
          },
          {
            id: 'keywords',
            header: t('Keywords'),
            className: 'min-w-64',
            cell: (row, index) => (
              <div className='space-y-1'>
                <Input
                  aria-label={t('Keywords')}
                  placeholder={t('Comma-separated, e.g. gpt, o1, dall-e')}
                  value={row.keywords}
                  aria-invalid={showErrors && Boolean(errors[index].keywords)}
                  onChange={(event) =>
                    updateRow(row._id, { keywords: event.target.value })
                  }
                />
                {showErrors && errors[index].keywords && (
                  <p className='text-destructive text-xs'>
                    {errors[index].keywords}
                  </p>
                )}
              </div>
            ),
          },
          {
            id: 'actions',
            header: t('Actions'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (row, index) => (
              <div className='flex justify-end gap-1'>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => moveRow(row._id, -1)}
                  disabled={index === 0}
                  aria-label={t('Move {{group}} up', {
                    group: row.name.trim(),
                  })}
                >
                  <ArrowUp className='h-4 w-4' />
                </Button>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => moveRow(row._id, 1)}
                  disabled={index === rows.length - 1}
                  aria-label={t('Move {{group}} down', {
                    group: row.name.trim(),
                  })}
                >
                  <ArrowDown className='h-4 w-4' />
                </Button>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() =>
                    setRows(rows.filter((item) => item._id !== row._id))
                  }
                  aria-label={t('Delete')}
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            ),
          },
        ]}
      />
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Restore defaults')}
        desc={t(
          'Restore the built-in vendor rules? Your changes will be replaced.'
        )}
        confirmText={t('Restore defaults')}
        handleConfirm={restoreDefaults}
        isLoading={updateOption.isPending}
      />
    </SettingsSection>
  )
}
