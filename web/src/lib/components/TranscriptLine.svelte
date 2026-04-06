<script lang="ts">
	import type { TranscriptSegment } from '$lib/api';

	let {
		segment,
		active = false,
		onclick,
		onclip
	}: {
		segment: TranscriptSegment;
		active?: boolean;
		onclick?: () => void;
		onclip?: () => void;
	} = $props();

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

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="line"
	class:active
	class:clickable={!!onclick}
	id="seg-{Math.floor(segment.start_time)}"
	onclick={onclick}
>
	<span class="timestamp">[{formatTime(segment.start_time)}]</span>
	<span class="name" style:color={nameColor}>{displayName}:</span>
	<span class="text">{segment.text}</span>
	{#if onclip}
		<button class="clip-btn" title="Create clip from this segment" onclick={(e) => { e.stopPropagation(); onclip?.(); }}>&#9986;</button>
	{/if}
</div>

<style>
	.line {
		padding: 0.25rem 0.4rem;
		line-height: 1.5;
		font-size: 0.9rem;
		border-left: 3px solid transparent;
		border-radius: 2px;
		transition: background 0.15s, border-color 0.15s;
	}
	.line:hover {
		background: var(--surface-hover);
	}
	.line.clickable {
		cursor: pointer;
	}
	.line.active {
		background: rgba(212, 175, 125, 0.1);
		border-left-color: var(--accent-gold);
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
	.clip-btn {
		opacity: 0;
		background: none;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		font-size: 0.85rem;
		padding: 0 0.25rem;
		margin-left: 0.25rem;
		transition: opacity 0.15s, color 0.15s;
	}
	.line:hover .clip-btn {
		opacity: 1;
	}
	.clip-btn:hover {
		color: var(--accent-gold);
	}
</style>
