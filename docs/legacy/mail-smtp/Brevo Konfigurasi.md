# **MASTER INSTRUCTION**

# **TAUTHUB TRANSACTIONAL EMAIL & SMTP NOTIFICATION SYSTEM**

## **TUJUAN**

Integrasikan **Brevo SMTP** ke Tauthub sebagai provider transactional email untuk kebutuhan email yang benar-benar penting bagi user.

Namun jangan mengubah seluruh sistem notification Tauthub menjadi email-based notification.

Tauthub harus memiliki strategi notification yang membedakan:

IN-APP NOTIFICATION  
        vs  
EMAIL NOTIFICATION  
        vs  
NO NOTIFICATION

Tujuan utamanya adalah:

1. Menggunakan Brevo SMTP untuk email transactional yang penting.  
2. Menghindari pengiriman email untuk setiap event kecil.  
3. Mengurangi email yang tidak penting/spammy.  
4. Mempertahankan notification center/in-app notification sebagai media utama untuk event yang sering.  
5. Memastikan email hanya dikirim untuk event yang benar-benar membutuhkan perhatian user.  
6. Membuat sistem email reusable sehingga event baru dapat ditambahkan tanpa membuat logic email berantakan.

---

# **1\. PROVIDER SMTP**

Gunakan:

SMTP Provider:  
Brevo

SMTP Host:  
smtp-relay.brevo.com

SMTP Port:  
587

Credential harus dibaca dari environment variable.

Contoh:

SMTP\_HOST=smtp-relay.brevo.com  
SMTP\_PORT=587  
SMTP\_USER=...  
SMTP\_PASSWORD=...  
SMTP\_FROM\_EMAIL=...  
SMTP\_FROM\_NAME=Tauthub

**JANGAN hardcode credential di source code.**

SMTP password/key tidak boleh:

* masuk Git.  
* masuk frontend.  
* masuk response API.  
* masuk log.  
* masuk database plaintext jika tidak diperlukan.

---

# **2\. SMTP KEY**

Gunakan **Brevo SMTP Key**, bukan API key, karena integrasi yang diminta adalah SMTP.

Credential berasal dari Brevo:

SMTP Server  
Port  
Login  
SMTP Key

Gunakan port:

587

dengan konfigurasi TLS/STARTTLS yang sesuai library email yang digunakan.

Jangan membuat frontend mengetahui credential SMTP.

---

# **3\. FIRST TASK — AUDIT EXISTING SYSTEM**

SEBELUM CODING:

Audit seluruh sistem notification dan email Tauthub.

Cari:

notification  
email  
mailer  
smtp  
sendEmail  
sendMail  
nodemailer  
resend  
transactional  
verification  
password reset  
notification service

Kemudian cari semua event penting di backend:

user registration  
email verification  
password reset  
Google authentication  
campaign  
content  
media  
partner  
referral  
commission  
wallet  
withdrawal  
payment  
order  
curation  
admin

Jangan langsung membuat sistem email baru sebelum memahami existing implementation.

---

# **4\. IDENTIFIKASI EXISTING NOTIFICATION SYSTEM**

Cari apakah Tauthub sudah memiliki:

Notification model  
Notification service  
Notification center  
Unread count  
Notification preferences  
Event bus  
Background job  
Queue  
Email service

Jika sudah ada:

**REUSE existing system.**

Jangan membuat:

EmailNotificationService2

atau sistem notification kedua hanya untuk Brevo.

---

# **5\. BEDAKAN NOTIFICATION CHANNEL**

Buat konsep channel:

IN\_APP  
EMAIL

Jika architecture existing sudah memiliki channel, gunakan.

Jika belum ada, tambahkan secara minimal.

Contoh:

Notification  
├── type  
├── recipient  
├── channel  
├── title  
├── message  
├── metadata  
└── created\_at

Channel dapat berupa:

in\_app  
email

atau jika architecture lebih baik:

channels \= \["in\_app", "email"\]

---

# **6\. JANGAN KIRIM SEMUA NOTIFICATION KE EMAIL**

Ini requirement penting.

Audit semua notification/event yang ada.

Kemudian klasifikasikan menjadi:

### **A. Email wajib / sangat disarankan**

Event yang membutuhkan tindakan user atau berkaitan dengan keamanan/uang.

Contoh:

Account verification  
Password reset  
Security/login alert  
Withdrawal status  
Payment status  
Important transaction status  
Account suspension  
Important moderation result

### **B. In-app \+ optional email**

Event yang penting tetapi tidak harus selalu masuk email.

Contoh:

Media approved  
Media rejected  
Campaign requires action  
Content review result  
Important partner update  
Referral milestone

### **C. In-app only**

Event yang terlalu sering atau tidak cukup penting untuk email.

Contoh:

Minor status changes  
Routine internal updates  
Small progress events  
Frequent AI processing updates  
Background processing updates  
Unread notification

### **D. Jangan kirim notification**

