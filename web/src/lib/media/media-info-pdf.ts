/**
 * Laporan info media (PDF) — dokumen A4 bermerek Tauthub.
 *
 * Menggunakan tata letak 2-kolom seimbang (parity 1:1 dengan modal web)
 * agar spasi lega, hierarki visual jelas, dan seluruh konten tampil rapi
 * dalam 1 halaman A4 tanpa berantakan atau bertumpuk.
 */
import { jsPDF } from 'jspdf';

export type MediaInfoMetric = {
	label: string;
	value: string;
	/** 0–100, dipakai untuk bar progres dan warna ambang. */
	pct: number;
};

export type MediaInfoParam = { label: string; value: string };

export type MediaInfoPdfLabels = {
	reportTitle: string;
	generatedAt: string;
	traffic: string;
	da: string;
	aiTrust: string;
	publicationPath: string;
	partnerKpi: string;
	parameters: string;
	gatekeeper: string;
	publicationCost: string;
	directPathTitle: string;
	directPathDesc: string;
	reviewPathTitle: string;
	reviewPathDesc: string;
	restrictedTopics: string;
	footerNote: string;
	page: string;
};

export type MediaInfoPdfData = {
	name: string;
	domainLabel: string;
	websiteUrl: string | null;
	categories: string[];
	mediaTypeLabel: string;
	verified: boolean;
	dewanPers: boolean;
	trafficText: string;
	daText: string;
	aiTrustText: string;
	autoPath: boolean;
	healthScore: number | null;
	healthLabel: string;
	healthMax: number;
	metrics: MediaInfoMetric[];
	historyLine: string;
	parameters: MediaInfoParam[];
	gatekeeper: string[];
	priceText: string;
	// Catatan tambahan di sisi kanan pita harga (mis. satuan kredit). Kosong =
	// tidak dicetak, karena tampilan klien kini memakai rupiah.
	creditText?: string;
	generatedAt: string;
	reference: string;
	labels: MediaInfoPdfLabels;
};

type RGB = [number, number, number];

const PAGE_W = 210;
const PAGE_H = 297;
const MARGIN = 14;
const CONTENT_W = PAGE_W - MARGIN * 2; // 182mm
const COL_GAP = 8;
const COL_W = (CONTENT_W - COL_GAP) / 2; // 87mm

const C = {
	ink: [15, 23, 42] as RGB,
	muted: [100, 116, 139] as RGB,
	soft: [148, 163, 184] as RGB,
	line: [226, 232, 240] as RGB,
	primary: [43, 103, 236] as RGB,
	primarySoft: [238, 244, 255] as RGB,
	canvas: [248, 250, 252] as RGB,
	track: [226, 232, 240] as RGB,
	success: [22, 121, 75] as RGB,
	successSoft: [234, 248, 241] as RGB,
	warning: [161, 92, 0] as RGB,
	warningSoft: [255, 246, 223] as RGB,
	danger: [197, 43, 54] as RGB,
	dangerSoft: [254, 242, 242] as RGB,
	white: [255, 255, 255] as RGB,
	headerMuted: [148, 163, 184] as RGB
};

function thresholdColor(pct: number): RGB {
	if (pct >= 75) return C.success;
	if (pct >= 40) return C.primary;
	return C.warning;
}

