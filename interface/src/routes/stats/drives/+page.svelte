<script lang="ts">
	import { onMount } from "svelte";
	import { page } from "$app/stores";
	import { t } from "$lib/i18n";
	import StatsChart from "$lib/components/charts/StatsChart.svelte";
	import { getStatsDrive } from "$lib/api";

	type RangeKey = "7d" | "30d" | "90d" | "all";
	let selectedRange = $state<RangeKey>("7d");
	let data = $state<{ date: Date; value: number }[]>([]);
	let used = $state(0);
	let free = $state(0);
	let total = $state(0);
	let ready = $state(false);

	const mountpoint = $derived($page.url.searchParams.get("mountpoint") ?? "");

	function rangeDays(key: RangeKey): number {
		if (key === "7d") return 7;
		if (key === "30d") return 30;
		if (key === "90d") return 90;
		return 365;
	}

	function toISO(daysAgo: number): string {
		const d = new Date();
		d.setDate(d.getDate() - daysAgo);
		return d.toISOString();
	}

	async function load() {
		ready = false;
		const from = toISO(rangeDays(selectedRange));
		const rows = await getStatsDrive(mountpoint, from);
		data = rows.map(r => ({ date: new Date(r.captured_at), value: r.used_bytes }));
		if (rows.length > 0) {
			const last = rows[rows.length - 1];
			used = last.used_bytes;
			free = last.free_bytes;
			total = last.total_bytes;
		}
		ready = true;
	}

	onMount(load);

	function formatBytes(v: number): string {
		if (v === 0) return "0 B";
		const units = ["B", "KB", "MB", "GB", "TB"];
		const i = Math.floor(Math.log(v) / Math.log(1024));
		return `${(v / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
	}
</script>

<div class="space-y-6">
	<div class="flex items-start justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{mountpoint}</h1>
			<p class="text-muted-foreground">{$t("stats.storage")} {$t("stats.evolution")}</p>
		</div>
		<div class="flex">
			{#each ["7d", "30d", "90d", "all"] as range (range)}
				<button
					data-active={selectedRange === range}
					class="data-[active=true]:bg-muted/50 relative z-30 flex flex-1 flex-col justify-center gap-1 border-t px-4 py-3 text-start even:border-s sm:border-s sm:border-t-0 sm:px-6 sm:py-4"
					onclick={() => { selectedRange = range as RangeKey; load(); }}
				>
					<span class="text-xs text-muted-foreground">{$t(`stats.range.${range}`)}</span>
				</button>
			{/each}
		</div>
	</div>

	{#if ready}
		<div class="grid gap-4 sm:grid-cols-3">
			<div class="rounded-lg border bg-card p-4">
				<p class="text-sm text-muted-foreground">{$t("drives.table.total")}</p>
				<p class="text-2xl font-bold">{formatBytes(total)}</p>
			</div>
			<div class="rounded-lg border bg-card p-4">
				<p class="text-sm text-muted-foreground">{$t("drives.table.free")}</p>
				<p class="text-2xl font-bold">{formatBytes(free)}</p>
			</div>
			<div class="rounded-lg border bg-card p-4">
				<p class="text-sm text-muted-foreground">{$t("drives.table.usage")}</p>
				<p class="text-2xl font-bold">{total > 0 ? ((used / total) * 100).toFixed(1) : "0"}%</p>
			</div>
		</div>
	{/if}

	<StatsChart
		title={mountpoint}
		data={data}
		height="350px"
		formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
		formatY={(v: number) => formatBytes(v)}
	/>
</div>