Event internal yang tidak memiliki value untuk user.

Contoh:

internal worker completed  
cache updated  
background synchronization  
internal retry  
technical logs

---

# **7\. JANGAN MEMBUAT KEPUTUSAN SECARA SEMBARANGAN**

Agent harus terlebih dahulu melakukan audit dan membuat rekomendasi.

Output yang saya inginkan:

| Event | Existing Notification | Email? | Priority | Alasan |
| ----- | ----- | ----- | ----- | ----- |
| Password reset | Yes | YES | Critical | Security |
| Media approved | Yes | MAYBE | Medium | User action/value |
| AI generation completed | Yes | NO | Low | Terlalu sering |
| Withdrawal approved | Yes | YES | Critical | Financial |
| Background sync | No | NO | None | Internal |

**Tabel tersebut hanya contoh format.**

Isi berdasarkan event nyata yang ditemukan di codebase.

Jangan mengarang event yang tidak ada.

---

# **8\. EMAIL PRIORITY**

Gunakan klasifikasi:

CRITICAL  
IMPORTANT  
NORMAL  
IN\_APP\_ONLY  
NONE

### **CRITICAL**

Email harus dikirim.

Contoh:

Password reset  
Email verification  
Security alert  
Withdrawal status  
Payment failure/status

### **IMPORTANT**

Email sangat disarankan, tetapi dapat dikonfigurasi.

Contoh:

Campaign needs action  
Media rejected  
Media approved  
Important account change

### **NORMAL**

Default-nya in-app.

Contoh:

Routine campaign update  
Content status  
Referral update

### **IN\_APP\_ONLY**

Tidak dikirim email.

### **NONE**

Tidak perlu user notification.

---

# **9\. USER NOTIFICATION PREFERENCES**

Jika Tauthub sudah memiliki notification preference system:

REUSE.

Jika belum ada dan memang diperlukan, buat minimal preference system.

Contoh:

Email Notifications

☑ Security & Account  
☑ Payments & Withdrawals  
☑ Campaign Updates  
☐ Marketing

Namun:

**Security email seperti password reset tidak boleh dimatikan hanya karena user mematikan notification preference.**

Transactional security email tetap dikirim.

---

# **10\. EMAIL CATEGORY**

Bedakan:

### **Transactional**

Email yang berkaitan dengan fungsi aplikasi.

Contoh:

Verification  
Password Reset  
Payment  
Withdrawal  
Campaign  
Media  
Referral  
Commission

### **Marketing**

Jangan otomatis memasukkan marketing email dalam SMTP transactional system ini.

Marketing memiliki consent/unsubscribe requirements yang berbeda.

Untuk tahap ini:

**Fokus hanya transactional email.**

---

# **11\. EMAIL TEMPLATE SYSTEM**

Jangan membuat HTML email langsung di setiap service.

Jangan:

sendEmail({  
  html: "\<html\>..."  
})

berulang-ulang di banyak file.

Buat reusable email template system.

Konsep:

emails/  
├── layouts/  
│   └── TauthubLayout  
├── templates/  
│   ├── VerifyEmail  
│   ├── PasswordReset  
│   ├── WithdrawalStatus  
│   ├── MediaApproved  
│   └── ...  
└── emailService

Gunakan convention project yang sudah ada.

---

# **12\. EMAIL BRANDING**

Semua email Tauthub harus menggunakan branding yang konsisten:

Tauthub

Gunakan:

* logo jika tersedia.  
* typography yang aman untuk email.  
* warna brand Tauthub.  
* footer sederhana.  
* support/contact link jika tersedia.  
* link ke aplikasi Tauthub.

Jangan membuat design email yang terlalu kompleks.

Prioritaskan:

simple  
clean  
responsive  
fast  
readable

---

# **13\. EMAIL SUBJECT**

Subject harus singkat dan informatif.

Contoh:

Verifikasi email Tauthub Anda  
Reset password Tauthub  
Withdrawal Anda telah diproses  
Media Anda telah disetujui  
Campaign membutuhkan tindakan Anda

Jangan menggunakan subject clickbait.

---

# **14\. EMAIL CONTENT**

Email harus menjawab:

Apa yang terjadi?  
Apa dampaknya?  
Apa yang harus dilakukan?

Contoh:

Media Anda telah disetujui

Domain:  
example.com

Status:  
Approved

Anda sekarang dapat menggunakan media tersebut  
untuk campaign Tauthub.

\[ Lihat Media \]

Jangan mengirim seluruh detail internal database.

---

# **15\. EMAIL LINK**

Setiap email action-oriented sebaiknya memiliki CTA:

\[ Lihat Campaign \]  
\[ Lihat Media \]  
\[ Lihat Withdrawal \]  
\[ Verifikasi Email \]  
\[ Reset Password \]

Gunakan absolute URL dari environment:

APP\_URL=https://tauthub.com

Jangan hardcode production URL jika project memiliki environment configuration.

---

# **16\. EMAIL SERVICE**

