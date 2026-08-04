<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { t } from '$lib/i18n';
	import { getVersion, logout as apiLogout } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import type { VersionResponse } from '$lib/types';
import Box from '@lucide/svelte/icons/box';
import HardDrive from '@lucide/svelte/icons/hard-drive';
import Home from '@lucide/svelte/icons/house';
import Settings from '@lucide/svelte/icons/settings';
import AreaChart from '@lucide/svelte/icons/area-chart';
import HistoryIcon from '@lucide/svelte/icons/clock';
import LogOut from '@lucide/svelte/icons/log-out';
import UserIcon from '@lucide/svelte/icons/user';
import UsersIcon from '@lucide/svelte/icons/users';
import Database from '@lucide/svelte/icons/database';
import Trash2 from '@lucide/svelte/icons/trash-2';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import Badge from '$lib/components/ui/badge/badge.svelte';

	const VERSION_URL = 'https://raw.githubusercontent.com/thirdshop-org/dokvol/main/VERSION';

	const baseLinks = [
		{ href: '/', label: 'nav.home', icon: Home },
		{ href: '/drives', label: 'nav.drives', icon: HardDrive },
		{ href: '/applications', label: 'nav.applications', icon: Box },
		{ href: '/volumes/trash', label: 'nav.trash', icon: Trash2 },
		{ href: '/backup', label: 'nav.backup', icon: Database },
		{ href: '/history', label: 'nav.history', icon: HistoryIcon },
		{ href: '/stats', label: 'nav.stats', icon: AreaChart },
		{ href: '/preferences', label: 'nav.preferences', icon: Settings },
	];

	let version = $state<VersionResponse | null>(null);
	let latestTag = $state<string | null>(null);
	let updateCheckFailed = $state(false);

	let currentUser = $state(auth.user);
	let isLoggedIn = $state(false);
	let isAdmin = $state(false);

	auth.user.subscribe(u => currentUser = u);
	auth.isLoggedIn.subscribe(l => isLoggedIn = l);
	auth.isAdmin.subscribe(a => isAdmin = a);

	let links = $derived(
		isAdmin
			? [...baseLinks, { href: '/users', label: 'nav.users', icon: UsersIcon }]
			: baseLinks
	);

	function semverGt(a: string, b: string): boolean {
		const pa = a.replace(/^v/, '').split('.').map(Number);
		const pb = b.replace(/^v/, '').split('.').map(Number);
		for (let i = 0; i < 3; i++) {
			if (pa[i] !== pb[i]) return pa[i] > pb[i];
		}
		return false;
	}

	let updateAvailable = $derived(
		version && latestTag && version.version
			? semverGt(latestTag, version.version)
			: false
	);

	async function handleLogout() {
		const rt = auth.getRefreshToken();
		if (rt) {
			try {
				await apiLogout({ refresh_token: rt });
			} catch {
				// silencieux
			}
		}
		auth.logout();
		goto('/login');
	}

	onMount(async () => {
		try {
			version = await getVersion();
		} catch {
			// silencieux
		}

		try {
			const res = await fetch(VERSION_URL);
			if (res.ok) {
				latestTag = (await res.text()).trim();
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
		{#if isLoggedIn && currentUser}
			<div class="flex items-center justify-between px-2 py-1 border-b border-border mb-1">
				<div class="flex items-center gap-2 min-w-0">
					<UserIcon class="size-4 shrink-0 text-muted-foreground" />
					<span class="text-sm truncate">{currentUser.username}</span>
				</div>
				<button onclick={handleLogout} class="text-muted-foreground hover:text-foreground transition-colors" title={$t('auth.logout')}>
					<LogOut class="size-4" />
				</button>
			</div>
		{/if}
		{#if version}
			<div class="flex items-center justify-between px-2 py-1">
				<span class="text-xs text-muted-foreground">
					{$t('update.currentVersion', { version: version.version.replace(/^v/, '') })}
				</span>
				<div class="flex items-center gap-1">
					<ThemeToggle />
					{#if updateCheckFailed}
						<span class="text-xs text-muted-foreground">{$t('update.checkFailed')}</span>
					{:else if updateAvailable}
						<Badge variant="warning">{$t('update.available')}</Badge>
					{:else if latestTag}
						<span class="text-xs text-green-600 dark:text-green-400">{$t('update.latest')}</span>
					{/if}
				</div>
			</div>
		{/if}
	</Sidebar.Footer>
	<Sidebar.Rail />
</Sidebar.Root>
