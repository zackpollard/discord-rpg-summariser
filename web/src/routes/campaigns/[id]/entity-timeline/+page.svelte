<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { fetchEntityTimeline, type EntityTimelineEntry } from '$lib/api';

	const campaignId = $derived(Number($page.params.id));

	let entries = $state<EntityTimelineEntry[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let activeType = $state<string>('');
	let tooltip = $state<{ x: number; y: number; entry: EntityTimelineEntry } | null>(null);

	const entityTypes = [
		{ value: '', label: 'All' },
		{ value: 'pc', label: 'PCs' },
		{ value: 'npc', label: 'NPCs' },
		{ value: 'place', label: 'Places' },
		{ value: 'organisation', label: 'Organisations' },
		{ value: 'item', label: 'Items' },
		{ value: 'event', label: 'Events' }
	];

	const typeColors: Record<string, string> = {
		pc: '#f472b6',
		npc: '#a78bfa',
		place: '#4ade80',
		organisation: '#60a5fa',
		item: '#facc15',
		event: '#f87171'
	};

	const typeBgColors: Record<string, string> = {
		pc: 'rgba(236, 72, 153, 0.25)',
		npc: 'rgba(139, 92, 246, 0.25)',
		place: 'rgba(34, 197, 94, 0.25)',
		organisation: 'rgba(59, 130, 246, 0.25)',
		item: 'rgba(234, 179, 8, 0.25)',
		event: 'rgba(239, 68, 68, 0.25)'
	};

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		});
	}

	function formatShortDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric',
			month: 'short'
		});
	}

	const filtered = $derived(
		activeType ? entries.filter(e => e.entity_type === activeType) : entries
	);

	// Group filtered entries by type for the swimlane view
	const grouped = $derived(
		filtered.reduce<Record<string, EntityTimelineEntry[]>>((acc, entry) => {
			if (!acc[entry.entity_type]) acc[entry.entity_type] = [];
			acc[entry.entity_type].push(entry);
			return acc;
		}, {})
	);

	// Compute the overall date range from all entries
	const dateRange = $derived(() => {
		if (entries.length === 0) return { min: new Date(), max: new Date(), span: 1 };
		const times = entries.flatMap(e => [
			new Date(e.first_seen).getTime(),
			new Date(e.last_seen).getTime()
		]);
		const min = new Date(Math.min(...times));
		const max = new Date(Math.max(...times));
		// Add 1 day padding on each side
		min.setDate(min.getDate() - 1);
		max.setDate(max.getDate() + 1);
		const span = max.getTime() - min.getTime();
		return { min, max, span: span || 1 };
	});

	function dateToPercent(dateStr: string): number {
		const range = dateRange();
		const t = new Date(dateStr).getTime();
		return ((t - range.min.getTime()) / range.span) * 100;
	}

	// Max mentions for opacity scaling
	const maxMentions = $derived(
		filtered.length > 0 ? Math.max(...filtered.map(e => e.total_mentions)) : 1
	);

	function barOpacity(mentions: number): number {
		return 0.4 + 0.6 * (mentions / maxMentions);
	}

	// Generate X-axis date ticks
	const dateTicks = $derived(() => {
		const range = dateRange();
		const ticks: { date: Date; percent: number }[] = [];
		const spanDays = range.span / (1000 * 60 * 60 * 24);
		// Aim for ~6-10 ticks
		let stepDays = Math.max(1, Math.ceil(spanDays / 8));
		if (spanDays > 60) stepDays = Math.ceil(spanDays / 6);
		const cursor = new Date(range.min);
		cursor.setHours(0, 0, 0, 0);
		while (cursor.getTime() <= range.max.getTime()) {
			const pct = ((cursor.getTime() - range.min.getTime()) / range.span) * 100;
			if (pct >= 0 && pct <= 100) {
				ticks.push({ date: new Date(cursor), percent: pct });
			}
			cursor.setDate(cursor.getDate() + stepDays);
		}
		return ticks;
	});

	// Total rows for SVG height calculation
	const totalRows = $derived(filtered.length);
	const ROW_HEIGHT = 28;
	const HEADER_HEIGHT = 20;
	const AXIS_HEIGHT = 30;
	const LEFT_MARGIN = 160;
	const RIGHT_MARGIN = 20;
	const CHART_WIDTH = 900;
	const svgHeight = $derived(totalRows * ROW_HEIGHT + AXIS_HEIGHT + 10);

	function showTooltip(e: MouseEvent, entry: EntityTimelineEntry) {
		tooltip = { x: e.clientX, y: e.clientY, entry };
	}

	function hideTooltip() {
		tooltip = null;
	}

	async function loadData() {
		loading = true;
		error = null;
		try {
			entries = await fetchEntityTimeline(campaignId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load entity timeline';
		} finally {
			loading = false;
		}
	}

	onMount(() => { loadData(); });
</script>

<svelte:head>
	<title>Entity Timeline - RPG Summariser</title>
</svelte:head>

<div class="entity-timeline-page">
	<h2>Entity Timeline</h2>
	<p class="page-desc">Activity of entities across sessions. Bar length shows the span from first to last appearance; opacity reflects mention density.</p>

	<div class="controls">
		<div class="type-filters">
			{#each entityTypes as t (t.value)}
				<button
					class="filter-btn"
					class:active={activeType === t.value}
					onclick={() => { activeType = t.value; }}
				>{t.label}</button>
			{/each}
		</div>
	</div>

	{#if loading}
		<p class="muted">Loading entity timeline...</p>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if entries.length === 0}
		<div class="empty-state">
			<p>No entity references found.</p>
			<p class="muted">Entity references are created when sessions are processed.</p>
		</div>
	{:else if filtered.length === 0}
		<div class="empty-state">
			<p>No entities of this type have references.</p>
		</div>
	{:else}
		<div class="timeline-container">
			<svg
				class="timeline-svg"
				viewBox="0 0 900 {svgHeight}"
				preserveAspectRatio="xMinYMin meet"
			>
				<!-- X-axis date ticks -->
				{#each dateTicks() as tick}
					{@const x = LEFT_MARGIN + (tick.percent / 100) * (900 - LEFT_MARGIN - RIGHT_MARGIN)}
					<line
						x1={x} y1="0" x2={x} y2={svgHeight - AXIS_HEIGHT}
						stroke="rgba(255,255,255,0.06)"
						stroke-width="1"
					/>
					<text
						x={x} y={svgHeight - 8}
						fill="rgba(255,255,255,0.4)"
						font-size="10"
						text-anchor="middle"
						font-family="system-ui, sans-serif"
					>
						{formatShortDate(tick.date.toISOString())}
					</text>
				{/each}

				<!-- Swimlane rows -->
				{#each Object.entries(grouped) as [type, typeEntries], gi}
					{@const offset = filtered.indexOf(typeEntries[0])}
					<!-- Type group header background -->
					{#each typeEntries as entry, i}
						{@const rowIndex = offset + i}
						{@const y = rowIndex * ROW_HEIGHT}
						{@const leftPct = dateToPercent(entry.first_seen)}
						{@const rightPct = dateToPercent(entry.last_seen)}
						{@const barWidth = Math.max(rightPct - leftPct, 0.5)}
						{@const barX = LEFT_MARGIN + (leftPct / 100) * (900 - LEFT_MARGIN - RIGHT_MARGIN)}
						{@const barW = Math.max((barWidth / 100) * (900 - LEFT_MARGIN - RIGHT_MARGIN), 4)}

						<!-- Alternating row background -->
						{#if rowIndex % 2 === 0}
							<rect
								x="0" y={y} width="900" height={ROW_HEIGHT}
								fill="rgba(255,255,255,0.02)"
							/>
						{/if}

						<!-- Entity name label -->
						<a href="/campaigns/{campaignId}/lore/{entry.entity_id}">
							<text
								x={LEFT_MARGIN - 8}
								y={y + ROW_HEIGHT / 2 + 4}
								fill={typeColors[entry.entity_type] ?? '#999'}
								font-size="11"
								text-anchor="end"
								font-family="system-ui, sans-serif"
								font-weight="500"
								class="entity-label"
							>
								{entry.entity_name.length > 18
									? entry.entity_name.slice(0, 17) + '\u2026'
									: entry.entity_name}
							</text>
						</a>

						<!-- Activity bar -->
						<rect
							x={barX}
							y={y + 5}
							width={barW}
							height={ROW_HEIGHT - 10}
							rx="3"
							ry="3"
							fill={typeColors[entry.entity_type] ?? '#888'}
							opacity={barOpacity(entry.total_mentions)}
							class="bar"
							role="img"
							onmouseenter={(e) => showTooltip(e, entry)}
							onmouseleave={hideTooltip}
						/>

						<!-- Mention count -->
						{#if barW > 28}
							<text
								x={barX + barW / 2}
								y={y + ROW_HEIGHT / 2 + 4}
								fill="rgba(0,0,0,0.7)"
								font-size="9"
								text-anchor="middle"
								font-family="system-ui, sans-serif"
								font-weight="700"
							>
								{entry.total_mentions}
							</text>
						{/if}
					{/each}
				{/each}
			</svg>
		</div>

		<!-- Legend -->
		<div class="legend">
			{#each Object.entries(typeColors) as [type, color]}
				{#if !activeType || activeType === type}
					<span class="legend-item">
						<span class="legend-dot" style="background: {color};"></span>
						{type}
					</span>
				{/if}
			{/each}
		</div>
	{/if}

	{#if tooltip}
		<div
			class="tooltip"
			style="left: {tooltip.x + 12}px; top: {tooltip.y - 10}px;"
		>
			<strong style="color: {typeColors[tooltip.entry.entity_type] ?? '#ccc'}">
				{tooltip.entry.entity_name}
			</strong>
			<span class="tooltip-type">{tooltip.entry.entity_type}</span>
			<div class="tooltip-row">First seen: {formatDate(tooltip.entry.first_seen)}</div>
			<div class="tooltip-row">Last seen: {formatDate(tooltip.entry.last_seen)}</div>
			<div class="tooltip-row">Sessions: {tooltip.entry.session_count}</div>
			<div class="tooltip-row">Total mentions: {tooltip.entry.total_mentions}</div>
		</div>
	{/if}
</div>

<style>
	.entity-timeline-page {
		position: relative;
	}
	h2 {
		color: var(--accent-gold);
		font-size: 1.25rem;
		margin-bottom: 0.25rem;
	}
	.page-desc {
		color: var(--text-secondary);
		font-size: 0.85rem;
		margin-bottom: 1rem;
		line-height: 1.5;
	}

	.controls {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 1rem;
		flex-wrap: wrap;
	}
	.type-filters {
		display: flex;
		gap: 0.35rem;
		flex-wrap: wrap;
	}
	.filter-btn {
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.35rem 0.75rem;
		border-radius: var(--radius);
		cursor: pointer;
		font-size: 0.8rem;
		transition: all 0.15s;
	}
	.filter-btn:hover {
		border-color: var(--accent-gold-dim);
		color: var(--accent-gold);
	}
	.filter-btn.active {
		background: var(--accent-gold-dim);
		color: var(--bg-dark);
		border-color: var(--accent-gold);
		font-weight: 600;
	}

	.timeline-container {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem;
		overflow-x: auto;
	}
	.timeline-svg {
		width: 100%;
		min-width: 700px;
		display: block;
	}

	.timeline-svg :global(.entity-label) {
		cursor: pointer;
	}
	.timeline-svg :global(.entity-label:hover) {
		fill: var(--accent-gold) !important;
	}

	.timeline-svg :global(.bar) {
		cursor: pointer;
		transition: opacity 0.15s;
	}
	.timeline-svg :global(.bar:hover) {
		opacity: 1 !important;
		filter: brightness(1.2);
	}

	.legend {
		display: flex;
		gap: 1rem;
		margin-top: 0.75rem;
		flex-wrap: wrap;
	}
	.legend-item {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		font-size: 0.8rem;
		color: var(--text-secondary);
		text-transform: capitalize;
	}
	.legend-dot {
		width: 10px;
		height: 10px;
		border-radius: 2px;
		flex-shrink: 0;
	}

	.tooltip {
		position: fixed;
		z-index: 100;
		background: var(--bg-surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.65rem 0.85rem;
		pointer-events: none;
		box-shadow: 0 4px 12px rgba(0,0,0,0.4);
		max-width: 280px;
	}
	.tooltip strong {
		display: block;
		font-size: 0.9rem;
		margin-bottom: 0.15rem;
	}
	.tooltip-type {
		display: inline-block;
		font-size: 0.65rem;
		padding: 0.1rem 0.4rem;
		border-radius: 999px;
		background: rgba(255,255,255,0.08);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		font-weight: 600;
		margin-bottom: 0.4rem;
	}
	.tooltip-row {
		font-size: 0.8rem;
		color: var(--text-secondary);
		line-height: 1.5;
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
