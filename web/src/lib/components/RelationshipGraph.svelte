<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		forceSimulation,
		forceLink,
		forceManyBody,
		forceCenter,
		forceCollide,
		type SimulationNodeDatum,
		type SimulationLinkDatum
	} from 'd3-force';
	import { select } from 'd3-selection';
	import { zoom, zoomIdentity, type ZoomBehavior } from 'd3-zoom';
	import { drag } from 'd3-drag';
	import type { GraphNode, GraphEdge } from '$lib/api';

	let { nodes, edges, campaignId }: { nodes: GraphNode[]; edges: GraphEdge[]; campaignId: number } =
		$props();

	interface SimNode extends SimulationNodeDatum {
		id: number;
		name: string;
		type: string;
	}

	interface SimLink extends SimulationLinkDatum<SimNode> {
		relationship: string;
		description: string;
	}

	const nodeColors: Record<string, string> = {
		npc: '#a78bfa',
		place: '#4ade80',
		organisation: '#60a5fa',
		item: '#facc15',
		event: '#f87171',
		pc: '#f472b6'
	};

	const nodeColorsBg: Record<string, string> = {
		npc: 'rgba(139, 92, 246, 0.15)',
		place: 'rgba(34, 197, 94, 0.15)',
		organisation: 'rgba(59, 130, 246, 0.15)',
		item: 'rgba(234, 179, 8, 0.15)',
		event: 'rgba(239, 68, 68, 0.15)',
		pc: 'rgba(236, 72, 153, 0.15)'
	};

	let svgEl = $state<SVGSVGElement>(undefined!);
	let simulation: ReturnType<typeof forceSimulation<SimNode>> | null = null;
	let width = 0;
	let height = 0;

	function getNodeColor(type: string): string {
		return nodeColors[type] ?? '#d4d4d4';
	}

	function getNodeRadius(type: string): number {
		if (type === 'pc') return 28;
		if (type === 'npc') return 24;
		return 20;
	}

	function buildGraph() {
		if (!svgEl) return;

		// Clean up previous simulation
		if (simulation) {
			simulation.stop();
			simulation = null;
		}

		const svgSel = select(svgEl);
		svgSel.selectAll('*').remove();

		const rect = svgEl.getBoundingClientRect();
		width = rect.width;
		height = rect.height;

		if (width === 0 || height === 0) return;

		// Build simulation data
		const simNodes: SimNode[] = nodes.map((n) => ({
			id: n.id,
			name: n.name,
			type: n.type,
			x: width / 2 + (Math.random() - 0.5) * 200,
			y: height / 2 + (Math.random() - 0.5) * 200
		}));

		const nodeById = new Map(simNodes.map((n) => [n.id, n]));

		const simLinks: SimLink[] = edges
			.filter((e) => nodeById.has(e.source) && nodeById.has(e.target))
			.map((e) => ({
				source: nodeById.get(e.source)!,
				target: nodeById.get(e.target)!,
				relationship: e.relationship,
				description: e.description
			}));

		// Container group for zoom/pan
		const g = svgSel.append('g');

		// Zoom behavior
		const zoomBehavior: ZoomBehavior<SVGSVGElement, unknown> = zoom<SVGSVGElement, unknown>()
			.scaleExtent([0.1, 4])
			.on('zoom', (event) => {
				g.attr('transform', event.transform);
			});

		svgSel.call(zoomBehavior);

		// Fit initial view
		const initialScale = Math.min(width, height) < 600 ? 0.6 : 0.8;
		svgSel.call(
			zoomBehavior.transform,
			zoomIdentity.translate(width * (1 - initialScale) / 2, height * (1 - initialScale) / 2).scale(initialScale)
		);

		// Arrow marker definitions
		const defs = svgSel.append('defs');
		const markerTypes = [...new Set(edges.map((e) => e.relationship))];
		markerTypes.forEach((type) => {
			defs
				.append('marker')
				.attr('id', `arrow-${type}`)
				.attr('viewBox', '0 -5 10 10')
				.attr('refX', 30)
				.attr('refY', 0)
				.attr('markerWidth', 6)
				.attr('markerHeight', 6)
				.attr('orient', 'auto')
				.append('path')
				.attr('d', 'M0,-5L10,0L0,5')
				.attr('fill', '#7a6b8a');
		});

		// Links
		const link = g
			.append('g')
			.attr('class', 'links')
			.selectAll('line')
			.data(simLinks)
			.join('line')
			.attr('stroke', '#3d2e50')
			.attr('stroke-width', 1.5)
			.attr('stroke-opacity', 0.7)
			.attr('marker-end', (d) => `url(#arrow-${d.relationship})`);

		// Link labels
		const linkLabel = g
			.append('g')
			.attr('class', 'link-labels')
			.selectAll('text')
			.data(simLinks)
			.join('text')
			.attr('text-anchor', 'middle')
			.attr('fill', '#7a6b8a')
			.attr('font-size', '9px')
			.attr('font-family', 'system-ui, sans-serif')
			.attr('pointer-events', 'none')
			.text((d) => d.relationship.replace(/_/g, ' '));

		// Node groups
		const nodeGroup = g
			.append('g')
			.attr('class', 'nodes')
			.selectAll<SVGGElement, SimNode>('g')
			.data(simNodes)
			.join('g')
			.attr('cursor', 'pointer')
			.on('click', (_event: MouseEvent, d: SimNode) => {
				goto(`/campaigns/${campaignId}/lore/${d.id}`);
			});

		// Drag behavior
		const dragBehavior = drag<SVGGElement, SimNode>()
			.on('start', (event, d) => {
				if (!event.active) simulation?.alphaTarget(0.3).restart();
				d.fx = d.x;
				d.fy = d.y;
			})
			.on('drag', (event, d) => {
				d.fx = event.x;
				d.fy = event.y;
			})
			.on('end', (event, d) => {
				if (!event.active) simulation?.alphaTarget(0);
				d.fx = null;
				d.fy = null;
			});

		nodeGroup.call(dragBehavior);

		// Node circles
		nodeGroup
			.append('circle')
			.attr('r', (d) => getNodeRadius(d.type))
			.attr('fill', (d) => nodeColorsBg[d.type] ?? 'rgba(100, 100, 100, 0.15)')
			.attr('stroke', (d) => getNodeColor(d.type))
			.attr('stroke-width', 2);

		// Node labels
		nodeGroup
			.append('text')
			.attr('text-anchor', 'middle')
			.attr('dy', (d) => getNodeRadius(d.type) + 14)
			.attr('fill', '#e8e0f0')
			.attr('font-size', '11px')
			.attr('font-family', 'system-ui, sans-serif')
			.attr('pointer-events', 'none')
			.text((d) => d.name);

		// Type label inside node
		nodeGroup
			.append('text')
			.attr('text-anchor', 'middle')
			.attr('dy', '0.35em')
			.attr('fill', (d) => getNodeColor(d.type))
			.attr('font-size', '8px')
			.attr('font-weight', '600')
			.attr('font-family', 'system-ui, sans-serif')
			.attr('pointer-events', 'none')
			.attr('text-transform', 'uppercase')
			.text((d) => d.type.slice(0, 3).toUpperCase());

		// Tooltip on hover
		nodeGroup.append('title').text((d) => `${d.name} (${d.type})`);

		// Force simulation
		simulation = forceSimulation<SimNode>(simNodes)
			.force(
				'link',
				forceLink<SimNode, SimLink>(simLinks)
					.id((d) => d.id)
					.distance(120)
			)
			.force('charge', forceManyBody().strength(-300))
			.force('center', forceCenter(width / 2, height / 2))
			.force(
				'collide',
				forceCollide<SimNode>().radius((d) => getNodeRadius(d.type) + 20)
			)
			.on('tick', () => {
				link
					.attr('x1', (d) => (d.source as SimNode).x!)
					.attr('y1', (d) => (d.source as SimNode).y!)
					.attr('x2', (d) => (d.target as SimNode).x!)
					.attr('y2', (d) => (d.target as SimNode).y!);

				linkLabel
					.attr('x', (d) => ((d.source as SimNode).x! + (d.target as SimNode).x!) / 2)
					.attr('y', (d) => ((d.source as SimNode).y! + (d.target as SimNode).y!) / 2);

				nodeGroup.attr('transform', (d) => `translate(${d.x},${d.y})`);
			});
	}

	onMount(() => {
		buildGraph();
	});

	onDestroy(() => {
		if (simulation) {
			simulation.stop();
			simulation = null;
		}
	});

	// Rebuild graph when data changes
	$effect(() => {
		// Access reactive dependencies
		void nodes;
		void edges;
		if (svgEl) {
			buildGraph();
		}
	});
