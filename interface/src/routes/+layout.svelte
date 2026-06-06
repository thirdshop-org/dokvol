<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import MigrationStatusIndicator from '$lib/components/migration-status-indicator.svelte';

	let { children } = $props();
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
