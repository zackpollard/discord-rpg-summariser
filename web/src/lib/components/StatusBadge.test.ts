import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import StatusBadge from './StatusBadge.svelte';

describe('StatusBadge', () => {
	const statuses = ['recording', 'transcribing', 'summarising', 'complete', 'failed'];

	it.each(statuses)('renders correct text for status "%s"', (status) => {
		render(StatusBadge, { props: { status } });

		expect(screen.getByText(status)).toBeTruthy();
	});

	it('applies correct background color for "recording"', () => {
		render(StatusBadge, { props: { status: 'recording' } });

		const badge = screen.getByText('recording');
		expect(badge.style.backgroundColor).toBe('rgb(185, 28, 28)');
		expect(badge.style.color).toBe('rgb(254, 202, 202)');
	});

	it('applies correct background color for "complete"', () => {
		render(StatusBadge, { props: { status: 'complete' } });

		const badge = screen.getByText('complete');
		expect(badge.style.backgroundColor).toBe('rgb(21, 128, 61)');
		expect(badge.style.color).toBe('rgb(187, 247, 208)');
	});

	it('applies correct background color for "transcribing"', () => {
		render(StatusBadge, { props: { status: 'transcribing' } });

		const badge = screen.getByText('transcribing');
		expect(badge.style.backgroundColor).toBe('rgb(161, 98, 7)');
		expect(badge.style.color).toBe('rgb(254, 240, 138)');
	});

	it('applies correct background color for "summarising"', () => {
		render(StatusBadge, { props: { status: 'summarising' } });

		const badge = screen.getByText('summarising');
		expect(badge.style.backgroundColor).toBe('rgb(29, 78, 216)');
		expect(badge.style.color).toBe('rgb(191, 219, 254)');
	});

	it('applies correct background color for "failed"', () => {
		render(StatusBadge, { props: { status: 'failed' } });

		const badge = screen.getByText('failed');
		expect(badge.style.backgroundColor).toBe('rgb(127, 29, 29)');
		expect(badge.style.color).toBe('rgb(252, 165, 165)');
	});

	it('applies fallback colors for unknown status', () => {
		render(StatusBadge, { props: { status: 'unknown' } });

		const badge = screen.getByText('unknown');
		expect(badge.style.backgroundColor).toBe('rgb(82, 82, 82)');
		expect(badge.style.color).toBe('rgb(212, 212, 212)');
	});

	it('has the badge CSS class', () => {
		render(StatusBadge, { props: { status: 'complete' } });

		const badge = screen.getByText('complete');
		expect(badge.classList.contains('badge')).toBe(true);
	});
});