</script>

<div class="graph-container">
	{#if nodes.length === 0}
		<div class="empty-state">
			<p>No entities with relationships to display.</p>
			<p class="muted">Relationships are automatically extracted from session transcripts.</p>
		</div>
	{:else}
		<svg bind:this={svgEl} class="graph-svg"></svg>
		<div class="graph-legend">
			{#each Object.entries(nodeColors) as [type, color]}
				<span class="legend-item">
					<span class="legend-dot" style:background-color={color}></span>
					{type}
				</span>
			{/each}
		</div>
		<div class="graph-hint">
			Scroll to zoom, drag to pan, click a node to view details
		</div>
	{/if}
</div>

<style>
	.graph-container {
		position: relative;
		width: 100%;
		height: 600px;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
	}

	.graph-svg {
		width: 100%;
		height: 100%;
		display: block;
	}

	.graph-legend {
		position: absolute;
		top: 0.75rem;
		left: 0.75rem;
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		background: rgba(26, 16, 32, 0.85);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.5rem 0.75rem;
		z-index: 1;
	}

	.legend-item {
		display: flex;
		align-items: center;
		gap: 0.3rem;
		font-size: 0.7rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		font-weight: 600;
	}

	.legend-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.graph-hint {
		position: absolute;
		bottom: 0.75rem;
		right: 0.75rem;
		font-size: 0.7rem;
		color: var(--text-muted);
		background: rgba(26, 16, 32, 0.85);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.35rem 0.65rem;
		z-index: 1;
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		text-align: center;
		padding: 2rem;
		color: var(--text-secondary);
	}

	.empty-state .muted {
		color: var(--text-muted);
		font-size: 0.85rem;
	}
</style>