Buat satu abstraction:

sendTransactionalEmail()

Contoh konseptual:

sendTransactionalEmail({  
  template: "media-approved",  
  recipient: user.email,  
  data: {...}  
})

Service tersebut bertanggung jawab atas:

* template rendering.  
* sender.  
* SMTP.  
* error handling.  
* logging.  
* retry jika diperlukan.

Business logic jangan mengetahui detail SMTP.

---

# **17\. BUSINESS LOGIC JANGAN TERGANTUNG SMTP**

Contoh:

Media Approved  
      ↓  
Database status updated  
      ↓  
Notification created  
      ↓  
Email queued

Bukan:

Media Approved  
      ↓  
send SMTP  
      ↓  
kalau gagal  
      ↓  
database rollback

SMTP failure tidak boleh menyebabkan business transaction gagal.

---

# **18\. ASYNC EMAIL**

Jika infrastructure queue/background job tersedia:

**Gunakan queue.**

Contoh:

Media Approved  
      ↓  
Create Notification  
      ↓  
Queue Email Job  
      ↓  
Worker  
      ↓  
Brevo SMTP

Dengan begitu user tidak perlu menunggu SMTP selesai ketika melakukan action di aplikasi.

---

# **19\. RETRY**

Jika SMTP gagal karena temporary failure:

timeout  
connection failure  
temporary SMTP error

gunakan retry dengan batas wajar.

Jangan retry tanpa batas.

Contoh:

Attempt 1  
↓  
Attempt 2  
↓  
Attempt 3  
↓  
Failed

Setelah gagal, simpan status email.

---

# **20\. EMAIL DELIVERY LOG**

Jika memungkinkan, simpan internal email delivery record:

email\_delivery  
├── recipient  
├── template  
├── status  
├── provider  
├── provider\_message\_id  
├── error  
└── created\_at

Status:

queued  
sending  
sent  
failed

Jika webhook Brevo nanti diintegrasikan, status dapat diperluas:

delivered  
bounced  
opened  
clicked

---

# **21\. JANGAN MENYIMPAN EMAIL CONTENT BERLEBIHAN**

Tidak perlu menyimpan seluruh HTML email untuk setiap pengiriman jika tidak diperlukan.

Simpan metadata secukupnya:

template  
recipient  
provider  
message\_id  
status  
error  
timestamp

---

# **22\. BREVO WEBHOOK — TAHAP BERIKUTNYA**

Untuk fase pertama:

**SMTP saja sudah cukup.**

Jangan wajibkan webhook jika belum diperlukan.

Namun architecture harus memungkinkan nanti:

Brevo  
 ↓  
Webhook  
 ↓  
Tauthub

untuk event:

delivered  
bounced  
opened  
clicked  
spam  
unsubscribed

Jika webhook diimplementasikan, pastikan endpoint aman dan memvalidasi payload sesuai mekanisme Brevo.

---

# **23\. EMAIL RATE LIMITING**

Jangan biarkan bug di Tauthub mengirim:

1000 email

karena satu event ter-trigger berulang.

Tambahkan protection:

* idempotency.  
* deduplication.  
* rate limiting.  
* queue.

Contoh:

Password reset  
→ satu request  
→ satu email

Repeated request boleh mengikuti security policy existing, tetapi jangan membuat infinite send loop.

---

# **24\. DUPLICATE EMAIL PROTECTION**

Contoh:

Media Approved

Event ter-trigger dua kali.

Jangan mengirim:

Email \#1  
Email \#2

jika event sebenarnya sama.

Gunakan event/reference ID untuk deduplication.

---

# **25\. ADMIN EMAIL**

Jangan otomatis mengirim semua admin event ke email.

Admin biasanya lebih cocok menggunakan:

Admin Notification Center

Email hanya untuk event yang benar-benar membutuhkan perhatian.

Contoh kandidat:

Critical payment failure  
Withdrawal requiring manual action  
System-level incident

Tetapi keputusan final harus berdasarkan event yang ditemukan di codebase.

---

# **26\. PARTNER / VIP EMAIL**

Untuk fitur Partner VIP yang sedang dibangun, kandidat email:

### **Sangat layak**

Referral domain approved  
Referral domain rejected  
CPA credited  
Transaction commission credited  
Withdrawal status

### **Mungkin cukup in-app**

Referral click  
Routine referral update  
Dashboard statistic update

Jangan mengirim email setiap kali:

1 click  
2 clicks  
3 clicks  
...

Itu akan menjadi spam.

---

# **27\. MEDIA EMAIL**

Untuk media:

### **Layak email**

Media approved  
Media rejected  
Media requires action

### **Tidak perlu email**

Media viewed  
Media listed  
Media search

---

# **28\. CAMPAIGN EMAIL**

Kandidat:

Campaign created

Biasanya:

IN-APP

Kandidat email:

Campaign membutuhkan tindakan user  
Campaign mengalami masalah  
Campaign selesai  
Campaign payment issue

