<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchBestiary, type BestiaryEntry } from '$lib/api';

	const campaignId = $derived(Number($page.params.id));

	let creatures = $state<BestiaryEntry[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let activeType = $state('');
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout> | undefined;

	const creatureTypes = [
		{ value: '', label: 'All Types' },
		{ value: 'aberration', label: 'Aberration' },
		{ value: 'beast', label: 'Beast' },
		{ value: 'celestial', label: 'Celestial' },
		{ value: 'construct', label: 'Construct' },
		{ value: 'dragon', label: 'Dragon' },
		{ value: 'elemental', label: 'Elemental' },
		{ value: 'fey', label: 'Fey' },
		{ value: 'fiend', label: 'Fiend' },
		{ value: 'giant', label: 'Giant' },
		{ value: 'humanoid', label: 'Humanoid' },
		{ value: 'monstrosity', label: 'Monstrosity' },
		{ value: 'ooze', label: 'Ooze' },
		{ value: 'plant', label: 'Plant' },
		{ value: 'undead', label: 'Undead' }
	];

	async function loadCreatures() {
		loading = true;
		error = null;
		try {
			creatures = await fetchBestiary(campaignId, {
				creature_type: activeType || undefined,
				search: searchQuery || undefined
			});
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load bestiary';
		} finally {
			loading = false;
		}
	}

	function handleSearch(value: string) {
		searchQuery = value;
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(loadCreatures, 300);
	}

	function statusClass(status: string): string {
		if (status === 'dead') return 'status-dead';
		if (status === 'alive') return 'status-alive';
		return 'status-unknown';
	}

	onMount(loadCreatures);

	$effect(() => {
		activeType;
		loadCreatures();
	});
</script>

<svelte:head>
	<title>Bestiary - RPG Summariser</title>
</svelte:head>

<div class="bestiary-page">
	<h1>Bestiary</h1>

	<div class="filters">
		<div class="type-filters">
			{#each creatureTypes as ct}
				<button
					class="filter-chip"
					class:active={activeType === ct.value}
					onclick={() => activeType = ct.value}
				>{ct.label}</button>
			{/each}
		</div>
		<input
			type="text"
			class="search-input"
			placeholder="Search creatures..."
			value={searchQuery}
			oninput={(e) => handleSearch((e.target as HTMLInputElement).value)}
		/>
	</div>

	{#if loading}
		<p class="muted">Loading bestiary...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if creatures.length === 0}
		<div class="empty-state">
			<p>No creatures found.</p>
			<p class="muted">Creatures are automatically extracted from combat encounters during session processing.</p>
		</div>
	{:else}
		<div class="creature-grid">
			{#each creatures as creature (creature.id)}
				<a href="/campaigns/{campaignId}/bestiary/{creature.id}" class="creature-card">
					<div class="creature-header">
						<span class="creature-name">{creature.name}</span>
						<span class="creature-status {statusClass(creature.status)}">{creature.status}</span>
					</div>
					<div class="creature-meta">
						{#if creature.creature_stats.creature_type}
							<span class="creature-type-badge">{creature.creature_stats.creature_type}</span>
						{/if}
						{#if creature.creature_stats.challenge_rating}
							<span class="creature-cr">CR {creature.creature_stats.challenge_rating}</span>
						{/if}
						{#if creature.creature_stats.armor_class}
							<span class="creature-ac">AC {creature.creature_stats.armor_class}</span>
						{/if}
					</div>
					{#if creature.description}
						<p class="creature-desc">{creature.description}</p>
					{/if}
					{#if creature.creature_stats.abilities}
						<div class="creature-abilities">{creature.creature_stats.abilities}</div>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</div>

<style>
	.bestiary-page h1 {
		color: var(--accent-gold);
		font-size: 1.5rem;
		margin-bottom: 1rem;
	}

	.filters {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin-bottom: 1.25rem;
	}
	.type-filters {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.filter-chip {
		padding: 0.3rem 0.7rem;
		border-radius: 999px;
		border: 1px solid var(--border);
		background: var(--bg-surface);
		color: var(--text-secondary);
		font-size: 0.8rem;
		cursor: pointer;
		transition: all 0.15s;
	}
	.filter-chip:hover {
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
	}
	.filter-chip.active {
		background: var(--accent-gold);
		color: var(--bg-dark);
		border-color: var(--accent-gold);
		font-weight: 600;
	}
	.search-input {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--text-primary);
		padding: 0.5rem 0.75rem;
		font-size: 0.85rem;
		width: 100%;
		max-width: 300px;
	}
	.search-input::placeholder {
		color: var(--text-muted);
	}
	.search-input:focus {
		outline: none;
		border-color: var(--accent-gold);
	}

	.creature-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 1rem;
	}

	.creature-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem;
		display: block;
		color: inherit;
		text-decoration: none;
		transition: border-color 0.15s, background 0.15s;
	}
	.creature-card:hover {
		border-color: var(--accent-gold-dim);
		background: var(--bg-surface-2, var(--bg-surface));
		text-decoration: none;
	}

	.creature-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.5rem;
	}
	.creature-name {
		font-weight: 600;
		font-size: 1rem;
		color: var(--accent-gold);
	}
	.creature-status {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		font-weight: 600;
		padding: 0.1rem 0.4rem;
		border-radius: 3px;
	}
	.status-dead { background: rgba(239, 68, 68, 0.2); color: #fca5a5; }
	.status-alive { background: rgba(34, 197, 94, 0.2); color: #86efac; }
	.status-unknown { background: rgba(255, 255, 255, 0.08); color: var(--text-muted); }

	.creature-meta {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
		margin-bottom: 0.5rem;
	}
	.creature-type-badge {
		font-size: 0.7rem;
		text-transform: capitalize;
		padding: 0.1rem 0.5rem;
		border-radius: 3px;
		background: rgba(139, 92, 246, 0.2);
		color: #c4b5fd;
		font-weight: 600;
	}
	.creature-cr {
		font-size: 0.75rem;
		color: var(--accent-gold);
		font-weight: 600;
	}
	.creature-ac {
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.creature-desc {
		font-size: 0.85rem;
		color: var(--text-secondary);
		line-height: 1.5;
		margin-bottom: 0.5rem;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.creature-abilities {
		font-size: 0.75rem;
		color: var(--text-muted);
		font-style: italic;
	}

	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--text-secondary);
	}
	.muted { color: var(--text-muted); }
	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
	}
</style>
