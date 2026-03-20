<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchEntity, type EntityDetail } from '$lib/api';

	let entity = $state<EntityDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let expandedSessions = $state<Set<number>>(new Set());

	function typeBadgeClass(type: string): string {
		return `type-badge type-${type}`;
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		});
	}

	function formatTimestamp(seconds: number): string {
		const m = Math.floor(seconds / 60);
		const s = Math.floor(seconds % 60);
		return `${m}:${s.toString().padStart(2, '0')}`;
	}

	function relationshipLabel(rel: string): string {
		return rel.replace(/_/g, ' ');
	}

	function toggleSession(sessionId: number) {
		const next = new Set(expandedSessions);
		if (next.has(sessionId)) {
			next.delete(sessionId);
		} else {
			next.add(sessionId);
		}
		expandedSessions = next;
	}

	onMount(() => {
		const unsub = page.subscribe(async ($page) => {
			const id = Number($page.params.entityId);
			if (isNaN(id)) {
				error = 'Invalid entity ID';
				loading = false;
				return;
			}
			loading = true;
			error = null;
			try {
				entity = await fetchEntity(id);
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to load entity';
			} finally {
				loading = false;
			}
		});
		return unsub;
	});
</script>

<svelte:head>
	<title>{entity ? entity.name : 'Entity'} - Lore - RPG Summariser</title>
</svelte:head>

