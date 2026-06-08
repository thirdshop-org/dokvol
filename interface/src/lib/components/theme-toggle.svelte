<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { theme } from '$lib/stores/theme.svelte';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';

	let current = $state<'light' | 'dark'>('light');

	onMount(() => {
		theme.init();
		theme.subscribe(t => current = t);
	});
</script>

<button
	onclick={theme.toggle}
	class="text-muted-foreground hover:text-foreground transition-colors"
	title={current === 'dark' ? $t('theme.light') : $t('theme.dark')}
>
	{#if current === 'dark'}
		<Sun class="size-4" />
	{:else}
		<Moon class="size-4" />
	{/if}
</button>
