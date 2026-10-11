export type StatusFilter<T extends string> = 'ALL' | T;

export function statusFilterOptions<T extends string>(
  labels: Record<T, string>,
  statuses: readonly T[] = Object.keys(labels) as T[]
): { label: string; value: StatusFilter<T> }[] {
  return [
    { label: 'همه وضعیت‌ها', value: 'ALL' },
    ...statuses.map(value => ({ label: labels[value], value }))
  ];
}