<div class="entity-page">
	<a href="/campaigns/{$page.params.id}/lore" class="back-link">&larr; Back to Lore</a>

	{#if loading}
		<p class="muted">Loading entity...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if entity}
		<div class="entity-header">
			<h1>{entity.name}</h1>
			<span class={typeBadgeClass(entity.type)}>{entity.type}</span>
		</div>

		<p class="entity-description">{entity.description}</p>

		{#if entity.notes && entity.notes.length > 0}
			<section class="section-card">
				<h2>Session Notes</h2>
				<div class="notes-timeline">
					{#each entity.notes as note (note.id)}
						<div class="note-item">
							<div class="note-meta">
								<span class="note-date">{formatDate(note.created_at)}</span>
								<span class="note-session">Session #{note.session_id}</span>
							</div>
							<p class="note-content">{note.content}</p>
						</div>
					{/each}
				</div>
			</section>
		{/if}

		{#if entity.relationships && entity.relationships.length > 0}
			<section class="section-card">
				<h2>Relationships</h2>
				<div class="relationship-list">
					{#each entity.relationships as rel (rel.id)}
						<div class="rel-item">
							<div class="rel-entities">
								{#if rel.source_id === entity.id}
									<span class="rel-self">{rel.source_name}</span>
									<span class="rel-arrow">&rarr;</span>
									<a href="/campaigns/{$page.params.id}/lore/{rel.target_id}" class="rel-link">{rel.target_name}</a>
								{:else}
									<a href="/campaigns/{$page.params.id}/lore/{rel.source_id}" class="rel-link">{rel.source_name}</a>
									<span class="rel-arrow">&rarr;</span>
									<span class="rel-self">{rel.target_name}</span>
								{/if}
							</div>
							<span class="rel-type">{relationshipLabel(rel.relationship)}</span>
							{#if rel.description}
								<p class="rel-desc">{rel.description}</p>
							{/if}
						</div>
					{/each}
				</div>
			</section>
		{/if}

		{#if entity.sessions && entity.sessions.length > 0}
			<section class="section-card">
				<h2>Appearances</h2>
				<div class="appearances-list">
					{#each entity.sessions as sess (sess.session_id)}
						<div class="appearance-item">
							<button
								class="appearance-header"
								onclick={() => toggleSession(sess.session_id)}
							>
								<div class="appearance-info">
									<a href="/sessions/{sess.session_id}" class="appearance-session-link">Session #{sess.session_id}</a>
									<span class="appearance-date">{formatDate(sess.started_at)}</span>
								</div>
								<div class="appearance-meta">
									<span class="mention-count">{sess.mention_count} mention{sess.mention_count !== 1 ? 's' : ''}</span>
									<span class="expand-arrow" class:expanded={expandedSessions.has(sess.session_id)}>&#9662;</span>
								</div>
							</button>
							{#if expandedSessions.has(sess.session_id) && entity.references}
								<div class="reference-list">
									{#each entity.references.filter(r => r.session_id === sess.session_id) as ref}
										<div class="reference-item">
											<a
												href="/sessions/{ref.session_id}?t={Math.floor(ref.start_time)}"
												class="ref-timestamp"
											>{formatTimestamp(ref.start_time)}</a>
											<p class="ref-context">{ref.context}</p>
										</div>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</section>
		{/if}
	{/if}
</div>

<style>
	.entity-page {
		max-width: 800px;
	}

	.back-link {
		display: inline-block;
		margin-bottom: 1rem;
		font-size: 0.85rem;
		color: var(--accent-gold);
	}

	.entity-header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.75rem;
	}
	.entity-header h1 {
		color: var(--accent-gold);
		font-size: 1.5rem;
	}

	.entity-description {
		color: var(--text-secondary);
		font-size: 0.95rem;
		line-height: 1.6;
		margin-bottom: 1.5rem;
	}

	.section-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		margin-bottom: 1.25rem;
	}
	.section-card h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
	}

	.notes-timeline {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.note-item {
		padding: 0.75rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.note-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.35rem;
	}
	.note-date {
		color: var(--accent-gold-dim);
		font-size: 0.8rem;
		font-weight: 500;
	}
	.note-session {
		color: var(--text-muted);
		font-size: 0.75rem;
	}
	.note-content {
		color: var(--text-primary);
		font-size: 0.85rem;
		line-height: 1.5;
	}

	.relationship-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.rel-item {
		padding: 0.65rem 0.75rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.rel-entities {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		margin-bottom: 0.2rem;
	}
	.rel-self {
		color: var(--text-primary);
		font-weight: 600;
		font-size: 0.85rem;
	}
	.rel-arrow {
		color: var(--text-muted);
		font-size: 0.8rem;
	}
	.rel-link {
		color: var(--accent-gold);
		font-weight: 600;
		font-size: 0.85rem;
	}
	.rel-type {
		display: inline-block;
		font-size: 0.7rem;
		padding: 0.1rem 0.45rem;
		border-radius: 999px;
		background: rgba(139, 92, 246, 0.15);
		color: var(--accent-purple);
		border: 1px solid rgba(139, 92, 246, 0.25);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		font-weight: 600;
	}
	.rel-desc {
		color: var(--text-secondary);
		font-size: 0.8rem;
		margin-top: 0.25rem;
		line-height: 1.4;
	}

	.type-badge {
		font-size: 0.7rem;
		padding: 0.15rem 0.5rem;
		border-radius: 999px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.type-pc {
		background: rgba(236, 72, 153, 0.2);
		color: #f472b6;
		border: 1px solid rgba(236, 72, 153, 0.3);
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

	.appearances-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.appearance-item {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.appearance-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		width: 100%;
		padding: 0.65rem 0.75rem;
		background: none;
		border: none;
		color: inherit;
		cursor: pointer;
		text-align: left;
		font-size: inherit;
	}
	.appearance-header:hover {
		background: rgba(255, 255, 255, 0.03);
	}
	.appearance-info {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
	.appearance-session-link {
		color: var(--accent-gold);
		font-weight: 600;
		font-size: 0.85rem;
	}
	.appearance-date {
		color: var(--text-muted);
		font-size: 0.8rem;
	}
	.appearance-meta {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.mention-count {
		font-size: 0.75rem;
		padding: 0.1rem 0.45rem;
		border-radius: 999px;
		background: rgba(139, 92, 246, 0.15);
		color: var(--accent-purple);
		border: 1px solid rgba(139, 92, 246, 0.25);
		font-weight: 600;
	}
	.expand-arrow {
		color: var(--text-muted);
		font-size: 0.75rem;
		transition: transform 0.15s;
	}
	.expand-arrow.expanded {
		transform: rotate(180deg);
	}
	.reference-list {
		border-top: 1px solid var(--border);
		padding: 0.5rem 0.75rem;
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	.reference-item {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		padding: 0.35rem 0;
	}
	.ref-timestamp {
		color: var(--accent-gold-dim);
		font-size: 0.75rem;
		font-family: monospace;
		white-space: nowrap;
		min-width: 3.5rem;
		padding-top: 0.1rem;
	}
	.ref-context {
		color: var(--text-secondary);
		font-size: 0.8rem;
		line-height: 1.4;
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
