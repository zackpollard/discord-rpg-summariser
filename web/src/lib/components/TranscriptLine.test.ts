import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import TranscriptLine from './TranscriptLine.svelte';
import type { TranscriptSegment } from '$lib/api';

function makeSegment(overrides: Partial<TranscriptSegment> = {}): TranscriptSegment {
	return {
		id: 1,
		session_id: 1,
		user_id: 'user123',
		display_name: 'TestUser',
		character_name: null,
		start_time: 0,
		end_time: 5,
		text: 'Hello world',
		created_at: '2025-01-01T00:00:00Z',
		...overrides,
	};
}

describe('TranscriptLine', () => {
	it('displays character_name when set', () => {
		const segment = makeSegment({ character_name: 'Gandalf', display_name: 'Ian' });
		render(TranscriptLine, { props: { segment } });

		expect(screen.getByText('Gandalf:')).toBeTruthy();
	});

	it('falls back to display_name when character_name is null', () => {
		const segment = makeSegment({ character_name: null, display_name: 'BobThePlayer' });
		render(TranscriptLine, { props: { segment } });

		expect(screen.getByText('BobThePlayer:')).toBeTruthy();
	});

	it('falls back to user_id when both character_name and display_name are null', () => {
		const segment = makeSegment({
			character_name: null,
			display_name: null as unknown as string,
			user_id: 'uid_42',
		});
		render(TranscriptLine, { props: { segment } });

		expect(screen.getByText('uid_42:')).toBeTruthy();
	});

	it('displays the transcript text', () => {
		const segment = makeSegment({ text: 'I cast fireball!' });
		render(TranscriptLine, { props: { segment } });

		expect(screen.getByText('I cast fireball!')).toBeTruthy();
	});

	it('formats timestamp for seconds < 1 hour as m:ss', () => {
		const segment = makeSegment({ start_time: 125 }); // 2:05
		render(TranscriptLine, { props: { segment } });

		expect(screen.getByText('[2:05]')).toBeTruthy();
	});

	it('formats timestamp for seconds >= 1 hour as h:mm:ss', () => {
		const segment = makeSegment({ start_time: 3661 }); // 1:01:01
		render(TranscriptLine, { props: { segment } });

		expect(screen.getByText('[1:01:01]')).toBeTruthy();
	});

	it('formats zero timestamp as 0:00', () => {
		const segment = makeSegment({ start_time: 0 });
		render(TranscriptLine, { props: { segment } });

		expect(screen.getByText('[0:00]')).toBeTruthy();
	});

	it('applies color based on name hash', () => {
		const segment = makeSegment({ character_name: 'Aragorn' });
		render(TranscriptLine, { props: { segment } });

		const nameEl = screen.getByText('Aragorn:');
		// jsdom converts HSL inline styles to RGB, so check for a valid rgb() value
		expect(nameEl.style.color).toMatch(/^rgb\(\d+, \d+, \d+\)$/);
		// Verify a non-trivial color was applied (not black/white default)
		expect(nameEl.style.color).not.toBe('');
	});

	it('produces deterministic color for the same name', () => {
		const segment1 = makeSegment({ character_name: 'Legolas' });
		const segment2 = makeSegment({ character_name: 'Legolas', id: 2 });

		const { unmount: u1 } = render(TranscriptLine, { props: { segment: segment1 } });
		const el1 = screen.getByText('Legolas:');
		const color1 = el1.style.color;
		u1();

		render(TranscriptLine, { props: { segment: segment2 } });
		const el2 = screen.getByText('Legolas:');
		const color2 = el2.style.color;

		expect(color1).toBe(color2);
	});

	it('produces different colors for different names', () => {
		const seg1 = makeSegment({ character_name: 'Frodo' });
		const seg2 = makeSegment({ character_name: 'Sauron', id: 2 });

		const { unmount: u1 } = render(TranscriptLine, { props: { segment: seg1 } });
		const color1 = screen.getByText('Frodo:').style.color;
		u1();

		render(TranscriptLine, { props: { segment: seg2 } });
		const color2 = screen.getByText('Sauron:').style.color;

		expect(color1).not.toBe(color2);
	});
});
