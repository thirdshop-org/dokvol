<script lang="ts">
	import { onMount } from 'svelte';
	import { browseVolume, readVolumeFile } from '$lib/api';
	import type { FileEntry } from '$lib/types';
	import { t } from '$lib/i18n';
	import Folder from '@lucide/svelte/icons/folder';
	import FileText from '@lucide/svelte/icons/file-text';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';

	let { container, initialPath = '/', volumeName }: {
		container: string;
		initialPath?: string;
		volumeName?: string;
	} = $props();

	let currentPath = $state(initialPath);
	let entries = $state<FileEntry[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let previewPath = $state<string | null>(null);
	let previewContent = $state<string | null>(null);
	let previewLoading = $state(false);
	let previewError = $state<string | null>(null);
	let previewBinary = $state(false);
	let previewTruncated = $state(false);

	let pathParts = $derived(currentPath.split('/').filter(Boolean));
	let homePath = $derived('/' + pathParts[0]);

	function formatSize(bytes: number): string {
		if (!Number.isFinite(bytes) || bytes <= 0) return '—';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
	}

	function formatTime(t: string): string {
		try { return new Date(t).toLocaleString(); }
		catch { return t; }
	}

	function goUp() {
		if (currentPath === '/') return;
		const parts = currentPath.replace(/\/$/, '').split('/');
		parts.pop();
		navigate(parts.join('/') || '/');
	}

	async function navigate(path: string) {
		currentPath = path;
		previewPath = null;
		previewContent = null;
		loading = true;
		error = null;
		try {
			const res = await browseVolume({ container, path });
			entries = res.entries;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Erreur inconnue';
			entries = [];
		} finally {
			loading = false;
		}
	}

	async function openFile(path: string) {
		previewPath = path;
		previewContent = null;
		previewError = null;
		previewBinary = false;
		previewTruncated = false;
		previewLoading = true;
		try {
			const res = await readVolumeFile({ container, path });
			if (res.binary) {
				previewBinary = true;
			} else {
				previewContent = res.content;
				previewTruncated = res.truncated;
			}
		} catch (e) {
			previewError = e instanceof Error ? e.message : 'Erreur inconnue';
		} finally {
			previewLoading = false;
		}
	}

	function closePreview() {
		previewPath = null;
		previewContent = null;
		previewError = null;
		previewBinary = false;
	}

	onMount(() => {
		navigate(currentPath);
	});
</script>

<div class="flex flex-col h-full">
	<header class="px-6 pt-4 pb-2">
		<div class="flex items-center gap-2 mb-1">
			{#if volumeName}
				<span class="text-xs text-muted-foreground font-medium">{volumeName}</span>
			{/if}
		</div>
		{#if previewPath}
			<button onclick={closePreview} class="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors">
				<ArrowLeft class="size-3.5" />
				{$t('fileExplorer.backToFiles')}
			</button>
			<div class="flex items-center gap-1.5 mt-1 text-xs font-mono text-muted-foreground">
				<span class="truncate">{previewPath}</span>
			</div>
		{:else}
			<nav class="flex items-center gap-1 text-xs font-mono text-muted-foreground flex-wrap">
				<button onclick={() => navigate('/')} class="hover:text-foreground transition-colors shrink-0">/</button>
				{#each pathParts as part, i}
					<ChevronRight class="size-3 shrink-0" />
					<button
						onclick={() => navigate('/' + pathParts.slice(0, i + 1).join('/'))}
						class="hover:text-foreground transition-colors truncate max-w-[120px]"
					>{part}</button>
				{/each}
			</nav>
		{/if}
	</header>

	<Separator />

	<div class="flex-1 overflow-y-auto px-6 py-2">
		{#if loading || previewLoading}
			<div class="flex items-center justify-center py-12 text-sm text-muted-foreground">
				{$t('fileExplorer.loading')}
			</div>
		{:else if error && !previewPath}
			<div class="text-sm text-destructive py-4">{$t('fileExplorer.error')}: {error}</div>
		{:else if previewPath}
			{#if previewError}
				<div class="text-sm text-destructive py-4">{$t('fileExplorer.error')}: {previewError}</div>
			{:else if previewBinary}
				<div class="flex flex-col items-center justify-center py-12 text-sm text-muted-foreground gap-2">
					<FileText class="size-8 opacity-40" />
					<span>{$t('fileExplorer.binaryFile')}</span>
				</div>
			{:else}
				<div class="relative">
					{#if previewTruncated}
						<div class="sticky top-0 mb-2 text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/50 px-3 py-1.5 rounded border border-amber-200 dark:border-amber-800">
							{$t('fileExplorer.truncated')}
						</div>
					{/if}
					<pre class="text-xs font-mono leading-relaxed whitespace-pre-wrap break-all max-h-[70vh] overflow-y-auto rounded border bg-muted/30 p-3"><code>{previewContent}</code></pre>
				</div>
			{/if}
		{:else if entries.length === 0}
			<div class="flex flex-col items-center justify-center py-12 text-sm text-muted-foreground gap-2">
				<Folder class="size-8 opacity-40" />
				<span>{$t('fileExplorer.empty')}</span>
			</div>
		{:else}
			<div class="w-full text-sm">
				<div class="grid grid-cols-[1fr_auto_auto] gap-2 py-1.5 text-xs text-muted-foreground font-medium border-b">
					<span>{$t('fileExplorer.name')}</span>
					<span class="w-20 text-right">{$t('fileExplorer.size')}</span>
					<span class="w-16 text-center">{$t('fileExplorer.type')}</span>
				</div>
				{#if currentPath !== '/'}
					<button
						onclick={goUp}
						class="grid grid-cols-[1fr_auto_auto] gap-2 py-2 border-b hover:bg-muted/30 transition-colors w-full text-left"
					>
						<span class="flex items-center gap-2 text-muted-foreground">
							<Folder class="size-4 shrink-0" />
							<span class="truncate">..</span>
						</span>
						<span class="w-20 text-right text-muted-foreground">—</span>
						<span class="w-16 text-center text-muted-foreground">—</span>
					</button>
				{/if}
				{#each entries as entry}
					{#if entry.is_dir}
						<button
							onclick={() => navigate((currentPath.endsWith('/') ? currentPath : currentPath + '/') + entry.name)}
							class="grid grid-cols-[1fr_auto_auto] gap-2 py-2 border-b hover:bg-muted/30 transition-colors w-full text-left"
						>
							<span class="flex items-center gap-2">
								<Folder class="size-4 shrink-0 text-blue-500" />
								<span class="truncate">{entry.name}</span>
							</span>
							<span class="w-20 text-right text-muted-foreground text-xs">—</span>
							<span class="w-16 text-center">
								<Badge variant="outline" class="text-[10px] px-1 py-0">{$t('fileExplorer.folder')}</Badge>
							</span>
						</button>
					{:else}
						<button
							onclick={() => openFile((currentPath.endsWith('/') ? currentPath : currentPath + '/') + entry.name)}
							class="grid grid-cols-[1fr_auto_auto] gap-2 py-2 border-b hover:bg-muted/30 transition-colors w-full text-left"
						>
							<span class="flex items-center gap-2">
								<FileText class="size-4 shrink-0 text-muted-foreground" />
								<span class="truncate">{entry.name}</span>
							</span>
							<span class="w-20 text-right text-muted-foreground text-xs font-mono">{formatSize(entry.size)}</span>
							<span class="w-16 text-center">
								<Badge variant="outline" class="text-[10px] px-1 py-0">{entry.mode.substring(0, 3)}</Badge>
							</span>
						</button>
					{/if}
				{/each}
			</div>
		{/if}
	</div>
</div>
