package partner

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// blogger.go: port lib/publish/blogger.ts + app/api/blogger/callback/route.ts —
// OAuth2 Google (PKCE S256), kredensial ter-seal per media (enc_key_ver=1 →
// v3 t3:), refresh token, dan publikasi post.

const (
	// bloggerScope: akses Blogger + drive.file. API Blogger tidak punya endpoint
	// unggah media, jadi gambar artikel dititipkan ke Google Drive mitra dan URL
	// gambar Google itulah yang dipakai di dalam post.
	bloggerScope     = "https://www.googleapis.com/auth/blogger https://www.googleapis.com/auth/drive.file"
	bloggerAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	bloggerTokenURL  = "https://oauth2.googleapis.com/token"
	bloggerAPIBase   = "https://www.googleapis.com/blogger/v3"
	bloggerUserInfo  = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
	bloggerHTTPLimit = 10 * time.Second
)

// bloggerCredential: kredensial tersimpan ter-seal (parity BloggerCredential).
type bloggerCredential struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken *string `json:"refresh_token,omitempty"`
	ExpiresAt    int64   `json:"expires_at"`
	GoogleEmail  *string `json:"google_email,omitempty"`
	Scope        *string `json:"scope,omitempty"`
	// DriveFolderID: folder Drive mitra untuk gambar artikel (diisi jalur
	// publikasi). Ikut disegel supaya tidak hilang saat kredensial di-refresh.
	DriveFolderID *string `json:"drive_folder_id,omitempty"`
}

type bloggerBlog struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// bloggerPKCEChallenge: S256 dari code_verifier (base64url tanpa padding).
func bloggerPKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// bloggerBuildAuthURL: URL consent Google (parity buildBloggerAuthUrl).
func bloggerBuildAuthURL(clientID, redirectURI, state, codeChallenge string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", bloggerScope)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("state", state)
	q.Set("include_granted_scopes", "true")
	if codeChallenge != "" {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}
	return bloggerAuthURL + "?" + q.Encode()
}

// sealBloggerCredential: seal v3 per-entity (APP_SECRET + media.id).
func (d *Deps) sealBloggerCredential(cred bloggerCredential, mediaID string) (string, error) {
	raw, err := json.Marshal(cred)
	if err != nil {
		return "", err
	}
	return d.sealV3ForEntity(string(raw), mediaID)
}

// unsealBloggerCredential: buka seal — enc_key_ver >= 1 pakai v3 per-entity,
// selain itu kunci global t2 (data lama). Parity unsealBloggerCredential.
func (d *Deps) unsealBloggerCredential(sealed, mediaID string, encKeyVer int) *bloggerCredential {
	var plain string
	var err error
	if encKeyVer >= 1 {
		plain, err = d.unsealV3ForEntity(sealed, mediaID)
	} else {
		plain, err = d.unsealT2(sealed)
	}
	if err != nil || plain == "" {
		return nil
	}
	var cred bloggerCredential
	if json.Unmarshal([]byte(plain), &cred) != nil || cred.AccessToken == "" {
		return nil
	}
	return &cred
}

// bloggerExchangeCode: tukar authorization code → token (+ email best-effort).
func bloggerExchangeCode(code, clientID, clientSecret, redirectURI, codeVerifier string) (bloggerCredential, string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	data, err := bloggerPostForm(bloggerTokenURL, form)
	if err != nil {
		return bloggerCredential{}, "", err
	}
	accessToken, _ := data["access_token"].(string)
	if accessToken == "" {
		msg, _ := data["error_description"].(string)
		if msg == "" {
			if e, ok := data["error"].(string); ok {
				msg = e
			} else {
				msg = "Token exchange gagal"
			}
		}
		return bloggerCredential{}, "", fmt.Errorf("%s", msg)
	}

	cred := bloggerCredential{AccessToken: accessToken}
	if rt, ok := data["refresh_token"].(string); ok && rt != "" {
		cred.RefreshToken = &rt
	}
	expiresIn := 3600.0
	if v, ok := data["expires_in"].(float64); ok && v > 0 {
		expiresIn = v
	}
	cred.ExpiresAt = time.Now().UnixMilli() + int64(expiresIn)*1000
	if sc, ok := data["scope"].(string); ok && sc != "" {
		cred.Scope = &sc
	}

	email := ""
	if resp, uiErr := bloggerGet(bloggerUserInfo, accessToken); uiErr == nil {
		var uj struct {
			Email string `json:"email"`
		}
		if json.Unmarshal(resp, &uj) == nil && uj.Email != "" {
			email = uj.Email
			cred.GoogleEmail = &email
		}
	}
	return cred, email, nil
}

