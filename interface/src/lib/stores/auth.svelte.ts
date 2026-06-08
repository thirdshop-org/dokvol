import { writable, derived } from 'svelte/store';
import type { User } from '$lib/types';

const ACCESS_KEY = 'dokvol_access_token';
const REFRESH_KEY = 'dokvol_refresh_token';
const USER_KEY = 'dokvol_user';

function createAuthStore() {
	const accessToken = writable<string | null>(typeof localStorage !== 'undefined' ? localStorage.getItem(ACCESS_KEY) : null);
	const refreshToken = writable<string | null>(typeof localStorage !== 'undefined' ? localStorage.getItem(REFRESH_KEY) : null);
	const user = writable<User | null>(typeof localStorage !== 'undefined' ? JSON.parse(localStorage.getItem(USER_KEY) || 'null') : null);

	const isLoggedIn = derived(accessToken, ($token) => $token !== null);
	const isAdmin = derived(user, ($user) => $user?.role === 'admin');
	const passwordChangeRequired = derived(user, ($user) => $user?.password_change_required ?? false);

	function save() {
		let t: string | null = null;
		let rt: string | null = null;
		let u: User | null = null;
		accessToken.subscribe(v => t = v)();
		refreshToken.subscribe(v => rt = v)();
		user.subscribe(v => u = v)();

		if (t) localStorage.setItem(ACCESS_KEY, t);
		else localStorage.removeItem(ACCESS_KEY);

		if (rt) localStorage.setItem(REFRESH_KEY, rt);
		else localStorage.removeItem(REFRESH_KEY);

		if (u) localStorage.setItem(USER_KEY, JSON.stringify(u));
		else localStorage.removeItem(USER_KEY);
	}

	function setAuth(at: string, rt: string, u: User) {
		accessToken.set(at);
		refreshToken.set(rt);
		user.set(u);
		save();
	}

	function updateTokens(at: string, rt: string) {
		accessToken.set(at);
		refreshToken.set(rt);
		save();
	}

	function logout() {
		accessToken.set(null);
		refreshToken.set(null);
		user.set(null);
		save();
	}

	function getAccessToken(): string | null {
		let t: string | null = null;
		accessToken.subscribe(v => t = v)();
		return t;
	}

	function getRefreshToken(): string | null {
		let t: string | null = null;
		refreshToken.subscribe(v => t = v)();
		return t;
	}

	return {
		accessToken,
		refreshToken,
		user,
		isLoggedIn,
		isAdmin,
		passwordChangeRequired,
		setAuth,
		updateTokens,
		logout,
		getAccessToken,
		getRefreshToken,
	};
}

export const auth = createAuthStore();
