<script lang="ts">
	let { status }: { status: string } = $props();

	const colorMap: Record<string, { bg: string; fg: string }> = {
		recording: { bg: '#b91c1c', fg: '#fecaca' },
		transcribing: { bg: '#a16207', fg: '#fef08a' },
		summarising: { bg: '#1d4ed8', fg: '#bfdbfe' },
		complete: { bg: '#15803d', fg: '#bbf7d0' },
		failed: { bg: '#7f1d1d', fg: '#fca5a5' }
	};

	const colors = $derived(colorMap[status] ?? { bg: '#525252', fg: '#d4d4d4' });
</script>

<span class="badge" style:background-color={colors.bg} style:color={colors.fg}>
	{status}
</span>

<style>
	.badge {
		display: inline-block;
		padding: 0.15em 0.6em;
		border-radius: 9999px;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		white-space: nowrap;
	}
</style>
