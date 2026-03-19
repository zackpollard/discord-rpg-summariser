<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let navOpen = $state(false);
</script>

<div class="app">
	<nav class="sidebar" class:open={navOpen}>
		<div class="brand">
			<span class="brand-icon">&#x1f3b2;</span>
			<span class="brand-text">RPG Summariser</span>
		</div>

		<ul class="nav-links">
			<li><a href="/" onclick={() => (navOpen = false)}>Dashboard</a></li>
			<li><a href="/sessions" onclick={() => (navOpen = false)}>Sessions</a></li>
			<li><a href="/characters" onclick={() => (navOpen = false)}>Characters</a></li>
		</ul>
	</nav>

	<div class="main-area">
		<header class="topbar">
			<button class="menu-toggle" onclick={() => (navOpen = !navOpen)} aria-label="Toggle menu">
				&#9776;
			</button>
			<span class="topbar-title">RPG Session Summariser</span>
		</header>

		<main class="content">
			{@render children()}
		</main>
	</div>
</div>

{#if navOpen}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="overlay" onclick={() => (navOpen = false)} onkeydown={() => {}}></div>
{/if}

<style>
	:global(*) {
		margin: 0;
		padding: 0;
		box-sizing: border-box;
	}

	:global(:root) {
		--bg-dark: #1a1020;
		--bg-surface: #241832;
		--bg-surface-2: #2e2040;
		--surface-hover: rgba(212, 175, 125, 0.06);
		--border: #3d2e50;
		--accent-gold: #d4af7d;
		--accent-gold-dim: #a68a5b;
		--accent-purple: #8b5cf6;
		--text-primary: #e8e0f0;
		--text-secondary: #b8a8cc;
		--text-muted: #7a6b8a;
		--radius: 8px;
		--shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	:global(body) {
		background: var(--bg-dark);
		color: var(--text-primary);
		font-family: 'Segoe UI', system-ui, -apple-system, sans-serif;
		line-height: 1.6;
		min-height: 100vh;
	}

	:global(a) {
		color: var(--accent-gold);
		text-decoration: none;
	}
	:global(a:hover) {
		color: var(--accent-gold-dim);
		text-decoration: underline;
	}

	.app {
		display: flex;
		min-height: 100vh;
	}

	.sidebar {
		width: 240px;
		background: var(--bg-surface);
		border-right: 1px solid var(--border);
		padding: 1.25rem 0;
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0 1.25rem 1.25rem;
		border-bottom: 1px solid var(--border);
		margin-bottom: 0.75rem;
	}
	.brand-icon {
		font-size: 1.5rem;
	}
	.brand-text {
		font-weight: 700;
		font-size: 1rem;
		color: var(--accent-gold);
		letter-spacing: 0.02em;
	}

	.nav-links {
		list-style: none;
	}
	.nav-links li a {
		display: block;
		padding: 0.6rem 1.25rem;
		color: var(--text-secondary);
		font-size: 0.95rem;
		transition: background 0.15s, color 0.15s;
		border-left: 3px solid transparent;
	}
	.nav-links li a:hover {
		background: var(--surface-hover);
		color: var(--accent-gold);
		text-decoration: none;
		border-left-color: var(--accent-gold);
	}

	.main-area {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.topbar {
		display: none;
		background: var(--bg-surface);
		border-bottom: 1px solid var(--border);
		padding: 0.75rem 1rem;
		align-items: center;
		gap: 0.75rem;
	}
	.topbar-title {
		font-weight: 600;
		color: var(--accent-gold);
		font-size: 1rem;
	}

	.menu-toggle {
		background: none;
		border: 1px solid var(--border);
		color: var(--text-primary);
		font-size: 1.25rem;
		padding: 0.25rem 0.5rem;
		border-radius: var(--radius);
		cursor: pointer;
	}
	.menu-toggle:hover {
		background: var(--surface-hover);
	}

	.content {
		flex: 1;
		padding: 1.5rem;
		max-width: 1100px;
		width: 100%;
		margin: 0 auto;
	}

	.overlay {
		display: none;
	}

	@media (max-width: 768px) {
		.sidebar {
			position: fixed;
			top: 0;
			left: -260px;
			height: 100vh;
			z-index: 100;
			transition: left 0.2s ease;
			box-shadow: var(--shadow);
		}
		.sidebar.open {
			left: 0;
		}
		.topbar {
			display: flex;
		}
		.overlay {
			display: block;
			position: fixed;
			inset: 0;
			background: rgba(0, 0, 0, 0.5);
			z-index: 99;
		}
	}
</style>