Jangan email setiap perubahan minor.

---

# **29\. AI EMAIL**

AI biasanya sangat frequent.

Jangan mengirim email untuk:

AI started  
AI processing  
AI generated section  
AI token usage  
AI step completed

Email hanya jika benar-benar bernilai:

AI job membutuhkan user action  
Long-running generation selesai

dan hanya jika use case tersebut memang ada.

---

# **30\. AUTHENTICATION EMAIL**

Prioritas tinggi:

Email verification  
Password reset  
Security notification

Jika Google OAuth digunakan, jangan mengirim email setiap kali login Google normal.

Email hanya untuk security-sensitive event jika memang diperlukan.

---

# **31\. PAYMENT / WALLET EMAIL**

Prioritas tinggi karena menyangkut uang.

Kandidat:

Payment successful  
Payment failed  
Withdrawal submitted  
Withdrawal approved  
Withdrawal rejected  
Commission credited  
Refund/reversal

Tetapi jangan mengirim email untuk setiap perubahan internal ledger jika user tidak perlu mengetahuinya.

---

# **32\. EMAIL DIGEST**

Jika nantinya notification terlalu banyak, jangan langsung mengirim satu email per event.

Gunakan digest.

Contoh:

Daily Partner Summary

Hari ini:  
3 domain approved  
2 domain pending  
Rp45.000 CPA earned  
Rp100.000 transaction commission

Digest dapat ditambahkan kemudian.

Jangan implementasikan jika belum dibutuhkan.

---

# **33\. NOTIFICATION CENTER TETAP MENJADI SUMBER UTAMA**

Tauthub harus tetap memiliki in-app notification.

Email adalah **secondary channel** untuk event penting.

Model:

Event  
 ↓  
Notification Engine  
 ├── In-App  
 └── Email (jika eligible)

Bukan:

Event  
 ↓  
Email  
 ↓  
baru notification

---

# **34\. EVENT → NOTIFICATION MATRIX**

Sebelum implementasi final, buat matrix:

Event  
↓  
Audience  
↓  
In-App?  
↓  
Email?  
↓  
Priority  
↓  
Template

Contoh:

| Event | Audience | In-App | Email | Priority |
| ----- | ----- | ----- | ----- | ----- |
| Email Verification | User | ✓ | ✓ | Critical |
| Password Reset | User | \- | ✓ | Critical |
| Media Approved | Partner/Client | ✓ | ✓ | Important |
| Media Rejected | Partner/Client | ✓ | ✓ | Important |
| Referral Click | Partner | ✓ | ✗ | Normal |
| CPA Credited | VIP | ✓ | ✓ | Important |
| AI Processing | User | ✓ | ✗ | In-app |
| Withdrawal Approved | User | ✓ | ✓ | Critical |
| Background Sync | Internal | ✗ | ✗ | None |

**Jangan anggap tabel ini final.**

Agent harus membuat matrix berdasarkan event nyata yang ditemukan di codebase.

---

# **35\. ENVIRONMENT CONFIGURATION**

Gunakan environment variables:

SMTP\_HOST=smtp-relay.brevo.com  
SMTP\_PORT=587  
SMTP\_USER=  
SMTP\_PASSWORD=

SMTP\_FROM\_EMAIL=no-reply@tauthub.com  
SMTP\_FROM\_NAME=Tauthub

APP\_URL=https://tauthub.com

Untuk development:

SMTP\_ENABLED=false

atau gunakan test SMTP/provider sesuai environment.

Jangan sampai development tanpa sengaja mengirim email production.

---

# **36\. DEVELOPMENT MODE**

Buat mekanisme aman:

EMAIL\_MODE=development

atau equivalent.

Development dapat:

* log email metadata.  
* preview email.  
* redirect email ke test address.  
* menggunakan sandbox/test provider.

Jangan mengirim email ke user production dari local development tanpa sengaja.

---

# **37\. DOMAIN AUTHENTICATION**

Sebelum production:

Pastikan domain pengirim Tauthub sudah dikonfigurasi di Brevo dengan:

SPF  
DKIM

dan jika infrastructure/domain policy memungkinkan:

DMARC

Gunakan sender:

no-reply@tauthub.com

atau alamat transactional yang sesuai.

---

# **38\. TEST EMAIL**

Buat test:

SMTP connection  
Send verification  
Send reset password  
Send notification  
SMTP failure  
Retry  
Duplicate prevention  
Queue

Pastikan email:

* terkirim.  
* HTML valid.  
* mobile-friendly.  
* CTA berfungsi.  
* sender benar.  
* tidak mengandung credential.  
* tidak bocor data internal.

---

# **39\. SECURITY**

Jangan pernah memasukkan ke email:

* password.  
* SMTP credential.  
* API key.  
* internal token.  
* private data.  
* database ID yang tidak perlu.  
* sensitive admin information.

Untuk reset password gunakan secure one-time token dengan expiration sesuai auth system existing.

