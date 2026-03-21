import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
	fetchSessions,
	fetchSession,
	fetchTranscript,
	fetchCampaigns,
	fetchQuests,
	fetchTimeline,
	askLore,
	regenerateRecap,
	fetchCharacters,
	upsertCharacter,
	deleteCharacter,
	fetchMembers,
	fetchStatus,
	fetchEntities,
	fetchEntity,
	fetchQuest,
	searchLore,
	fetchRecap,
	createCampaign,
	setActiveCampaign,
	searchTranscripts,
	fetchSessionCombat,
	mergeEntity,
	fetchRelationshipGraph,
	reprocessSession,
	fetchCampaignStats,
} from './api';

function mockFetch(body: unknown, status = 200, statusText = 'OK') {
	return vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		statusText,
		json: () => Promise.resolve(body),
	});
}

beforeEach(() => {
	vi.restoreAllMocks();
});

describe('fetchSessions', () => {
	it('returns typed array with default params', async () => {
		const sessions = [{ id: 1, guild_id: 'g1', status: 'complete' }];
		globalThis.fetch = mockFetch(sessions);

		const result = await fetchSessions();

		expect(result).toEqual(sessions);
		expect(fetch).toHaveBeenCalledWith('/api/sessions?limit=20&offset=0', undefined);
	});

	it('passes custom limit, offset, and campaignId', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchSessions(10, 5, 42);

		expect(fetch).toHaveBeenCalledWith(
			'/api/sessions?limit=10&offset=5&campaign_id=42',
			undefined,
		);
	});
});

describe('fetchSession', () => {
	it('fetches a single session by id', async () => {
		const session = { id: 7, status: 'complete' };
		globalThis.fetch = mockFetch(session);

		const result = await fetchSession(7);

		expect(result).toEqual(session);
		expect(fetch).toHaveBeenCalledWith('/api/sessions/7', undefined);
	});
});

describe('fetchTranscript', () => {
	it('fetches transcript segments for a session', async () => {
		const segments = [{ id: 1, session_id: 3, text: 'hello' }];
		globalThis.fetch = mockFetch(segments);

		const result = await fetchTranscript(3);

		expect(result).toEqual(segments);
		expect(fetch).toHaveBeenCalledWith('/api/sessions/3/transcript', undefined);
	});
});

describe('fetchCampaigns', () => {
	it('returns typed array of campaigns', async () => {
		const campaigns = [{ id: 1, name: 'Lost Mines' }];
		globalThis.fetch = mockFetch(campaigns);

		const result = await fetchCampaigns();

		expect(result).toEqual(campaigns);
		expect(fetch).toHaveBeenCalledWith('/api/campaigns', undefined);
	});
});

describe('fetchQuests', () => {
	it('builds URL without status filter', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchQuests(5);

		expect(fetch).toHaveBeenCalledWith('/api/campaigns/5/quests', undefined);
	});

	it('builds URL with status filter', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchQuests(5, 'active');

		expect(fetch).toHaveBeenCalledWith('/api/campaigns/5/quests?status=active', undefined);
	});
});

describe('fetchTimeline', () => {
	it('builds URL without limit/offset', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchTimeline(3);

		expect(fetch).toHaveBeenCalledWith('/api/campaigns/3/timeline', undefined);
	});

	it('builds URL with limit and offset', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchTimeline(3, 10, 20);

		expect(fetch).toHaveBeenCalledWith(
			'/api/campaigns/3/timeline?limit=10&offset=20',
			undefined,
		);
	});

	it('builds URL with only limit', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchTimeline(3, 50);

		expect(fetch).toHaveBeenCalledWith('/api/campaigns/3/timeline?limit=50', undefined);
	});
});

describe('askLore', () => {
	it('sends POST with correct body', async () => {
		const answer = { answer: 'The dragon lives in the mountain.', sources: ['session 3'] };
		globalThis.fetch = mockFetch(answer);

		const result = await askLore(2, 'Where does the dragon live?');

		expect(result).toEqual(answer);
		expect(fetch).toHaveBeenCalledWith('/api/campaigns/2/lore/ask', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ question: 'Where does the dragon live?' }),
		});
	});
});

describe('regenerateRecap', () => {
	it('sends POST and returns recap', async () => {
		const recap = { recap: 'A new recap.', recap_generated_at: '2025-01-01T00:00:00Z' };
		globalThis.fetch = mockFetch(recap);

		const result = await regenerateRecap(4);

		expect(result).toEqual(recap);
		expect(fetch).toHaveBeenCalledWith('/api/campaigns/4/recap', { method: 'POST' });
	});
});

