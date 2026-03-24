<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchEntity, fetchEntities, mergeEntity, renameEntity, type EntityDetail, type Entity } from '$lib/api';

	let entity = $state<EntityDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let expandedSessions = $state<Set<number>>(new Set());

	// Rename state
	let editingName = $state(false);
	let editName = $state('');
	let renameLoading = $state(false);
	let renameError = $state<string | null>(null);

	// Merge state
	let showMergePanel = $state(false);
	let mergeLoading = $state(false);
	let mergeError = $state<string | null>(null);
	let mergeTargets = $state<Entity[]>([]);
	let mergeTargetsLoading = $state(false);
	let selectedMergeId = $state<number | null>(null);
	let showMergeConfirm = $state(false);

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

	function startRename() {
		if (!entity) return;
		editName = entity.name;
		editingName = true;
		renameError = null;
	}

	async function submitRename() {
		if (!entity || !editName.trim() || editName.trim() === entity.name) {
			editingName = false;
			return;
		}
		renameLoading = true;
		renameError = null;
		try {
			await renameEntity(entity.id, editName.trim());
			entity = await fetchEntity(entity.id);
			editingName = false;
		} catch (e) {
			renameError = e instanceof Error ? e.message : 'Failed to rename entity';
		} finally {
			renameLoading = false;
		}
	}

	function handleRenameKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') submitRename();
		if (e.key === 'Escape') editingName = false;
	}

	async function openMergePanel() {
		if (!entity) return;
		showMergePanel = true;
		mergeError = null;
		selectedMergeId = null;
		showMergeConfirm = false;
		mergeTargetsLoading = true;
		try {
			const allEntities = await fetchEntities(entity.campaign_id);
			mergeTargets = allEntities.filter(e => e.id !== entity!.id);
		} catch (e) {
			mergeError = e instanceof Error ? e.message : 'Failed to load entities';
		} finally {
			mergeTargetsLoading = false;
		}
	}

	function closeMergePanel() {
		showMergePanel = false;
		mergeError = null;
		selectedMergeId = null;
		showMergeConfirm = false;
	}

	function requestMergeConfirm() {
		if (!selectedMergeId) return;
		showMergeConfirm = true;
	}

	async function confirmMerge() {
		if (!entity || !selectedMergeId) return;
		mergeLoading = true;
		mergeError = null;
		try {
			await mergeEntity(entity.id, selectedMergeId);
			// Refresh entity data after merge
			entity = await fetchEntity(entity.id);
			closeMergePanel();
		} catch (e) {
			mergeError = e instanceof Error ? e.message : 'Failed to merge entities';
		} finally {
			mergeLoading = false;
		}
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
			{#if editingName}
				<input
					class="rename-input"
					bind:value={editName}
					onkeydown={handleRenameKeydown}
					onblur={submitRename}
					disabled={renameLoading}
					autofocus
				/>
			{:else}
				<h1 class="entity-name-editable" ondblclick={startRename} title="Double-click to rename">{entity.name}</h1>
				<button class="rename-btn" onclick={startRename} title="Rename">&#9998;</button>
			{/if}
			<span class={typeBadgeClass(entity.type)}>{entity.type}</span>
			{#if entity.status === 'dead'}
				<span class="status-badge status-badge-dead">Dead</span>
			{:else if entity.status === 'alive'}
				<span class="status-badge status-badge-alive">Alive</span>
			{:else}
				<span class="status-badge status-badge-unknown">Unknown</span>
			{/if}
			<button class="merge-btn" onclick={openMergePanel}>Merge with...</button>
		</div>
		{#if renameError}
			<div class="error-box rename-error">{renameError}</div>
		{/if}

		<p class="entity-description">{entity.description}</p>

		{#if entity.parent}
			<div class="location-info">
				<span class="location-label">Located in:</span>
				<a href="/campaigns/{$page.params.id}/lore/{entity.parent.id}" class="location-link">{entity.parent.name}</a>
			</div>
		{/if}

		{#if entity.children && entity.children.length > 0}
			<div class="location-info">
				<span class="location-label">Contains:</span>
				<span class="location-children">
					{#each entity.children as child, i (child.id)}
						<a href="/campaigns/{$page.params.id}/lore/{child.id}" class="location-link">{child.name}</a>{#if i < entity.children.length - 1}, {/if}
					{/each}
				</span>
			</div>
		{/if}

		{#if entity.status === 'dead' && entity.cause_of_death}
			<div class="cause-of-death">
				<span class="cod-label">Cause of Death:</span>
				<span class="cod-text">{entity.cause_of_death}</span>
			</div>
		{/if}

		{#if showMergePanel}
			<section class="section-card merge-panel">
				<div class="merge-header">
					<h2>Merge Entity</h2>
					<button class="merge-close" onclick={closeMergePanel}>&times;</button>
				</div>
				<p class="merge-hint">Select another entity to merge into <strong>{entity.name}</strong>. The selected entity will be deleted and its notes, relationships, and references will be moved here.</p>
				{#if mergeTargetsLoading}
					<p class="muted">Loading entities...</p>
				{:else if mergeTargets.length === 0}
					<p class="muted">No other entities available to merge.</p>
				{:else}
					<select class="merge-select" bind:value={selectedMergeId}>
						<option value={null}>-- Select an entity --</option>
						{#each mergeTargets as target (target.id)}
							<option value={target.id}>{target.name} ({target.type})</option>
						{/each}
					</select>
					{#if !showMergeConfirm}
						<button class="merge-confirm-btn" disabled={!selectedMergeId} onclick={requestMergeConfirm}>Merge</button>
					{:else}
						<div class="merge-confirm-box">
							<p>Are you sure you want to merge <strong>{mergeTargets.find(t => t.id === selectedMergeId)?.name}</strong> into <strong>{entity.name}</strong>? This cannot be undone.</p>
							<div class="merge-confirm-actions">
								<button class="merge-confirm-yes" disabled={mergeLoading} onclick={confirmMerge}>
									{mergeLoading ? 'Merging...' : 'Yes, merge'}
								</button>
								<button class="merge-confirm-no" onclick={() => showMergeConfirm = false}>Cancel</button>
							</div>
						</div>
					{/if}
				{/if}
				{#if mergeError}
					<div class="error-box">{mergeError}</div>
				{/if}
			</section>
		{/if}

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
	.entity-name-editable {
		cursor: default;
	}
	.rename-btn {
		background: none;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		font-size: 1rem;
		padding: 0.15rem 0.3rem;
		border-radius: var(--radius);
		line-height: 1;
	}
	.rename-btn:hover {
		color: var(--accent-gold);
		background: rgba(255, 255, 255, 0.05);
	}
	.rename-input {
		font-size: 1.5rem;
		font-weight: bold;
		color: var(--accent-gold);
		background: var(--bg-surface-2);
		border: 1px solid var(--accent-gold-dim);
		border-radius: var(--radius);
		padding: 0.15rem 0.5rem;
		outline: none;
		min-width: 200px;
	}
	.rename-input:focus {
		border-color: var(--accent-gold);
	}
	.rename-error {
		margin-bottom: 0.75rem;
	}

	.status-badge {
		font-size: 0.7rem;
		padding: 0.15rem 0.55rem;
		border-radius: 999px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.status-badge-dead {
		background: rgba(239, 68, 68, 0.2);
		color: #f87171;
		border: 1px solid rgba(239, 68, 68, 0.3);
	}
	.status-badge-alive {
		background: rgba(34, 197, 94, 0.2);
		color: #4ade80;
		border: 1px solid rgba(34, 197, 94, 0.3);
	}
	.status-badge-unknown {
		background: rgba(163, 163, 163, 0.15);
		color: #a3a3a3;
		border: 1px solid rgba(163, 163, 163, 0.3);
	}

	.cause-of-death {
		background: rgba(239, 68, 68, 0.08);
		border: 1px solid rgba(239, 68, 68, 0.2);
		border-radius: var(--radius);
		padding: 0.75rem 1rem;
		margin-bottom: 1.5rem;
	}
	.cod-label {
		color: #f87171;
		font-size: 0.8rem;
		font-weight: 600;
		margin-right: 0.4rem;
	}
	.cod-text {
		color: var(--text-secondary);
		font-size: 0.9rem;
	}

	.merge-btn {
		margin-left: auto;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.35rem 0.75rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.8rem;
		transition: all 0.15s;
	}
	.merge-btn:hover {
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
	}

	.merge-panel {
		border-color: var(--accent-gold-dim);
	}
	.merge-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.5rem;
	}
	.merge-close {
		background: none;
		border: none;
		color: var(--text-muted);
		font-size: 1.25rem;
		cursor: pointer;
		padding: 0;
		line-height: 1;
	}
	.merge-close:hover {
		color: var(--text-primary);
	}
	.merge-hint {
		color: var(--text-secondary);
		font-size: 0.85rem;
		margin-bottom: 0.75rem;
		line-height: 1.5;
	}
	.merge-select {
		width: 100%;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.5rem 0.75rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		margin-bottom: 0.75rem;
	}
	.merge-select:focus {
		outline: none;
		border-color: var(--accent-gold-dim);
	}
	.merge-confirm-btn {
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border: 1px solid var(--accent-gold);
		padding: 0.4rem 1rem;
		border-radius: var(--radius);
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s;
	}
	.merge-confirm-btn:hover:not(:disabled) {
		background: var(--accent-gold);
	}
	.merge-confirm-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.merge-confirm-box {
		background: rgba(185, 28, 28, 0.1);
		border: 1px solid rgba(185, 28, 28, 0.3);
		border-radius: var(--radius);
		padding: 0.75rem;
		margin-bottom: 0.5rem;
	}
	.merge-confirm-box p {
		color: var(--text-primary);
		font-size: 0.85rem;
		margin-bottom: 0.5rem;
		line-height: 1.5;
	}
	.merge-confirm-actions {
		display: flex;
		gap: 0.5rem;
	}
	.merge-confirm-yes {
		background: rgba(185, 28, 28, 0.6);
		color: #fca5a5;
		border: 1px solid rgba(185, 28, 28, 0.8);
		padding: 0.35rem 0.75rem;
		border-radius: var(--radius);
		font-size: 0.8rem;
		font-weight: 600;
		cursor: pointer;
	}
	.merge-confirm-yes:hover:not(:disabled) {
		background: rgba(185, 28, 28, 0.8);
	}
	.merge-confirm-yes:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.merge-confirm-no {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.35rem 0.75rem;
		border-radius: var(--radius);
		font-size: 0.8rem;
		cursor: pointer;
	}
	.merge-confirm-no:hover {
		border-color: var(--accent-gold-dim);
		color: var(--text-primary);
	}

	.location-info {
		display: flex;
		align-items: baseline;
		gap: 0.4rem;
		margin-bottom: 0.5rem;
		padding: 0.5rem 0.75rem;
		background: rgba(34, 197, 94, 0.08);
		border: 1px solid rgba(34, 197, 94, 0.2);
		border-radius: var(--radius);
	}
	.location-info:last-of-type {
		margin-bottom: 1.5rem;
	}
	.location-label {
		color: #4ade80;
		font-size: 0.8rem;
		font-weight: 600;
		white-space: nowrap;
	}
	.location-link {
		color: var(--accent-gold);
		font-size: 0.9rem;
		font-weight: 500;
	}
	.location-link:hover {
		text-decoration: underline;
	}
	.location-children {
		font-size: 0.9rem;
		color: var(--text-secondary);
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
