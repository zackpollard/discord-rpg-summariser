<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchTimeline, type TimelineEvent } from '$lib/api';

	const campaignId = $derived(Number($page.params.id));

	let events = $state<TimelineEvent[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state<string | null>(null);
	let hasMore = $state(true);

	const PAGE_SIZE = 20;

	function eventTypeClass(type: string): string {
		return `event-badge event-${type.replace('_', '-')}`;
	}

	function eventTypeLabel(type: string): string {
		switch (type) {
			case 'session': return 'Session';
			case 'entity': return 'Entity';
			case 'quest_new': return 'New Quest';
			case 'quest_completed': return 'Quest Done';
			case 'quest_failed': return 'Quest Failed';
			default: return type.replace(/_/g, ' ');
		}
	}

	function eventHref(ev: TimelineEvent): string | null {
		if (ev.type === 'session' && ev.session_id) return `/sessions/${ev.session_id}`;
		if (ev.type === 'entity' && ev.entity_id) return `/campaigns/${campaignId}/lore/${ev.entity_id}`;
		if ((ev.type === 'quest_new' || ev.type === 'quest_completed' || ev.type === 'quest_failed') && ev.quest_id) {
			return `/campaigns/${campaignId}/quests/${ev.quest_id}`;
		}
		return null;
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	async function loadEvents() {
		loading = true;
		error = null;
		try {
			const result = await fetchTimeline(campaignId, PAGE_SIZE, 0);
			events = result;
			hasMore = result.length === PAGE_SIZE;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load timeline';
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		loadingMore = true;
		try {
			const result = await fetchTimeline(campaignId, PAGE_SIZE, events.length);
			events = [...events, ...result];
			hasMore = result.length === PAGE_SIZE;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load more events';
		} finally {
			loadingMore = false;
		}
	}

	onMount(() => { loadEvents(); });
</script>

<svelte:head>
	<title>Timeline - RPG Summariser</title>
</svelte:head>

<div class="timeline-page">
	{#if loading}
		<p class="muted">Loading timeline...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if events.length === 0}
		<div class="empty-state">
			<p>No events yet.</p>
			<p class="muted">Events will appear here as your campaign progresses.</p>
		</div>
	{:else}
		<div class="timeline">
			{#each events as ev, i (i)}
				{@const href = eventHref(ev)}
				<div class="timeline-item">
					<div class="timeline-rail">
						<span class="timeline-dot dot-{ev.type.replace('_', '-')}"></span>
						{#if i < events.length - 1}
							<span class="timeline-line"></span>
						{/if}
					</div>
					<div class="timeline-content">
						<div class="event-meta">
							<span class={eventTypeClass(ev.type)}>{eventTypeLabel(ev.type)}</span>
							<span class="event-date">{formatDate(ev.timestamp)}</span>
						</div>
						{#if href}
							<a {href} class="event-title">{ev.title}</a>
						{:else}
							<span class="event-title-plain">{ev.title}</span>
						{/if}
						{#if ev.detail}
							<p class="event-detail">{ev.detail.slice(0, 200)}{ev.detail.length > 200 ? '...' : ''}</p>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		{#if hasMore}
			<div class="load-more-wrap">
				<button class="load-more-btn" onclick={loadMore} disabled={loadingMore}>
					{loadingMore ? 'Loading...' : 'Load more'}
				</button>
			</div>
		{/if}
	{/if}
</div>

<style>
	.timeline-page {
		max-width: 800px;
	}

	.timeline {
		display: flex;
		flex-direction: column;
	}

	.timeline-item {
		display: flex;
		gap: 1rem;
		min-height: 80px;
	}

	.timeline-rail {
		display: flex;
		flex-direction: column;
		align-items: center;
		width: 18px;
		flex-shrink: 0;
	}

	.timeline-dot {
		width: 12px;
		height: 12px;
		border-radius: 50%;
		flex-shrink: 0;
		margin-top: 0.35rem;
	}
	.dot-session { background: #3b82f6; box-shadow: 0 0 6px rgba(59, 130, 246, 0.4); }
	.dot-entity { background: #8b5cf6; box-shadow: 0 0 6px rgba(139, 92, 246, 0.4); }
	.dot-quest-new { background: #eab308; box-shadow: 0 0 6px rgba(234, 179, 8, 0.4); }
	.dot-quest-completed { background: #22c55e; box-shadow: 0 0 6px rgba(34, 197, 94, 0.4); }
	.dot-quest-failed { background: #ef4444; box-shadow: 0 0 6px rgba(239, 68, 68, 0.4); }

	.timeline-line {
		width: 2px;
		flex: 1;
		background: var(--border);
		margin-top: 4px;
	}

	.timeline-content {
		flex: 1;
		padding-bottom: 1.25rem;
	}

	.event-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.3rem;
	}

	.event-badge {
		font-size: 0.6rem;
		padding: 0.1rem 0.45rem;
		border-radius: 999px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.event-session { background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); }
	.event-entity { background: rgba(139, 92, 246, 0.15); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }
	.event-quest-new { background: rgba(234, 179, 8, 0.15); color: #facc15; border: 1px solid rgba(234, 179, 8, 0.3); }
	.event-quest-completed { background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); }
	.event-quest-failed { background: rgba(239, 68, 68, 0.15); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }

	.event-date {
		color: var(--text-muted);
		font-size: 0.75rem;
	}

	.event-title {
		display: block;
		font-size: 0.95rem;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 0.2rem;
	}
	.event-title:hover {
		color: var(--accent-gold);
		text-decoration: none;
	}
	.event-title-plain {
		display: block;
		font-size: 0.95rem;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 0.2rem;
	}

	.event-detail {
		color: var(--text-secondary);
		font-size: 0.85rem;
		line-height: 1.4;
	}

	.load-more-wrap {
		text-align: center;
		padding: 1rem 0;
	}
	.load-more-btn {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.5rem 1.5rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.85rem;
		transition: all 0.15s;
	}
	.load-more-btn:hover:not(:disabled) {
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
	}
	.load-more-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.empty-state { text-align: center; padding: 3rem 1rem; background: var(--bg-surface); border: 1px solid var(--border); border-radius: var(--radius); }
	.muted { color: var(--text-muted); }
	.error-box { background: rgba(185, 28, 28, 0.15); border: 1px solid #7f1d1d; color: #fca5a5; padding: 0.75rem; border-radius: var(--radius); font-size: 0.9rem; }
</style>