// bloggerRefreshToken: refresh access token (parity refreshBloggerToken).
func bloggerRefreshToken(refreshToken, clientID, clientSecret string) (string, int, error) {
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	data, err := bloggerPostForm(bloggerTokenURL, form)
	if err != nil {
		return "", 0, err
	}
	accessToken, _ := data["access_token"].(string)
	if accessToken == "" {
		msg, _ := data["error_description"].(string)
		if msg == "" {
			if e, ok := data["error"].(string); ok {
				msg = e
			} else {
				msg = "Refresh gagal"
			}
		}
		return "", 0, fmt.Errorf("%s", msg)
	}
	expiresIn := 3600
	if v, ok := data["expires_in"].(float64); ok && v > 0 {
		expiresIn = int(v)
	}
	return accessToken, expiresIn, nil
}

// bloggerListBlogs: daftar blog akun Google terhubung (parity listBloggerBlogs).
func bloggerListBlogs(accessToken string) ([]bloggerBlog, error) {
	raw, err := bloggerGet(bloggerAPIBase+"/users/self/blogs", accessToken)
	if err != nil {
		return nil, err
	}
	var data struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"items"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return nil, fmt.Errorf("respons Blogger tidak valid")
	}
	if data.Error != nil && data.Error.Message != "" {
		return nil, fmt.Errorf("%s", data.Error.Message)
	}
	out := make([]bloggerBlog, 0, len(data.Items))
	for _, b := range data.Items {
		out = append(out, bloggerBlog{ID: b.ID, Name: b.Name, URL: b.URL})
	}
	return out, nil
}

// bloggerResolveTarget: kredensial media, refresh bila kedaluwarsa; hasil
// refresh disimpan ulang sebagai v3 (enc_key_ver=1). Parity resolveBloggerTarget.
func (d *Deps) bloggerResolveTarget(r *http.Request, media rowMap, clientID, clientSecret string) *bloggerCredential {
	sealed := strings.TrimSpace(media.strOr("blogger_credentials_sealed", ""))
	if sealed == "" {
		return nil
	}
	mediaID := media.strOr("id", "")
	cred := d.unsealBloggerCredential(sealed, mediaID, media.intOr("enc_key_ver", 0))
	if cred == nil {
		return nil
	}
	if cred.ExpiresAt <= time.Now().UnixMilli()+60_000 {
		if cred.RefreshToken == nil || *cred.RefreshToken == "" {
			return nil
		}
		accessToken, expiresIn, err := bloggerRefreshToken(*cred.RefreshToken, clientID, clientSecret)
		if err != nil {
			return nil
		}
		cred.AccessToken = accessToken
		cred.ExpiresAt = time.Now().UnixMilli() + int64(expiresIn)*1000
		if resealed, sErr := d.sealBloggerCredential(*cred, mediaID); sErr == nil {
			_ = d.exec(r.Context(),
				"UPDATE media SET blogger_credentials_sealed = $1, enc_key_ver = 1, updated_at = NOW() WHERE id = $2",
				resealed, mediaID)
		}
	}
	return cred
}

// bloggerPublishPost: buat post Blogger; isDraft=false → pastikan LIVE
// (posts.publish bila perlu). Parity publishBloggerPost.
type bloggerPostResult struct {
	ID        string
	URL       string
	Published bool
}

