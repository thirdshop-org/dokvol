// A couple of pages (stats overview, history) also have their own
// formatDuration — kept separate rather than merged here, since their
// output formats genuinely differ (history never shows hours; stats does)
// rather than being copy-pasted duplicates of each other.
//
// The several formatBytes copies this replaces disagreed on zero: some
// returned "0 B" (a real, meaningful value), others "—" (treating 0 as "no
// data"), and without a zero guard Math.log(0) produces "NaN undefined".
// This version keeps the real value for zero and reserves "—" for actually
// missing/invalid data (negative or non-finite).
export function formatBytes(bytes: number): string {
	if (!Number.isFinite(bytes) || bytes < 0) return '—';
	if (bytes === 0) return '0 B';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	const i = Math.floor(Math.log(bytes) / Math.log(1024));
	return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
}
