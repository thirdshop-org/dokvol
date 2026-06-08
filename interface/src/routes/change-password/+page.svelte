<script lang="ts">
	import { goto } from '$app/navigation';
	import { t } from '$lib/i18n';
	import { changePassword as apiChangePassword } from '$lib/api';
	import { ApiError } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Input from '$lib/components/ui/input/index.js';
	import Button from '$lib/components/ui/button/button.svelte';
	import Box from '@lucide/svelte/icons/box';

	let oldPassword = $state('');
	let newPassword = $state('');
	let confirm = $state('');
	let error = $state<string | null>(null);
	let success = $state(false);
	let loading = $state(false);

	async function handleChange() {
		error = null;
		if (newPassword !== confirm) {
			error = $t('auth.passwordMismatch');
			return;
		}
		loading = true;
		try {
			await apiChangePassword({ old_password: oldPassword, new_password: newPassword });
			success = true;
			setTimeout(() => goto('/'), 2000);
		} catch (e) {
			if (e instanceof ApiError) {
				error = e.message;
			} else {
				error = String(e);
			}
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center">
	<Card.Root class="w-full max-w-sm">
		<Card.Header>
			<div class="flex items-center gap-2">
				<Box class="size-6" />
				<Card.Title class="text-xl">{$t('auth.changePasswordTitle')}</Card.Title>
			</div>
			<Card.Description>{$t('auth.changePasswordDesc')}</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if success}
				<p class="text-sm text-green-600">{$t('auth.passwordChanged')}</p>
			{:else}
				<div class="space-y-4">
					<div class="space-y-2">
						<label for="old" class="text-sm font-medium">{$t('auth.oldPassword')}</label>
						<Input.Input id="old" type="password" bind:value={oldPassword} required />
					</div>
					<div class="space-y-2">
						<label for="new" class="text-sm font-medium">{$t('auth.newPassword')}</label>
						<Input.Input id="new" type="password" bind:value={newPassword} required />
					</div>
					<div class="space-y-2">
						<label for="confirm" class="text-sm font-medium">{$t('auth.confirmPassword')}</label>
						<Input.Input id="confirm" type="password" bind:value={confirm} required />
					</div>
					{#if error}
						<p class="text-sm text-destructive">{error}</p>
					{/if}
					<Button onclick={handleChange} class="w-full" disabled={loading}>
						{loading ? $t('auth.loading') : $t('auth.changePassword')}
					</Button>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>
