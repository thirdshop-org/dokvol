<script lang="ts">
	import type { DriveInfo } from '$lib/types';
	import { t } from '$lib/i18n';

	let {
		drives,
		value = $bindable(''),
		placeholder,
		disabled = false,
		showFreeSpace = true,
		compact = false,
		id,
		onchange,
	}: {
		drives: DriveInfo[];
		value?: string;
		placeholder: string;
		disabled?: boolean;
		showFreeSpace?: boolean;
		compact?: boolean;
		id?: string;
		onchange?: (mountpoint: string) => void;
	} = $props();

	let classes = $derived(
		compact
			? 'border-input bg-background ring-offset-background focus-visible:ring-ring flex h-8 w-full rounded-md border px-2 py-1 text-xs shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'
			: 'border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'
	);
</script>

<select
	{id}
	class={classes}
	bind:value
	{disabled}
	onchange={(e) => onchange?.((e.target as HTMLSelectElement).value)}
>
	<option value="" disabled>{placeholder}</option>
	{#each drives as drive (drive.mountpoint)}
		<option value={drive.mountpoint}>
			{drive.device} — {drive.mountpoint}{showFreeSpace ? ` ${$t('driveSelect.freeSpace', { gb: drive.free_gb })}` : ''}
		</option>
	{/each}
</select>
