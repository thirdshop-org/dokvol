<script lang="ts">
	import { t } from '$lib/i18n';
	import { RANGE_KEYS, type RangeKey } from '$lib/utils/dates';

	let { value = $bindable('7d' as RangeKey), onchange }: { value?: RangeKey; onchange?: (v: RangeKey) => void } = $props();
</script>

<div class="flex">
	{#each RANGE_KEYS as range (range)}
		<button
			data-active={value === range}
			class="data-[active=true]:bg-muted/50 relative z-30 flex flex-1 flex-col justify-center gap-1 border-t px-4 py-3 text-start even:border-s sm:border-s sm:border-t-0 sm:px-6 sm:py-4"
			onclick={() => { value = range; onchange?.(range); }}
		>
			<span class="text-xs text-muted-foreground">{$t(`stats.range.${range}`)}</span>
		</button>
	{/each}
</div>