---

# **40\. PERFORMANCE**

Email tidak boleh memperlambat request utama.

Ideal:

POST /media/approve  
       ↓  
Database transaction  
       ↓  
Commit  
       ↓  
Queue notification  
       ↓  
HTTP response

Bukan:

POST /media/approve  
       ↓  
SMTP connection  
       ↓  
send email  
       ↓  
wait  
       ↓  
HTTP response

---

# **41\. LOGGING**

Log:

email queued  
email sent  
email failed

Jangan log:

SMTP password  
SMTP key  
full sensitive email body  
reset token

---

# **42\. MONITORING**

Siapkan kemampuan untuk mengetahui:

Total emails sent  
Failed emails  
Queued emails  
Provider errors  
Template failures

Jika Brevo webhook belum digunakan, minimal internal delivery status harus tersedia.

---

# **43\. IMPLEMENTATION PHASE**

## **Phase 1 — Audit**

Temukan:

Existing email service  
Existing notification  
Existing events  
Existing queue  
Existing auth  
Existing templates

Kemudian hasilkan:

Notification/Event Matrix

---

## **Phase 2 — SMTP Integration**

Implement:

Brevo SMTP  
Environment config  
SMTP connection  
Email service

---

## **Phase 3 — Template System**

Implement:

Base Tauthub email layout  
Transactional templates

---

## **Phase 4 — Notification Routing**

Implement:

Event  
 ↓  
Notification  
 ↓  
In-app  
 ↓  
Email if eligible

---

## **Phase 5 — Queue**

Jika existing queue tersedia:

Email Job  
 ↓  
Worker  
 ↓  
Brevo

---

## **Phase 6 — Delivery Tracking**

Implement:

queued  
sent  
failed

Kemudian webhook Brevo dapat ditambahkan sebagai enhancement.

---

# **44\. ACCEPTANCE CRITERIA**

Fitur dianggap berhasil jika:

### **SMTP**

* Tauthub dapat mengirim email melalui Brevo.  
* SMTP credential berasal dari environment.  
* Tidak ada credential di frontend/source code.  
* Port 587 dapat digunakan.

### **Notification**

* Existing in-app notification tetap berjalan.  
* Tidak semua event dikirim email.  
* Event email ditentukan berdasarkan matrix.  
* Security/financial email memiliki prioritas tinggi.  
* Frequent low-value events tetap in-app.

### **Architecture**

* Business logic tidak tergantung langsung pada Brevo.  
* Ada abstraction email service.  
* Email dapat diganti provider di masa depan tanpa mengubah business logic.

### **Reliability**

* Email dapat di-queue jika queue tersedia.  
* Ada retry untuk temporary failure.  
* Tidak ada duplicate email akibat duplicate event.

### **Security**

* SMTP key aman.  
* Reset token tidak masuk log/email secara tidak aman.  
* Email tidak membocorkan data internal.

---

# **45\. OUTPUT WAJIB SEBELUM CODING**

Sebelum melakukan perubahan besar, tampilkan kepada saya:

## **A. Existing Notification Audit**

Event:  
Existing notification:  
Current channel:  
Recommended channel:  
Reason:

## **B. Email Recommendation**

Buat tabel:

Event | Email? | Priority | Template | Reason

## **C. Existing Email Architecture**

Mailer:  
...

Notification service:  
...

Queue:  
...

Templates:  
...

## **D. Required Changes**

Backend:  
...

Frontend:  
...

Database:  
...

Environment:  
...

## **E. Risk**

Potential issue:  
...

Mitigation:  
...

---

# **46\. FINAL PRINCIPLE**

Jangan berpikir:

> "Semua notification Tauthub harus dikirim lewat email."

Yang benar:

> **Tauthub memiliki Notification Engine dengan beberapa channel. Email adalah salah satu channel untuk event yang memang layak dikirim melalui email.**

Arsitektur yang diinginkan:

                    TAUTHUB EVENT  
                           │  
                           ↓  
                  Notification Engine  
                           │  
              ┌────────────┴────────────┐  
              ↓                         ↓  
          IN-APP                      EMAIL  
              │                         │  
        Notification Center         Email Service  
                                        │  
                                        ↓  
                                  Brevo SMTP  
                                        │  
                                        ↓  
                                      User

Prioritaskan:

Security  
Financial  
Account-critical  
Action-required  
Important status

Hindari:

Frequent  
Low-value  
Background  
Internal  
No-action-needed

**Jangan mengirim email hanya karena secara teknis bisa.**

Setiap email harus punya alasan yang jelas:

> **"Apakah user benar-benar membutuhkan informasi ini di luar aplikasi?"**

Jika jawabannya tidak, cukup gunakan in-app notification.

---

# **47\. IMPORTANT**

Saya ingin agent **tidak langsung mengimplementasikan semua kemungkinan email**.

Urutannya harus:

