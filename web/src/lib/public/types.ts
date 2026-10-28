/** Bentuk data katalog publik — cermin respons /v1/public/* di API Go. */

export type PublicMedia = {
	slug: string;
	name: string;
	url: string;
	domain: string;
	website_url: string | null;
	description: string | null;
	categories: string[];
	media_type: string | null;
	tier: string | null;
	language: string[];
	traffic: { monthly: number | null; source: 'similarweb' | 'estimate'; as_of: string | null };
	seo: {
		da: number | null;
		pa: number | null;
		spam_score: number | null;
		ai_trust_score: number | null;
		ai_citation: boolean | null;
		checked_at: string | null;
	};
	price: { amount: number; currency: string; type: 'nett' | 'nego' };
	link_policy: { max_links: number | null; dofollow: boolean; duration_months: number };
	content: { word_count_mode: string; image_required: boolean; allow_embed: boolean; sponsor_label: string };
	trust: {
		dewan_pers_verified: boolean;
		domain_verified: boolean;
		published_count: number;
		rating_avg: number | null;
		rating_count: number;
	};
	/** SLA terbit: rata-rata jam dari kirim artikel sampai tayang (dari riwayat). */
	sla: { avg_publish_hours: number | null };
	example_article_url: string | null;
	updated_at: string | null;
	about?: string;
	standards?: {
		min_words: number | null;
		max_words: number | null;
		journalistic_style: string | null;
		quote_required: boolean;
		min_quotes: number;
		category_required: boolean;
	};
};

export type PublicCategory = { name: string; slug: string; url: string; count: number };

export type PublicStats = {
	total: number;
	categories: number;
	tier1: number;
	tier2: number;
	tier3: number;
	price_min: number;
	avg_da: number;
	updated_at: string | null;
};

export type PublicList = {
	items: PublicMedia[];
	total: number;
	page: number;
	limit: number;
	pages: number;
};
