<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import MigrationStatusIndicator from '$lib/components/migration-status-indicator.svelte';

	let { children } = $props();

	const publicRoutes = ['/login', '/register'];

	onMount(() => {
		const unsub = auth.isLoggedIn.subscribe((loggedIn) => {
			const pathname = window.location.pathname;
			if (pathname === '/change-password') return;
			if (!loggedIn && !publicRoutes.includes(pathname)) {
				goto('/login');
			}
		});

		const unsub2 = auth.passwordChangeRequired.subscribe((required) => {
			if (required && window.location.pathname !== '/change-password') {
				goto('/change-password');
			}
		});

		return () => {
			unsub();
			unsub2();
		};
	});
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<Sidebar.Provider>
	<AppSidebar />
	<main class="flex-1 p-6">
		<div class="flex items-start justify-between">
			<Sidebar.Trigger />
			<MigrationStatusIndicator />
		</div>
		{@render children()}
	</main>
</Sidebar.Provider>