describe('error handling', () => {
	it('throws ApiError on non-200 response with error body', async () => {
		globalThis.fetch = vi.fn().mockResolvedValue({
			ok: false,
			status: 404,
			statusText: 'Not Found',
			json: () => Promise.resolve({ error: 'session not found' }),
		});

		await expect(fetchSession(999)).rejects.toThrow('session not found');
	});

	it('throws ApiError with statusText when body parse fails', async () => {
		globalThis.fetch = vi.fn().mockResolvedValue({
			ok: false,
			status: 500,
			statusText: 'Internal Server Error',
			json: () => Promise.reject(new Error('invalid json')),
		});

		await expect(fetchSessions()).rejects.toThrow('Internal Server Error');
	});

	it('includes status code on ApiError', async () => {
		globalThis.fetch = vi.fn().mockResolvedValue({
			ok: false,
			status: 403,
			statusText: 'Forbidden',
			json: () => Promise.resolve({}),
		});

		try {
			await fetchSessions();
			expect.unreachable('should have thrown');
		} catch (err: unknown) {
			expect((err as { status: number }).status).toBe(403);
		}
	});
});

describe('204 responses', () => {
	it('returns undefined for 204 status', async () => {
		globalThis.fetch = vi.fn().mockResolvedValue({
			ok: true,
			status: 204,
			statusText: 'No Content',
			json: () => Promise.reject(new Error('no body')),
		});

		const result = await deleteCharacter('user123');

		expect(result).toBeUndefined();
	});
});

describe('fetchCharacters', () => {
	it('fetches character mappings', async () => {
		const chars = [{ user_id: 'u1', guild_id: 'g1', character_name: 'Aragorn' }];
		globalThis.fetch = mockFetch(chars);

		const result = await fetchCharacters();

		expect(result).toEqual(chars);
		expect(fetch).toHaveBeenCalledWith('/api/characters', undefined);
	});
});

describe('upsertCharacter', () => {
	it('sends PUT with correct body', async () => {
		globalThis.fetch = mockFetch(undefined, 204);

		await upsertCharacter('u1', 'g1', 'Gandalf');

		expect(fetch).toHaveBeenCalledWith('/api/characters', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ user_id: 'u1', guild_id: 'g1', character_name: 'Gandalf' }),
		});
	});
});

describe('fetchMembers', () => {
	it('fetches guild members', async () => {
		const members = [{ user_id: 'u1', username: 'bob', display_name: 'Bob' }];
		globalThis.fetch = mockFetch(members);

		const result = await fetchMembers();

		expect(result).toEqual(members);
		expect(fetch).toHaveBeenCalledWith('/api/members', undefined);
	});
});

describe('fetchStatus', () => {
	it('fetches bot status', async () => {
		const status = { recording: false, active_session: null };
		globalThis.fetch = mockFetch(status);

		const result = await fetchStatus();

		expect(result).toEqual(status);
		expect(fetch).toHaveBeenCalledWith('/api/status', undefined);
	});
});

describe('fetchEntities', () => {
	it('fetches entities without filters', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchEntities(1);

		expect(fetch).toHaveBeenCalledWith('/api/campaigns/1/entities', undefined);
	});

	it('builds URL with type and search filters', async () => {
		globalThis.fetch = mockFetch([]);

		await fetchEntities(1, { type: 'npc', search: 'dragon' });

		expect(fetch).toHaveBeenCalledWith(
			'/api/campaigns/1/entities?type=npc&search=dragon',
			undefined,
		);
	});
});

describe('fetchEntity', () => {
	it('fetches entity detail by id', async () => {
		const entity = { id: 5, name: 'Strahd', type: 'npc' };
		globalThis.fetch = mockFetch(entity);

		const result = await fetchEntity(5);

		expect(result).toEqual(entity);
		expect(fetch).toHaveBeenCalledWith('/api/entities/5', undefined);
	});
});

describe('fetchQuest', () => {
	it('fetches quest detail by id', async () => {
		const quest = { id: 3, name: 'Slay the Dragon', status: 'active' };
		globalThis.fetch = mockFetch(quest);

		const result = await fetchQuest(3);

		expect(result).toEqual(quest);
		expect(fetch).toHaveBeenCalledWith('/api/quests/3', undefined);
	});
});

describe('searchLore', () => {
	it('builds URL with encoded query', async () => {
		globalThis.fetch = mockFetch([]);

		await searchLore(2, 'ancient ruins');

		expect(fetch).toHaveBeenCalledWith(
			'/api/campaigns/2/lore/search?q=ancient%20ruins',
			undefined,
		);
	});
});

describe('fetchRecap', () => {
	it('fetches campaign recap', async () => {
		const recap = { recap: 'The story so far...', recap_generated_at: null };
		globalThis.fetch = mockFetch(recap);

		const result = await fetchRecap(1);

		expect(result).toEqual(recap);
		expect(fetch).toHaveBeenCalledWith('/api/campaigns/1/recap', undefined);
	});
});

