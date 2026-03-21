<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { fetchCampaignStats, type CampaignStats } from '$lib/api';
	import {
		Chart,
		LineController,
		BarController,
		DoughnutController,
		PieController,
		CategoryScale,
		LinearScale,
		PointElement,
		LineElement,
		BarElement,
		ArcElement,
		Tooltip,
		Legend,
		Title
	} from 'chart.js';

	Chart.register(
		LineController,
		BarController,
		DoughnutController,
		PieController,
		CategoryScale,
		LinearScale,
		PointElement,
		LineElement,
		BarElement,
		ArcElement,
		Tooltip,
		Legend,
		Title
	);

	const campaignId = $derived(Number($page.params.id));

	let stats = $state<CampaignStats | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Canvas refs
	let durationChartEl = $state<HTMLCanvasElement | null>(null);
	let wordsChartEl = $state<HTMLCanvasElement | null>(null);
	let speakerChartEl = $state<HTMLCanvasElement | null>(null);
	let entityTypeChartEl = $state<HTMLCanvasElement | null>(null);
	let topEntitiesChartEl = $state<HTMLCanvasElement | null>(null);
	let questChartEl = $state<HTMLCanvasElement | null>(null);
	let combatChartEl = $state<HTMLCanvasElement | null>(null);
	let npcStatusChartEl = $state<HTMLCanvasElement | null>(null);

	let charts: Chart[] = [];

	const palette = [
		'rgba(212, 175, 125, 0.85)', // gold
		'rgba(139, 92, 246, 0.85)',   // purple
		'rgba(236, 72, 153, 0.85)',   // pink
		'rgba(34, 197, 94, 0.85)',    // green
		'rgba(59, 130, 246, 0.85)',   // blue
		'rgba(234, 179, 8, 0.85)',    // yellow
		'rgba(239, 68, 68, 0.85)',    // red
		'rgba(6, 182, 212, 0.85)',    // cyan
		'rgba(249, 115, 22, 0.85)',   // orange
		'rgba(168, 85, 247, 0.85)',   // violet
	];

	const paletteSolid = [
		'rgba(212, 175, 125, 1)',
		'rgba(139, 92, 246, 1)',
		'rgba(236, 72, 153, 1)',
		'rgba(34, 197, 94, 1)',
		'rgba(59, 130, 246, 1)',
		'rgba(234, 179, 8, 1)',
		'rgba(239, 68, 68, 1)',
		'rgba(6, 182, 212, 1)',
		'rgba(249, 115, 22, 1)',
		'rgba(168, 85, 247, 1)',
	];

	const chartDefaults = {
		color: '#b8a8cc',
		borderColor: '#3d2e50',
		backgroundColor: 'transparent',
	};

	function createCharts() {
		destroyCharts();
		if (!stats) return;

		// Chart.js global defaults for dark theme
		Chart.defaults.color = '#b8a8cc';
		Chart.defaults.borderColor = '#3d2e50';
		Chart.defaults.plugins.legend.labels.color = '#b8a8cc';
		Chart.defaults.plugins.title.color = '#e8e0f0';

		// 1. Session Duration Trend
		if (durationChartEl && stats.session_timeline.length > 0) {
			const labels = stats.session_timeline.map((s, i) => {
				const d = new Date(s.started_at);
				return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
			});
			charts.push(new Chart(durationChartEl, {
				type: 'line',
				data: {
					labels,
					datasets: [{
						label: 'Duration (min)',
						data: stats.session_timeline.map(s => Math.round(s.duration_min)),
						borderColor: paletteSolid[0],
						backgroundColor: 'rgba(212, 175, 125, 0.15)',
						fill: true,
						tension: 0.3,
						pointBackgroundColor: paletteSolid[0],
						pointRadius: 4,
						pointHoverRadius: 6,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					plugins: { legend: { display: false }, title: { display: false } },
					scales: {
						y: { beginAtZero: true, grid: { color: 'rgba(61, 46, 80, 0.5)' } },
						x: { grid: { display: false } }
					}
				}
			}));
		}

		// 2. Words Per Session
		if (wordsChartEl && stats.session_timeline.length > 0) {
			const labels = stats.session_timeline.map((s, i) => {
				const d = new Date(s.started_at);
				return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
			});
			charts.push(new Chart(wordsChartEl, {
				type: 'bar',
				data: {
					labels,
					datasets: [{
						label: 'Words',
						data: stats.session_timeline.map(s => s.word_count),
						backgroundColor: palette[1],
						borderColor: paletteSolid[1],
						borderWidth: 1,
						borderRadius: 4,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					plugins: { legend: { display: false } },
					scales: {
						y: { beginAtZero: true, grid: { color: 'rgba(61, 46, 80, 0.5)' } },
						x: { grid: { display: false } }
					}
				}
			}));
		}

		// 3. Speaking Time by Character (horizontal bar)
		if (speakerChartEl && stats.speaker_stats.length > 0) {
			charts.push(new Chart(speakerChartEl, {
				type: 'bar',
				data: {
					labels: stats.speaker_stats.map(s => s.character_name),
					datasets: [{
						label: 'Words Spoken',
						data: stats.speaker_stats.map(s => s.word_count),
						backgroundColor: stats.speaker_stats.map((_, i) => palette[i % palette.length]),
						borderColor: stats.speaker_stats.map((_, i) => paletteSolid[i % paletteSolid.length]),
						borderWidth: 1,
						borderRadius: 4,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					indexAxis: 'y',
					plugins: { legend: { display: false } },
					scales: {
						x: { beginAtZero: true, grid: { color: 'rgba(61, 46, 80, 0.5)' } },
						y: { grid: { display: false } }
					}
				}
			}));
		}

		// 4. Entity Type Breakdown (doughnut)
		if (entityTypeChartEl && Object.keys(stats.entity_counts).length > 0) {
			const types = Object.keys(stats.entity_counts);
			charts.push(new Chart(entityTypeChartEl, {
				type: 'doughnut',
				data: {
					labels: types.map(t => t.charAt(0).toUpperCase() + t.slice(1)),
					datasets: [{
						data: types.map(t => stats!.entity_counts[t]),
						backgroundColor: types.map((_, i) => palette[i % palette.length]),
						borderColor: '#1a1020',
						borderWidth: 2,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					plugins: {
						legend: { position: 'right', labels: { padding: 12, font: { size: 12 } } }
					}
				}
			}));
		}

		// 5. Most Mentioned Entities (horizontal bar)
		if (topEntitiesChartEl && stats.top_entities.length > 0) {
			charts.push(new Chart(topEntitiesChartEl, {
				type: 'bar',
				data: {
					labels: stats.top_entities.map(e => e.name),
					datasets: [{
						label: 'Mentions',
						data: stats.top_entities.map(e => e.mentions),
						backgroundColor: stats.top_entities.map((_, i) => palette[i % palette.length]),
						borderColor: stats.top_entities.map((_, i) => paletteSolid[i % paletteSolid.length]),
						borderWidth: 1,
						borderRadius: 4,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					indexAxis: 'y',
					plugins: { legend: { display: false } },
					scales: {
						x: { beginAtZero: true, grid: { color: 'rgba(61, 46, 80, 0.5)' } },
						y: { grid: { display: false } }
					}
				}
			}));
		}

		// 6. Quest Status (pie)
		if (questChartEl && stats.total_quests > 0) {
			const questData = [
				{ label: 'Active', value: stats.active_quests, color: palette[4] },
				{ label: 'Completed', value: stats.completed_quests, color: palette[3] },
				{ label: 'Failed', value: stats.failed_quests, color: palette[6] },
			].filter(d => d.value > 0);

			charts.push(new Chart(questChartEl, {
				type: 'pie',
				data: {
					labels: questData.map(d => d.label),
					datasets: [{
						data: questData.map(d => d.value),
						backgroundColor: questData.map(d => d.color),
						borderColor: '#1a1020',
						borderWidth: 2,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					plugins: {
						legend: { position: 'right', labels: { padding: 12, font: { size: 12 } } }
					}
				}
			}));
		}

		// 7. Combat Damage Leaderboard
		if (combatChartEl && stats.combat_actor_stats.length > 0) {
			charts.push(new Chart(combatChartEl, {
				type: 'bar',
				data: {
					labels: stats.combat_actor_stats.map(a => a.actor),
					datasets: [{
						label: 'Total Damage',
						data: stats.combat_actor_stats.map(a => a.total_damage),
						backgroundColor: stats.combat_actor_stats.map((_, i) => palette[i % palette.length]),
						borderColor: stats.combat_actor_stats.map((_, i) => paletteSolid[i % paletteSolid.length]),
						borderWidth: 1,
						borderRadius: 4,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					indexAxis: 'y',
					plugins: { legend: { display: false } },
					scales: {
						x: { beginAtZero: true, grid: { color: 'rgba(61, 46, 80, 0.5)' } },
						y: { grid: { display: false } }
					}
				}
			}));
		}

		// 8. NPC Status (pie)
		if (npcStatusChartEl && Object.keys(stats.npc_status_counts).length > 0) {
			const statusColors: Record<string, string> = {
				alive: palette[3],
				dead: palette[6],
				unknown: palette[5],
			};
			const statuses = Object.keys(stats.npc_status_counts);
			charts.push(new Chart(npcStatusChartEl, {
				type: 'pie',
				data: {
					labels: statuses.map(s => s.charAt(0).toUpperCase() + s.slice(1)),
					datasets: [{
						data: statuses.map(s => stats!.npc_status_counts[s]),
						backgroundColor: statuses.map(s => statusColors[s] ?? palette[7]),
						borderColor: '#1a1020',
						borderWidth: 2,
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					plugins: {
						legend: { position: 'right', labels: { padding: 12, font: { size: 12 } } }
					}
				}
			}));
		}
	}

	function destroyCharts() {
		charts.forEach(c => c.destroy());
		charts = [];
	}

	function formatDuration(mins: number): string {
		const h = Math.floor(mins / 60);
		const m = Math.round(mins % 60);
		if (h === 0) return `${m}m`;
		return `${h}h ${m}m`;
	}

	function formatNumber(n: number): string {
		return n.toLocaleString();
	}

	onMount(async () => {
		try {
			stats = await fetchCampaignStats(campaignId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load stats';
		} finally {
			loading = false;
		}
	});

	// Create charts after stats are loaded and DOM is updated
	$effect(() => {
		if (stats && !loading) {
			// Use a microtask to ensure canvas elements are rendered
			queueMicrotask(() => createCharts());
		}
	});

	onDestroy(() => {
		destroyCharts();
	});
</script>

<svelte:head>
	<title>Stats - RPG Summariser</title>
</svelte:head>

<div class="stats-page">
	{#if loading}
		<div class="loading-state">
			<span class="spinner"></span>
			<span>Crunching numbers...</span>
		</div>
	{:else if error}
		<div class="error-box">{error}</div>
	{:else if stats}
		<!-- Summary cards row -->
		<div class="summary-row">
			<div class="summary-card">
				<div class="summary-value">{stats.total_sessions}</div>
				<div class="summary-label">Sessions</div>
			</div>
			<div class="summary-card">
				<div class="summary-value">{formatDuration(stats.total_duration_min)}</div>
				<div class="summary-label">Total Playtime</div>
			</div>
			<div class="summary-card">
				<div class="summary-value">{formatDuration(stats.avg_duration_min)}</div>
				<div class="summary-label">Avg Session</div>
			</div>
			<div class="summary-card">
				<div class="summary-value">{formatNumber(stats.total_words)}</div>
				<div class="summary-label">Words Spoken</div>
			</div>
			<div class="summary-card">
				<div class="summary-value">{formatNumber(stats.total_segments)}</div>
				<div class="summary-label">Transcript Lines</div>
			</div>
			<div class="summary-card">
				<div class="summary-value">{stats.total_quests}</div>
				<div class="summary-label">Quests</div>
			</div>
			<div class="summary-card">
				<div class="summary-value">{stats.total_encounters}</div>
				<div class="summary-label">Battles</div>
			</div>
			<div class="summary-card">
				<div class="summary-value">{formatNumber(stats.total_damage)}</div>
				<div class="summary-label">Total Damage</div>
			</div>
		</div>

		<!-- Charts grid -->
		<div class="charts-grid">
			{#if stats.session_timeline.length > 0}
				<div class="chart-card">
					<h3 class="chart-title">Session Duration Trend</h3>
					<div class="chart-container">
						<canvas bind:this={durationChartEl}></canvas>
					</div>
				</div>

				<div class="chart-card">
					<h3 class="chart-title">Words Per Session</h3>
					<div class="chart-container">
						<canvas bind:this={wordsChartEl}></canvas>
					</div>
				</div>
			{/if}

			{#if stats.speaker_stats.length > 0}
				<div class="chart-card">
					<h3 class="chart-title">Speaking by Character</h3>
					<div class="chart-container chart-tall">
						<canvas bind:this={speakerChartEl}></canvas>
					</div>
				</div>
			{/if}

			{#if Object.keys(stats.entity_counts).length > 0}
				<div class="chart-card">
					<h3 class="chart-title">Entity Type Breakdown</h3>
					<div class="chart-container">
						<canvas bind:this={entityTypeChartEl}></canvas>
					</div>
				</div>
			{/if}

			{#if stats.top_entities.length > 0}
				<div class="chart-card">
					<h3 class="chart-title">Most Mentioned Entities</h3>
					<div class="chart-container chart-tall">
						<canvas bind:this={topEntitiesChartEl}></canvas>
					</div>
				</div>
			{/if}

			{#if stats.total_quests > 0}
				<div class="chart-card">
					<h3 class="chart-title">Quest Status</h3>
					<div class="chart-container">
						<canvas bind:this={questChartEl}></canvas>
					</div>
				</div>
			{/if}

			{#if stats.combat_actor_stats.length > 0}
				<div class="chart-card">
					<h3 class="chart-title">Combat Damage Leaderboard</h3>
					<div class="chart-container chart-tall">
						<canvas bind:this={combatChartEl}></canvas>
					</div>
				</div>
			{/if}

			{#if Object.keys(stats.npc_status_counts).length > 0}
				<div class="chart-card">
					<h3 class="chart-title">NPC Status</h3>
					<div class="chart-container">
						<canvas bind:this={npcStatusChartEl}></canvas>
					</div>
				</div>
			{/if}
		</div>

		{#if stats.session_timeline.length === 0 && stats.speaker_stats.length === 0 && stats.total_quests === 0 && stats.total_encounters === 0 && Object.keys(stats.entity_counts).length === 0}
			<div class="empty-state">
				<p>No data to visualize yet.</p>
				<p class="muted">Complete some sessions to see campaign statistics and charts here.</p>
			</div>
		{/if}
	{/if}
</div>

<style>
	.stats-page {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.loading-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
		padding: 3rem;
		color: var(--text-muted);
	}
	.spinner {
		width: 18px;
		height: 18px;
		border: 2px solid var(--border);
		border-top-color: var(--accent-gold);
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }

	.error-box {
		background: rgba(185, 28, 28, 0.15);
		border: 1px solid #7f1d1d;
		color: #fca5a5;
		padding: 0.75rem;
		border-radius: var(--radius);
		font-size: 0.9rem;
	}

	/* Summary cards */
	.summary-row {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
		gap: 0.75rem;
	}
	.summary-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1rem;
		text-align: center;
		transition: border-color 0.15s;
	}
	.summary-card:hover {
		border-color: var(--accent-gold-dim);
	}
	.summary-value {
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--accent-gold);
		line-height: 1.2;
	}
	.summary-label {
		font-size: 0.75rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		margin-top: 0.25rem;
	}

	/* Charts grid */
	.charts-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));
		gap: 1rem;
	}
	.chart-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 1.25rem;
		transition: border-color 0.15s;
	}
	.chart-card:hover {
		border-color: var(--accent-gold-dim);
	}
	.chart-title {
		font-size: 0.9rem;
		color: var(--text-secondary);
		font-weight: 600;
		margin-bottom: 0.75rem;
	}
	.chart-container {
		position: relative;
		height: 260px;
	}
	.chart-container.chart-tall {
		height: 320px;
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
		font-size: 0.9rem;
		margin-top: 0.5rem;
	}

	@media (max-width: 900px) {
		.charts-grid {
			grid-template-columns: 1fr;
		}
	}
	@media (max-width: 600px) {
		.summary-row {
			grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
		}
		.summary-value {
			font-size: 1.2rem;
		}
	}
</style>