1\. Audit existing event & notification  
2\. Buat recommendation matrix  
3\. Tunjukkan kepada saya  
4\. Setelah disetujui, implementasikan SMTP  
5\. Implementasikan email yang prioritas  
6\. Test  
7\. Baru pertimbangkan webhook Brevo

Jangan melakukan perubahan production atau migration besar sebelum audit selesai.

**Target utama:** sistem email Tauthub yang reliable, hemat quota, tidak spammy, reusable, dan mudah diganti provider di masa depan.

# **48\. ADMIN SMTP CONFIGURATION**

Selain konfigurasi environment variable, Tauthub harus menyediakan **SMTP Configuration di Admin Dashboard**.

Tujuannya agar administrator dapat mengatur provider email transactional dari dalam dashboard tanpa harus mengubah source code.

Lokasi yang direkomendasikan:

Admin Dashboard

→ Pengaturan

→ Email / SMTP

Jika Tauthub sudah memiliki halaman:

Admin

→ Settings

gunakan struktur settings yang sudah ada dan tambahkan section:

Email & SMTP

Jangan membuat halaman settings terpisah jika existing Settings dapat diperluas.

---

# **49\. SMTP SETTINGS UI**

Buat halaman/section:

Email & SMTP

Dengan field:

SMTP Provider

\[ Brevo \]

SMTP Host

\[ smtp-relay.brevo.com \]

SMTP Port

\[ 587 \]

SMTP Username / Login

\[ \*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\* \]

SMTP Password / SMTP Key

\[ \*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\* \]

From Name

\[ Tauthub \]

From Email

\[ no-reply@tauthub.com \]

Tambahkan:

\[ Test Connection \]

\[ Send Test Email \]

\[ Save Configuration \]

---

# **50\. SECURITY — SMTP CREDENTIALS**

Ini sangat penting.

SMTP credential **tidak boleh ditampilkan sebagai plaintext** setelah disimpan.

Contoh UI:

SMTP Password

••••••••••••••••••

atau:

SMTP Password

••••••••••••••••••

Configured ✓

Jangan pernah mengembalikan password/SMTP key melalui API GET biasa.

Contoh response yang salah:

{

  "smtp\_password": "actual-secret"

}

Jangan lakukan ini.

Response admin cukup:

{

  "smtp\_configured": true,

  "host": "smtp-relay.brevo.com",

  "port": 587,

  "username": "configured",

  "password\_configured": true

}

---

# **51\. SECRET STORAGE**

Audit existing Tauthub architecture untuk menentukan tempat penyimpanan secret.

Prioritas:

1. Existing secret/configuration management.  
2. Encrypted database configuration.  
3. Environment/secret manager.

Jika credential disimpan di database, **SMTP password/key wajib dienkripsi at rest**.

Jangan menyimpan:

smtp\_password \= plaintext

di database.

Jika project sudah memiliki encrypted settings/secret storage, gunakan mekanisme tersebut.

Jangan membuat encryption system kedua jika existing system sudah tersedia.

---

# **52\. ENVIRONMENT VS ADMIN CONFIGURATION**

Bedakan:

### **Environment**

Digunakan untuk:

initial/default configuration

development

deployment

secret bootstrap

### **Admin Dashboard**

Digunakan untuk:

production configuration

provider settings

sender settings

testing

enable/disable

Namun jangan membuat dua konfigurasi yang saling membingungkan.

Tentukan satu **runtime source of truth**.

Contoh:

Admin SMTP Settings

        ↓

Encrypted Configuration

        ↓

Email Service

        ↓

Brevo SMTP

Environment dapat digunakan sebagai fallback/bootstrap jika architecture project memang membutuhkan.

---

# **53\. ENABLE / DISABLE SMTP**

Tambahkan toggle:

Transactional Email

\[ ON \]

Jika OFF:

SMTP email sending disabled

Tetapi:

**Jangan mematikan security-critical email hanya karena UI toggle dimatikan jika sistem membutuhkan email tersebut untuk keamanan akun.**

Audit behavior existing authentication terlebih dahulu.

Jika verification/reset password bergantung pada email, pastikan disabling SMTP tidak menyebabkan account recovery flow rusak secara diam-diam.

Berikan warning di UI:

Mematikan transactional email dapat memengaruhi

verifikasi akun, reset password, dan notifikasi penting.

---

# **54\. TEST SMTP CONNECTION**

Tombol:

\[ Test Connection \]

harus melakukan test dari backend.

Jangan melakukan koneksi SMTP langsung dari browser.

Flow:

Admin Browser

      ↓

POST /admin/settings/email/test-connection

      ↓

Backend

      ↓

SMTP

      ↓

Result

Response:

✓ SMTP connection successful

atau:

✕ SMTP connection failed

Periksa host, port, username, atau SMTP key.

Jangan menampilkan secret dalam error message.

---

# **55\. SEND TEST EMAIL**

Tambahkan:

Test Email Address

\[ admin@example.com \]

\[ Send Test Email \]

Ketika diklik:

