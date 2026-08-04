<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { t } from '$lib/i18n';
	import { auth } from '$lib/stores/auth.svelte';
	import { getUsers, createUser, deleteUser, ApiError } from '$lib/api';
	import type { User } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { LoaderCircle, Plus, Trash2, Users as UsersIcon } from '@lucide/svelte';

	let isAdmin = $state(false);
	auth.isAdmin.subscribe((v) => (isAdmin = v));

	let users = $state<User[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let showForm = $state(false);
	let newUsername = $state('');
	let newEmail = $state('');
	let newPassword = $state('');
	let creating = $state(false);
	let createError = $state<string | null>(null);

	let confirmDeleteUser = $state<User | null>(null);
	let deleting = $state(false);

	function errorMessage(e: unknown): string {
		if (e instanceof ApiError) {
			const key = `error.${e.errorCode}`;
			const msg = $t(key);
			return msg !== key ? msg : e.message;
		}
		return e instanceof Error ? e.message : $t('error.default');
	}

	async function load() {
		loading = true;
		error = null;
		try {
			users = await getUsers();
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		if (!isAdmin) {
			goto('/');
			return;
		}
		load();
	});

	async function handleCreate() {
		if (!newUsername || !newEmail || !newPassword) return;
		creating = true;
		createError = null;
		try {
			await createUser({ username: newUsername, email: newEmail, password: newPassword });
			newUsername = '';
			newEmail = '';
			newPassword = '';
			showForm = false;
			await load();
		} catch (e) {
			createError = errorMessage(e);
		} finally {
			creating = false;
		}
	}

	async function handleDelete() {
		if (!confirmDeleteUser) return;
		deleting = true;
		try {
			await deleteUser(confirmDeleteUser.id);
			confirmDeleteUser = null;
			await load();
		} catch (e) {
			error = errorMessage(e);
			confirmDeleteUser = null;
		} finally {
			deleting = false;
		}
	}

	function formatDate(s: string): string {
		if (!s) return '—';
		return new Date(s).toLocaleDateString();
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight flex items-center gap-2">
				<UsersIcon class="size-6" /> {$t('users.title')}
			</h1>
			<p class="text-muted-foreground">{$t('users.description')}</p>
		</div>
		<Button onclick={() => (showForm = !showForm)}>
			<Plus class="size-4" />
			{$t('users.newUser')}
		</Button>
	</div>

	{#if showForm}
		<Card.Root class="max-w-lg">
			<Card.Content class="space-y-4 pt-6">
				<div class="space-y-2">
					<label for="new-username" class="text-sm font-medium">{$t('auth.username')}</label>
					<Input id="new-username" bind:value={newUsername} disabled={creating} />
				</div>
				<div class="space-y-2">
					<label for="new-email" class="text-sm font-medium">{$t('auth.email')}</label>
					<Input id="new-email" type="email" bind:value={newEmail} disabled={creating} />
				</div>
				<div class="space-y-2">
					<label for="new-password" class="text-sm font-medium">{$t('users.temporaryPassword')}</label>
					<Input id="new-password" type="password" bind:value={newPassword} disabled={creating} />
					<p class="text-xs text-muted-foreground">{$t('users.createDesc')}</p>
				</div>
				{#if createError}
					<p class="text-sm text-destructive">{createError}</p>
				{/if}
				<div class="flex justify-end">
					<Button onclick={handleCreate} disabled={creating || !newUsername || !newEmail || !newPassword}>
						{#if creating}<LoaderCircle class="size-4 animate-spin" />{/if}
						{$t('users.create')}
					</Button>
				</div>
			</Card.Content>
		</Card.Root>
	{/if}

	{#if loading}
		<p class="text-muted-foreground">{$t('users.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if users.length === 0}
		<div class="rounded-lg border border-dashed p-12 text-center">
			<p class="text-muted-foreground">{$t('users.empty')}</p>
		</div>
	{:else}
		<div class="rounded-lg border">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>{$t('users.table.username')}</Table.Head>
						<Table.Head>{$t('users.table.email')}</Table.Head>
						<Table.Head>{$t('users.table.role')}</Table.Head>
						<Table.Head>{$t('users.table.createdAt')}</Table.Head>
						<Table.Head class="text-right">{$t('users.table.actions')}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each users as user (user.id)}
						<Table.Row>
							<Table.Cell class="font-medium">{user.username}</Table.Cell>
							<Table.Cell class="text-muted-foreground">{user.email}</Table.Cell>
							<Table.Cell>
								<Badge variant={user.role === 'admin' ? 'default' : 'outline'}>{user.role}</Badge>
							</Table.Cell>
							<Table.Cell class="text-muted-foreground">{formatDate(user.created_at)}</Table.Cell>
							<Table.Cell class="text-right">
								<Button
									size="sm"
									variant="outline"
									onclick={() => (confirmDeleteUser = user)}
									disabled={user.role === 'admin'}
									title={user.role === 'admin' ? undefined : $t('users.delete')}
								>
									<Trash2 class="size-3.5" />
								</Button>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	{/if}
</div>

<Dialog.Root open={confirmDeleteUser !== null} onOpenChange={(v) => { if (!v) confirmDeleteUser = null; }}>
	<Dialog.Content class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>{$t('users.confirmDeleteTitle')}</Dialog.Title>
			<Dialog.Description>
				{$t('users.confirmDeleteDesc', { username: confirmDeleteUser?.username ?? '' })}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => (confirmDeleteUser = null)} disabled={deleting}>
				{$t('users.cancel')}
			</Button>
			<Button variant="destructive" onclick={handleDelete} disabled={deleting}>
				{#if deleting}<LoaderCircle class="size-4 animate-spin" />{/if}
				{$t('users.delete')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
