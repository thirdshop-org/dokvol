<script lang="ts">
	import { onMount } from "svelte";
	import { t } from "$lib/i18n";
	import { getPreferences } from "$lib/api";
	import * as Card from "$lib/components/ui/card/index.js";
	import { Settings } from "@lucide/svelte";

	let prefs = $state<Record<string, string>>({});
	let ready = $state(false);

	onMount(async () => {
		try {
			prefs = await getPreferences();
		} catch {
			// silent
		} finally {
			ready = true;
		}
	});

	const knownKeys: Record<string, { label: string; hint: string }> = {
		stats_interval_seconds: {
			label: "preferences.stats_interval_seconds",
			hint: "preferences.stats_interval_seconds_hint",
		},
		stats_retention_days: {
			label: "preferences.stats_retention_days",
			hint: "preferences.stats_retention_days_hint",
		},
	};

	function keyLabel(k: string): string {
		return knownKeys[k] ? $t(knownKeys[k].label) : k;
	}

	function keyHint(k: string): string {
		return knownKeys[k] ? $t(knownKeys[k].hint) : "";
	}

	function formatValue(k: string, v: string): string {
		if (k === "stats_interval_seconds") {
			const secs = parseInt(v, 10);
			if (secs >= 3600) return `${secs / 3600} h`;
			if (secs >= 60) return `${secs / 60} min`;
			return `${secs} s`;
		}
		if (k === "stats_retention_days") {
			const days = parseInt(v, 10);
			return `${days} j`;
		}
		return v;
	}
</script>

<div class="space-y-6">
	<div class="flex items-start justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight flex items-center gap-2">
				<Settings class="size-6" /> {$t("preferences.title")}
			</h1>
			<p class="text-muted-foreground">{$t("preferences.description")}</p>
		</div>
	</div>

	{#if !ready}
		<p class="text-muted-foreground">{$t("preferences.loading")}</p>
	{:else if Object.keys(prefs).length === 0}
		<p class="text-muted-foreground">{$t("preferences.noData")}</p>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2">
			{#each Object.entries(prefs) as [key, value]}
				<Card.Root>
					<Card.Header>
						<Card.Title>{keyLabel(key)}</Card.Title>
						{#if keyHint(key)}
							<Card.Description>{keyHint(key)}</Card.Description>
						{/if}
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold tabular-nums">{formatValue(key, value)}</p>
						<p class="mt-1 text-xs text-muted-foreground font-mono">{key} = {value}</p>
					</Card.Content>
				</Card.Root>
			{/each}
		</div>
	{/if}
</div>
