// Markdown ringan untuk balasan Brief Assistant.
//
// Balasan model dibatasi "Markdown ringan", tapi tidak hanya teks: ringkasan
// brief sering lebih jernih sebagai tabel, kutipan, atau blok kode. Perender ini
// menangani: paragraf, daftar, heading kecil, kutipan (blockquote), tabel pipa,
// blok kode berpagar, garis pemisah, plus penekanan/inline code/link.
//
// Keamanan: seluruh teks di-escape LEBIH DULU, lalu hasilnya tetap wajib
// dilewatkan sanitizeHtml sebelum di-render.

const escape = (s: string) =>
	s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

function inline(raw: string): string {
	let out = escape(raw);
	// URL tautan: hanya http(s); tanda kutip sudah dimatikan oleh escape.
	out = out.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, (_m, text: string, href: string) => {
		return `<a href="${href}" target="_blank" rel="noopener noreferrer">${text}</a>`;
	});
	out = out.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
	out = out.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
	out = out.replace(/~~([^~\n]+)~~/g, '<del>$1</del>');
	out = out.replace(/`([^`\n]+)`/g, '<code>$1</code>');
	return out;
}

/** Pisahkan baris tabel pipa jadi sel-sel. */
function splitRow(line: string): string[] {
	const trimmed = line.trim().replace(/^\|/, '').replace(/\|$/, '');
	return trimmed.split('|').map((c) => c.trim());
}

const isTableSeparator = (line: string) =>
	/^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/.test(line) && line.includes('-');

const isTableRow = (line: string) => line.includes('|');

/** Render teks markdown ringan jadi HTML. Belum disanitasi — selalu panggil
 *  `sanitizeHtml(renderMarkdown(text))` sebelum `{@html}`. */
export function renderMarkdown(text: string): string {
	const lines = (text ?? '').split(/\r?\n/);
	const blocks: string[] = [];
	let para: string[] = [];
	let list: { ordered: boolean; items: string[] } | null = null;
	let quote: string[] = [];
	let code: { lang: string; lines: string[] } | null = null;

	const flushList = () => {
		if (!list) return;
		const tag = list.ordered ? 'ol' : 'ul';
		blocks.push(`<${tag}>${list.items.map((i) => `<li>${i}</li>`).join('')}</${tag}>`);
		list = null;
	};
	const flushQuote = () => {
		if (quote.length === 0) return;
		blocks.push(`<blockquote>${quote.map(inline).join('<br>')}</blockquote>`);
		quote = [];
	};
	const flushPara = () => {
		if (para.length === 0) return;
		blocks.push(`<p>${para.map(inline).join('<br>')}</p>`);
		para = [];
	};
	const flushAll = () => {
		flushList();
		flushQuote();
		flushPara();
	};

	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];
		const trimmed = line.trim();

		// Di dalam blok kode: apa pun ditulis apa adanya sampai pagar penutup.
		if (code) {
			if (trimmed.startsWith('```')) {
				blocks.push(
					`<pre><code>${code.lines.map(escape).join('\n')}</code></pre>`
				);
				code = null;
			} else {
				code.lines.push(line);
			}
			continue;
		}

		if (trimmed.startsWith('```')) {
			flushAll();
			code = { lang: trimmed.slice(3).trim(), lines: [] };
			continue;
		}

		if (trimmed === '') {
			flushAll();
			continue;
		}

		// Tabel pipa: baris judul diikuti baris pemisah.
		if (isTableRow(trimmed) && i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
			flushAll();
			const header = splitRow(trimmed);
			const rows: string[][] = [];
			i += 2;
			while (i < lines.length && isTableRow(lines[i]) && lines[i].trim() !== '') {
				rows.push(splitRow(lines[i]));
				i++;
			}
			i--;
			const head = header.map((h) => `<th>${inline(h)}</th>`).join('');
			const body = rows
				.map((r) => `<tr>${r.map((c) => `<td>${inline(c)}</td>`).join('')}</tr>`)
				.join('');
			blocks.push(`<table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`);
			continue;
		}

		// Kutipan.
		if (trimmed.startsWith('>')) {
			flushList();
			flushPara();
			quote.push(trimmed.replace(/^>\s?/, ''));
			continue;
		}

		// Garis pemisah.
		if (/^(?:-{3,}|\*{3,}|_{3,})$/.test(trimmed)) {
			flushAll();
			blocks.push('<hr>');
			continue;
		}

		// Heading kecil — dibatasi sampai h4 supaya balasan chat tidak seperti dokumen.
		const heading = trimmed.match(/^(#{1,4})\s+(.*)$/);
		if (heading) {
			flushAll();
			const level = Math.min(4, heading[1].length + 2); // # → h3, ## → h4, dst.
			blocks.push(`<h${level}>${inline(heading[2])}</h${level}>`);
			continue;
		}

		const bulleted = trimmed.match(/^[-*•]\s+(.*)$/);
		const ordered = trimmed.match(/^\d+[.)]\s+(.*)$/);
		if (bulleted || ordered) {
			flushQuote();
			flushPara();
			if (!list || list.ordered !== Boolean(ordered)) {
				flushList();
				list = { ordered: Boolean(ordered), items: [] };
			}
			list.items.push(inline((bulleted?.[1] ?? ordered?.[1] ?? '').trim()));
			continue;
		}

		list = null;
		para.push(trimmed);
	}

	if (code) {
		// Pagar tidak pernah ditutup: tetap tampilkan sebagai blok kode.
		blocks.push(`<pre><code>${code.lines.map(escape).join('\n')}</code></pre>`);
	}
	flushAll();
	return blocks.join('');
}
