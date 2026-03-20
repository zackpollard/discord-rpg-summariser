<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchAuthMe, logout, type AuthUser } from '$lib/api';

	let { children }: { children: Snippet } = $props();

	let navOpen = $state(false);
	let user = $state<AuthUser | null>(null);
	let authChecked = $state(false);

	const isLoginPage = $derived($page.url.pathname === '/login');

	onMount(async () => {
		if (isLoginPage) {
			authChecked = true;
			return;
		}
		try {
			user = await fetchAuthMe();
		} catch {
			// 401 or network error — redirect to login
			goto('/login');
			return;
		}
		authChecked = true;
	});

	function avatarURL(u: AuthUser): string {
		if (u.avatar) {
			return `https://cdn.discordapp.com/avatars/${u.id}/${u.avatar}.png?size=64`;
		}
		// Default Discord avatar
		const index = (BigInt(u.id) >> 22n) % 6n;
		return `https://cdn.discordapp.com/embed/avatars/${index}.png`;
	}

	async function handleLogout() {
		try {
			await logout();
		} catch {
			// ignore
		}
		user = null;
		goto('/login');
	}
</script>

{#if isLoginPage}
	{@render children()}
{:else if !authChecked}
	<div class="loading-screen">
		<p>Loading...</p>
	</div>
{:else}
	<div class="app">
		<nav class="sidebar" class:open={navOpen}>
			<div class="brand">
				<span class="brand-icon">&#x1f3b2;</span>
				<span class="brand-text">RPG Summariser</span>
			</div>

			<ul class="nav-links">
				<li><a href="/" onclick={() => (navOpen = false)}>Campaigns</a></li>
			</ul>

			{#if user}
				<div class="sidebar-user">
					<img src={avatarURL(user)} alt={user.username} class="user-avatar" />
					<span class="user-name">{user.username}</span>
					<button class="logout-btn" onclick={handleLogout} title="Logout">
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
					</button>
				</div>
			{/if}
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

	.loading-screen {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		color: var(--text-muted);
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
		flex: 1;
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

	.sidebar-user {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem 1.25rem;
		border-top: 1px solid var(--border);
		margin-top: auto;
	}
	.user-avatar {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		flex-shrink: 0;
	}
	.user-name {
		font-size: 0.85rem;
		color: var(--text-secondary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
	}
	.logout-btn {
		background: none;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		padding: 0.2rem;
		display: flex;
		align-items: center;
		flex-shrink: 0;
	}
	.logout-btn:hover {
		color: var(--accent-gold);
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
