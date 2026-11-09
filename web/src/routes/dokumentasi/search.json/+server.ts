import type { RequestHandler } from './$types';
import { searchIndex } from '$lib/server/docs';

export const GET: RequestHandler = () =>
	new Response(JSON.stringify(searchIndex()), {
		headers: {
			'content-type': 'application/json; charset=utf-8',
			'cache-control': 'public, max-age=300, stale-while-revalidate=3600'
		}
	});
