<script lang="ts">
	import type { TranscriptSegment } from '$lib/api';

	let { segment }: { segment: TranscriptSegment } = $props();

	function hashColor(name: string): string {
		let hash = 0;
		for (let i = 0; i < name.length; i++) {
			hash = name.charCodeAt(i) + ((hash << 5) - hash);
		}
		const hue = ((hash % 360) + 360) % 360;
		return `hsl(${hue}, 60%, 70%)`;
	}

	function formatTime(seconds: number): string {
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = Math.floor(seconds % 60);
		if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	const displayName = $derived(segment.character_name ?? segment.display_name ?? segment.user_id);
	const nameColor = $derived(hashColor(displayName));
</script>

<div class="line" id="seg-{Math.floor(segment.start_time)}">
	<span class="timestamp">[{formatTime(segment.start_time)}]</span>
	<span class="name" style:color={nameColor}>{displayName}:</span>
	<span class="text">{segment.text}</span>
</div>

<style>
	.line {
		padding: 0.25rem 0;
		line-height: 1.5;
		font-size: 0.9rem;
	}
	.line:hover {
		background: var(--surface-hover);
	}
	:global(.line.highlighted) {
		background: rgba(234, 179, 8, 0.15);
		border-left: 3px solid var(--accent-gold);
		padding-left: 0.5rem;
	}
	.timestamp {
		color: var(--text-muted);
		font-family: 'Courier New', Courier, monospace;
		font-size: 0.8rem;
		margin-right: 0.5rem;
		user-select: none;
	}
	.name {
		font-weight: 600;
		margin-right: 0.4rem;
	}
	.text {
		color: var(--text-primary);
	}
</style>
