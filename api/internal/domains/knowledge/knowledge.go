// Package knowledge: port app/v1/brands/[brandId]/_knowledge.ts + route
// knowledge/*. Chunking, insertSource (chunk+embed), retrieval hybrid,
// sitemap crawler (safe-fetch, dedupe, deprecate/restore), chat-learning.
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// ---------- helpers _content ----------

var ErrNotFoundCollection = fmt.Errorf("Knowledge collection not found")
var ErrNotFoundSource = fmt.Errorf("Knowledge source not found")

// Collection: port collection() — brand-scoped; tidak ada → NotFound (404).
func Collection(ctx context.Context, pool *pgxpool.Pool, brandID, collectionID string) (map[string]any, error) {
	var id, name string
	var description *string
	var createdAt time.Time
	err := pool.QueryRow(ctx,
		"SELECT id, name, description, created_at FROM knowledge_collections WHERE id = $1 AND brand_id = $2 LIMIT 1",
		collectionID, brandID).Scan(&id, &name, &description, &createdAt)
	if err != nil {
		return nil, ErrNotFoundCollection
	}
	return map[string]any{"id": id, "name": name, "description": description, "created_at": isoMilli(createdAt)}, nil
}

// Source: port source().
func Source(ctx context.Context, pool *pgxpool.Pool, brandID, sourceID string) (map[string]any, error) {
	var id, collectionID, title string
	var sourceType *string
	var originalFilename *string
	var status string
	var errorMessage *string
	var createdAt time.Time
	err := pool.QueryRow(ctx, `
		SELECT id, collection_id, title, source_type, original_filename, status, error_message, created_at
		FROM knowledge_sources WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL LIMIT 1`,
		sourceID, brandID).Scan(&id, &collectionID, &title, &sourceType, &originalFilename, &status, &errorMessage, &createdAt)
	if err != nil {
		return nil, ErrNotFoundSource
	}
	return map[string]any{
		"id": id, "collection_id": collectionID, "title": title, "source_type": sourceType,
		"original_filename": originalFilename, "status": status, "error_message": errorMessage,
		"created_at": isoMilli(createdAt),
	}, nil
}

// ---------- chunkText (parity) ----------

var blankLine = regexp.MustCompile(`\n\s*\n`)

