import type { LayoutServerLoad } from './$types';
import { navTree } from '$lib/server/docs';

export const load: LayoutServerLoad = ({ setHeaders }) => {
	setHeaders({ 'cache-control': 'public, max-age=60, stale-while-revalidate=300' });
	return { nav: navTree() };
};
