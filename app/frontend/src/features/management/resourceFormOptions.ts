import type { ResourceRow } from '@/api/management'

import type { ResourceField, ResourceLookupDefinition, ResourceOption } from './resourceDefinitions'

export function resolveFieldLookup(
  field: ResourceField,
  form: ResourceRow
): ResourceLookupDefinition | undefined {
  if (field.lookup) return field.lookup
  if (!field.lookupBy) return undefined
  return field.lookupBy.values[String(form[field.lookupBy.field] ?? '')]
}

export function buildLookupOptions(
  rows: ResourceRow[],
  lookup: ResourceLookupDefinition,
  currentID = '',
  excludedValues = new Set<string>()
): ResourceOption[] {
  const valueKey = lookup.valueKey ?? 'id'
  const visibleRows = rows.filter(
    (row) =>
      (!lookup.onlyActive || Number(row.status) === 1) &&
      !excludedValues.has(String(row[valueKey] ?? ''))
  )
  const excludedIDs = lookup.excludeCurrentTree
    ? collectTreeIDs(visibleRows, currentID, lookup.parentKey ?? 'parent_id')
    : new Set<string>()
  const availableRows = visibleRows.filter((row) => !excludedIDs.has(String(row.id ?? '')))
  const options = lookup.parentKey
    ? buildTreeOptions(availableRows, lookup)
    : availableRows.map((row) => buildOption(row, lookup))
  return lookup.emptyLabel ? [{ value: 0, label: lookup.emptyLabel }, ...options] : options
}

function collectTreeIDs(rows: ResourceRow[], currentID: string, parentKey: string) {
  const excluded = new Set<string>()
  if (!currentID) return excluded
  excluded.add(currentID)
  let changed = true
  while (changed) {
    changed = false
    for (const row of rows) {
      const id = String(row.id ?? '')
      if (!excluded.has(id) && excluded.has(String(row[parentKey] ?? '0'))) {
        excluded.add(id)
        changed = true
      }
    }
  }
  return excluded
}

function buildTreeOptions(rows: ResourceRow[], lookup: ResourceLookupDefinition) {
  const parentKey = lookup.parentKey ?? 'parent_id'
  const children = new Map<string, ResourceRow[]>()
  for (const row of rows) {
    const parentID = String(row[parentKey] ?? '0')
    children.set(parentID, [...(children.get(parentID) ?? []), row])
  }
  const build = (parentID: string): ResourceOption[] =>
    (children.get(parentID) ?? []).map((row) => {
      const option = buildOption(row, lookup)
      const nested = build(String(row.id ?? ''))
      return nested.length ? { ...option, children: nested } : option
    })
  return build('0')
}

function buildOption(row: ResourceRow, lookup: ResourceLookupDefinition): ResourceOption {
  const valueKey = lookup.valueKey ?? 'id'
  const rawValue = row[valueKey]
  const value = typeof rawValue === 'number' ? rawValue : String(rawValue ?? '')
  const labels = lookup.labelKeys
    .map((key) => String(row[key] ?? '').trim())
    .filter((label, index, values) => label && values.indexOf(label) === index)
  if (row.id !== undefined) labels.push(`ID ${String(row.id)}`)
  return { value, label: labels.join(' · ') }
}