// ChunkText: split paragraf kosong-baris, gabung hingga size (1200), paragraf
// lebih panjang dari size dipotong per size karakter (parity persis).
func ChunkText(text string, size int) []string {
	if size <= 0 {
		size = 1200
	}
	parts := blankLine.Split(text, -1)
	paragraphs := []string{}
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			paragraphs = append(paragraphs, t)
		}
	}
	if len(paragraphs) == 0 {
		paragraphs = []string{strings.TrimSpace(text)}
	}
	chunks := []string{}
	current := ""
	for _, paragraph := range paragraphs {
		if current != "" && len(current)+len(paragraph)+2 > size {
			chunks = append(chunks, current)
			current = ""
		}
		if len(paragraph) > size {
			for i := 0; i < len(paragraph); i += size {
				end := i + size
				if end > len(paragraph) {
					end = len(paragraph)
				}
				chunks = append(chunks, paragraph[i:end])
			}
			continue
		}
		if current != "" {
			current = current + "\n\n" + paragraph
		} else {
			current = paragraph
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

// ---------- insertSource (parity: tx + batch chunks + embedding) ----------

type InsertSourceOptions struct {
	BrandID         string
	CollectionID    string
	UserID          string
	Title           string
	Content         string
	SourceType      string
	OriginalFilename *string
	OriginSitemap   *string
	Status          string // "processing" | "active"; default "active"
}

// InsertSource: INSERT source + (status aktif) batch-insert chunks dengan
// embedding lokal — dalam SATU transaksi (parity persis).
func InsertSource(ctx context.Context, pool *pgxpool.Pool, opts InsertSourceOptions) (map[string]any, error) {
	id := uuid.NewString()
	status := opts.Status
	if status == "" {
		status = "active"
	}
	sourceType := opts.SourceType
	if sourceType == "" {
		sourceType = "manual"
	}
	checksumInput := opts.Content
	if checksumInput == "" && opts.OriginalFilename != nil {
		checksumInput = *opts.OriginalFilename
	}
	if checksumInput == "" {
		checksumInput = id
	}
	sum := sha256.Sum256([]byte(checksumInput))
	checksum := hex.EncodeToString(sum[:])

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_sources (id, brand_id, collection_id, title, source_type, original_filename, origin_sitemap, content, checksum, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		id, opts.BrandID, opts.CollectionID, opts.Title, sourceType, opts.OriginalFilename, opts.OriginSitemap,
		opts.Content, checksum, status, opts.UserID); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	if status == "active" {
		chunks := ChunkText(opts.Content, 1200)
		if len(chunks) > 0 {
			// Batch-insert satu statement (parity: placeholders + values flatten).
			placeholders := make([]string, len(chunks))
			values := make([]any, 0, len(chunks)*8)
			for i, chunk := range chunks {
				placeholders[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
					i*8+1, i*8+2, i*8+3, i*8+4, i*8+5, i*8+6, i*8+7, i*8+8)
				values = append(values,
					uuid.NewString(), opts.BrandID, opts.CollectionID, id, i, chunk,
					maxInt(1, len(chunk)/4),
					mustJSON(ai.LocalEmbedding(chunk)),
				)
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO knowledge_chunks (id, brand_id, collection_id, source_id, chunk_index, content, token_estimate, embedding) VALUES `+strings.Join(placeholders, ", "),
				values...); err != nil {
				_ = tx.Rollback(ctx)
				return nil, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return Source(ctx, pool, opts.BrandID, id)
}

// ---------- retrieval (parity retrieveChunks: 0.65 semantic + 0.35 lexical) ----------

// lexicalHits: varian _knowledge.ts — hits/queryTerms (BEDA dari Jaccard embed.ts).
func lexicalHits(query, content string) float64 {
	termRe := regexp.MustCompile(`[\w-]+`)
	queryTerms := map[string]bool{}
	for _, t := range termRe.FindAllString(strings.ToLower(query), -1) {
		queryTerms[t] = true
	}
	if len(queryTerms) == 0 {
		return 0
	}
	contentTerms := map[string]bool{}
	for _, t := range termRe.FindAllString(strings.ToLower(content), -1) {
		contentTerms[t] = true
	}
	hits := 0
	for term := range queryTerms {
		if contentTerms[term] {
			hits++
		}
	}
	return float64(hits) / float64(len(queryTerms))
}

type ScoredChunk struct {
	ChunkID     string  `json:"chunk_id"`
	SourceID    string  `json:"source_id"`
	SourceTitle string  `json:"source_title"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
	ChunkIndex  int     `json:"chunk_index"`
}

// RetrieveChunks: ambil kandidat ≤500, skor = semantic*0.65 + lexical*0.35,
// bulatkan 1e6, urutkan desc, ambil limit (parity persis).
func RetrieveChunks(ctx context.Context, pool *pgxpool.Pool, brandID, query string, limit int) ([]ScoredChunk, error) {
	rowsQ, err := pool.Query(ctx, `
		SELECT c.id AS chunk_id, c.source_id, COALESCE(s.title, c.source_id) AS source_title, c.content, c.chunk_index, c.embedding
		FROM knowledge_chunks c
		JOIN knowledge_sources s ON s.id = c.source_id
		WHERE c.brand_id = $1 AND s.status = 'active' AND s.deleted_at IS NULL
		LIMIT 500`, brandID)
	if err != nil {
		return nil, err
	}
	defer rowsQ.Close()

	queryVector := ai.LocalEmbedding(query)
	scored := []ScoredChunk{}
	for rowsQ.Next() {
		var chunkID, sourceID, sourceTitle, content string
		var chunkIndex int
		var embeddingRaw []byte
		if err := rowsQ.Scan(&chunkID, &sourceID, &sourceTitle, &content, &chunkIndex, &embeddingRaw); err != nil {
			return nil, err
		}
		var embedding []float64
		if err := json.Unmarshal(embeddingRaw, &embedding); err != nil {
			continue
		}
		semantic := 0.0
		if len(embedding) == len(queryVector) {
			semantic = ai.CosineSimilarity(queryVector, embedding)
		}
		score := semantic*0.65 + lexicalHits(query, content)*0.35
		scored = append(scored, ScoredChunk{
			ChunkID: chunkID, SourceID: sourceID, SourceTitle: sourceTitle,
			Content: content, Score: mathRound6(score), ChunkIndex: chunkIndex,
		})
	}
	// sort desc by score
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0 && scored[j].Score > scored[j-1].Score; j-- {
			scored[j], scored[j-1] = scored[j-1], scored[j]
		}
	}
	if limit > len(scored) {
		limit = len(scored)
	}
	return scored[:limit], nil
}

func mathRound6(v float64) float64 {
	return float64(int64(v*1e6+copysignHalf(v))) / 1e6
}

func copysignHalf(v float64) float64 {
	if v < 0 {
		return -0.5
	}
	return 0.5
}

// ---------- sitemap crawler (port crawlSitemap) ----------

var publicHTTPRe = regexp.MustCompile(`^https?://`)

func isPublicHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := parsed.Hostname()
	if host == "" || host == "localhost" {
		return false
	}
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}
	return true
}

const crawlUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

func crawlFetch(ctx context.Context, target string, timeout time.Duration) (*http.Response, error) {
	return security.Get(ctx, target, map[string]string{
		"User-Agent":      crawlUserAgent,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7",
	}, timeout)
}

var locRe = regexp.MustCompile(`(?s)<loc>(.*?)</loc>`)

var skipParts = []string{"/tag/", "/tags/", "/author/", "/category/", "/kategori/", "/cart", "/checkout", "/login", "/register", "/wp-admin", "/feed", "/page/", "/search", "/privacy", "/terms"}

// ParseSitemapUrls: parity parseSitemapUrls — pisah pages vs nested .xml.
func ParseSitemapUrls(xmlText, baseHost string) (pages, nested []string) {
	seenPages := map[string]bool{}
	seenNested := map[string]bool{}
	for _, match := range locRe.FindAllStringSubmatch(xmlText, -1) {
		href := strings.TrimSpace(match[1])
		if href == "" {
			continue
		}
		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}
		if parsed.Hostname() != baseHost {
			continue
		}
		lower := strings.ToLower(parsed.Path)
		if strings.HasSuffix(lower, ".xml") {
			if !seenNested[href] {
				seenNested[href] = true
				nested = append(nested, href)
			}
		} else {
			skip := false
			for _, part := range skipParts {
				if strings.Contains(lower, part) {
					skip = true
					break
				}
			}
			if !skip && !seenPages[href] {
				seenPages[href] = true
				pages = append(pages, href)
			}
		}
	}
	return pages, nested
}

// PageContent: parity pageContent — buang script/style/comment/tag, decode
// entitas umum, rapikan spasi, potong 12.000 char.
func PageContent(htmlText string) string {
	s := htmlText
	s = regexpReplace(s, `(?i)<script\b[^>]*>[\s\S]*?</script>`, " ")
	s = regexpReplace(s, `(?i)<style\b[^>]*>[\s\S]*?</style>`, " ")
	s = regexpReplace(s, `<!--[\s\S]*?-->`, " ")
	s = regexpReplace(s, `<[^>]+>`, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 12_000 {
		s = s[:12_000]
	}
	return strings.TrimSpace(s)
}

// TitleOf: parity titleOf — ambil <title>, fallback nama halaman, maks 255.
func TitleOf(htmlText, fallback string) string {
	re := regexp.MustCompile(`(?i)<title[^>]*>([\s\S]*?)</title>`)
	if m := re.FindStringSubmatch(htmlText); m != nil {
		title := strings.Join(strings.Fields(m[1]), " ")
		if title != "" {
			if len(title) > 255 {
				title = title[:255]
			}
			return title
		}
	}
	if len(fallback) > 255 {
		return fallback[:255]
	}
	return fallback
}

func regexpReplace(s, pattern, replacement string) string {
	return regexp.MustCompile(pattern).ReplaceAllString(s, replacement)
}

// CreateCrawlRun / UpdateCrawlRun / GetCrawlRun: parity helpers.
func CreateCrawlRun(ctx context.Context, pool *pgxpool.Pool, brandID, collectionID, sitemapURL string, maxPages int) (string, error) {
	id := uuid.NewString()
	_, err := pool.Exec(ctx,
		"INSERT INTO knowledge_crawl_runs (id, brand_id, collection_id, sitemap_url, max_pages, status) VALUES ($1, $2, $3, $4, $5, 'pending')",
		id, brandID, collectionID, sitemapURL, maxPages)
	return id, err
}

type CrawlRunUpdate struct {
	Status       *string
	TotalURLs    *int
	Processed    *int
	FailedURLs   *string // JSON string
	ErrorMessage *string
}

func UpdateCrawlRun(ctx context.Context, pool *pgxpool.Pool, runID string, u CrawlRunUpdate) {
	sets := []string{}
	args := []any{}
	if u.Status != nil {
		args = append(args, *u.Status)
		sets = append(sets, fmt.Sprintf("status = $%d", len(args)))
	}
	if u.TotalURLs != nil {
		args = append(args, *u.TotalURLs)
		sets = append(sets, fmt.Sprintf("total_urls = $%d", len(args)))
	}
	if u.Processed != nil {
		args = append(args, *u.Processed)
		sets = append(sets, fmt.Sprintf("processed = $%d", len(args)))
	}
	if u.FailedURLs != nil {
		args = append(args, *u.FailedURLs)
		sets = append(sets, fmt.Sprintf("failed_urls = $%d::jsonb", len(args)))
	}
	if u.ErrorMessage != nil {
		args = append(args, *u.ErrorMessage)
		sets = append(sets, fmt.Sprintf("error_message = $%d", len(args)))
	}
	if len(sets) == 0 {
		return
	}
	args = append(args, runID)
	_, _ = pool.Exec(ctx, fmt.Sprintf("UPDATE knowledge_crawl_runs SET %s, updated_at = NOW() WHERE id = $%d", strings.Join(sets, ", "), len(args)), args...)
}

func fetchSitemapXML(ctx context.Context, target string) (string, bool) {
	resp, err := crawlFetch(ctx, target, 20*time.Second)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if len(buf) >= 5_000_000 {
			break
		}
		if err != nil {
			break
		}
	}
	return string(buf), true
}

// DiscoverSitemaps: parity discoverSitemaps — kandidat wp/sitemap_index,
// nested hingga 50.
func DiscoverSitemaps(ctx context.Context, submittedURL, baseHost string) []string {
	candidates := []string{submittedURL}
	if strings.HasSuffix(submittedURL, ".xml") {
		base, err := url.Parse(submittedURL)
		if err == nil {
			root := *base
			root.Path = "/sitemap_index.xml"
			wp := *base
			wp.Path = "/wp-sitemap.xml"
			plain := *base
			plain.Path = "/sitemap.xml"
			candidates = append(candidates, root.String(), wp.String(), plain.String())
		}
	}
	found := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		xmlText, ok := fetchSitemapXML(ctx, candidate)
		if !ok {
			continue
		}
		_, nested := ParseSitemapUrls(xmlText, baseHost)
		if len(nested) > 0 {
			for i, nestedURL := range nested {
				if i >= 50 {
					break
				}
				if !seen[nestedURL] {
					seen[nestedURL] = true
					found = append(found, nestedURL)
				}
			}
		} else if !seen[candidate] {
			seen[candidate] = true
			found = append(found, candidate)
		}
	}
	if !seen[submittedURL] && !strings.HasSuffix(submittedURL, ".xml") {
		found = append(found, submittedURL)
	}
	return found
}

