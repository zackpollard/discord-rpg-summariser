<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { fetchCampaigns, type Campaign } from '$lib/api';

	let { children }: { children: Snippet } = $props();
	const campaignId = $derived(Number($page.params.id));
	let campaign = $state<Campaign | null>(null);

	onMount(async () => {
		try {
			const camps = await fetchCampaigns();
			campaign = camps.find(c => c.id === campaignId) ?? null;
		} catch { }
	});
</script>

<div class="campaign-layout">
	<div class="campaign-header">
		<a href="/" class="back-link">&larr; All Campaigns</a>
		{#if campaign}
			<h1>{campaign.name}</h1>
			{#if campaign.description}
				<p class="campaign-desc">{campaign.description}</p>
			{/if}
		{/if}
	</div>

	<nav class="campaign-nav">
		<a href="/campaigns/{campaignId}" class="nav-tab" class:active={$page.url.pathname === `/campaigns/${campaignId}`}>Dashboard</a>
		<a href="/campaigns/{campaignId}/sessions" class="nav-tab" class:active={$page.url.pathname.includes('/sessions')}>Sessions</a>
		<a href="/campaigns/{campaignId}/characters" class="nav-tab" class:active={$page.url.pathname.includes('/characters')}>Characters</a>
		<a href="/campaigns/{campaignId}/lore" class="nav-tab" class:active={$page.url.pathname.includes('/lore')}>Lore</a>
	</nav>

	{@render children()}
</div>

<style>
	.campaign-layout {
		display: flex;
		flex-direction: column;
	}
	.campaign-header {
		margin-bottom: 1rem;
	}
	.back-link {
		font-size: 0.85rem;
		color: var(--text-muted);
	}
	.back-link:hover {
		color: var(--accent-gold);
	}
	.campaign-header h1 {
		color: var(--accent-gold);
		font-size: 1.5rem;
		margin-top: 0.25rem;
	}
	.campaign-desc {
		color: var(--text-secondary);
		font-size: 0.9rem;
	}

	.campaign-nav {
		display: flex;
		gap: 0;
		border-bottom: 1px solid var(--border);
		margin-bottom: 1.25rem;
	}
	.nav-tab {
		padding: 0.6rem 1.25rem;
		font-size: 0.9rem;
		color: var(--text-secondary);
		border-bottom: 2px solid transparent;
		transition: color 0.15s, border-color 0.15s;
	}
	.nav-tab:hover {
		color: var(--accent-gold);
		text-decoration: none;
	}
	.nav-tab.active {
		color: var(--accent-gold);
		border-bottom-color: var(--accent-gold);
	}
</style>
