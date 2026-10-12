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
import { Alert, AlertDescription } from '@/components/ui/alert'
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

/** Null when the stored value is missing or unreadable. */
function toVendorRuleRows(value: string): VendorRuleRow[] | null {
  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return null
  }
  if (!Array.isArray(parsed)) return null
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
  // Null forces a rebuild from defaultValue even when it did not change.
  const [baseline, setBaseline] = useState<string | null>(props.defaultValue)
  const [parsedRows, setParsedRows] = useState(() =>
    toVendorRuleRows(props.defaultValue)
  )
  const [showErrors, setShowErrors] = useState(false)
  const [confirmRestoreOpen, setConfirmRestoreOpen] = useState(false)
  const [confirmEmptyOpen, setConfirmEmptyOpen] = useState(false)
  const [draggingRowId, setDraggingRowId] = useState<string | null>(null)
  const [dropTarget, setDropTarget] = useState<{
    id: string
    after: boolean
  } | null>(null)

  // A saved value comes back through system-options; start over from it.
  if (baseline !== props.defaultValue) {
    setBaseline(props.defaultValue)
    setParsedRows(toVendorRuleRows(props.defaultValue))
    setShowErrors(false)
  }
  const readFailed = parsedRows === null
  const rows = parsedRows ?? []
  const setRows = setParsedRows
  const rowLabel = (row: VendorRuleRow, index: number) =>
    row.name.trim() || t('Row {{row}}', { row: index + 1 })

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
    // Mirrors maxVendorRuleIconLength in model/vendor_rules.go.
    const icon =
      row.icon.trim().length > 128
        ? t('Icon name must be at most 128 characters')
        : undefined
    return { name, keywords, icon }
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

  const submitRules = () => {
    const rules: VendorRule[] = rows.map((row) => ({
      name: row.name.trim(),
      ...(row.icon.trim() ? { icon: row.icon.trim() } : {}),
      keywords: splitKeywords(row.keywords),
    }))
    updateOption.mutate(
      { key: 'VendorRules', value: JSON.stringify(rules) },
      { onSuccess: () => setConfirmEmptyOpen(false) }
    )
  }

  const save = () => {
    if (readFailed) return
    if (errors.some((error) => error.name || error.keywords || error.icon)) {
      setShowErrors(true)
      return
    }
    // An empty list sends every model to "Other", so ask first.
    if (rows.length === 0) {
      setConfirmEmptyOpen(true)
      return
    }
    submitRules()
  }

  const restoreDefaults = () => {
    // The server treats an empty value as "use the built-in rules".
    updateOption.mutate(
      { key: 'VendorRules', value: '' },
      {
        onSuccess: () => {
          setBaseline(null)
          setConfirmRestoreOpen(false)
        },
      }
    )
  }

  return (
    <SettingsSection title={t('Vendor rules')}>
      <SettingsPageFormActions
        onSave={save}
        onReset={() => setConfirmRestoreOpen(true)}
        resetLabel='Restore defaults'
        isSaving={updateOption.isPending}
        isSaveDisabled={readFailed}
      />
      {readFailed && (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'The stored vendor rules could not be read. Restore defaults to continue.'
            )}
          </AlertDescription>
        </Alert>
      )}
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
          disabled={readFailed}
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
            cell: (row, index) => (
              // Native drag from the handle only, so text in the inputs stays
              // selectable; the arrow keys mirror Move up/down.
              <Button
                variant='ghost'
                size='icon-sm'
                className='text-muted-foreground cursor-grab active:cursor-grabbing'
                draggable
                aria-label={t('Drag {{group}} to reorder', {
                  group: rowLabel(row, index),
                })}
                title={t('Drag {{group}} to reorder', {
                  group: rowLabel(row, index),
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
                  aria-label={t('Vendor name, row {{row}}', {
                    row: index + 1,
                  })}
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
            cell: (row, index) => (
              <div className='space-y-1'>
                <LobeIconField
                  compact
                  aria-label={t('Icon, row {{row}}', { row: index + 1 })}
                  value={row.icon}
                  onChange={(icon) => updateRow(row._id, { icon })}
                />
                {showErrors && errors[index].icon && (
                  <p className='text-destructive text-xs'>
                    {errors[index].icon}
                  </p>
                )}
              </div>
            ),
          },
          {
            id: 'keywords',
            header: t('Keywords'),
            className: 'min-w-64',
            cell: (row, index) => (
              <div className='space-y-1'>
                <Input
                  aria-label={t('Keywords, row {{row}}', {
                    row: index + 1,
                  })}
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
                    group: rowLabel(row, index),
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
                    group: rowLabel(row, index),
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
        open={confirmRestoreOpen}
        onOpenChange={setConfirmRestoreOpen}
        title={t('Restore defaults')}
        desc={t(
          'Restore the built-in vendor rules? The current rules will be replaced.'
        )}
        confirmText={t('Restore defaults')}
        handleConfirm={restoreDefaults}
        isLoading={updateOption.isPending}
      />
      <ConfirmDialog
        open={confirmEmptyOpen}
        onOpenChange={setConfirmEmptyOpen}
        title={t('Save Changes')}
        desc={t(
          'Save an empty rule list? Every model will be shown under Other.'
        )}
        confirmText={t('Save Changes')}
        destructive
        handleConfirm={submitRules}
        isLoading={updateOption.isPending}
      />
    </SettingsSection>
  )
}
