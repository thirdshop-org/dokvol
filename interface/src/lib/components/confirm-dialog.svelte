<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { LoaderCircle } from '@lucide/svelte';

	let {
		open,
		onOpenChange,
		title,
		description,
		confirmLabel,
		cancelLabel,
		destructive = true,
		loading = false,
		error = null,
		onconfirm,
	}: {
		open: boolean;
		onOpenChange: (open: boolean) => void;
		title: string;
		description?: string;
		confirmLabel: string;
		cancelLabel: string;
		destructive?: boolean;
		loading?: boolean;
		error?: string | null;
		onconfirm: () => void;
	} = $props();
</script>

<Dialog.Root {open} {onOpenChange}>
	<Dialog.Content class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			{#if description}
				<Dialog.Description>{description}</Dialog.Description>
			{/if}
		</Dialog.Header>
		{#if error}
			<p class="text-sm text-destructive">{error}</p>
		{/if}
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => onOpenChange(false)} disabled={loading}>{cancelLabel}</Button>
			<Button variant={destructive ? 'destructive' : 'default'} onclick={onconfirm} disabled={loading}>
				{#if loading}<LoaderCircle class="size-4 animate-spin" />{/if}
				{confirmLabel}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
