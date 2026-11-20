import { computed, reactive } from 'vue'

const collator = new Intl.Collator('zh-CN', { numeric: true, sensitivity: 'base' })

export function sortTableRows(rows, valueOf, direction) {
  return rows.map(row => ({ row, value: valueOf(row) })).sort((left, right) => {
    const a = left.value
    const b = right.value
    const missing = value => value == null || value === '' || (typeof value === 'number' && !Number.isFinite(value))
    if (missing(a)) return missing(b) ? 0 : 1
    if (missing(b)) return -1
    const comparison = typeof a === 'number' && typeof b === 'number' ? a - b : collator.compare(a, b)
    return direction === 'ascending' ? comparison : -comparison
  }).map(entry => entry.row)
}

export function useTableSort(source, accessors) {
  const table = reactive({ key: '', direction: 'ascending' })
  const rows = computed(() => {
    const original = source()
    if (!table.key) return original
    return sortTableRows(original, accessors[table.key] || (row => row[table.key]), table.direction)
  })
  return reactive({ rows, table, toggle(key) {
    table.direction = table.key === key && table.direction === 'ascending' ? 'descending' : 'ascending'
    table.key = key
  } })
}

export function sortSize(value) {
  const match = /^(\d+(?:\.\d+)?)\s*([KMGTPE]?)(i?)B$/i.exec(value?.trim() || '')
  if (!match) return null
  const power = ' KMGTPE'.indexOf(match[2].toUpperCase() || ' ')
  return Number(match[1]) * (match[3] ? 1024 : 1000) ** power
}
