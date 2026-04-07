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

	const isTableTalk = $derived(segment.classification === 'table_talk');
	const displayText = $derived(segment.corrected_text || segment.text);

	const speakerLabel = $derived.by(() => {
		let name = segment.character_name ?? segment.display_name ?? segment.user_id;
		if (segment.npc_voice) {
			name = `${name} (as ${segment.npc_voice})`;
		}
		return name;
	});

	const nameColor = $derived(hashColor(segment.character_name ?? segment.display_name ?? segment.user_id));

	const toneColors: Record<string, string> = {
		dramatic: '#c4b5fd',
		funny: '#86efac',
		tense: '#fca5a5',
		sad: '#93c5fd',
		triumphant: '#fde047',
		mysterious: '#d8b4fe',
		angry: '#f87171',
		casual: '#a5b4fc',
		neutral: '#9ca3af',
		badass: '#fdba74',
		wholesome: '#f9a8d4',
	};
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="line"
	class:active
	class:clickable={!!onclick}
	class:table-talk={isTableTalk}
	id="seg-{Math.floor(segment.start_time)}"
	onclick={onclick}
>
	{#if segment.scene}
		<div class="scene-marker">--- {segment.scene} ---</div>
	{/if}
	<span class="timestamp">[{formatTime(segment.start_time)}]</span>
	{#if isTableTalk}
		<span class="table-talk-badge">OOC</span>
	{/if}
	<span class="name" style:color={nameColor}>{speakerLabel}:</span>
	<span class="text">{displayText}</span>
	{#if segment.corrected_text && segment.corrected_text !== segment.text}
		<span class="corrected-indicator" title="ASR corrected">*</span>
	{/if}
	{#if segment.tone && !isTableTalk}
		<span class="tone-badge" style:background={toneColors[segment.tone] ?? '#9ca3af'}>{segment.tone}</span>
	{/if}
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
	.line.table-talk {
		opacity: 0.5;
	}
	.line.table-talk:hover {
		opacity: 0.8;
	}
	:global(.line.highlighted) {
		background: rgba(234, 179, 8, 0.15);
		border-left: 3px solid var(--accent-gold);
		padding-left: 0.5rem;
	}
	.scene-marker {
		color: var(--accent-gold-dim);
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		padding: 0.4rem 0 0.15rem;
	}
	.timestamp {
		color: var(--text-muted);
		font-family: 'Courier New', Courier, monospace;
		font-size: 0.8rem;
		margin-right: 0.5rem;
		user-select: none;
	}
	.table-talk-badge {
		font-size: 0.6rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		background: rgba(255, 255, 255, 0.08);
		color: var(--text-muted);
		padding: 0.05rem 0.3rem;
		border-radius: 3px;
		margin-right: 0.35rem;
		vertical-align: middle;
	}
	.name {
		font-weight: 600;
		margin-right: 0.4rem;
	}
	.text {
		color: var(--text-primary);
	}
	.corrected-indicator {
		color: var(--accent-gold-dim);
		font-size: 0.7rem;
		margin-left: 0.15rem;
		vertical-align: super;
	}
	.tone-badge {
		display: inline-block;
		font-size: 0.55rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		color: var(--bg-dark);
		padding: 0.05rem 0.3rem;
		border-radius: 3px;
		margin-left: 0.35rem;
		vertical-align: middle;
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
