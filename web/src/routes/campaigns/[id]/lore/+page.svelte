<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchEntities, askLore, fetchRelationshipGraph, type Entity, type LoreAnswer, type RelationshipGraphData } from '$lib/api';
	import RelationshipGraph from '$lib/components/RelationshipGraph.svelte';

	const campaignId = $derived(Number($page.params.id));

	let entities = $state<Entity[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let activeType = $state<string>('');
	let activeStatus = $state<string>('');
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout> | undefined;

	let loreQuestion = $state('');
	let loreAnswer = $state<LoreAnswer | null>(null);
	let loreLoading = $state(false);
	let loreError = $state<string | null>(null);

	let viewMode = $state<'grid' | 'graph'>('grid');
	let graphData = $state<RelationshipGraphData | null>(null);
	let graphLoading = $state(false);
	let graphError = $state<string | null>(null);

	async function handleAskLore() {
		if (!loreQuestion.trim()) return;
		loreLoading = true;
		loreError = null;
		loreAnswer = null;
		try {
			loreAnswer = await askLore(campaignId, loreQuestion.trim());
		} catch (e) {
			loreError = e instanceof Error ? e.message : 'Failed to get answer';
		} finally {
			loreLoading = false;
		}
	}

	const entityTypes = [
		{ value: '', label: 'All' },
		{ value: 'pc', label: 'Player Characters' },
		{ value: 'npc', label: 'NPCs' },
		{ value: 'place', label: 'Places' },
		{ value: 'organisation', label: 'Organisations' },
		{ value: 'item', label: 'Items' },
		{ value: 'event', label: 'Events' }
	];

	const statusFilters = [
		{ value: '', label: 'All' },
		{ value: 'alive', label: 'Alive' },
		{ value: 'dead', label: 'Dead' },
		{ value: 'unknown', label: 'Unknown' }
	];

	function typeBadgeClass(type: string): string {
		return `type-badge type-${type}`;
	}

	async function loadEntities() {
		loading = true;
		error = null;
		try {
			entities = await fetchEntities(campaignId, {
				type: activeType || undefined,
				search: searchQuery || undefined,
				status: activeStatus || undefined
			});
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load entities';
		} finally {
			loading = false;
		}
	}

	async function loadGraphData() {
		if (graphData) return; // Already loaded
		graphLoading = true;
		graphError = null;
		try {
			graphData = await fetchRelationshipGraph(campaignId);
		} catch (e) {
			graphError = e instanceof Error ? e.message : 'Failed to load relationship graph';
		} finally {
			graphLoading = false;
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

	function selectStatus(status: string) {
		activeStatus = status;
		loadEntities();
	}

	function switchView(mode: 'grid' | 'graph') {
		viewMode = mode;
		if (mode === 'graph') {
			loadGraphData();
		}
	}

	onMount(() => { loadEntities(); });
</script>

<svelte:head>
	<title>Lore - RPG Summariser</title>
</svelte:head>

<div class="lore-page">
	<section class="ask-lore-section">
		<h2>Ask the Lore</h2>
		<form class="ask-form" onsubmit={(e) => { e.preventDefault(); handleAskLore(); }}>
			<input
				type="text"
				placeholder="Ask a question about your campaign lore..."
				bind:value={loreQuestion}
				class="ask-input"
				disabled={loreLoading}
			/>
			<button type="submit" class="ask-btn" disabled={loreLoading || !loreQuestion.trim()}>
				{loreLoading ? 'Thinking...' : 'Ask'}
			</button>
		</form>
		{#if loreLoading}
			<div class="ask-loading">
				<span class="spinner"></span>
				<span>Consulting the archives...</span>
			</div>
		{/if}
		{#if loreError}
			<div class="error-box">{loreError}</div>
		{/if}
		{#if loreAnswer}
			<div class="answer-box">
				<p class="answer-text">{loreAnswer.answer}</p>
				{#if loreAnswer.sources && loreAnswer.sources.length > 0}
					<div class="answer-sources">
						<span class="sources-label">Sources:</span>
						<ul class="sources-list">
							{#each loreAnswer.sources as source}
								<li><span class="source-type">[{source.type}]</span> {source.name}</li>
							{/each}
						</ul>
					</div>
				{/if}
			</div>
		{/if}
	</section>

	<div class="controls">
		<div class="view-toggle">
			<button
				class="toggle-btn"
				class:active={viewMode === 'grid'}
				onclick={() => switchView('grid')}
			>Grid</button>
			<button
				class="toggle-btn"
				class:active={viewMode === 'graph'}
				onclick={() => switchView('graph')}
			>Graph</button>
		</div>
		{#if viewMode === 'grid'}
			<div class="type-filters">
				{#each entityTypes as t (t.value)}
					<button
						class="filter-btn"
						class:active={activeType === t.value}
						onclick={() => selectType(t.value)}
					>{t.label}</button>
				{/each}
			</div>
			<div class="status-filters">
				{#each statusFilters as sf (sf.value)}
					<button
						class="filter-btn"
						class:active={activeStatus === sf.value}
						onclick={() => selectStatus(sf.value)}
					>{sf.label}</button>
				{/each}
			</div>
			<input
				type="text"
				placeholder="Search entities..."
				value={searchQuery}
				oninput={(e) => handleSearch(e.currentTarget.value)}
				class="search-input"
			/>
		{/if}
	</div>

	{#if viewMode === 'grid'}
		{#if loading}
			<p class="muted">Loading entities...</p>
		{:else if error}
			<div class="error-box">{error}</div>
		{:else if entities.length === 0}
			<div class="empty-state">
				<p>No entities found.</p>
				<p class="muted">Entities are automatically extracted from session transcripts.</p>
			</div>
		{:else}
			<div class="entity-grid">
				{#each entities as entity (entity.id)}
					<a href="/campaigns/{campaignId}/lore/{entity.id}" class="entity-card">
						<div class="card-top">
							<span class={typeBadgeClass(entity.type)}>{entity.type}</span>
							{#if entity.status === 'dead'}
								<span class="status-indicator status-dead" title="Dead">&#9760;</span>
							{:else if entity.status === 'alive'}
								<span class="status-indicator status-alive" title="Alive"></span>
							{/if}
						</div>
						<h3>{entity.name}</h3>
						{#if entity.description}
							<p class="entity-desc">{entity.description.slice(0, 120)}{entity.description.length > 120 ? '...' : ''}</p>
						{/if}
					</a>
				{/each}
			</div>
		{/if}
	{:else}
		{#if graphLoading}
			<p class="muted">Loading relationship graph...</p>
		{:else if graphError}
			<div class="error-box">{graphError}</div>
		{:else if graphData}
			<RelationshipGraph
				nodes={graphData.nodes}
				edges={graphData.edges}
				{campaignId}
			/>
		{/if}
	{/if}
</div>

<style>
	.ask-lore-section {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.25rem;
	}
	.ask-lore-section h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
	}
	.ask-form {
		display: flex;
		gap: 0.5rem;
	}
	.ask-input {
		flex: 1;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.5rem 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
	}
	.ask-input:focus { outline: none; border-color: var(--accent-gold-dim); }
	.ask-input:disabled { opacity: 0.6; }
	.ask-btn {
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border: 1px solid var(--accent-gold);
		padding: 0.5rem 1.25rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s;
	}
	.ask-btn:hover:not(:disabled) { background: var(--accent-gold); }
	.ask-btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.ask-loading {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-top: 0.75rem;
		color: var(--text-muted);
		font-size: 0.85rem;
	}
	.spinner {
		width: 14px;
		height: 14px;
		border: 2px solid var(--border);
		border-top-color: var(--accent-gold);
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }
	.answer-box {
		margin-top: 0.75rem;
		padding: 1rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.answer-text {
		color: var(--text-primary);
		font-size: 0.9rem;
		line-height: 1.6;
		margin-bottom: 0.75rem;
	}
	.answer-sources {
		border-top: 1px solid var(--border);
		padding-top: 0.5rem;
	}
	.sources-label {
		font-size: 0.75rem;
		color: var(--text-muted);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.sources-list {
		list-style: none;
		padding: 0;
		margin-top: 0.25rem;
	}
	.sources-list li {
		font-size: 0.8rem;
		color: var(--text-secondary);
		padding: 0.15rem 0;
	}
	.source-type {
		color: var(--text-muted);
		font-size: 0.7rem;
		text-transform: uppercase;
	}

	.controls { display: flex; gap: 1rem; align-items: center; flex-wrap: wrap; margin-bottom: 1.25rem; }
	.view-toggle { display: flex; gap: 0; }
	.toggle-btn {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.35rem 0.85rem;
		cursor: pointer;
		font-size: 0.8rem;
		transition: all 0.15s;
	}
	.toggle-btn:first-child { border-radius: var(--radius) 0 0 var(--radius); }
	.toggle-btn:last-child { border-radius: 0 var(--radius) var(--radius) 0; border-left: none; }
	.toggle-btn:hover { border-color: var(--accent-gold-dim); color: var(--accent-gold); }
	.toggle-btn.active { background: var(--accent-gold-dim); color: var(--bg-dark); border-color: var(--accent-gold); font-weight: 600; }
	.type-filters { display: flex; gap: 0.35rem; flex-wrap: wrap; }
	.filter-btn { background: var(--bg-surface-2); border: 1px solid var(--border); color: var(--text-secondary); padding: 0.35rem 0.75rem; border-radius: var(--radius); cursor: pointer; font-size: 0.8rem; transition: all 0.15s; }
	.filter-btn:hover { border-color: var(--accent-gold-dim); color: var(--accent-gold); }
	.filter-btn.active { background: var(--accent-gold-dim); color: var(--bg-dark); border-color: var(--accent-gold); font-weight: 600; }
	.search-input { background: var(--bg-surface-2); border: 1px solid var(--border); color: var(--text-primary); padding: 0.4rem 0.75rem; border-radius: var(--radius); font-size: 0.85rem; flex: 1; min-width: 180px; }
	.search-input:focus { outline: none; border-color: var(--accent-gold-dim); }

	.entity-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 0.75rem; }
	.entity-card { display: block; background: var(--bg-surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 1rem; transition: border-color 0.15s; }
	.entity-card:hover { border-color: var(--accent-gold-dim); text-decoration: none; }
	.card-top { margin-bottom: 0.4rem; display: flex; align-items: center; gap: 0.5rem; }
	.entity-card h3 { font-size: 1rem; color: var(--text-primary); margin-bottom: 0.3rem; }
	.entity-desc { color: var(--text-secondary); font-size: 0.85rem; line-height: 1.4; }

	.status-filters { display: flex; gap: 0.35rem; flex-wrap: wrap; }
	.status-indicator { margin-left: auto; }
	.status-dead { font-size: 0.9rem; color: #f87171; }
	.status-alive { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #4ade80; }

	.type-badge { font-size: 0.65rem; padding: 0.15rem 0.5rem; border-radius: 999px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
	.type-pc { background: rgba(236, 72, 153, 0.15); color: #f472b6; border: 1px solid rgba(236, 72, 153, 0.3); }
	.type-npc { background: rgba(139, 92, 246, 0.15); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }
	.type-place { background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); }
	.type-organisation { background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); }
	.type-item { background: rgba(234, 179, 8, 0.15); color: #facc15; border: 1px solid rgba(234, 179, 8, 0.3); }
	.type-event { background: rgba(239, 68, 68, 0.15); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }

	.empty-state { text-align: center; padding: 3rem 1rem; background: var(--bg-surface); border: 1px solid var(--border); border-radius: var(--radius); }
	.muted { color: var(--text-muted); }
	.error-box { background: rgba(185, 28, 28, 0.15); border: 1px solid #7f1d1d; color: #fca5a5; padding: 0.75rem; border-radius: var(--radius); font-size: 0.9rem; }
</style>