Admin

 ↓

Backend

 ↓

SMTP

 ↓

Test Email

Email test harus menggunakan template sederhana:

Subject:

Tauthub SMTP Test

Body:

SMTP Tauthub berhasil dikonfigurasi.

Provider:

Brevo

Timestamp:

...

Jangan mengirim test email ke seluruh user.

---

# **56\. SENDER VERIFICATION**

Admin UI harus memberikan status:

Sender Status

✓ Configured

atau:

⚠ Sender belum dikonfigurasi

Jika memungkinkan berdasarkan provider/API integration, tampilkan:

Domain:

tauthub.com

Authentication:

✓ SPF

✓ DKIM

⚠ DMARC

Namun jangan membuat klaim status tersebut jika informasi memang tidak tersedia dari provider.

Untuk fase awal, minimal admin dapat menyimpan:

From Name

From Email

---

# **57\. PROVIDER ABSTRACTION**

Jangan membuat Email Service bergantung langsung pada Brevo.

Gunakan abstraction:

Email Provider

      ↓

SMTP Provider

      ↓

Brevo

Contoh konseptual:

EmailService

    ↓

SMTPTransport

    ↓

Configured SMTP Provider

Dengan demikian jika suatu hari Tauthub pindah dari:

Brevo

ke:

Resend

Postmark

Amazon SES

business logic tidak perlu diubah.

---

# **58\. PROVIDER SELECTION**

Jika architecture memungkinkan, Admin UI dapat memiliki:

Email Provider

\[ SMTP ▼ \]

Untuk fase sekarang:

SMTP

adalah satu-satunya provider yang diimplementasikan.

**Jangan mengimplementasikan Resend/Postmark/SES sekarang hanya karena UI memiliki dropdown.**

Jika future provider belum tersedia, jangan tampilkan sebagai pilihan aktif.

---

# **59\. SMTP CONFIGURATION VALIDATION**

Saat admin menyimpan:

Validasi:

Host tidak kosong

Port valid

Username valid

From Email valid

Port harus berupa integer.

Contoh:

587

465

2525

Jangan membatasi hanya 587 jika provider lain nantinya menggunakan port berbeda.

---

# **60\. ADMIN PERMISSION**

Hanya admin dengan permission yang sesuai yang boleh:

View SMTP Settings

Edit SMTP Settings

Test SMTP

Send Test Email

Jangan hanya mengecek role di frontend.

Backend harus melakukan authorization.

Contoh:

admin.settings.email.read

admin.settings.email.write

admin.settings.email.test

Jika Tauthub sudah mempunyai ACL/permission system, **reuse existing ACL**.

---

# **61\. AUDIT LOG SMTP CHANGES**

Setiap perubahan SMTP configuration harus dicatat.

Contoh:

Admin

→ Changed SMTP host

→ Changed SMTP username

→ Updated sender email

→ Enabled SMTP

→ Disabled SMTP

→ Sent SMTP test email

Namun:

**Jangan memasukkan SMTP password/key ke audit log.**

Audit cukup mencatat:

actor

action

timestamp

changed fields

Contoh:

{

  "action": "smtp\_config\_updated",

  "changed\_fields": \[

    "smtp\_host",

    "smtp\_port",

    "from\_email"

  \]

}

Jangan:

{

  "smtp\_password": "secret"

}

---

# **62\. ADMIN UI STATUS**

Halaman Email Settings sebaiknya memiliki status summary di bagian atas:

Email Delivery

● SMTP Connected

Provider: Brevo

Sender: no-reply@tauthub.com

Transactional Email:

ON

Jika belum dikonfigurasi:

⚠ SMTP belum dikonfigurasi

Tauthub belum dapat mengirim email transactional.

---

# **63\. LAST TEST STATUS**

Jika memungkinkan simpan metadata:

Last Connection Test

Successful

23 Aug 2026, 10:42

dan:

Last Test Email

Sent

23 Aug 2026, 10:45

Jangan menyimpan credential dalam metadata tersebut.

---

# **64\. ADMIN EMAIL CONFIG FLOW**

Flow final:

Admin

 ↓

Settings

 ↓

Email / SMTP

 ↓

Configure Brevo

 ↓

Host

Port

Username

SMTP Key

From Name

From Email

 ↓

Save

 ↓

Test Connection

 ↓

Send Test Email

 ↓

SMTP Ready

Setelah berhasil:

Application Event

 ↓

Notification Engine

 ↓

Email Service

 ↓

Admin SMTP Configuration

 ↓

Brevo SMTP

 ↓

User

---

# **65\. IMPORTANT — DO NOT STORE SECRETS IN FRONTEND**

Frontend hanya boleh menerima:

smtp\_configured

provider

host

port

username\_masked

from\_email

status

Frontend **tidak boleh menerima**:

smtp\_password

smtp\_key

api\_key

secret

Bahkan untuk halaman edit sekalipun.

Jika admin ingin mengganti password:

SMTP Password