export function buildMediaInfoPdf(data: MediaInfoPdfData): jsPDF {
	const doc = new jsPDF({ unit: 'mm', format: 'a4', compress: true });
	const { labels } = data;

	const setFont = (size: number, style: 'normal' | 'bold' | 'italic' = 'normal') => {
		doc.setFont('helvetica', style);
		doc.setFontSize(size);
	};
	const fill = (c: RGB) => doc.setFillColor(c[0], c[1], c[2]);
	const stroke = (c: RGB) => doc.setDrawColor(c[0], c[1], c[2]);
	const ink = (c: RGB) => doc.setTextColor(c[0], c[1], c[2]);

	const print = (
		str: string,
		x: number,
		baseline: number,
		o: { size?: number; style?: 'normal' | 'bold' | 'italic'; color?: RGB; align?: 'left' | 'right' | 'center'; space?: number } = {}
	) => {
		setFont(o.size ?? 9, o.style ?? 'normal');
		ink(o.color ?? C.ink);
		if (o.space) doc.setCharSpace(o.space);
		doc.text(str, x, baseline, { align: o.align ?? 'left' });
		if (o.space) doc.setCharSpace(0);
	};

	const wrapped = (str: string, size: number, style: 'normal' | 'bold' | 'italic', maxW: number) => {
		setFont(size, style);
		return doc.splitTextToSize(str, maxW) as string[];
	};

	const chip = (
		label: string,
		x: number,
		top: number,
		o: { bg: RGB; fg: RGB; border?: RGB; size?: number; height?: number }
	): number => {
		const size = o.size ?? 6.5;
		const ph = o.height ?? 5.5;
		setFont(size, 'bold');
		const tw = doc.getTextWidth(label);
		const pw = tw + 5.5;
		fill(o.bg);
		if (o.border) {
			stroke(o.border);
			doc.setLineWidth(0.25);
			doc.roundedRect(x, top, pw, ph, ph / 2, ph / 2, 'FD');
		} else {
			doc.roundedRect(x, top, pw, ph, ph / 2, ph / 2, 'F');
		}
		ink(o.fg);
		doc.text(label, x + pw / 2, top + ph / 2 + 1.15, { align: 'center' });
		return x + pw;
	};

	// ── 1. Top Header Bar ──
	const HEADER_H = 18;
	fill(C.ink);
	doc.rect(0, 0, PAGE_W, HEADER_H, 'F');
	print('TAUTHUB', MARGIN, 8.5, { size: 12, style: 'bold', color: C.white, space: 0.8 });
	print(labels.reportTitle.toUpperCase(), MARGIN, 13.5, { size: 6.2, color: C.headerMuted, space: 0.6 });
	print(data.domainLabel, PAGE_W - MARGIN, 8.5, { size: 8, style: 'bold', color: C.white, align: 'right' });
	print(`${labels.generatedAt}: ${data.generatedAt}`, PAGE_W - MARGIN, 13.5, { size: 6.8, color: C.headerMuted, align: 'right' });

	// ── 2. Media Title & Badges ──
	let y = HEADER_H + 9;
	print(data.name, MARGIN, y, { size: 16.5, style: 'bold', color: C.ink });
	y += 5.5;
	if (data.websiteUrl) {
		setFont(9, 'bold');
		ink(C.primary);
		doc.textWithLink(data.domainLabel, MARGIN, y, { url: data.websiteUrl });
	} else {
		print(data.domainLabel, MARGIN, y, { size: 9, style: 'bold', color: C.primary });
	}
	y += 4.5;

	// Chips row
	const chips: { text: string; bg: RGB; fg: RGB; border?: RGB }[] = [];
	if (data.verified) chips.push({ text: 'VERIFIED', bg: C.successSoft, fg: C.success });
	if (data.dewanPers) chips.push({ text: 'DEWAN PERS', bg: C.primarySoft, fg: C.primary });
	if (data.mediaTypeLabel) chips.push({ text: data.mediaTypeLabel.toUpperCase(), bg: C.canvas, fg: C.muted, border: C.line });
	for (const cat of data.categories) chips.push({ text: cat.toUpperCase(), bg: C.canvas, fg: C.muted, border: C.line });

	let chipX = MARGIN;
	for (const c of chips) {
		chipX = chip(c.text, chipX, y - 4, { bg: c.bg, fg: c.fg, border: c.border }) + 2.5;
	}
	y += 8;

	// ── 3. Top Metrics Strip (3 Cards across full width) ──
	const cards = [
		{ label: labels.traffic, value: data.trafficText },
		{ label: labels.da, value: data.daText },
		{ label: labels.aiTrust, value: data.aiTrustText }
	];
	const metricCardGap = 5;
	const metricCardW = (CONTENT_W - metricCardGap * 2) / 3;
	const metricCardH = 17;
	cards.forEach((card, i) => {
		const cx = MARGIN + i * (metricCardW + metricCardGap);
		fill(C.canvas);
		stroke(C.line);
		doc.setLineWidth(0.3);
		doc.roundedRect(cx, y, metricCardW, metricCardH, 2.5, 2.5, 'FD');
		print(card.label.toUpperCase(), cx + 4.5, y + 5.5, { size: 6.5, style: 'bold', color: C.muted, space: 0.3 });
		print(card.value, cx + 4.5, y + 12.5, { size: 12, style: 'bold', color: C.ink });
	});
	y += metricCardH + 11;

	// ── 4. Main Two-Column Layout ──
	const bodyTopY = y;
	const leftX = MARGIN;
	const rightX = MARGIN + COL_W + COL_GAP;

	const drawSectionTitle = (title: string, x: number, curY: number, w: number) => {
		print(title.toUpperCase(), x, curY + 4, { size: 7.2, style: 'bold', color: C.primary, space: 0.5 });
		stroke(C.line);
		doc.setLineWidth(0.3);
		doc.line(x, curY + 6.5, x + w, curY + 6.5);
		return curY + 11;
	};

	// ─── LEFT COLUMN ───
	let leftY = bodyTopY;

	// Left Section 1: Jalur Publikasi
	leftY = drawSectionTitle(labels.publicationPath, leftX, leftY, COL_W);
	const pathTitle = data.autoPath ? labels.directPathTitle : labels.reviewPathTitle;
	const pathDesc = data.autoPath ? labels.directPathDesc : labels.reviewPathDesc;
	const pathAccent = data.autoPath ? C.success : C.primary;
	const descLines = wrapped(pathDesc, 7.6, 'normal', COL_W - 12);
	const pathCardH = 8 + 4 + descLines.length * 3.6 + 6;

	fill(C.canvas);
	stroke(C.line);
	doc.setLineWidth(0.3);
	doc.roundedRect(leftX, leftY, COL_W, pathCardH, 2.5, 2.5, 'FD');

	// Accent vertical stripe
	fill(pathAccent);
	doc.roundedRect(leftX + 2.5, leftY + 3, 1.2, pathCardH - 6, 0.6, 0.6, 'F');

	print(pathTitle, leftX + 7, leftY + 6.8, { size: 9, style: 'bold', color: C.ink });
	let pLineY = leftY + 11.5;
	for (const line of descLines) {
		print(line, leftX + 7, pLineY, { size: 7.6, color: C.muted });
		pLineY += 3.6;
	}
	leftY += pathCardH + 9;

	// Left Section 2: Indikator Kinerja Mitra
	leftY = drawSectionTitle(labels.partnerKpi, leftX, leftY, COL_W);

	// Health score box
	const healthH = 14;
	fill(C.canvas);
	stroke(C.line);
	doc.setLineWidth(0.3);
	doc.roundedRect(leftX, leftY, COL_W, healthH, 2.5, 2.5, 'FD');
	print('HEALTH SCORE', leftX + 4.5, leftY + 5.2, { size: 6.2, style: 'bold', color: C.muted, space: 0.3 });
	const scoreText = data.healthScore != null ? `${data.healthScore} / ${data.healthMax}` : '—';
	print(scoreText, leftX + 4.5, leftY + 10.8, { size: 11.5, style: 'bold', color: C.ink });

	// Health badge on right
	setFont(6.8, 'bold');
	const hLabelW = doc.getTextWidth(data.healthLabel.toUpperCase()) + 5;
	const hColor = data.healthScore == null ? C.muted : thresholdColor(data.healthScore);
	chip(data.healthLabel.toUpperCase(), leftX + COL_W - hLabelW - 4, leftY + 4.5, {
		bg: C.white,
		fg: hColor,
		border: hColor,
		size: 6.8,
		height: 5.2
	});
	leftY += healthH + 6.5;

	// 4 Metrics progress bars
	const barH = 2.2;
	for (const metric of data.metrics) {
		print(metric.label, leftX, leftY + 3.2, { size: 7.6, style: 'bold', color: C.ink });
		print(metric.value, leftX + COL_W, leftY + 3.2, { size: 7.6, style: 'bold', color: C.ink, align: 'right' });
		const trackY = leftY + 5;
		fill(C.track);
		doc.roundedRect(leftX, trackY, COL_W, barH, barH / 2, barH / 2, 'F');
		const pct = Math.max(0, Math.min(100, metric.pct));
		if (pct > 0) {
			fill(thresholdColor(pct));
			doc.roundedRect(leftX, trackY, (COL_W * pct) / 100, barH, barH / 2, barH / 2, 'F');
		}
		leftY += 9.5;
	}
	leftY += 1;
	const histLines = wrapped(data.historyLine, 6.8, 'italic', COL_W);
	for (const hl of histLines) {
		print(hl, leftX, leftY + 3, { size: 6.8, style: 'italic', color: C.muted });
		leftY += 3.2;
	}

	// ─── RIGHT COLUMN ───
	let rightY = bodyTopY;

	// Right Section 1: Parameter AI & Redaksi
	rightY = drawSectionTitle(labels.parameters, rightX, rightY, COL_W);

	for (let i = 0; i < data.parameters.length; i++) {
		const item = data.parameters[i];
		print(item.label.toUpperCase(), rightX, rightY + 3.2, { size: 6.2, style: 'bold', color: C.muted, space: 0.3 });
		const valLines = wrapped(item.value, 7.8, 'bold', COL_W);
		valLines.forEach((vl: string, idx: number) => {
			print(vl, rightX, rightY + 7 + idx * 3.6, { size: 7.8, style: 'bold', color: C.ink });
		});
		rightY += 8.2 + (valLines.length - 1) * 3.6 + 2.5;
		if (i < data.parameters.length - 1) {
			stroke(C.line);
			doc.setLineWidth(0.2);
			doc.line(rightX, rightY - 1, rightX + COL_W, rightY - 1);
		}
	}
	rightY += 5;

	// Right Section 2: Pembatasan Gatekeeper
	rightY = drawSectionTitle(labels.gatekeeper, rightX, rightY, COL_W);
	const gkLines = wrapped(labels.restrictedTopics, 7.2, 'normal', COL_W);
	for (const gl of gkLines) {
		print(gl, rightX, rightY + 3, { size: 7.2, color: C.muted });
		rightY += 3.4;
	}
	rightY += 3.5;

	let rx = rightX;
	for (const topic of data.gatekeeper) {
		setFont(6.6, 'bold');
		const width = doc.getTextWidth(topic) + 5.5;
		if (rx + width > rightX + COL_W) {
			rx = rightX;
			rightY += 6.5;
		}
		rx = chip(topic, rx, rightY, { bg: C.dangerSoft, fg: C.danger, size: 6.6, height: 5.2 }) + 2;
	}

	// ── 5. Bottom Price Band ──
	const maxColY = Math.max(leftY, rightY);
	const bandY = Math.max(maxColY + 8, 252);
	const bandH = 20;
	fill(C.ink);
	doc.roundedRect(MARGIN, bandY, CONTENT_W, bandH, 3, 3, 'F');
	print(labels.publicationCost.toUpperCase(), MARGIN + 7, bandY + 7, { size: 6.5, style: 'bold', color: C.headerMuted, space: 0.5 });
	print(data.priceText, MARGIN + 7, bandY + 15, { size: 15, style: 'bold', color: C.white });
	print(data.reference, PAGE_W - MARGIN - 7, bandY + 7, { size: 6.5, color: C.headerMuted, align: 'right' });
	if (data.creditText) print(data.creditText, PAGE_W - MARGIN - 7, bandY + 15, { size: 10, style: 'bold', color: C.white, align: 'right' });

	// ── 6. Document Footer ──
	const footerY = 282;
	stroke(C.line);
	doc.setLineWidth(0.3);
	doc.line(MARGIN, footerY, PAGE_W - MARGIN, footerY);
	print(labels.footerNote, MARGIN, footerY + 4.5, { size: 6.8, color: C.soft });
	print(`${labels.page} 1 / 1`, PAGE_W - MARGIN, footerY + 4.5, { size: 6.8, color: C.soft, align: 'right' });

	doc.setProperties({
		title: `${labels.reportTitle} - ${data.name}`,
		subject: labels.reportTitle,
		author: 'Tauthub',
		creator: 'Tauthub'
	});

	return doc;
}

/** Ubah nama media jadi nama berkas yang aman. */
export function mediaInfoPdfFileName(name: string): string {
	const slug = name
		.trim()
		.replace(/[^\w\s.-]+/g, '')
		.replace(/[\s.]+/g, '-')
		.replace(/-+/g, '-')
		.replace(/^-|-$/g, '')
		.slice(0, 60);
	return `Info-Media_Tauthub_${slug || 'media'}.pdf`;
}
