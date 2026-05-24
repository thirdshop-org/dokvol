<script lang="ts">
	import { page } from '$app/stores';
	import Box from '@lucide/svelte/icons/box';
	import HardDrive from '@lucide/svelte/icons/hard-drive';
	import Home from '@lucide/svelte/icons/house';
	import Layers from '@lucide/svelte/icons/layers';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	const links = [
		{ href: '/', label: 'Accueil', icon: Home },
		{ href: '/drives', label: 'Disques', icon: HardDrive },
		{ href: '/volumes', label: 'Volumes', icon: Layers },
		{ href: '/applications', label: 'Applications', icon: Box },
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
							<span>DokVol</span>
						</a>
					{/snippet}
				</Sidebar.MenuButton>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Header>
	<Sidebar.Content>
		<Sidebar.Group>
			<Sidebar.GroupLabel>Navigation</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				<Sidebar.Menu>
					{#each links as { href, label, icon: Icon } (href)}
						<Sidebar.MenuItem>
							<Sidebar.MenuButton isActive={$page.url.pathname === href}>
								{#snippet child({ props })}
									<a href={href} {...props}>
										<Icon />
										<span>{label}</span>
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
