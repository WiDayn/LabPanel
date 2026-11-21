const positiveInteger = value => Number.isInteger(value) && value > 0
const rateAvailable = value => Number.isFinite(value) && value >= 0
const hasRates = link => rateAvailable(link.rxBytesPerSecond) || rateAvailable(link.txBytesPerSecond)

function pcieSpec(generation, width) {
  return [
    positiveInteger(generation) ? `Gen ${generation}` : '',
    positiveInteger(width) ? `×${width}` : '',
  ].filter(Boolean).join(' ')
}

export function gpuInterconnects({ pcie, nvlink }) {
  const links = []
  if (pcie) {
    const spec = pcieSpec(pcie.linkGeneration, pcie.linkWidth)
    const maxSpec = pcieSpec(pcie.maxLinkGeneration, pcie.maxLinkWidth)
    if (spec || maxSpec || hasRates(pcie)) {
      links.push({ ...pcie, protocol: 'PCIe', spec, maxSpec })
    }
  }
  if (nvlink && (nvlink.detected === true || positiveInteger(nvlink.activeLinks) || hasRates(nvlink))) {
    links.push({
      ...nvlink,
      protocol: 'NVLink',
      spec: Number.isInteger(nvlink.activeLinks) && nvlink.activeLinks >= 0
        ? `${nvlink.activeLinks} 条活跃链路` : '',
    })
  }
  return links
}

export function formatGpuLinkRate(value) {
  if (!rateAvailable(value)) return 'N/A'
  for (const [unit, divisor] of [['GiB', 1024 ** 3], ['MiB', 1024 ** 2], ['KiB', 1024]]) {
    if (value >= divisor) return `${(value / divisor).toFixed(1)} ${unit}/s`
  }
  return `${Math.round(value)} B/s`
}
