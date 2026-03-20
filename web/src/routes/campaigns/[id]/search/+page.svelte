<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { searchTranscripts, type TranscriptSearchResult, type TranscriptSearchResponse } from '$lib/api';

	const campaignId = $derived(Number($page.params.id));

	let query = $state('');
	let results = $state<TranscriptSearchResult[]>([]);
	let total = $state(0);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let offset = $state(0);
	const pageSize = 20;

	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	function handleInput(value: string) {
		query = value;
		offset = 0;
		if (debounceTimer) clearTimeout(debounceTimer);
		if (!value.trim()) {
			results = [];
			total = 0;
			error = null;
			return;
		}
		debounceTimer = setTimeout(() => doSearch(), 300);
	}

	async function doSearch() {
		if (!query.trim()) return;
		loading = true;
		error = null;
		try {
			const resp: TranscriptSearchResponse = await searchTranscripts(campaignId, query.trim(), pageSize, offset);
			results = resp.results;
			total = resp.total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Search failed';
			results = [];
			total = 0;
		} finally {
			loading = false;
		}
	}

	function nextPage() {
		offset += pageSize;
		doSearch();
	}

	function prevPage() {
		offset = Math.max(0, offset - pageSize);
		doSearch();
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit'
		});
	}

	function formatTimestamp(seconds: number): string {
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = Math.floor(seconds % 60);
		if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function speakerLabel(r: TranscriptSearchResult): string {
		if (r.character_name) return r.character_name;
		return r.display_name || r.user_id;
	}

	function charColor(name: string): string {
		let hash = 0;
		for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
		return `hsl(${((hash % 360) + 360) % 360}, 70%, 65%)`;
	}

	const currentPage = $derived(Math.floor(offset / pageSize) + 1);
	const totalPages = $derived(Math.ceil(total / pageSize));
	const hasMore = $derived(offset + pageSize < total);
</script>

<svelte:head>
	<title>Search Transcripts - RPG Summariser</title>
</svelte:head>

<div class="search-page">
	<section class="search-section">
		<h2>Search Transcripts</h2>
		<input
			type="text"
			placeholder="Search across all session transcripts..."
			value={query}
			oninput={(e) => handleInput(e.currentTarget.value)}
			class="search-input"
		/>
	</section>

	{#if loading}
		<div class="loading">
			<span class="spinner"></span>
			<span>Searching...</span>
		</div>
	{/if}

	{#if error}
		<div class="error-box">{error}</div>
	{/if}

	{#if !loading && query.trim() && results.length === 0 && !error}
		<div class="empty-state">
			<p>No results found for "{query}"</p>
			<p class="muted">Try different keywords or a simpler search term.</p>
		</div>
	{/if}

	{#if results.length > 0}
		<div class="results-header">
			<span class="result-count">{total} result{total !== 1 ? 's' : ''} found</span>
		</div>

		<div class="results-list">
			{#each results as result (result.segment_id)}
				<div class="result-card">
					<div class="result-meta">
						<a href="/sessions/{result.session_id}" class="session-link">Session #{result.session_id}</a>
						<span class="session-date">{formatDate(result.session_started_at)}</span>
						<span class="speaker" style="color: {charColor(speakerLabel(result))}">{speakerLabel(result)}</span>
						<span class="timestamp">[{formatTimestamp(result.start_time)}]</span>
					</div>
					<div class="result-headline">{@html result.headline}</div>
					<div class="result-actions">
						<a href="/sessions/{result.session_id}#seg-{Math.floor(result.start_time)}" class="view-link">View in transcript</a>
					</div>
				</div>
			{/each}
		</div>

		{#if totalPages > 1}
			<div class="pagination">
				<button onclick={prevPage} disabled={offset === 0}>Previous</button>
				<span class="page-info">Page {currentPage} of {totalPages}</span>
				<button onclick={nextPage} disabled={!hasMore}>Next</button>
			</div>
		{/if}
	{/if}
</div>

<style>
	.search-page {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.search-section {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
	}
	.search-section h2 {
		font-size: 1rem;
		color: var(--text-secondary);
		margin-bottom: 0.75rem;
		font-weight: 600;
	}

	.search-input {
		width: 100%;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-primary);
		padding: 0.6rem 0.75rem;
		border-radius: var(--radius);
		font-size: 0.95rem;
		box-sizing: border-box;
	}
	.search-input:focus {
		outline: none;
		border-color: var(--accent-gold-dim);
	}

	.loading {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		color: var(--text-muted);
		font-size: 0.85rem;
		padding: 0.5rem 0;
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

	.results-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}
	.result-count {
		font-size: 0.85rem;
		color: var(--text-muted);
	}

	.results-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.result-card {
		display: block;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.75rem 1rem;
		transition: border-color 0.15s;
	}
	.result-card:hover {
		border-color: var(--accent-gold-dim);
	}

	.result-meta {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 0.35rem;
		flex-wrap: wrap;
	}
	.session-link {
		font-weight: 600;
		font-size: 0.85rem;
		color: var(--accent-gold);
	}
	.session-link:hover {
		text-decoration: underline;
	}
	.speaker {
		font-weight: 600;
		font-size: 0.85rem;
	}
	.timestamp {
		font-family: monospace;
		font-size: 0.8rem;
		color: var(--text-muted);
	}
	.session-date {
		font-size: 0.8rem;
		color: var(--text-muted);
	}

	.result-actions {
		margin-top: 0.35rem;
	}
	.view-link {
		font-size: 0.8rem;
		color: var(--accent-gold-dim);
	}
	.view-link:hover {
		color: var(--accent-gold);
		text-decoration: underline;
	}

	.result-headline {
		font-size: 0.9rem;
		color: var(--text-secondary);
		line-height: 1.5;
	}
	.result-headline :global(mark) {
		background: rgba(234, 179, 8, 0.3);
		color: var(--accent-gold);
		padding: 0.05rem 0.15rem;
		border-radius: 2px;
	}

	.pagination {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 1rem;
		margin-top: 0.5rem;
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
	.muted { color: var(--text-muted); }
	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
	}
</style>
