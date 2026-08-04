export type RangeKey = '7d' | '30d' | '90d' | 'all';

export const RANGE_KEYS: RangeKey[] = ['7d', '30d', '90d', 'all'];

export function rangeDays(key: RangeKey): number {
	if (key === '7d') return 7;
	if (key === '30d') return 30;
	if (key === '90d') return 90;
	return 365;
}

export function toISO(daysAgo: number): string {
	const d = new Date();
	d.setDate(d.getDate() - daysAgo);
	return d.toISOString();
}

export function dateRange(key: RangeKey): [Date, Date] {
	const to = new Date();
	const from = new Date();
	from.setDate(from.getDate() - rangeDays(key));
	return [from, to];
}
