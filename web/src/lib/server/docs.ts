/**
 * Dokumentasi publik: membaca Dokumentasi/**.md saat build (import.meta.glob),
 * membangun peta navigasi, merender Markdown ke HTML, dan indeks pencarian.
 *
 * Satu berkas .md = satu halaman. README.md sebuah folder = halaman induk
 * folder itu. Awalan angka pada nama berkas/folder hanya untuk urutan dan
 * dibuang dari URL. README.md akar dan CATATAN-PENYUSUN.md tidak dipublikasikan.
 */
import { Marked, type Token } from 'marked';

const raw = import.meta.glob('../../../../Dokumentasi/**/*.md', {
	query: '?raw',
	import: 'default',
	eager: true
}) as Record<string, string>;

export type Section = 'umum' | 'klien' | 'mitra';
export const SECTION_LABEL: Record<Section, string> = {
	umum: 'Umum',
	klien: 'Klien',
	mitra: 'Mitra'
};
const SECTIONS: Section[] = ['umum', 'klien', 'mitra'];

export type DocMeta = {
	path: string;
	url: string;
	section: Section;
	title: string;
	summary: string;
	isIndex: boolean;
};

export type NavNode = { title: string; url: string; children: NavNode[] };
export type NavSection = { key: Section; label: string; url: string; children: NavNode[] };
export type TocItem = { id: string; text: string; depth: number };

const BASE = '/dokumentasi';

function stripNumber(segment: string): string {
	return segment.replace(/^\d+-/, '');
}

function urlOf(path: string): string {
	const parts = path.replace(/\.md$/, '').split('/');
	if (parts[parts.length - 1] === 'README') parts.pop();
	return `${BASE}/${parts.map(stripNumber).join('/')}`;
}

function sortKey(path: string): string[] {
	// README sebuah folder tampil lebih dulu daripada isinya.
	return path.split('/').map((s) => (s === 'README.md' ? '' : s));
}

function compare(a: string, b: string): number {
	const ka = sortKey(a);
	const kb = sortKey(b);
	for (let i = 0; i < Math.min(ka.length, kb.length); i++) {
		if (ka[i] !== kb[i]) return ka[i] < kb[i] ? -1 : 1;
	}
	return ka.length - kb.length;
}

