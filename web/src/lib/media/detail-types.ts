/**
 * Tipe data untuk modal detail media bersama (klien & mitra).
 * Field yang hanya dipakai fitur klien (harga/CTA/ekspor) dibuat opsional
 * supaya payload mitra — yang tidak memuat effective_quality/quality_source/
 * credit_cost — tetap lolos type-check.
 */
export type CrawlStatusData = {
	articles_7d?: number | null;
	last_crawled_at?: string | null;
	stale?: boolean;
	last_status?: string | null;
	source?: string | null;
};

export type MediaDetailItem = {
	id: string;
	name: string;
	category: string | null;
	categories?: string[];
	description: string | null;
	website_url: string | null;
	contoh_artikel_url?: string | null;
	bahasa_artikel?: string[] | null;
	price: number;
	pricing_type?: string | null;
	client_price: number;
	markup_percent?: number | null;
	credit_cost?: number;
	integration_type: string;
	traffic_estimate: number;
	media_tier: string | null;
	media_type?: string | null;
	media_quality_score: number | null;
	effective_quality?: number | null;
	quality_source?: 'media' | 'tier_default' | null;
	health_score: number | null;
	health_computed_at?: string | null;
	health_breakdown?: Record<string, unknown> | null;
	published_count?: number | null;
	dewan_pers_verified?: boolean | null;
	domain_verified?: boolean | null;
	seo_da?: number | null;
	seo_pa?: number | null;
	seo_spam_score?: number | null;
	seo_checked_at?: string | null;
	seo_ai_trust?: number | null;
	seo_ai_citation?: boolean | null;
	gsc_connected?: boolean;
	sw_visits?: number | null;
	sw_snapshot_date?: string | null;
	rating_avg?: number | null;
	rating_count?: number | null;
	partner_id?: string | null;
	partner_public_id?: string | null;
	partner_name?: string | null;
	status?: string | null;
	label_sponsor?: string | null;
	topik_terlarang?: string[] | null;
	max_links?: number | null;	allow_hyperlink?: boolean | null;
	/** Rel tautan: dofollow | nofollow | sponsored. */
	link_rel?: string | null;
	link_duration_months?: number | null;
	image_required?: boolean | null;
	publication_mode?: string | null;
	word_count_mode?: string | null;
	allow_embed?: boolean | null;
	/** Izin subjudul H2/H3: null = ikut bawaan tipe media. */
	allow_heading?: boolean | null;
	auto_publish?: boolean | null;
	price_status?: string | null;
	created_at?: string | null;
};
