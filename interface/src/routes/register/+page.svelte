<script lang="ts">
	import { goto } from '$app/navigation';
	import { t } from '$lib/i18n';
	import { register as apiRegister } from '$lib/api';
	import { ApiError } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Input from '$lib/components/ui/input/index.js';
	import Button from '$lib/components/ui/button/button.svelte';
	import Box from '@lucide/svelte/icons/box';

	let email = $state('');
	let username = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state<string | null>(null);
	let loading = $state(false);

	async function handleRegister() {
		error = null;
		if (password !== confirm) {
			error = $t('auth.passwordMismatch');
			return;
		}
		loading = true;
		try {
			const res = await apiRegister({ email, username, password });
			auth.setAuth(res.access_token, res.refresh_token, res.user);
			goto('/');
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
				<Card.Title class="text-xl">{$t('app.name')}</Card.Title>
			</div>
			<Card.Description>{$t('auth.registerTitle')}</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="space-y-4">
				<div class="space-y-2">
					<label for="email" class="text-sm font-medium">{$t('auth.email')}</label>
					<Input.Input id="email" type="email" bind:value={email} required placeholder="you@example.com" />
				</div>
				<div class="space-y-2">
					<label for="username" class="text-sm font-medium">{$t('auth.username')}</label>
					<Input.Input id="username" bind:value={username} required placeholder="johndoe" />
				</div>
				<div class="space-y-2">
					<label for="password" class="text-sm font-medium">{$t('auth.password')}</label>
					<Input.Input id="password" type="password" bind:value={password} required />
				</div>
				<div class="space-y-2">
					<label for="confirm" class="text-sm font-medium">{$t('auth.confirmPassword')}</label>
					<Input.Input id="confirm" type="password" bind:value={confirm} required />
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
				<Button onclick={handleRegister} class="w-full" disabled={loading}>
					{loading ? $t('auth.loading') : $t('auth.register')}
				</Button>
			</div>
		</Card.Content>
		<Card.Footer class="justify-center">
			<p class="text-sm text-muted-foreground">
				{$t('auth.hasAccount')}
				<a href="/login" class="text-primary underline-offset-4 hover:underline">{$t('auth.login')}</a>
			</p>
		</Card.Footer>
	</Card.Root>
</div>