\[ Masukkan SMTP Key baru \]

Jika kosong:

→ pertahankan credential lama

Jika diisi:

→ encrypt/store credential baru

---

# **66\. FALLBACK BEHAVIOR**

Tentukan behavior jika:

Admin SMTP configuration belum tersedia

Aplikasi tidak boleh crash.

Contoh:

Notification created

Email skipped

Reason:

SMTP not configured

Namun security-critical flow harus memberikan error yang jelas kepada user jika memang email diperlukan untuk menyelesaikan proses.

Jangan:

SMTP missing

→ application 500

untuk notification biasa.

---

# **67\. ADMIN SMTP TESTING ACCEPTANCE CRITERIA**

Fitur dianggap berhasil jika:

### **Configuration**

* Admin dapat memasukkan SMTP Host.  
* Admin dapat memasukkan Port.  
* Admin dapat memasukkan SMTP Login.  
* Admin dapat memasukkan SMTP Key.  
* Admin dapat mengatur From Name.  
* Admin dapat mengatur From Email.  
* Credential tersimpan secara aman.  
* Credential tidak dikirim kembali plaintext.

### **Testing**

* Admin dapat test connection.  
* Admin dapat mengirim test email.  
* Error SMTP ditampilkan secara aman.  
* Test tidak mempengaruhi business transaction.

### **Security**

* Hanya admin berwenang yang dapat mengubah settings.  
* SMTP key tidak muncul di frontend response.  
* SMTP key tidak masuk log.  
* SMTP key tidak masuk audit log.  
* SMTP key tidak masuk Git.

### **Runtime**

* Email Service menggunakan konfigurasi admin.  
* Notification Engine tetap bekerja walaupun SMTP gagal.  
* Security-critical email ditangani secara eksplisit.  
* Email tidak dikirim dua kali karena duplicate event.

---

# **68\. FINAL ADMIN SETTINGS STRUCTURE**

Jika Tauthub sudah mempunyai Settings page, struktur yang direkomendasikan:

Admin

└── Pengaturan

    ├── General

    ├── Authentication

    ├── Notifications

    ├── Email / SMTP

    │

    │   Email Delivery

    │   ● Enabled

    │

    │   Provider

    │   \[ SMTP \]

    │

    │   SMTP Host

    │   \[ smtp-relay.brevo.com \]

    │

    │   Port

    │   \[ 587 \]

    │

    │   SMTP Login

    │   \[ \*\*\*\*\*\*\*\* \]

    │

    │   SMTP Key

    │   \[ \*\*\*\*\*\*\*\* \]

    │

    │   From Name

    │   \[ Tauthub \]

    │

    │   From Email

    │   \[ no-reply@tauthub.com \]

    │

    │   \[ Test Connection \]

    │   \[ Send Test Email \]

    │

    └── ...

Jika struktur existing berbeda, **ikuti UI/architecture existing** dan jangan memaksakan struktur di atas.

---

# **69\. IMPLEMENTATION ORDER**

Implementasi harus dilakukan dengan urutan:

1\. Audit existing settings system

2\. Audit existing secret/configuration storage

3\. Audit existing notification system

4\. Audit existing email service

5\. Audit existing ACL/admin permission

6\. Design SMTP configuration

7\. Implement secure credential storage

8\. Implement Admin SMTP UI

9\. Implement Test Connection

10\. Implement Send Test Email

11\. Connect Email Service to SMTP config

12\. Build Notification → Email routing

13\. Implement recommended high-priority email templates

14\. Test

15\. Production verification

**Jangan langsung menambahkan field database sebelum mengetahui apakah Tauthub sudah mempunyai generic settings/secret storage.**

---

# **70\. VERY IMPORTANT**

Jangan membuat sistem seperti:

Admin UI

 ↓

save SMTP password

 ↓

plain database

dan jangan:

Frontend

 ↓

SMTP credential

 ↓

Browser

Yang benar:

Admin Browser

      ↓

Authenticated Admin API

      ↓

Secure Configuration / Encrypted Secret

      ↓

Email Service

      ↓

SMTP Transport

      ↓

Brevo

---

# **71\. FINAL GOAL**

Tauthub harus memiliki sistem:

                   ADMIN

                      │

                      ↓

              Email / SMTP Settings

                      │

              ┌───────┴────────┐

              ↓                ↓

       SMTP Configuration   Test Tools

              │                │

              └───────┬────────┘

                      ↓

                 Email Service

                      ↑

                      │

               Notification Engine

                      ↑

                      │

                Application Event

Admin dapat mengelola konfigurasi SMTP dengan aman, sementara developer/business logic tidak perlu mengetahui credential atau detail provider.

**Target akhirnya bukan sekadar "Tauthub bisa kirim email".**

Targetnya adalah:

> **Tauthub memiliki transactional email infrastructure yang aman, configurable dari Admin Dashboard, hemat quota, tidak spammy, reusable, dan provider-agnostic.**