function plainInline(text: string): string {
	return text
		.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
		.replace(/[*_`]/g, '')
		.trim();
}

const sources = new Map<string, string>();
for (const [key, text] of Object.entries(raw)) {
	const rel = key.split('Dokumentasi/')[1];
	if (!rel || !rel.includes('/')) continue; // README.md & CATATAN di akar
	if (!SECTIONS.includes(rel.split('/')[0] as Section)) continue;
	sources.set(rel, text);
}

const pages: DocMeta[] = [];
const byUrl = new Map<string, DocMeta>();
const byPath = new Map<string, DocMeta>();

for (const path of [...sources.keys()].sort(compare)) {
	const text = sources.get(path)!;
	const title = plainInline(/^# (.+)$/m.exec(text)?.[1] ?? path);
	const lines = text.split('\n');
	let j = lines.findIndex((l) => l.startsWith('# ')) + 1;
	while (j < lines.length && lines[j].trim() === '') j++;
	const quoted: string[] = [];
	while (j < lines.length && lines[j].startsWith('>')) quoted.push(lines[j++].replace(/^>\s?/, ''));
	const summary = plainInline(quoted.join(' '));
	const meta: DocMeta = {
		path,
		url: urlOf(path),
		section: path.split('/')[0] as Section,
		title,
		summary,
		isIndex: path.endsWith('README.md')
	};
	pages.push(meta);
	if (!byUrl.has(meta.url)) byUrl.set(meta.url, meta);
	byPath.set(path, meta);
}

export function pageByUrl(url: string): DocMeta | undefined {
	return byUrl.get(url.replace(/\/+$/, ''));
}

/** URL halaman untuk path sumber (mis. "umum/05-glosarium.md"); beranda bila tak ada. */
export function urlForSource(path: string): string {
	return byPath.get(path)?.url ?? BASE;
}

export function allPages(): DocMeta[] {
	return pages;
}

// ---------- navigasi ----------

export function navTree(): NavSection[] {
	return SECTIONS.map((section) => {
		const index = byPath.get(`${section}/README.md`);
		const root: NavSection = {
			key: section,
			label: SECTION_LABEL[section],
			url: index?.url ?? `${BASE}/${section}`,
			children: []
		};
		const dirs = new Map<string, NavNode[]>([[section, root.children]]);
		for (const p of pages.filter((x) => x.section === section)) {
			if (p.path === `${section}/README.md`) continue;
			const segs = p.path.split('/');
			const file = segs.pop();
			const node: NavNode = { title: p.title, url: p.url, children: [] };
			if (file === 'README.md') {
				const parent = dirs.get(segs.slice(0, -1).join('/')) ?? root.children;
				parent.push(node);
				dirs.set(segs.join('/'), node.children);
			} else {
				(dirs.get(segs.join('/')) ?? root.children).push(node);
			}
		}
		return root;
	});
}

export function neighbors(meta: DocMeta): { prev?: DocMeta; next?: DocMeta } {
	const list = pages.filter((p) => p.section === meta.section);
	const i = list.findIndex((p) => p.path === meta.path);
	return { prev: i > 0 ? list[i - 1] : undefined, next: list[i + 1] };
}

export function breadcrumbs(meta: DocMeta): { title: string; url: string }[] {
	const dirs = meta.path.split('/').slice(0, -1);
	const upto = meta.isIndex ? dirs.length - 1 : dirs.length;
	const out: { title: string; url: string }[] = [];
	for (let i = 1; i <= upto; i++) {
		const readme = byPath.get(`${dirs.slice(0, i).join('/')}/README.md`);
		if (readme) out.push({ title: readme.title, url: readme.url });
	}
	return out;
}

// ---------- render ----------

/** Menyelesaikan tautan relatif terhadap path sumber (tanpa dependensi node). */
function resolveRel(from: string, rel: string): string {
	const out = from.split('/').slice(0, -1);
	for (const part of rel.split('/')) {
		if (part === '..') out.pop();
		else if (part !== '.' && part !== '') out.push(part);
	}
	return out.join('/');
}

function esc(s: string): string {
	return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function stripTags(html: string): string {
	return html
		.replace(/<[^>]+>/g, '')
		.replace(/&amp;/g, '&')
		.replace(/&lt;/g, '<')
		.replace(/&gt;/g, '>')
		.replace(/&quot;/g, '"')
		.replace(/&#39;/g, "'");
}

function slugify(text: string): string {
	const s = text
		.toLowerCase()
		.normalize('NFKD')
		.replace(/[^a-z0-9\s-]/g, '')
		.trim()
		.replace(/\s+/g, '-');
	return s || 'bagian';
}

// Keadaan per-render (render sinkron, jadi aman memakai variabel modul).
let curPath = '';
let curToc: TocItem[] = [];
let curIds = new Map<string, number>();

const CALLOUT = /^<p><strong class="dk-lead">(Catatan|Penting|Tips):<\/strong>/;

const md = new Marked({
	gfm: true,
	renderer: {
		heading({ tokens, depth }) {
			const html = this.parser.parseInline(tokens);
			const plain = stripTags(html);
			const base = slugify(plain);
			const n = curIds.get(base) ?? 0;
			curIds.set(base, n + 1);
			const id = n === 0 ? base : `${base}-${n + 1}`;
			if (depth === 2 || depth === 3) curToc.push({ id, text: plain, depth });
			return `<h${depth} id="${id}"><a class="dk-anchor" href="#${id}" aria-label="Tautan ke bagian ini">#</a>${html}</h${depth}>\n`;
		},
		link({ href, title, tokens }) {
			const text = this.parser.parseInline(tokens);
			const t = title ? ` title="${esc(title)}"` : '';
			if (/^https?:\/\//i.test(href)) {
				return `<a href="${esc(href)}"${t} rel="noopener noreferrer">${text}</a>`;
			}
			if (href.startsWith('#')) return `<a href="${esc(href)}"${t}>${text}</a>`;
			const [pathPart, hash] = href.split('#');
			const resolved = resolveRel(curPath, pathPart);
			const target = byPath.get(resolved);
			const url = target ? target.url : BASE;
			return `<a href="${url}${hash ? `#${esc(hash)}` : ''}"${t}>${text}</a>`;
		},
		strong({ tokens }) {
			const inner = this.parser.parseInline(tokens);
			const plain = stripTags(inner).trim();
			const cls = plain.endsWith('?') ? 'dk-q' : plain.endsWith(':') ? 'dk-lead' : 'dk-ui';
			return `<strong class="${cls}">${inner}</strong>`;
		},
		blockquote({ tokens }) {
			const body = this.parser.parse(tokens);
			const m = CALLOUT.exec(body);
			if (!m) return `<blockquote>\n${body}</blockquote>\n`;
			const kind = m[1].toLowerCase();
			const inner = body.replace(
				CALLOUT,
				`<p><strong class="dk-callout-label">${m[1]}</strong> `
			);
			return `<aside class="dk-callout dk-callout-${kind}" role="note">\n${inner}</aside>\n`;
		}
	}
});

