<script lang="ts">
	import { goto } from '$app/navigation';
	import { createBackupTarget } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { LoaderCircle, ArrowLeft } from '@lucide/svelte';

	type Provider = 's3' | 'sftp' | 'local';

	let name = $state('');
	let provider = $state<Provider>('s3');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	let s3Config = $state({ endpoint: '', bucket: '', region: '', access_key: '', secret_key: '', path_style: false });
	let sftpConfig = $state({ host: '', port: 22, user: '', password: '', base_path: '' });
	let localConfig = $state({ path: '' });

	function getConfig(): Record<string, unknown> {
		switch (provider) {
			case 's3': return { ...s3Config };
			case 'sftp': return { ...sftpConfig };
			case 'local': return { ...localConfig };
		}
	}

	async function handleSubmit() {
		if (!name.trim()) {
			error = 'Name is required';
			return;
		}
		submitting = true;
		error = null;
		try {
			await createBackupTarget({ name: name.trim(), provider, config: getConfig() });
			goto('/backup/targets');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create target';
		} finally {
			submitting = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<a href="/backup/targets" class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
			<ArrowLeft class="size-4" />
			Back to targets
		</a>
		<h1 class="mt-2 text-2xl font-bold tracking-tight">New Backup Target</h1>
		<p class="text-muted-foreground">Configure a new backup destination</p>
	</div>

	<Card.Root class="max-w-xl">
		<Card.Content class="space-y-4 pt-6">
			<div class="space-y-2">
				<label class="text-sm font-medium">Name</label>
				<Input bind:value={name} placeholder="My backup target" disabled={submitting} />
			</div>

			<div class="space-y-2">
				<label class="text-sm font-medium">Provider</label>
				<select
					class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
					bind:value={provider}
					disabled={submitting}
				>
					<option value="s3">Amazon S3 (or compatible)</option>
					<option value="sftp">SFTP</option>
					<option value="local">Local Path</option>
				</select>
			</div>

			{#if provider === 's3'}
				<div class="space-y-3 rounded-lg border p-4">
					<h3 class="text-sm font-semibold">S3 Configuration</h3>
					<div class="space-y-2">
						<label class="text-sm font-medium">Endpoint</label>
						<Input bind:value={s3Config.endpoint} placeholder="https://s3.amazonaws.com" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Bucket</label>
						<Input bind:value={s3Config.bucket} placeholder="my-bucket" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Region</label>
						<Input bind:value={s3Config.region} placeholder="us-east-1" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Access Key</label>
						<Input bind:value={s3Config.access_key} placeholder="AKIA..." disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Secret Key</label>
						<Input bind:value={s3Config.secret_key} type="password" placeholder="••••••••" disabled={submitting} />
					</div>
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={s3Config.path_style} class="accent-primary" disabled={submitting} />
						Use path-style addressing
					</label>
				</div>
			{:else if provider === 'sftp'}
				<div class="space-y-3 rounded-lg border p-4">
					<h3 class="text-sm font-semibold">SFTP Configuration</h3>
					<div class="space-y-2">
						<label class="text-sm font-medium">Host</label>
						<Input bind:value={sftpConfig.host} placeholder="example.com" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Port</label>
						<Input bind:value={sftpConfig.port} type="number" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Username</label>
						<Input bind:value={sftpConfig.user} placeholder="username" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Password</label>
						<Input bind:value={sftpConfig.password} type="password" placeholder="••••••••" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label class="text-sm font-medium">Base Path</label>
						<Input bind:value={sftpConfig.base_path} placeholder="/backups" disabled={submitting} />
					</div>
				</div>
			{:else if provider === 'local'}
				<div class="space-y-3 rounded-lg border p-4">
					<h3 class="text-sm font-semibold">Local Configuration</h3>
					<div class="space-y-2">
						<label class="text-sm font-medium">Path</label>
						<Input bind:value={localConfig.path} placeholder="/mnt/backup" disabled={submitting} />
					</div>
				</div>
			{/if}

			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}

			<div class="flex justify-end gap-2 pt-2">
				<a href="/backup/targets">
					<Button variant="outline" disabled={submitting}>Cancel</Button>
				</a>
				<Button onclick={handleSubmit} disabled={submitting || !name.trim()}>
					{#if submitting}
						<LoaderCircle class="size-4 animate-spin" />
					{/if}
					Create Target
				</Button>
			</div>
		</Card.Content>
	</Card.Root>
</div>
