<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchSessions, type Session } from '$lib/api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let sessions = $state<Session[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let offset = $state(0);
	let hasMore = $state(true);
	const pageSize = 20;

	async function loadSessions() {
		loading = true;
		error = null;
		try {
			const data = await fetchSessions(pageSize, offset);
			sessions = data;
			hasMore = data.length === pageSize;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load sessions';
		} finally {
			loading = false;
		}
	}

	function nextPage() {
		offset += pageSize;
		loadSessions();
	}

	function prevPage() {
		offset = Math.max(0, offset - pageSize);
		loadSessions();
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

	function formatDuration(start: string, end: string | null): string {
		if (!end) return 'In progress';
		const ms = new Date(end).getTime() - new Date(start).getTime();
		const mins = Math.floor(ms / 60000);
		const hours = Math.floor(mins / 60);
		const remainMins = mins % 60;
		if (hours > 0) return `${hours}h ${remainMins}m`;
		return `${mins}m`;
	}

	const currentPage = $derived(Math.floor(offset / pageSize) + 1);

	onMount(() => {
		loadSessions();
	});
</script>

<svelte:head>
	<title>Sessions - RPG Summariser</title>
</svelte:head>

<div class="sessions-page">
	<h1>Sessions</h1>

	{#if loading}
		<p class="muted">Loading sessions...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if sessions.length === 0}
		<div class="empty-state">
			<p>No sessions recorded yet.</p>
			<p class="muted">Sessions will appear here once the bot starts recording.</p>
		</div>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Date</th>
						<th>Duration</th>
						<th>Status</th>
						<th>Summary</th>
					</tr>
				</thead>
				<tbody>
					{#each sessions as session (session.id)}
						<tr onclick={() => { window.location.href = `/sessions/${session.id}`; }}>
							<td class="nowrap">{formatDate(session.started_at)}</td>
							<td class="nowrap">{formatDuration(session.started_at, session.ended_at)}</td>
							<td><StatusBadge status={session.status} /></td>
							<td class="summary-cell">
								{#if session.summary}
									{session.summary.slice(0, 100)}{session.summary.length > 100 ? '...' : ''}
								{:else}
									<span class="muted">--</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<div class="pagination">
			<button onclick={prevPage} disabled={offset === 0}>Previous</button>
			<span class="page-info">Page {currentPage}</span>
			<button onclick={nextPage} disabled={!hasMore}>Next</button>
		</div>
	{/if}
</div>

<style>
	.sessions-page h1 {
		color: var(--accent-gold);
		margin-bottom: 1.25rem;
		font-size: 1.5rem;
	}

	.table-wrap {
		overflow-x: auto;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}
	thead {
		border-bottom: 1px solid var(--border);
	}
	th {
		text-align: left;
		padding: 0.75rem 1rem;
		font-size: 0.8rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 600;
	}
	td {
		padding: 0.75rem 1rem;
		font-size: 0.9rem;
		border-top: 1px solid var(--border);
	}
	tr {
		cursor: pointer;
		transition: background 0.15s;
	}
	tbody tr:hover {
		background: var(--surface-hover);
	}
	.nowrap {
		white-space: nowrap;
	}
	.summary-cell {
		color: var(--text-secondary);
		max-width: 400px;
	}

	.pagination {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 1rem;
		margin-top: 1rem;
	}
	.page-info {
		font-size: 0.9rem;
		color: var(--text-secondary);
	}
	button {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.4rem 1rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.85rem;
		transition: background 0.15s, border-color 0.15s;
	}
	button:hover:not(:disabled) {
		background: var(--surface-hover);
		border-color: var(--accent-gold-dim);
	}
	button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
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
