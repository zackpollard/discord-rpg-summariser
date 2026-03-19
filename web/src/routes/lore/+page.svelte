<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchCampaigns, fetchEntities, type Campaign, type Entity } from '$lib/api';

	let campaigns = $state<Campaign[]>([]);
	let activeCampaign = $state<Campaign | null>(null);
	let entities = $state<Entity[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let activeType = $state<string>('');
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout> | undefined;

	const entityTypes = [
		{ value: '', label: 'All' },
		{ value: 'npc', label: 'NPCs' },
		{ value: 'place', label: 'Places' },
		{ value: 'organisation', label: 'Organisations' },
		{ value: 'item', label: 'Items' },
		{ value: 'event', label: 'Events' }
	];

	function typeBadgeClass(type: string): string {
		return `type-badge type-${type}`;
	}

	async function loadEntities() {
		if (!activeCampaign) return;
		loading = true;
		error = null;
		try {
			entities = await fetchEntities(activeCampaign.id, {
				type: activeType || undefined,
				search: searchQuery || undefined
			});
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load entities';
		} finally {
			loading = false;
		}
	}

	function handleSearch(value: string) {
		searchQuery = value;
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => loadEntities(), 300);
	}

	function selectType(type: string) {
		activeType = type;
		loadEntities();
	}

	onMount(async () => {
		try {
			campaigns = await fetchCampaigns();
			activeCampaign = campaigns.find((c) => c.is_active) ?? null;
			if (activeCampaign) {
				await loadEntities();
			} else {
				loading = false;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Lore - RPG Summariser</title>
</svelte:head>

<div class="lore-page">
	<h1>Lore & Knowledge Base</h1>

	{#if !activeCampaign && !loading}
		<div class="empty-state">
			<p>No active campaign.</p>
			<p class="muted">Set an active campaign in <a href="/campaigns">Campaigns</a> to view lore.</p>
		</div>
	{:else}
		{#if activeCampaign}
			<p class="campaign-label">Campaign: <strong>{activeCampaign.name}</strong></p>
		{/if}

		<div class="controls">
			<div class="type-filters">
				{#each entityTypes as et}
					<button
						class="type-btn"
						class:active={activeType === et.value}
						onclick={() => selectType(et.value)}
					>
						{et.label}
					</button>
				{/each}
			</div>
			<input
				type="text"
				class="search-input"
				placeholder="Search entities..."
				value={searchQuery}
				oninput={(e) => handleSearch(e.currentTarget.value)}
			/>
		</div>

		{#if loading}
			<p class="muted">Loading entities...</p>
		{:else if error}
			<div class="error-box">{error}</div>
		{:else if entities.length === 0}
			<div class="empty-state">
				<p>No entities found.</p>
				<p class="muted">Entities will appear here after sessions are processed.</p>
			</div>
		{:else}
			<div class="entity-grid">
				{#each entities as entity (entity.id)}
					<a href="/lore/{entity.id}" class="entity-card">
						<div class="entity-header">
							<span class="entity-name">{entity.name}</span>
							<span class={typeBadgeClass(entity.type)}>{entity.type}</span>
						</div>
						<p class="entity-desc">
							{entity.description.length > 120
								? entity.description.slice(0, 120) + '...'
								: entity.description}
						</p>
					</a>
				{/each}
			</div>
		{/if}
	{/if}
</div>

<style>
	.lore-page h1 {
		color: var(--accent-gold);
		margin-bottom: 1.25rem;
		font-size: 1.5rem;
	}

	.campaign-label {
		color: var(--text-secondary);
		font-size: 0.9rem;
		margin-bottom: 1rem;
	}
	.campaign-label strong {
		color: var(--text-primary);
	}

	.controls {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		align-items: center;
		margin-bottom: 1.25rem;
	}

	.type-filters {
		display: flex;
		gap: 0.35rem;
		flex-wrap: wrap;
	}
	.type-btn {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.35rem 0.75rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.8rem;
		transition: background 0.15s, border-color 0.15s, color 0.15s;
	}
	.type-btn:hover {
		background: var(--surface-hover);
		border-color: var(--accent-gold-dim);
	}
	.type-btn.active {
		background: rgba(212, 175, 125, 0.15);
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
	}

	.search-input {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		padding: 0.4rem 0.75rem;
		font-size: 0.85rem;
		min-width: 200px;
		flex: 1;
		max-width: 300px;
	}
	.search-input:focus {
		outline: none;
		border-color: var(--accent-gold-dim);
	}

	.entity-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 0.75rem;
	}

	.entity-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem;
		transition: border-color 0.15s, background 0.15s;
		display: block;
	}
	.entity-card:hover {
		border-color: var(--accent-gold-dim);
		background: var(--surface-hover);
		text-decoration: none;
	}

	.entity-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 0.5rem;
	}
	.entity-name {
		font-weight: 600;
		color: var(--text-primary);
		font-size: 0.95rem;
	}

	.type-badge {
		font-size: 0.65rem;
		padding: 0.1rem 0.45rem;
		border-radius: 999px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.type-npc {
		background: rgba(139, 92, 246, 0.2);
		color: #a78bfa;
		border: 1px solid rgba(139, 92, 246, 0.3);
	}
	.type-place {
		background: rgba(34, 197, 94, 0.2);
		color: #86efac;
		border: 1px solid rgba(34, 197, 94, 0.3);
	}
	.type-organisation {
		background: rgba(59, 130, 246, 0.2);
		color: #93c5fd;
		border: 1px solid rgba(59, 130, 246, 0.3);
	}
	.type-item {
		background: rgba(234, 179, 8, 0.2);
		color: #fde047;
		border: 1px solid rgba(234, 179, 8, 0.3);
	}
	.type-event {
		background: rgba(239, 68, 68, 0.2);
		color: #fca5a5;
		border: 1px solid rgba(239, 68, 68, 0.3);
	}

	.entity-desc {
		color: var(--text-secondary);
		font-size: 0.8rem;
		line-height: 1.4;
	}

	.empty-state {
		text-align: center;
		padding: 3rem 1rem;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.muted {
		color: var(--text-muted);
	}
	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
	}
</style>