func bloggerPublishPost(accessToken, blogID, title, contentHTML string, labels []string, isDraft bool) (bloggerPostResult, error) {
	body := map[string]any{
		"kind":    "blogger#post",
		"title":   title,
		"content": contentHTML,
		"isDraft": isDraft,
	}
	if len(labels) > 0 {
		body["labels"] = labels
	}
	raw, _ := json.Marshal(body)
	req, reqErr := http.NewRequest(http.MethodPost, bloggerAPIBase+"/blogs/"+url.PathEscape(blogID)+"/posts", strings.NewReader(string(raw)))
	if reqErr != nil {
		return bloggerPostResult{}, reqErr
	}
	req.Header.Set("authorization", "Bearer "+accessToken)
	req.Header.Set("content-type", "application/json")
	resp, err := bloggerDo(req)
	if err != nil {
		return bloggerPostResult{}, err
	}
	var data struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(resp.Body, &data)
	if resp.Status >= 400 {
		if data.Error != nil && data.Error.Message != "" {
			return bloggerPostResult{}, fmt.Errorf("%s", data.Error.Message)
		}
		switch resp.Status {
		case http.StatusNotFound:
			return bloggerPostResult{}, fmt.Errorf("Blog tidak ditemukan atau tidak dapat diakses.")
		case http.StatusUnauthorized:
			return bloggerPostResult{}, fmt.Errorf("Kredensial Blogger tidak valid — perlu hubungkan ulang.")
		case http.StatusForbidden:
			return bloggerPostResult{}, fmt.Errorf("Akses ditolak Blogger — periksa izin akun.")
		}
		return bloggerPostResult{}, fmt.Errorf("Blogger menolak request (%d)", resp.Status)
	}
	result := bloggerPostResult{ID: data.ID, URL: data.URL, Published: !isDraft}
	if isDraft || data.Status == "LIVE" {
		return result, nil
	}

	// Masih draft padahal minta publikasi → publish eksplisit.
	preq, pErr := http.NewRequest(http.MethodPost,
		bloggerAPIBase+"/blogs/"+url.PathEscape(blogID)+"/posts/"+url.PathEscape(data.ID)+"/publish", nil)
	if pErr != nil {
		return result, pErr
	}
	preq.Header.Set("authorization", "Bearer "+accessToken)
	presp, err := bloggerDo(preq)
	if err != nil {
		return result, err
	}
	var pdata struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(presp.Body, &pdata)
	if presp.Status >= 400 {
		if pdata.Error != nil && pdata.Error.Message != "" {
			return result, fmt.Errorf("%s", pdata.Error.Message)
		}
		return result, fmt.Errorf("Gagal mempublikasikan post (%d)", presp.Status)
	}
	if pdata.ID != "" {
		result.ID = pdata.ID
	}
	if pdata.URL != "" {
		result.URL = pdata.URL
	}
	result.Published = true
	return result, nil
}

// ---------- HTTP kecil ----------

type bloggerHTTPResult struct {
	Status int
	Body   []byte
}

func bloggerDo(req *http.Request) (bloggerHTTPResult, error) {
	// Klien dengan guard SSRF (private/loopback/metadata + batas redirect).
	client := security.SafeClient(bloggerHTTPLimit)
	resp, err := client.Do(req)
	if err != nil {
		return bloggerHTTPResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return bloggerHTTPResult{Status: resp.StatusCode, Body: body}, nil
}

func bloggerGet(rawURL, accessToken string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+accessToken)
	res, err := bloggerDo(req)
	if err != nil {
		return nil, err
	}
	if res.Status >= 400 {
		return nil, fmt.Errorf("permintaan Blogger gagal (%d)", res.Status)
	}
	return res.Body, nil
}

func bloggerPostForm(rawURL string, form url.Values) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	res, err := bloggerDo(req)
	if err != nil {
		return nil, err
	}
	data := map[string]any{}
	_ = json.Unmarshal(res.Body, &data)
	if res.Status >= 400 {
		msg, _ := data["error_description"].(string)
		if msg == "" {
			if e, ok := data["error"].(string); ok {
				msg = e
			} else {
				msg = fmt.Sprintf("OAuth gagal (%d)", res.Status)
			}
		}
		return data, fmt.Errorf("%s", msg)
	}
	return data, nil
}

// bloggerRandomToken: state / code_verifier acak base64url.
func bloggerRandomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// Fallback deterministik-jelas: UUID (tidak akan dipakai normal).
		return strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// bloggerRedirectURI: callback khusus Blogger (redirect URI terdaftar di
// Google Console). Parity `${appBase}/api/blogger/callback` — TIDAK memakai
// redirect_uri GSC karena endpoint callback-nya berbeda.
func (d *Deps) bloggerRedirectURI(r *http.Request) string {
	// Prioritas: base publik dari konfigurasi server, lalu origin request.
	base := strings.TrimRight(strings.TrimSpace(d.Cfg.PublicAppURL), "/")
	if base == "" {
		base = strings.TrimRight(appBaseFromRequest(r), "/")
	}
	if base == "" {
		return "/api/blogger/callback"
	}
	return base + "/api/blogger/callback"
}

