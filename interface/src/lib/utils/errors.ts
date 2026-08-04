import { get } from 'svelte/store';
import { t } from '$lib/i18n';
import { ApiError } from '$lib/api';

// Shared across every route that catches an API call: translates a
// structured backend error_code (e.g. "MIGRATION.VOLUME_LOCKED") via the
// error.* i18n namespace, which already falls back to error.default for any
// code without a specific translation — so this never surfaces a raw error
// code to the user.
export function errorMessage(e: unknown): string {
	if (e instanceof ApiError) {
		return get(t)(`error.${e.errorCode}`);
	}
	if (e instanceof Error) return e.message;
	return get(t)('error.default');
}
