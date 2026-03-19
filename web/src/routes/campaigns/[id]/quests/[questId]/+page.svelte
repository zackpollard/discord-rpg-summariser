<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchQuest, type QuestDetail } from '$lib/api';

	let quest = $state<QuestDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	function statusBadgeClass(status: string): string {
		return `status-badge status-${status}`;
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		});
	}

	onMount(() => {
		const unsub = page.subscribe(async ($page) => {
			const id = Number($page.params.questId);
			if (isNaN(id)) {
				error = 'Invalid quest ID';
				loading = false;
				return;
			}
			loading = true;
			error = null;
			try {
				quest = await fetchQuest(id);
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to load quest';
			} finally {
				loading = false;
			}
		});
		return unsub;
	});
</script>

<svelte:head>
	<title>{quest ? quest.name : 'Quest'} - Quests - RPG Summariser</title>
</svelte:head>

<div class="quest-page">
	<a href="/campaigns/{$page.params.id}/quests" class="back-link">&larr; Back to Quests</a>

	{#if loading}
		<p class="muted">Loading quest...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if quest}
		<div class="quest-header">
			<h1>{quest.name}</h1>
			<span class={statusBadgeClass(quest.status)}>{quest.status}</span>
		</div>

		{#if quest.giver}
			<p class="quest-giver">Given by: <span class="giver-name">{quest.giver}</span></p>
		{/if}

		<p class="quest-description">{quest.description}</p>

		{#if quest.updates && quest.updates.length > 0}
			<section class="section-card">
				<h2>Quest Updates</h2>
				<div class="updates-timeline">
					{#each quest.updates as update (update.id)}
						<div class="update-item">
							<div class="update-meta">
								<span class="update-date">{formatDate(update.created_at)}</span>
								<span class="update-session">Session #{update.session_id}</span>
								{#if update.new_status}
									<span class={statusBadgeClass(update.new_status)}>{update.new_status}</span>
								{/if}
							</div>
							<p class="update-content">{update.content}</p>
						</div>
					{/each}
				</div>
			</section>
		{/if}
	{/if}
</div>

<style>
	.quest-page {
		max-width: 800px;
	}

	.back-link {
		display: inline-block;
		margin-bottom: 1rem;
		font-size: 0.85rem;
		color: var(--accent-gold);
	}

	.quest-header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.5rem;
	}
	.quest-header h1 {
		color: var(--accent-gold);
		font-size: 1.5rem;
	}

	.quest-giver {
		font-size: 0.9rem;
		color: var(--text-muted);
		margin-bottom: 0.75rem;
	}
	.giver-name {
		color: var(--accent-purple);
		font-weight: 600;
	}

	.quest-description {
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

	.updates-timeline {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.update-item {
		padding: 0.75rem;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.update-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.35rem;
	}
	.update-date {
		color: var(--accent-gold-dim);
		font-size: 0.8rem;
		font-weight: 500;
	}
	.update-session {
		color: var(--text-muted);
		font-size: 0.75rem;
	}
	.update-content {
		color: var(--text-primary);
		font-size: 0.85rem;
		line-height: 1.5;
	}

	.status-badge { font-size: 0.65rem; padding: 0.15rem 0.5rem; border-radius: 999px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
	.status-active { background: rgba(234, 179, 8, 0.15); color: #facc15; border: 1px solid rgba(234, 179, 8, 0.3); }
	.status-completed { background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); }
	.status-failed { background: rgba(239, 68, 68, 0.15); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }

	.muted { color: var(--text-muted); }
	.error-box { background: rgba(185, 28, 28, 0.15); border: 1px solid #7f1d1d; color: #fca5a5; padding: 0.75rem; border-radius: var(--radius); font-size: 0.9rem; }
</style>