// CrawlSitemap: port crawlSitemap — discovery, fetch halaman (delay 350ms),
// insertSource type=url, skip duplikat (23505), deprecate/restore sumber lama.
func CrawlSitemap(ctx context.Context, pool *pgxpool.Pool, runID, brandID, collectionID, sitemapURL string, maxPages int, userID string) (totalFound, queued, skipped int, failed []string, err error) {
	if !isPublicHTTPURL(sitemapURL) {
		return 0, 0, 0, nil, fmt.Errorf("Sitemap URL must be a public http(s) address")
	}
	baseHost, _ := url.Parse(sitemapURL)
	discovered := DiscoverSitemaps(ctx, sitemapURL, baseHost.Hostname())
	pages := []string{}
	originByPage := map[string]string{}
	for i, sitemap := range discovered {
		if i >= 20 || len(pages) >= maxPages {
			break
		}
		xmlText, ok := fetchSitemapXML(ctx, sitemap)
		if !ok {
			continue
		}
		pageList, _ := ParseSitemapUrls(xmlText, baseHost.Hostname())
		for _, page := range pageList {
			if len(pages) >= maxPages {
				break
			}
			if !containsStr(pages, page) {
				pages = append(pages, page)
				originByPage[page] = sitemap
			}
		}
	}
	if len(pages) == 0 {
		return 0, 0, 0, nil, fmt.Errorf("No crawlable URLs found in sitemap")
	}
	totalFound = len(pages)
	selected := pages
	if len(selected) > maxPages {
		selected = selected[:maxPages]
	}

	// existing URL sources: original_filename → origin_sitemap
	existing := map[string]string{}
	exRows, err := pool.Query(ctx,
		"SELECT original_filename, origin_sitemap FROM knowledge_sources WHERE collection_id = $1 AND source_type = 'url' AND deleted_at IS NULL", collectionID)
	if err == nil {
		for exRows.Next() {
			var fn string
			var origin *string
			if exRows.Scan(&fn, &origin) == nil {
				o := ""
				if origin != nil {
					o = *origin
				}
				existing[fn] = o
			}
		}
		exRows.Close()
	}

	queuedIDs := []string{}
	failed = []string{}
	skipped = 0
	running := "running"
	UpdateCrawlRun(ctx, pool, runID, CrawlRunUpdate{Status: &running, TotalURLs: &totalFound, Processed: intPtr(0)})

	for _, pageURL := range selected {
		origin := originByPage[pageURL]
		var originPtr *string
		if origin != "" {
			o := origin
			originPtr = &o
		}
		if o, dup := existing[pageURL]; dup {
			if o != origin {
				_, _ = pool.Exec(ctx,
					"UPDATE knowledge_sources SET origin_sitemap = $1 WHERE collection_id = $2 AND source_type = 'url' AND original_filename = $3 AND deleted_at IS NULL",
					originPtr, collectionID, pageURL)
			}
			skipped++
			continue
		}
		select {
		case <-time.After(350 * time.Millisecond):
		case <-ctx.Done():
			return totalFound, len(queuedIDs), skipped, failed, ctx.Err()
		}
		resp, fetchErr := crawlFetch(ctx, pageURL, 20*time.Second)
		if fetchErr == nil {
			bodyBytes := readAll(resp)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				htmlText := string(bodyBytes)
				_, insErr := InsertSource(ctx, pool, InsertSourceOptions{
					BrandID: brandID, CollectionID: collectionID, UserID: userID,
					Title: TitleOf(htmlText, pageURL), Content: PageContent(htmlText),
					SourceType: "url", OriginalFilename: &pageURL, OriginSitemap: originPtr,
				})
				if insErr != nil {
					if strings.Contains(insErr.Error(), "23505") || strings.Contains(insErr.Error(), "duplicate") {
						skipped++
					} else {
						failed = append(failed, pageURL)
					}
				} else {
					queuedIDs = append(queuedIDs, pageURL)
				}
			} else {
				failed = append(failed, pageURL)
			}
		} else {
			failed = append(failed, pageURL)
		}

		done := len(queuedIDs) + skipped + len(failed)
		if done%10 == 0 || done == len(selected) {
			UpdateCrawlRun(ctx, pool, runID, CrawlRunUpdate{Processed: &done})
		}
	}

	// Deprecate sumber lama dari sitemap yang sama yang tak ada lagi; restore
	// yang kembali muncul (parity).
	crawledSet := map[string]bool{}
	for _, p := range selected {
		crawledSet[p] = true
	}
	crawledOrigins := map[string]bool{}
	for i, o := range discovered {
		if i < 20 {
			crawledOrigins[o] = true
		}
	}
	toDeprecate := []string{}
	for fn, origin := range existing {
		if crawledSet[fn] || origin == "" || !crawledOrigins[origin] {
			continue
		}
		toDeprecate = append(toDeprecate, fn)
	}
	restoreRows, err := pool.Query(ctx,
		"SELECT original_filename FROM knowledge_sources WHERE collection_id = $1 AND source_type = 'url' AND status = 'deprecated' AND deleted_at IS NOT NULL", collectionID)
	if err == nil {
		for restoreRows.Next() {
			var fn string
			if restoreRows.Scan(&fn) == nil {
				if crawledSet[fn] {
					_, _ = pool.Exec(ctx,
						"UPDATE knowledge_sources SET status = 'active', deleted_at = NULL WHERE collection_id = $1 AND source_type = 'url' AND original_filename = $2 AND status = 'deprecated'",
						collectionID, fn)
				}
			}
		}
		restoreRows.Close()
	}
	for _, fn := range toDeprecate {
		_, _ = pool.Exec(ctx,
			"UPDATE knowledge_sources SET status = 'deprecated', deleted_at = NOW() WHERE collection_id = $1 AND source_type = 'url' AND original_filename = $2 AND deleted_at IS NULL",
			collectionID, fn)
	}

	completed := "completed"
	processed := len(queuedIDs) + skipped + len(failed)
	failedJSON, _ := json.Marshal(failed)
	UpdateCrawlRun(ctx, pool, runID, CrawlRunUpdate{
		Status: &completed, TotalURLs: &totalFound, Processed: &processed, FailedURLs: strPtr(string(failedJSON)),
	})
	_, _ = pool.Exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'knowledge.sitemap.imported', 'knowledge_collection', $4)",
		uuid.NewString(), brandID, userID, collectionID)

	return totalFound, len(queuedIDs), skipped, failed, nil
}

