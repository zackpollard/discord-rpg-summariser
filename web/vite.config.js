import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: true,
				ws: true,
				configure: (proxy) => {
					// Prevent buffering for SSE endpoints
					proxy.on('proxyRes', (proxyRes) => {
						const contentType = proxyRes.headers['content-type'] || '';
						if (contentType.includes('text/event-stream')) {
							proxyRes.headers['cache-control'] = 'no-cache';
							proxyRes.headers['x-accel-buffering'] = 'no';
						}
					});
				}
			}
		}
	}
});
