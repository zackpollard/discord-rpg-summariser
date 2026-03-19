import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	plugins: [sveltekit()],
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'jsdom',
		setupFiles: ['./node_modules/@testing-library/svelte/src/vitest.js'],
	},
	resolve: {
		conditions: ['browser'],
	},
});
