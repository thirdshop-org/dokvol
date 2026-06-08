import { writable } from 'svelte/store';

type Theme = 'light' | 'dark';

const KEY = 'dokvol_theme';

function createThemeStore() {
	const stored = typeof localStorage !== 'undefined' ? localStorage.getItem(KEY) : null;
	const prefersDark = typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)').matches : false;
	const initial: Theme = stored === 'light' || stored === 'dark' ? stored : (prefersDark ? 'dark' : 'light');

	const theme = writable<Theme>(initial);

	function apply(t: Theme) {
		document.documentElement.classList.toggle('dark', t === 'dark');
		localStorage.setItem(KEY, t);
	}

	function init() {
		let t: Theme = 'light';
		theme.subscribe(v => t = v)();
		apply(t);
	}

	function toggle() {
		theme.update(t => {
			const next: Theme = t === 'light' ? 'dark' : 'light';
			apply(next);
			return next;
		});
	}

	function set(t: Theme) {
		theme.set(t);
		apply(t);
	}

	return { subscribe: theme.subscribe, init, toggle, set };
}

export const theme = createThemeStore();
