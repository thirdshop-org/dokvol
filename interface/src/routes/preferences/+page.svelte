<script lang="ts">
	import { onMount } from "svelte";
	import { t } from "$lib/i18n";
	import { getPreferences, updatePreference } from "$lib/api";
	import { errorMessage } from "$lib/utils/errors";
	import * as Card from "$lib/components/ui/card/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Settings, LoaderCircle, Check } from "@lucide/svelte";

	let prefs = $state<Record<string, string>>({});
	let edited = $state<Record<string, string>>({});
	let ready = $state(false);
	let loadError = $state<string | null>(null);

	let saving = $state<Record<string, boolean>>({});
	let saveError = $state<Record<string, string | null>>({});
	let saved = $state<Record<string, boolean>>({});

	onMount(async () => {
		try {
			prefs = await getPreferences();
			edited = { ...prefs };
		} catch (e) {
			loadError = errorMessage(e);
		} finally {
			ready = true;
		}
	});

	const knownKeys: Record<string, { label: string; hint: string; unit: string }> = {
		stats_interval_seconds: {
			label: "preferences.stats_interval_seconds",
			hint: "preferences.stats_interval_seconds_hint",
			unit: "s",
		},
		stats_retention_days: {
			label: "preferences.stats_retention_days",
			hint: "preferences.stats_retention_days_hint",
			unit: "d",
		},
	};

	function keyLabel(k: string): string {
		return knownKeys[k] ? $t(knownKeys[k].label) : k;
	}

	function keyHint(k: string): string {
		return knownKeys[k] ? $t(knownKeys[k].hint) : "";
	}

	function isDirty(k: string): boolean {
		return edited[k] !== prefs[k];
	}

	async function handleSave(k: string) {
		const value = edited[k];
		saving = { ...saving, [k]: true };
		saveError = { ...saveError, [k]: null };
		saved = { ...saved, [k]: false };
		try {
			await updatePreference(k, value);
			prefs = { ...prefs, [k]: value };
			saved = { ...saved, [k]: true };
			setTimeout(() => { saved = { ...saved, [k]: false }; }, 2000);
		} catch (e) {
			saveError = { ...saveError, [k]: errorMessage(e) };
		} finally {
			saving = { ...saving, [k]: false };
		}
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
	{:else if loadError}
		<p class="text-destructive">{loadError}</p>
	{:else if Object.keys(prefs).length === 0}
		<p class="text-muted-foreground">{$t("preferences.noData")}</p>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2">
			{#each Object.entries(prefs) as [key]}
				<Card.Root>
					<Card.Header>
						<Card.Title>{keyLabel(key)}</Card.Title>
						{#if keyHint(key)}
							<Card.Description>{keyHint(key)}</Card.Description>
						{/if}
					</Card.Header>
					<Card.Content class="space-y-3">
						<div class="flex items-center gap-2">
							<Input
								type="number"
								min="1"
								bind:value={edited[key]}
								disabled={saving[key]}
								class="max-w-32"
							/>
							{#if knownKeys[key]}
								<span class="text-sm text-muted-foreground">{knownKeys[key].unit}</span>
							{/if}
							<Button size="sm" onclick={() => handleSave(key)} disabled={saving[key] || !isDirty(key)}>
								{#if saving[key]}
									<LoaderCircle class="size-3.5 animate-spin" />
								{:else if saved[key]}
									<Check class="size-3.5" />
								{/if}
								{$t("preferences.save")}
							</Button>
						</div>
						{#if saveError[key]}
							<p class="text-xs text-destructive">{saveError[key]}</p>
						{/if}
						<p class="text-xs text-muted-foreground font-mono">{key} = {prefs[key]}</p>
					</Card.Content>
				</Card.Root>
			{/each}
		</div>
	{/if}
</div>
