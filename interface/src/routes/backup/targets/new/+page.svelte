<script lang="ts">
	import { goto } from '$app/navigation';
	import { createBackupTarget } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { LoaderCircle, ArrowLeft } from '@lucide/svelte';
	import { errorMessage } from '$lib/utils/errors';
	import { t } from '$lib/i18n';

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
			error = $t('backup.newTargetPage.nameRequired');
			return;
		}
		submitting = true;
		error = null;
		try {
			await createBackupTarget({ name: name.trim(), provider, config: getConfig() });
			goto('/backup/targets');
		} catch (e) {
			error = errorMessage(e);
		} finally {
			submitting = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<a href="/backup/targets" class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
			<ArrowLeft class="size-4" />
			{$t('backup.newTargetPage.backToTargets')}
		</a>
		<h1 class="mt-2 text-2xl font-bold tracking-tight">{$t('backup.newTargetPage.title')}</h1>
		<p class="text-muted-foreground">{$t('backup.newTargetPage.description')}</p>
	</div>

	<Card.Root class="max-w-xl">
		<Card.Content class="space-y-4 pt-6">
			<div class="space-y-2">
				<label for="target-name" class="text-sm font-medium">{$t('backup.newTargetPage.name')}</label>
				<Input id="target-name" bind:value={name} placeholder={$t('backup.newTargetPage.namePlaceholder')} disabled={submitting} />
			</div>

			<div class="space-y-2">
				<label for="target-provider" class="text-sm font-medium">{$t('backup.newTargetPage.provider')}</label>
				<select
					id="target-provider"
					class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
					bind:value={provider}
					disabled={submitting}
				>
					<option value="s3">{$t('backup.newTargetPage.providerS3')}</option>
					<option value="sftp">{$t('backup.newTargetPage.providerSftp')}</option>
					<option value="local">{$t('backup.newTargetPage.providerLocal')}</option>
				</select>
			</div>

			{#if provider === 's3'}
				<div class="space-y-3 rounded-lg border p-4">
					<h3 class="text-sm font-semibold">{$t('backup.newTargetPage.s3.title')}</h3>
					<div class="space-y-2">
						<label for="s3-endpoint" class="text-sm font-medium">{$t('backup.newTargetPage.s3.endpoint')}</label>
						<Input id="s3-endpoint" bind:value={s3Config.endpoint} placeholder="https://s3.amazonaws.com" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="s3-bucket" class="text-sm font-medium">{$t('backup.newTargetPage.s3.bucket')}</label>
						<Input id="s3-bucket" bind:value={s3Config.bucket} placeholder="my-bucket" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="s3-region" class="text-sm font-medium">{$t('backup.newTargetPage.s3.region')}</label>
						<Input id="s3-region" bind:value={s3Config.region} placeholder="us-east-1" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="s3-access-key" class="text-sm font-medium">{$t('backup.newTargetPage.s3.accessKey')}</label>
						<Input id="s3-access-key" bind:value={s3Config.access_key} placeholder="AKIA..." disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="s3-secret-key" class="text-sm font-medium">{$t('backup.newTargetPage.s3.secretKey')}</label>
						<Input id="s3-secret-key" bind:value={s3Config.secret_key} type="password" placeholder="••••••••" disabled={submitting} />
					</div>
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={s3Config.path_style} class="accent-primary" disabled={submitting} />
						{$t('backup.newTargetPage.s3.pathStyle')}
					</label>
				</div>
			{:else if provider === 'sftp'}
				<div class="space-y-3 rounded-lg border p-4">
					<h3 class="text-sm font-semibold">{$t('backup.newTargetPage.sftp.title')}</h3>
					<div class="space-y-2">
						<label for="sftp-host" class="text-sm font-medium">{$t('backup.newTargetPage.sftp.host')}</label>
						<Input id="sftp-host" bind:value={sftpConfig.host} placeholder="example.com" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="sftp-port" class="text-sm font-medium">{$t('backup.newTargetPage.sftp.port')}</label>
						<Input id="sftp-port" bind:value={sftpConfig.port} type="number" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="sftp-user" class="text-sm font-medium">{$t('backup.newTargetPage.sftp.username')}</label>
						<Input id="sftp-user" bind:value={sftpConfig.user} placeholder="username" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="sftp-password" class="text-sm font-medium">{$t('backup.newTargetPage.sftp.password')}</label>
						<Input id="sftp-password" bind:value={sftpConfig.password} type="password" placeholder="••••••••" disabled={submitting} />
					</div>
					<div class="space-y-2">
						<label for="sftp-base-path" class="text-sm font-medium">{$t('backup.newTargetPage.sftp.basePath')}</label>
						<Input id="sftp-base-path" bind:value={sftpConfig.base_path} placeholder="/backups" disabled={submitting} />
					</div>
				</div>
			{:else if provider === 'local'}
				<div class="space-y-3 rounded-lg border p-4">
					<h3 class="text-sm font-semibold">{$t('backup.newTargetPage.local.title')}</h3>
					<div class="space-y-2">
						<label for="local-path" class="text-sm font-medium">{$t('backup.newTargetPage.local.path')}</label>
						<Input id="local-path" bind:value={localConfig.path} placeholder="/mnt/backup" disabled={submitting} />
					</div>
				</div>
			{/if}

			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}

			<div class="flex justify-end gap-2 pt-2">
				<a href="/backup/targets">
					<Button variant="outline" disabled={submitting}>{$t('backup.newTargetPage.cancel')}</Button>
				</a>
				<Button onclick={handleSubmit} disabled={submitting || !name.trim()}>
					{#if submitting}
						<LoaderCircle class="size-4 animate-spin" />
					{/if}
					{$t('backup.newTargetPage.create')}
				</Button>
			</div>
		</Card.Content>
	</Card.Root>
</div>
