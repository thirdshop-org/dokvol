// Shared status → Tailwind badge class mapping, consolidating the several
// per-page `class:foo:bar={cond}` chains that never worked: Svelte's `class:`
// directive only toggles a single literal class token, so a colon-joined
// string like `class:bg-green-100:text-green-800={cond}` never matches any
// real class. Use `class={statusBadgeClass(status)}` instead.

export function statusBadgeClass(status: string): string {
	switch (status) {
		case 'completed':
		case 'success':
			return 'bg-green-100 text-green-800 hover:bg-green-100 dark:bg-green-900 dark:text-green-100 dark:hover:bg-green-900';
		case 'failed':
		case 'error':
			return 'bg-red-100 text-red-800 hover:bg-red-100 dark:bg-red-900 dark:text-red-100 dark:hover:bg-red-900';
		case 'running':
		case 'backing_up':
			return 'bg-blue-100 text-blue-800 hover:bg-blue-100 dark:bg-blue-900 dark:text-blue-100 dark:hover:bg-blue-900';
		default:
			return 'bg-gray-100 text-gray-800 hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-100 dark:hover:bg-gray-800';
	}
}

// Backup schedules use a plain boolean (enabled/disabled) rather than a
// status string.
export function enabledBadgeClass(enabled: boolean): string {
	return enabled
		? 'bg-green-100 text-green-800 hover:bg-green-100 dark:bg-green-900 dark:text-green-100 dark:hover:bg-green-900'
		: 'bg-gray-100 text-gray-800 hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-100 dark:hover:bg-gray-800';
}

// Success/failure feedback box border+text color (test connection results,
// restore results, etc.) — a plain boolean, not a status string.
export function feedbackBoxClass(success: boolean): string {
	return success
		? 'border-green-500 text-green-700 dark:text-green-400'
		: 'border-red-500 text-destructive';
}