// appBaseFromRequest: origin publik dari request (proxy-aware).
func appBaseFromRequest(r *http.Request) string {
	scheme := "https"
	if proto := strings.TrimSpace(r.Header.Get("x-forwarded-proto")); proto != "" {
		scheme = strings.Split(proto, ",")[0]
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := strings.TrimSpace(r.Header.Get("x-forwarded-host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

// ---------- callback OAuth publik ----------

// BloggerCallback: GET /api/blogger/callback — validasi state → tukar code →
// simpan kredensial ter-seal → redirect ke picker atau mitra. Parity
// app/api/blogger/callback/route.ts. PUBLIK (dipanggil Google, tanpa sesi).
func (d *Deps) BloggerCallback(w http.ResponseWriter, r *http.Request) {
	appBase := strings.TrimRight(strings.TrimSpace(d.Cfg.PublicAppURL), "/")
	if appBase == "" {
		appBase = strings.TrimRight(appBaseFromRequest(r), "/")
	}
	q := r.URL.Query()

	redirectErr := func(msg string) {
		http.Redirect(w, r, appBase+"/mitra?blogger_error="+url.QueryEscape(msg), http.StatusFound)
	}

	if errParam := q.Get("error"); errParam != "" {
		msg := "OAuth dibatalkan: " + q.Get("error_description")
		if errParam == "access_denied" {
			msg = "Akses ditolak — Anda membatalkan consent Google."
		}
		redirectErr(msg)
		return
	}
	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		redirectErr("Callback tidak valid (code/state hilang).")
		return
	}

	oauth, err := d.queryRow(r.Context(),
		"SELECT partner_id, media_id, code_verifier FROM blogger_oauth_states WHERE state = $1 LIMIT 1", state)
	if err != nil || oauth == nil {
		redirectErr("State OAuth tidak valid atau kedaluwarsa. Silakan coba hubungkan ulang.")
		return
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(r)
	if cfgErr != nil || cfg.ClientID == "" || cfg.ClientSecret == "" {
		redirectErr("Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub.")
		return
	}
	mediaID := oauth.strOr("media_id", "")
	partnerID := oauth.strOr("partner_id", "")
	redirectURI := appBase + "/api/blogger/callback"

	cred, _, exErr := bloggerExchangeCode(code, cfg.ClientID, cfg.ClientSecret, redirectURI, oauth.strOr("code_verifier", ""))
	if exErr != nil {
		_ = d.exec(r.Context(), "DELETE FROM blogger_oauth_states WHERE state = $1", state)
		redirectErr(exErr.Error())
		return
	}
	if cred.RefreshToken == nil || *cred.RefreshToken == "" {
		redirectErr("Google tidak memberikan refresh token. Coba hubungkan ulang.")
		return
	}

	sealed, sErr := d.sealBloggerCredential(cred, mediaID)
	if sErr != nil {
		redirectErr("Gagal menyimpan kredensial Blogger.")
		return
	}

	// Pilihan blog diambil SEKARANG (bukan setelah redirect), parity legacy.
	blogs, bErr := bloggerListBlogs(cred.AccessToken)
	if bErr == nil && len(blogs) == 1 {
		if err := d.exec(r.Context(),
			"UPDATE media SET blogger_credentials_sealed = $1, blogger_blog_id = $2, blogger_blog_name = $3, blogger_status = 'connected', blogger_last_test_at = NOW(), blogger_last_test_message = 'Terhubung', enc_key_ver = 1, updated_at = NOW() WHERE id = $4",
			sealed, blogs[0].ID, blogs[0].Name, mediaID); err != nil {
			redirectErr("Gagal menyimpan kredensial Blogger.")
			return
		}
		_ = d.exec(r.Context(), "DELETE FROM blogger_oauth_states WHERE state = $1", state)
		publicID := partnerID
		if prow, _ := d.queryRow(r.Context(), "SELECT public_id FROM partners WHERE id = $1 LIMIT 1", partnerID); prow != nil {
			publicID = prow.strOr("public_id", partnerID)
		}
		http.Redirect(w, r, appBase+"/mitra/"+publicID+"/integrations#blogger?blogger_connected=1&media="+url.QueryEscape(mediaID), http.StatusFound)
		return
	}

	if err := d.exec(r.Context(),
		"UPDATE media SET blogger_credentials_sealed = $1, blogger_status = 'connected', blogger_last_test_at = NOW(), blogger_last_test_message = 'Kredensial tersimpan. Pilih blog untuk melanjutkan.', enc_key_ver = 1, updated_at = NOW() WHERE id = $2",
		sealed, mediaID); err != nil {
		redirectErr("Gagal menyimpan kredensial Blogger.")
		return
	}
	_ = d.exec(r.Context(), "DELETE FROM blogger_oauth_states WHERE state = $1", state)
	http.Redirect(w, r, appBase+"/blogger-pick-blog?media="+url.QueryEscape(mediaID), http.StatusFound)
}