describe('createCampaign', () => {
	it('sends POST with name and description', async () => {
		const campaign = { id: 1, name: 'New Campaign', description: 'A fresh start' };
		globalThis.fetch = mockFetch(campaign);

		const result = await createCampaign('New Campaign', 'A fresh start');

		expect(result).toEqual(campaign);
		expect(fetch).toHaveBeenCalledWith('/api/campaigns', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name: 'New Campaign', description: 'A fresh start' }),
		});
	});
});

describe('setActiveCampaign', () => {
	it('sends PUT to activate campaign', async () => {
		globalThis.fetch = mockFetch(undefined, 204);

		await setActiveCampaign(3);

		expect(fetch).toHaveBeenCalledWith('/api/campaigns/3/active', { method: 'PUT' });
	});
});

describe('searchTranscripts', () => {
	it('builds URL with default limit and offset', async () => {
		const response = { results: [], total: 0, limit: 20, offset: 0 };
		globalThis.fetch = mockFetch(response);

		const result = await searchTranscripts(5, 'dragon');

		expect(result).toEqual(response);
		expect(fetch).toHaveBeenCalledWith(
			'/api/campaigns/5/transcript-search?q=dragon&limit=20&offset=0',
			undefined,
		);
	});

	it('builds URL with custom limit and offset', async () => {
		const response = { results: [], total: 50, limit: 10, offset: 30 };
		globalThis.fetch = mockFetch(response);

		const result = await searchTranscripts(3, 'tavern', 10, 30);

		expect(result).toEqual(response);
		expect(fetch).toHaveBeenCalledWith(
			'/api/campaigns/3/transcript-search?q=tavern&limit=10&offset=30',
			undefined,
		);
	});

	it('encodes special characters in the query', async () => {
		globalThis.fetch = mockFetch({ results: [], total: 0, limit: 20, offset: 0 });

		await searchTranscripts(1, 'fire & ice');

		expect(fetch).toHaveBeenCalledWith(
			'/api/campaigns/1/transcript-search?q=fire+%26+ice&limit=20&offset=0',
			undefined,
		);
	});
});

describe('fetchSessionCombat', () => {
	it('fetches combat encounters for a session', async () => {
		const encounters = [
			{ id: 1, session_id: 5, name: 'Goblin Ambush', actions: [] },
		];
		globalThis.fetch = mockFetch(encounters);

		const result = await fetchSessionCombat(5);

		expect(result).toEqual(encounters);
		expect(fetch).toHaveBeenCalledWith('/api/sessions/5/combat', undefined);
	});
});

describe('mergeEntity', () => {
	it('sends POST with merge_id in body', async () => {
		globalThis.fetch = mockFetch(undefined, 204);

		await mergeEntity(10, 20);

		expect(fetch).toHaveBeenCalledWith('/api/entities/10/merge', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ merge_id: 20 }),
		});
	});
});

describe('fetchRelationshipGraph', () => {
	it('fetches relationship graph for a campaign', async () => {
		const graph = {
			nodes: [{ id: 1, name: 'Strahd', type: 'npc' }],
			edges: [{ source: 1, target: 2, relationship: 'enemy', description: 'Mortal enemies' }],
		};
		globalThis.fetch = mockFetch(graph);

		const result = await fetchRelationshipGraph(7);

		expect(result).toEqual(graph);
		expect(fetch).toHaveBeenCalledWith('/api/campaigns/7/relationship-graph', undefined);
	});
});

describe('reprocessSession', () => {
	it('sends POST with retranscribe false by default', async () => {
		globalThis.fetch = mockFetch(undefined, 204);

		await reprocessSession(42);

		expect(fetch).toHaveBeenCalledWith('/api/sessions/42/reprocess', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ retranscribe: false }),
		});
	});

	it('sends POST with retranscribe true when specified', async () => {
		globalThis.fetch = mockFetch(undefined, 204);

		await reprocessSession(42, true);

		expect(fetch).toHaveBeenCalledWith('/api/sessions/42/reprocess', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ retranscribe: true }),
		});
	});
});

describe('fetchCampaignStats', () => {
	it('fetches campaign stats by campaign id', async () => {
		const stats = {
			total_sessions: 5,
			total_duration_min: 300,
			avg_duration_min: 60,
			total_segments: 100,
			total_words: 5000,
			speaker_stats: [],
			entity_counts: { npc: 3, place: 2 },
			top_entities: [],
			total_quests: 2,
			active_quests: 1,
			completed_quests: 1,
			failed_quests: 0,
			total_encounters: 3,
			total_actions: 15,
			total_damage: 120,
			combat_actor_stats: [],
			session_timeline: [],
			npc_status_counts: { alive: 2, dead: 1 },
		};
		globalThis.fetch = mockFetch(stats);

		const result = await fetchCampaignStats(7);

		expect(result).toEqual(stats);
		expect(fetch).toHaveBeenCalledWith('/api/campaigns/7/stats', undefined);
	});
});
