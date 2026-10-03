export function useChartTheme() {
  const baseOptions = {
    backgroundColor: 'transparent',
    textStyle: { color: '#8b90a0', fontFamily: 'Inter' },
    grid: { left: 60, right: 20, top: 30, bottom: 30, containLabel: false },
    tooltip: {
      backgroundColor: '#242836',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
    },
    xAxis: {
      axisLine: { lineStyle: { color: '#2e3347' } },
      axisTick: { lineStyle: { color: '#2e3347' } },
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      splitLine: { lineStyle: { color: '#2e3347', type: 'dashed' as const } },
    },
    yAxis: {
      axisLine: { lineStyle: { color: '#2e3347' } },
      axisTick: { lineStyle: { color: '#2e3347' } },
      axisLabel: { color: '#8b90a0', fontSize: 11, fontFamily: 'JetBrains Mono' },
      splitLine: { lineStyle: { color: '#2e3347', type: 'dashed' as const } },
    },
  }

  const colors = {
    blue: '#3b82f6',
    purple: '#8b5cf6',
    cyan: '#06b6d4',
    amber: '#f59e0b',
    red: '#ef4444',
    green: '#22c55e',
    orange: '#f97316',
  }

  const seriesColors = [colors.blue, colors.amber, colors.green, colors.red, colors.purple, colors.cyan]

  return { baseOptions, colors, seriesColors }
}
