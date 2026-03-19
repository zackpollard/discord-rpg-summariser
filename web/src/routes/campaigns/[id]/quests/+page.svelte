<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchQuests, type Quest } from '$lib/api';

	const campaignId = $derived(Number($page.params.id));

	let quests = $state<Quest[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let activeStatus = $state<string>('');

	const statusFilters = [
		{ value: '', label: 'All' },
		{ value: 'active', label: 'Active' },
		{ value: 'completed', label: 'Completed' },
		{ value: 'failed', label: 'Failed' }
	];

	function statusBadgeClass(status: string): string {
		return `status-badge status-${status}`;
	}

	async function loadQuests() {
		loading = true;
		error = null;
		try {
			quests = await fetchQuests(campaignId, activeStatus || undefined);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load quests';
		} finally {
			loading = false;
		}
	}

	function selectStatus(status: string) {
		activeStatus = status;
		loadQuests();
	}

	onMount(() => { loadQuests(); });
</script>

<svelte:head>
	<title>Quests - RPG Summariser</title>
</svelte:head>

<div class="quests-page">
	<div class="controls">
		<div class="status-filters">
			{#each statusFilters as f (f.value)}
				<button
					class="filter-btn"
					class:active={activeStatus === f.value}
					onclick={() => selectStatus(f.value)}
				>{f.label}</button>
			{/each}
		</div>
	</div>

	{#if loading}
		<p class="muted">Loading quests...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if quests.length === 0}
		<div class="empty-state">
			<p>No quests found.</p>
			<p class="muted">Quests are automatically extracted from session summaries.</p>
		</div>
	{:else}
		<div class="quest-grid">
			{#each quests as quest (quest.id)}
				<a href="/campaigns/{campaignId}/quests/{quest.id}" class="quest-card">
					<div class="card-top">
						<span class={statusBadgeClass(quest.status)}>{quest.status}</span>
					</div>
					<h3>{quest.name}</h3>
					{#if quest.giver}
						<p class="quest-giver">Given by: <span class="giver-name">{quest.giver}</span></p>
					{/if}
					{#if quest.description}
						<p class="quest-desc">{quest.description.slice(0, 140)}{quest.description.length > 140 ? '...' : ''}</p>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</div>

<style>
	.controls { display: flex; gap: 1rem; align-items: center; flex-wrap: wrap; margin-bottom: 1.25rem; }
	.status-filters { display: flex; gap: 0.35rem; flex-wrap: wrap; }
	.filter-btn { background: var(--bg-surface-2); border: 1px solid var(--border); color: var(--text-secondary); padding: 0.35rem 0.75rem; border-radius: var(--radius); cursor: pointer; font-size: 0.8rem; transition: all 0.15s; }
	.filter-btn:hover { border-color: var(--accent-gold-dim); color: var(--accent-gold); }
	.filter-btn.active { background: var(--accent-gold-dim); color: var(--bg-dark); border-color: var(--accent-gold); font-weight: 600; }

	.quest-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 0.75rem; }
	.quest-card { display: block; background: var(--bg-surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 1rem; transition: border-color 0.15s; }
	.quest-card:hover { border-color: var(--accent-gold-dim); text-decoration: none; }
	.card-top { margin-bottom: 0.4rem; }
	.quest-card h3 { font-size: 1rem; color: var(--text-primary); margin-bottom: 0.3rem; }
	.quest-giver { font-size: 0.8rem; color: var(--text-muted); margin-bottom: 0.3rem; }
	.giver-name { color: var(--accent-purple); font-weight: 500; }
	.quest-desc { color: var(--text-secondary); font-size: 0.85rem; line-height: 1.4; }

	.status-badge { font-size: 0.65rem; padding: 0.15rem 0.5rem; border-radius: 999px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
	.status-active { background: rgba(234, 179, 8, 0.15); color: #facc15; border: 1px solid rgba(234, 179, 8, 0.3); }
	.status-completed { background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); }
	.status-failed { background: rgba(239, 68, 68, 0.15); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }

	.empty-state { text-align: center; padding: 3rem 1rem; background: var(--bg-surface); border: 1px solid var(--border); border-radius: var(--radius); }
	.muted { color: var(--text-muted); }
	.error-box { background: rgba(185, 28, 28, 0.15); border: 1px solid #7f1d1d; color: #fca5a5; padding: 0.75rem; border-radius: var(--radius); font-size: 0.9rem; }
</style>