/**
 * Bagian "Pertanyaan yang sering diajukan": tiap paragraf yang diawali teks
 * tebal (pertanyaan) menjadi satu akordeon; blok sesudahnya sampai pertanyaan
 * berikutnya ikut menjadi jawaban.
 */
function faqify(tokens: Token[]): Token[] {
	const out: Token[] = [];
	let inFaq = false;
	let cur: { q: Token[]; a: Token[] } | null = null;
	const para = (t: Token[]) => ({ type: 'paragraph', raw: '', text: '', tokens: t }) as unknown as Token;
	const flush = () => {
		if (!cur) return;
		const q = md.parser([para(cur.q)]).replace(/^<p>|<\/p>\s*$/g, '');
		const a = md.parser(cur.a);
		out.push({
			type: 'html',
			raw: '',
			pre: false,
			block: true,
			text: `<details class="dk-faq"><summary>${q}</summary><div class="dk-faq-a">${a}</div></details>\n`
		} as unknown as Token);
		cur = null;
	};
	for (const t of tokens) {
		if (t.type === 'heading') {
			flush();
			inFaq = t.depth === 2 && /^pertanyaan/i.test(t.text);
			out.push(t);
			continue;
		}
		if (inFaq) {
			if (t.type === 'space') continue;
			const first = t.type === 'paragraph' ? t.tokens?.[0] : undefined;
			if (t.type === 'paragraph' && first?.type === 'strong') {
				flush();
				const rest = (t.tokens ?? []).slice(1);
				const lead = rest[0];
				if (lead?.type === 'text') {
					const text = lead.text.replace(/^\s+/, '');
					rest[0] = text ? ({ type: 'text', raw: text, text } as Token) : (undefined as never);
					if (!text) rest.shift();
				} else if (lead?.type === 'br') rest.shift();
				cur = { q: first.tokens ?? [], a: rest.length ? [para(rest)] : [] };
				continue;
			}
			if (cur) {
				cur.a.push(t);
				continue;
			}
		}
		out.push(t);
	}
	flush();
	return out;
}

export type Rendered = { html: string; lead: string; toc: TocItem[] };
const cache = new Map<string, Rendered>();

export function renderPage(meta: DocMeta): Rendered {
	const hit = cache.get(meta.path);
	if (hit) return hit;

	curPath = meta.path;
	curToc = [];
	curIds = new Map();

	const tokens = md.lexer(sources.get(meta.path) ?? '');
	const firstReal = () => {
		let i = 0;
		while (tokens[i]?.type === 'space') i++;
		return i;
	};
	let i = firstReal();
	const h1 = tokens[i];
	if (h1?.type === 'heading' && h1.depth === 1) tokens.splice(0, i + 1);
	i = firstReal();
	let lead = '';
	const quote = tokens[i];
	if (quote?.type === 'blockquote') {
		lead = md.parser(quote.tokens ?? []);
		tokens.splice(0, i + 1);
	}
	const html = md
		.parser(faqify(tokens))
		.replaceAll('<table>', '<div class="dk-table"><table>')
		.replaceAll('</table>', '</table></div>');

	const out: Rendered = { html, lead, toc: curToc };
	cache.set(meta.path, out);
	return out;
}

// ---------- pencarian ----------

export type SearchEntry = {
	u: string;
	t: string;
	g: string;
	s: string;
	h: string[];
	x: string;
};

let searchCache: SearchEntry[] | null = null;

export function searchIndex(): SearchEntry[] {
	if (searchCache) return searchCache;
	searchCache = pages.map((p) => {
		const text = sources.get(p.path) ?? '';
		const headings = [...text.matchAll(/^#{2,3} (.+)$/gm)].map((m) => plainInline(m[1]));
		const body = text
			.replace(/^# .+$/m, '')
			.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
			.replace(/[*_`>#|]+/g, ' ')
			.replace(/\s+/g, ' ')
			.trim()
			.slice(0, 1500);
		return {
			u: p.url,
			t: p.title,
			g: SECTION_LABEL[p.section],
			s: p.summary,
			h: headings,
			x: body
		};
	});
	return searchCache;
}
