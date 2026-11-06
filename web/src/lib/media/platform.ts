// Platform website media: WordPress / EmDash / Blogspot / Custom.
// Media yang artikelnya dikirim lewat API memakai kanalnya (integration_type);
// media Manual memakai hasil deteksi website_platform. Belum terdeteksi → '—'.
type PlatformSource = { integration_type?: string | null; website_platform?: string | null };

const NAMES: Record<string, string> = {
	wordpress: 'WordPress',
	emdash: 'EmDash',
	blogger: 'Blogspot',
	blogspot: 'Blogspot',
	custom_web: 'Custom',
	custom: 'Custom',
	tulisio_api: 'Custom'
};

export function mediaPlatform(m: PlatformSource): string {
	const channel = (m.integration_type ?? '').trim();
	if (channel && channel !== 'manual' && NAMES[channel]) return NAMES[channel];
	const detected = (m.website_platform ?? '').trim();
	return NAMES[detected] ?? '—';
}
