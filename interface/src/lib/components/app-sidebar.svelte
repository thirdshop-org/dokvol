<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { t } from '$lib/i18n';
	import { getVersion } from '$lib/api';
	import type { VersionResponse } from '$lib/types';
	import Box from '@lucide/svelte/icons/box';
	import HardDrive from '@lucide/svelte/icons/hard-drive';
	import Home from '@lucide/svelte/icons/house';
	import Settings from '@lucide/svelte/icons/settings';
	import AreaChart from '@lucide/svelte/icons/area-chart';
	import HistoryIcon from '@lucide/svelte/icons/clock';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import Badge from '$lib/components/ui/badge/badge.svelte';

	const GITHUB_API = 'https://api.github.com/repos/thirdshop-org/dokvol/releases/latest';

	const links = [
		{ href: '/', label: 'nav.home', icon: Home },
		{ href: '/drives', label: 'nav.drives', icon: HardDrive },
		{ href: '/applications', label: 'nav.applications', icon: Box },
		{ href: '/history', label: 'nav.history', icon: HistoryIcon },
		{ href: '/stats', label: 'nav.stats', icon: AreaChart },
		{ href: '/preferences', label: 'nav.preferences', icon: Settings },
	];

	let version = $state<VersionResponse | null>(null);
	let latestTag = $state<string | null>(null);
	let updateCheckFailed = $state(false);

	function semverGt(a: string, b: string): boolean {
		const pa = a.replace(/^v/, '').split('.').map(Number);
		const pb = b.replace(/^v/, '').split('.').map(Number);
		for (let i = 0; i < 3; i++) {
			if (pa[i] !== pb[i]) return pa[i] > pb[i];
		}
		return false;
	}

	let updateAvailable = $derived(
		version && latestTag && !version.version.startsWith('0.0.1')
			? semverGt(latestTag, version.version)
			: false
	);

	onMount(async () => {
		try {
			version = await getVersion();
		} catch {
			// silencieux
		}

		try {
			const res = await fetch(GITHUB_API);
			if (res.ok) {
				const data = await res.json();
				latestTag = data.tag_name as string;
			} else {
				updateCheckFailed = true;
			}
		} catch {
			updateCheckFailed = true;
		}
	});
</script>

<Sidebar.Root>
	<Sidebar.Header>
		<Sidebar.Menu>
			<Sidebar.MenuItem>
				<Sidebar.MenuButton size="lg">
					{#snippet child({ props })}
						<a href="/" {...props}>
							<Box class="size-5" />
							<span>{$t('app.name')}</span>
						</a>
					{/snippet}
				</Sidebar.MenuButton>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Header>
	<Sidebar.Content>
		<Sidebar.Group>
			<Sidebar.GroupLabel>{$t('nav.navigation')}</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				<Sidebar.Menu>
					{#each links as { href, label, icon: Icon } (href)}
						<Sidebar.MenuItem>
							<Sidebar.MenuButton isActive={$page.url.pathname === href}>
								{#snippet child({ props })}
									<a href={href} {...props}>
										<Icon />
										<span>{$t(label)}</span>
									</a>
								{/snippet}
							</Sidebar.MenuButton>
						</Sidebar.MenuItem>
					{/each}
				</Sidebar.Menu>
			</Sidebar.GroupContent>
		</Sidebar.Group>
	</Sidebar.Content>
	<Sidebar.Footer>
		{#if version}
			<div class="flex items-center justify-between px-2 py-1">
				<span class="text-xs text-muted-foreground">
					{$t('update.currentVersion', { version: version.version.replace(/^v/, '') })}
				</span>
				{#if updateAvailable}
					<Badge variant="warning">{$t('update.available')}</Badge>
				{:else if latestTag && !updateAvailable}
					<span class="text-xs text-green-600 dark:text-green-400">{$t('update.latest')}</span>
				{/if}
			</div>
		{/if}
	</Sidebar.Footer>
	<Sidebar.Rail />
</Sidebar.Root>
