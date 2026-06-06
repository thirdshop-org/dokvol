<script lang="ts">
	import { page } from '$app/stores';
	import { t } from '$lib/i18n';
	import Box from '@lucide/svelte/icons/box';
	import HardDrive from '@lucide/svelte/icons/hard-drive';
	import Home from '@lucide/svelte/icons/house';
	import Settings from '@lucide/svelte/icons/settings';
	import AreaChart from '@lucide/svelte/icons/area-chart';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	const links = [
		{ href: '/', label: 'nav.home', icon: Home },
		{ href: '/drives', label: 'nav.drives', icon: HardDrive },
		{ href: '/applications', label: 'nav.applications', icon: Box },
		{ href: '/stats', label: 'nav.stats', icon: AreaChart },
		{ href: '/preferences', label: 'nav.preferences', icon: Settings },
	];
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
	<Sidebar.Rail />
</Sidebar.Root>
