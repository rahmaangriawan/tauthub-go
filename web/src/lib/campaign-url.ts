/**
 * Segmen URL kampanye yang dipakai di semua tautan klien.
 *
 * Kampanye punya dua identitas: `id` (UUID, primary key) dan `public_id`
 * (6 karakter, migrasi 170). URL memakai `public_id` supaya seragam dengan
 * segmen brand, sementara API tetap menerima keduanya — `campaigns.Resolver`
 * mengubah public_id menjadi UUID sebelum handler jalan. Karena itu tautan
 * lama berbentuk UUID tidak pernah mati.
 */
type CampaignRef = { id: string; public_id?: string | null };

export function campaignUrlId(campaign: CampaignRef | null | undefined): string {
	if (!campaign) return '';
	return campaign.public_id?.trim() || campaign.id;
}
