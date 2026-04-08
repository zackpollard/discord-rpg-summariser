<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchCreature, type CreatureDetail } from '$lib/api';

	const campaignId = $derived(Number($page.params.id));
	const creatureId = $derived(Number($page.params.creatureId));

	let creature = $state<CreatureDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	function formatTime(seconds: number): string {
		const m = Math.floor(seconds / 60);
		const s = Math.floor(seconds % 60);
		return `${m}:${s.toString().padStart(2, '0')}`;
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		});
	}

	onMount(async () => {
		try {
			creature = await fetchCreature(creatureId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load creature';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>{creature?.name ?? 'Creature'} - Bestiary - RPG Summariser</title>
</svelte:head>

<div class="creature-detail">
	<a href="/campaigns/{campaignId}/bestiary" class="back-link">&larr; Back to Bestiary</a>

	{#if loading}
		<p class="muted">Loading creature...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if creature}
		<div class="header">
			<h1>{creature.name}</h1>
			<div class="header-badges">
				{#if creature.creature_stats?.creature_type}
					<span class="type-badge">{creature.creature_stats.creature_type}</span>
				{/if}
				<span class="status-badge status-{creature.status}">{creature.status}</span>
			</div>
		</div>

		{#if creature.description}
			<p class="description">{creature.description}</p>
		{/if}

		{#if creature.creature_stats}
			<section class="stat-block">
				<h2>Statistics</h2>
				<div class="stat-grid">
					{#if creature.creature_stats.challenge_rating}
						<div class="stat-item">
							<span class="stat-label">Challenge Rating</span>
							<span class="stat-value">{creature.creature_stats.challenge_rating}</span>
						</div>
					{/if}
					{#if creature.creature_stats.armor_class}
						<div class="stat-item">
							<span class="stat-label">Armor Class</span>
							<span class="stat-value">{creature.creature_stats.armor_class}</span>
						</div>
					{/if}
					{#if creature.creature_stats.hit_points}
						<div class="stat-item">
							<span class="stat-label">Hit Points</span>
							<span class="stat-value">{creature.creature_stats.hit_points}</span>
						</div>
					{/if}
				</div>
				{#if creature.creature_stats.abilities}
					<div class="stat-section">
						<span class="stat-label">Abilities</span>
						<p class="stat-text">{creature.creature_stats.abilities}</p>
					</div>
				{/if}
				{#if creature.creature_stats.loot}
					<div class="stat-section">
						<span class="stat-label">Loot</span>
						<p class="stat-text">{creature.creature_stats.loot}</p>
					</div>
				{/if}
			</section>
		{/if}

		{#if creature.combat_stats}
			<section class="card">
				<h2>Combat Statistics</h2>
				<div class="combat-stats-grid">
					<div class="combat-stat">
						<span class="combat-stat-value">{creature.combat_stats.total_encounters}</span>
						<span class="combat-stat-label">Encounters</span>
					</div>
					<div class="combat-stat">
						<span class="combat-stat-value">{creature.combat_stats.total_damage_dealt}</span>
						<span class="combat-stat-label">Damage Dealt</span>
					</div>
					<div class="combat-stat">
						<span class="combat-stat-value">{creature.combat_stats.total_damage_taken}</span>
						<span class="combat-stat-label">Damage Taken</span>
					</div>
				</div>
				{#if creature.combat_stats.defeated_by.length > 0}
					<div class="defeated-by">
						<span class="stat-label">Attacked By</span>
						<div class="defeated-list">
							{#each creature.combat_stats.defeated_by as name}
								<span class="defeated-name">{name}</span>
							{/each}
						</div>
					</div>
				{/if}
			</section>
		{/if}

		{#if creature.encounter_history && creature.encounter_history.length > 0}
			<section class="card">
				<h2>Encounter History</h2>
				<div class="encounter-list">
					{#each creature.encounter_history as enc (enc.id)}
						<a href="/sessions/{enc.session_id}#combat" class="encounter-item">
							<div class="encounter-info">
								<span class="encounter-name">{enc.name}</span>
								<span class="encounter-time">{formatTime(enc.start_time)} - {formatTime(enc.end_time)}</span>
							</div>
							{#if enc.summary}
								<p class="encounter-summary">{enc.summary}</p>
							{/if}
							<span class="encounter-date">{formatDate(enc.created_at)}</span>
						</a>
					{/each}
				</div>
			</section>
		{/if}

		{#if creature.notes && creature.notes.length > 0}
			<section class="card">
				<h2>Session Notes</h2>
				<div class="notes-list">
					{#each creature.notes as note (note.id)}
						<div class="note-item">
							<span class="note-session">Session {note.session_id}</span>
							<p class="note-content">{note.content}</p>
							<span class="note-date">{formatDate(note.created_at)}</span>
						</div>
					{/each}
				</div>
			</section>
		{/if}

		{#if creature.relationships && creature.relationships.length > 0}
			<section class="card">
				<h2>Relationships</h2>
				<div class="relationships-list">
					{#each creature.relationships as rel (rel.id)}
						<div class="relationship-item">
							<a href="/campaigns/{campaignId}/lore/{rel.source_id === creature.id ? rel.target_id : rel.source_id}">
								{rel.source_id === creature.id ? rel.target_name : rel.source_name}
							</a>
							<span class="relationship-type">{rel.relationship.replace('_', ' ')}</span>
							{#if rel.description}
								<span class="relationship-desc">{rel.description}</span>
							{/if}
						</div>
					{/each}
				</div>
			</section>
		{/if}
	{/if}
</div>

<style>
	.creature-detail {
		max-width: 900px;
	}

	.back-link {
		display: inline-block;
		margin-bottom: 1rem;
		font-size: 0.9rem;
		color: var(--text-muted);
	}
	.back-link:hover { color: var(--accent-gold); }

	.header {
		margin-bottom: 1rem;
	}
	.header h1 {
		color: var(--accent-gold);
		font-size: 1.5rem;
		margin-bottom: 0.5rem;
	}
	.header-badges {
		display: flex;
		gap: 0.5rem;
	}
	.type-badge {
		font-size: 0.7rem;
		text-transform: capitalize;
		padding: 0.15rem 0.5rem;
		border-radius: 3px;
		background: rgba(139, 92, 246, 0.2);
		color: #c4b5fd;
		font-weight: 600;
	}
	.status-badge {
		font-size: 0.7rem;
		text-transform: uppercase;
		padding: 0.15rem 0.5rem;
		border-radius: 3px;
		font-weight: 600;
	}
	.status-dead { background: rgba(239, 68, 68, 0.2); color: #fca5a5; }
	.status-alive { background: rgba(34, 197, 94, 0.2); color: #86efac; }
	.status-unknown { background: rgba(255, 255, 255, 0.08); color: var(--text-muted); }

	.description {
		color: var(--text-secondary);
		line-height: 1.6;
		margin-bottom: 1.25rem;
	}

	/* Stat block - D&D style */
	.stat-block {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.25rem;
		border-top: 3px solid var(--accent-gold);
	}
	.stat-block h2 {
		font-size: 1rem;
		color: var(--accent-gold);
		margin-bottom: 0.75rem;
		font-weight: 600;
	}
	.stat-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 1rem;
		margin-bottom: 0.75rem;
	}
	.stat-item {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}
	.stat-label {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		font-weight: 600;
	}
	.stat-value {
		font-size: 1.1rem;
		color: var(--text-primary);
		font-weight: 600;
	}
	.stat-section {
		margin-top: 0.75rem;
	}
	.stat-text {
		color: var(--text-primary);
		font-size: 0.9rem;
		line-height: 1.5;
		margin-top: 0.25rem;
	}

	/* Cards */
	.card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.25rem;
	}
	.card h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--border);
	}

	/* Combat stats */
	.combat-stats-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 1rem;
		margin-bottom: 0.75rem;
	}
	.combat-stat {
		text-align: center;
		padding: 0.75rem;
		background: var(--bg-dark);
		border-radius: var(--radius);
	}
	.combat-stat-value {
		display: block;
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--accent-gold);
	}
	.combat-stat-label {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}

	.defeated-by {
		margin-top: 0.75rem;
	}
	.defeated-list {
		display: flex;
		gap: 0.4rem;
		flex-wrap: wrap;
		margin-top: 0.25rem;
	}
	.defeated-name {
		font-size: 0.8rem;
		padding: 0.15rem 0.5rem;
		background: var(--bg-dark);
		border-radius: 3px;
		color: var(--text-primary);
	}

	/* Encounters */
	.encounter-list {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.encounter-item {
		display: block;
		padding: 0.75rem;
		background: var(--bg-dark);
		border-radius: var(--radius);
		color: inherit;
		text-decoration: none;
		transition: background 0.15s;
	}
	.encounter-item:hover {
		background: rgba(212, 175, 125, 0.06);
		text-decoration: none;
	}
	.encounter-info {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.25rem;
	}
	.encounter-name {
		font-weight: 600;
		color: var(--accent-gold);
	}
	.encounter-time {
		font-size: 0.8rem;
		color: var(--text-muted);
		font-family: 'Courier New', monospace;
	}
	.encounter-summary {
		font-size: 0.85rem;
		color: var(--text-secondary);
		line-height: 1.4;
		margin: 0;
	}
	.encounter-date {
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	/* Notes */
	.notes-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.note-item {
		padding: 0.5rem 0;
		border-bottom: 1px solid rgba(255, 255, 255, 0.03);
	}
	.note-item:last-child { border-bottom: none; }
	.note-session {
		font-size: 0.7rem;
		text-transform: uppercase;
		color: var(--text-muted);
		font-weight: 600;
	}
	.note-content {
		color: var(--text-primary);
		font-size: 0.9rem;
		line-height: 1.5;
		margin: 0.15rem 0;
	}
	.note-date {
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	/* Relationships */
	.relationships-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.relationship-item {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.9rem;
	}
	.relationship-type {
		font-size: 0.7rem;
		text-transform: uppercase;
		padding: 0.1rem 0.4rem;
		background: rgba(255, 255, 255, 0.06);
		border-radius: 3px;
		color: var(--text-muted);
	}
	.relationship-desc {
		color: var(--text-secondary);
		font-size: 0.85rem;
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
