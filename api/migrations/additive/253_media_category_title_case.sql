-- Kategori media yang diisi mitra dengan huruf kecil (mis. "berita") diseragamkan
-- jadi "Berita". Normalisasi saat menulis sudah ada di Go (catutil.Canonical);
-- migrasi ini membereskan data lama TANPA mengubah `normalized`, jadi filter
-- kategori dan link media yang sudah ada tetap utuh.
--
-- Urutan penting: gabungkan dulu kategori yang hanya beda besar-kecil huruf,
-- baru rapikan sisanya. Kalau dibalik, "berita" jadi "Berita" kedua dan unique
-- index normalized tidak bisa menampungnya.
--
-- Fungsi lokal dipakai karena INITCAP bawaan PostgreSQL memperlakukan semua
-- huruf non-ASCII sebagai pemisah ("UMKM" → "Umkm"), sedangkan regexp_replace
-- tidak mendukung escape uppercase.

CREATE FUNCTION pg_temp.media_category_title(name text) RETURNS text
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
	words text[];
	i int;
	w text;
	out text[] := '{}';
BEGIN
	words := regexp_split_to_array(btrim(name), '\s+');
	FOR i IN 1 .. COALESCE(array_length(words, 1), 0) LOOP
		w := words[i];
		-- Kata kapital semua ("SEO", "UMKM") atau berangka/simbol dibiarkan,
		-- supaya singkatan dan nama merek tidak rusak.
		IF w <> upper(w) AND w ~ '^[[:alpha:]]+$' THEN
			w := upper(left(w, 1)) || lower(substr(w, 2));
		END IF;
		out := out || w;
	END LOOP;
	RETURN array_to_string(out, ' ');
END
$$;

-- 1) Gabungkan kategori duplikat (beda kapitalisasi). Yang dipertahankan: baris
--    yang sudah berkapitalisasi normal, lalu yang paling tua.
CREATE TEMP TABLE _cat_merge ON COMMIT DROP AS
WITH ranked AS (
	SELECT id,
	       first_value(id) OVER (
		   PARTITION BY lower(btrim(name))
		   ORDER BY (btrim(name) = pg_temp.media_category_title(btrim(name))) DESC,
		            created_at, id
	       ) AS keep_id
	FROM media_categories
)
SELECT id AS dup_id, keep_id FROM ranked WHERE id <> keep_id;

UPDATE media_category_links l
SET category_id = m.keep_id
FROM _cat_merge m
WHERE l.category_id = m.dup_id
  AND NOT EXISTS (
	SELECT 1 FROM media_category_links x
	WHERE x.media_id = l.media_id AND x.category_id = m.keep_id
  );

DELETE FROM media_category_links l USING _cat_merge m WHERE l.category_id = m.dup_id;
DELETE FROM media_categories c USING _cat_merge m WHERE c.id = m.dup_id;

-- 2) Kategori sisanya ikut diseragamkan, termasuk kolom legacy media.category.
UPDATE media_categories
SET name = pg_temp.media_category_title(name)
WHERE name <> pg_temp.media_category_title(name);

UPDATE media
SET category = pg_temp.media_category_title(category)
WHERE category IS NOT NULL AND category <> pg_temp.media_category_title(category);
