<script lang="ts">
	import { goto } from '$app/navigation';
	import { t } from '$lib/i18n';
	import { login as apiLogin } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { errorMessage } from '$lib/utils/errors';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Input from '$lib/components/ui/input/index.js';
	import Button from '$lib/components/ui/button/button.svelte';
	import Box from '@lucide/svelte/icons/box';

	let email = $state('');
	let password = $state('');
	let error = $state<string | null>(null);
	let loading = $state(false);

	async function handleLogin() {
		error = null;
		if (!email || !password) {
			error = $t('auth.requiredFields');
			return;
		}
		loading = true;
		try {
			const res = await apiLogin({ email, password });
			auth.setAuth(res.access_token, res.refresh_token, res.user);
			if (res.user.password_change_required) {
				goto('/change-password');
			} else {
				goto('/');
			}
		} catch (e) {
			error = errorMessage(e);
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
				<Card.Title class="text-xl">{$t('app.name')}</Card.Title>
			</div>
			<Card.Description>{$t('auth.loginTitle')}</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="space-y-4">
				<div class="space-y-2">
					<label for="email" class="text-sm font-medium">{$t('auth.email')}</label>
					<Input.Input id="email" type="email" bind:value={email} required placeholder="you@example.com" />
				</div>
				<div class="space-y-2">
					<label for="password" class="text-sm font-medium">{$t('auth.password')}</label>
					<Input.Input id="password" type="password" bind:value={password} required />
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
				<Button onclick={handleLogin} class="w-full" disabled={loading}>
					{loading ? $t('auth.loading') : $t('auth.login')}
				</Button>
			</div>
		</Card.Content>
	</Card.Root>
</div>
