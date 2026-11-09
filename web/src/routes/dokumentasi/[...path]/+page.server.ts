import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { appOrigin } from '$lib/server/data-site';
import { breadcrumbs, neighbors, pageByUrl, renderPage, SECTION_LABEL } from '$lib/server/docs';

export const load: PageServerLoad = ({ params }) => {
	const meta = pageByUrl(`/dokumentasi/${params.path}`);
	if (!meta) throw error(404, 'Halaman dokumentasi tidak ditemukan.');

	const { html, lead, toc } = renderPage(meta);
	const { prev, next } = neighbors(meta);
	const pick = (m?: { title: string; url: string }) => (m ? { title: m.title, url: m.url } : null);

	return {
		doc: { title: meta.title, url: meta.url, section: meta.section, sectionLabel: SECTION_LABEL[meta.section] },
		html,
		lead,
		toc,
		crumbs: breadcrumbs(meta),
		prev: pick(prev),
		next: pick(next),
		seo: {
			title: `${meta.title} | Dokumentasi Tauthub`,
			description: meta.summary || undefined,
			canonical: `${appOrigin()}${meta.url}`,
			ogType: 'article'
		}
	};
};