func readAll(resp *http.Response) []byte {
	defer resp.Body.Close()
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
		if len(buf) > 5_000_000 {
			break
		}
	}
	return buf
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func intPtr(v int) *int { return &v }

func strPtr(s string) *string { return &s }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

var _ = pgx.ErrNoRows
var _ = slog.Info

// ---------- tambahan fase 3 (poolKey, uuidNew, recoverCrawlPanic, process job) ----------

type poolKey struct{}

func uuidNew() string { return uuid.NewString() }

func recoverCrawlPanic() {
	if r := recover(); r != nil {
		slog.Error("[knowledge] crawl panic", "err", r)
	}
}

// ProcessSitemapCrawlJob: port processJob kind 'sitemap_crawl' — klaim job
// (FOR UPDATE SKIP LOCKED), jalankan crawl, tandai completed/failed+backoff
// (parity queue.ts).
func ProcessSitemapCrawlJob(ctx context.Context, pool *pgxpool.Pool, payload []byte) error {
	var p struct {
		RunID        string `json:"runId"`
		BrandID      string `json:"brandId"`
		CollectionID string `json:"collectionId"`
		SitemapURL   string `json:"sitemapUrl"`
		MaxPages     int    `json:"maxPages"`
		UserID       string `json:"userId"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	var jobID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM jobs
		WHERE kind = 'sitemap_crawl' AND status IN ('pending','failed') AND payload->>'runId' = $1
		ORDER BY run_at FOR UPDATE SKIP LOCKED LIMIT 1`, p.RunID).Scan(&jobID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err // tidak ada job yang bisa diklaim
	}
	if _, err := tx.Exec(ctx,
		"UPDATE jobs SET status = 'running', attempts = attempts + 1, locked_at = NOW(), locked_by = $1 WHERE id = $2",
		"inline:"+uuid.NewString()[:8], jobID); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	_, _, _, _, crawlErr := CrawlSitemap(ctx, pool, p.RunID, p.BrandID, p.CollectionID, p.SitemapURL, p.MaxPages, p.UserID)
	if crawlErr == nil {
		_, _ = pool.Exec(ctx,
			"UPDATE jobs SET status = 'completed', updated_at = NOW() WHERE id = $1", jobID)
		return nil
	}

	// Parity markFailed: backoff 1 menit * 2^attempts, dead-letter setelah max.
	var attempts, maxAttempts int
	_ = pool.QueryRow(ctx, "SELECT attempts, max_attempts FROM jobs WHERE id = $1", jobID).Scan(&attempts, &maxAttempts)
	status := "failed"
	if attempts >= maxAttempts {
		status = "dead_letter"
	}
	// Parity queue.ts: backoff 1 menit * 2^attempts (menit dibulatkan).
	_, _ = pool.Exec(ctx, `
		UPDATE jobs SET status = $1, last_error = $2,
		  run_at = NOW() + make_interval(mins => LEAST($3::int, 30))
		WHERE id = $4`,
		status, truncateStr(crawlErr.Error(), 1000), attempts, jobID)
	slog.Error("[knowledge] crawl job gagal", "job", jobID, "err", crawlErr)
	return crawlErr
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// GetCrawlRun: port getCrawlRun — status run crawl untuk polling UI.
func GetCrawlRun(ctx context.Context, pool *pgxpool.Pool, runID string) (map[string]any, error) {
	var id, brandID, collectionID, sitemapURL, status string
	var maxPages int
	var totalURLs, processed *int
	var failedURLs []byte
	var errorMessage *string
	var createdAt, updatedAt any
	err := pool.QueryRow(ctx, `
		SELECT id, brand_id, collection_id, sitemap_url, max_pages, status, total_urls, processed, failed_urls, error_message, created_at, updated_at
		FROM knowledge_crawl_runs WHERE id = $1`, runID).
		Scan(&id, &brandID, &collectionID, &sitemapURL, &maxPages, &status, &totalURLs, &processed, &failedURLs, &errorMessage, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	failed := []any{}
	if len(failedURLs) > 0 {
		_ = json.Unmarshal(failedURLs, &failed)
	}
	return map[string]any{
		"run_id": id, "status": status,
		"total_urls": totalURLs, "processed": processed,
		"failed_urls": failed, "error": errorMessage,
	}, nil
}
