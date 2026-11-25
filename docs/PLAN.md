# lazyjail — fail2ban için klavye odaklı TUI: Mimari Plan

Kaynak: TEK-187 (ana issue TEK-186). Yazan: ArchitectAgent, 2026-11-21.
Bu belge plan; kod yok. "DOĞRULA" ile işaretli maddeler uygulamada Docker'lı gerçek fail2ban ile teyit edilecek varsayımlardır.

---

## 0. Özet (TL;DR)

| Karar | Seçim |
|---|---|
| Repo / binary adı | **`lazyjail`** (yedek: `fail2tui`) |
| Dil / framework | **Go + Bubble Tea** (+ Bubbles, Lip Gloss) |
| fail2ban entegrasyonu | **Hibrit**: okuma ve çalışma anı işlemleri **unix socket (pickle)**; `reload`/`start`/config testi **`fail2ban-client` exec**; regex testi **`fail2ban-regex` exec**. Hepsi `Backend` arayüzü arkasında. |
| Lisans | **MIT** |
| Dağıtım | Statik tek binary (amd64, arm64, armv7), GoReleaser ile GitHub Releases + `.deb` + `install.sh` |
| MVP | Jail listesi/durum, banlı IP'ler, ban/unban, jail start/stop/reload, config görüntüleme (salt okunur), canlı log, regex testi, ignoreip (çalışma anı), arama, yardım, hata ekranları |
| MVP iş bölümü | 5 alt issue (bkz. §10.3) |

---

## 1. Mevcut çözümler

| Araç | Tür | Ne yapıyor | Eksik |
|---|---|---|---|
| `hexajohnny/f2bs` | TUI | İki panel: jail'ler / banlı IP'ler, filtre-sırala, unban | Yalnız tarama+unban; ban, start/stop/reload, config, log, regex testi yok |
| `stef-k/Offenders` (PyPI `offenders`) | TUI (Python/Textual) | Log'dan geçmiş analiz, top IP, WHOIS/RDNS | Yönetim değil, teşhis aracı; Python ortamı gerekir |
| `aganet/fail2ban-list-unban-script` | Bash + whiptail | Banlı IP listele, unban | Çok dar kapsam |
| `swissmakers/fail2ban-ui` | Web (Go) | Çok host, ban/unban, jail/filtre yönetimi, bildirim | Web sunucusu, port, kimlik doğrulama, reverse proxy gerekir; SSH'ta hızlı iş için ağır |
| `webishdev/fail2ban-dashboard` | Web (Go) | Socket üzerinden durum, Prometheus metrikleri | Ağırlıklı salt görüntüleme; web |
| `sean-der/fail2web` + `fail2rest` / `fail2go` | Web + REST (Go) | Socket kütüphanesi + REST | Eski, bakımsız görünüyor |
| `oweitman/fail2bancontrol`, `oussemos/fail2ban-dashboard` | Web | Basit panel | Web, sınırlı |

**Farkımız:** SSH ile girilen sunucuda port açmadan, kurulumsuz (tek binary), *tüm* günlük fail2ban işlerini (durum, ban/unban, jail yaşam döngüsü, config görüntüleme, canlı log, regex testi, ignoreip) tek ekrandan, lazygit/k9s hissiyle yapan TUI. Dar kapsamlı TUI'ler ile ağır web panelleri arasındaki boşluk.

Ad çakışması: GitHub'da `lazyjail`, `lazyf2b`, `jailtui` adlı depo bulunamadı (web araması, 2026-11-21). Repo açılmadan önce `gh search repos lazyjail` ile tekrar bakılmalı.

---

## 2. Dil / framework seçimi

| Ölçüt | Go + Bubble Tea | Python + Textual | Rust + ratatui |
|---|---|---|---|
| Tek binary | ✅ `CGO_ENABLED=0` statik | ❌ Python ≥3.8 + paketler; PyInstaller ile bile büyük ve glibc'ye bağlı | ✅ statik (musl) |
| Sunucuda kurulum | `curl | sh` veya `.deb`, bağımlılık yok | pipx gerekir; Ubuntu 23.04+ PEP 668 sistem pip'i kapatıyor | Go ile aynı |
| ARM / Raspberry Pi | ✅ `GOARCH=arm64/arm GOARM=7` tek komut | Python var ama Textual'ın açılışı Pi'de yavaş | ✅ ama çapraz derleme (cross/zig) daha zahmetli |
| Pickle desteği | `kisielk/og-rek` (encode+decode, protokol 0–5) | Yerleşik `pickle` (ama Python pickle güvensiz `loads`) | `serde-pickle` (protokol kapsamı sınırlı — DOĞRULA) |
| Geliştirme hızı / bakım | Yüksek; Elm mimarisi basit; ajanların en iyi bildiği dillerden | En hızlı prototip | En yavaş; borrow checker, daha uzun derleme |
| Ekosistem örnekleri | lazygit*, k9s*, gh-dash, soft-serve | posting, harlequin | gitui, bottom |
| Bellek / açılış | ~10–15 MB, <100 ms | ~50+ MB, Pi'de 1–2 s | En düşük |

(*lazygit gocui, k9s tview kullanır; Bubble Tea topluluğu en aktif Go TUI ekosistemi.)

**Öneri: Go + Bubble Tea.** Tek statik binary ve ARM çapraz derlemesi fail2ban sunucuları (çoğu küçük VPS ve Pi) için belirleyici; Go'da hem pickle kütüphanesi hem fail2ban socket istemcisi örnekleri (fail2go, webishdev) var; bakım maliyeti Rust'tan düşük.
Bağımlılıklar (minimum): `charmbracelet/bubbletea`, `bubbles`, `lipgloss`, `kisielk/og-rek`. CLI bayrakları için standart `flag` paketi yeter (cobra gereksiz).

---

## 3. fail2ban entegrasyonu

### 3.1 Seçenekler

**A) `fail2ban-client` çıktısını parse etmek**
- ➕ Resmi arayüz; sürüm farklarını client kendisi halleder; config okuma (reload/start) client tarafında yapılır.
- ➖ Her çağrı bir Python süreci: Pi'de 200–500 ms → 20 jail'li bir yenilemede saniyeler. Çıktı insan için biçimlendirilmiş (`|- Filter`, `` `- Actions``), sürümler arası küçük farklar, yerelleştirme yok ama kırılgan.

**B) Unix socket protokolü (`/var/run/fail2ban/fail2ban.sock`)**
- Protokol (fail2ban `client/csocket.py`): istemci komut listesini (`["status","sshd"]`) pickle'layıp sonuna `b"<F2B_END_COMMAND>"` ekleyerek yollar; sunucu aynı sonlandırıcıyla pickle'lanmış `(kod, sonuç)` döner (`kod == 0` başarı; aksi halde sonuç bir istisna nesnesi). Bağlantı birden çok komut için açık tutulabilir; kapatırken `CSPROTO.CLOSE` gönderilir (DOĞRULA: 0.10'da yalnız tek komut/bağlantı olabilir — her komut için yeni bağlantı açmak güvenli varsayılan).
- ➕ Hızlı (ms), yapısal veri (listeler, tuple'lar), süreç yok.
- ➖ Pickle Python'a özgü; Go'da og-rek ile çözülür. Sunucu yanıtı `HIGHEST_PROTOCOL` ile (py3.8+ → protokol 5) gelir; og-rek'in desteklediği doğrulanmalı. Hata yanıtlarında istisna sınıfı `GLOBAL`/`STACK_GLOBAL` opcode'uyla gelir; og-rek bunu `ogórek.Class`/`Call` olarak verir → mesajı metne çevirmeliyiz.
- ➖ **Kritik sınır:** `reload` ve "sunucuda yüklü olmayan jail'i `start`" işlemleri **client tarafında config okur** ve sunucuya `add jail` + ayar akışı gönderir. Bunu Go'da yeniden yazmak = fail2ban'in config ayrıştırıcısını (`.conf`/`.local`, `%(var)s` interpolasyonu, `before/after` include) kopyalamak → kesinlikle yapmıyoruz. Ayrıca socket'ten `stop <jail>` jail'i sunucudan kaldırır; tekrar başlatmak config gerektirir.

### 3.2 Öneri: hibrit, `Backend` arayüzü arkasında

| İşlem | Yol |
|---|---|
| ping, version, status, status <jail>, get … (banip, ignoreip, logpath, findtime, bantime, maxretry, failregex, actions, logtarget, dbfile), set <jail> banip/unbanip, unban <ip>/--all, set <jail> addignoreip/delignoreip, stop <jail> | **Socket** |
| reload [<jail>], start <jail>, config testi (`fail2ban-client -t` — DOĞRULA sürüm) | **exec `fail2ban-client`** |
| Filtre regex testi | **exec `fail2ban-regex`** |
| Canlı log | Dosya tail (`get logtarget` ile yol bulunur) |

Tüm exec çağrıları `exec.Command(path, args...)` ile, **asla shell yok**; argümanlar doğrulanmış değerler (bkz. §9).

```go
// internal/f2b/backend.go (MVP sözleşmesi — paralel çalışan ajanlar buna göre mock yazar)
type JailStatus struct {
    Name string
    CurrentlyFailed, TotalFailed, CurrentlyBanned, TotalBanned int
    FileList []string
    BannedIPs []string
}
type BanEntry struct { IP string; BannedAt, ExpiresAt time.Time } // --with-time yoksa zamanlar sıfır
type JailConfig struct {
    BanTime, FindTime time.Duration; MaxRetry int
    LogPaths, FailRegex, IgnoreRegex, IgnoreIPs, Actions []string
}
type Backend interface {
    Ping(ctx) error
    Version(ctx) (string, error)
    Jails(ctx) ([]string, error)
    JailStatus(ctx, jail string) (JailStatus, error)
    BannedWithTime(ctx, jail string) ([]BanEntry, error)
    JailConfig(ctx, jail string) (JailConfig, error)
    Ban(ctx, jail, ip string) error
    Unban(ctx, jail, ip string) error       // jail=="" → tüm jail'lerden
    StartJail(ctx, jail string) error        // exec
    StopJail(ctx, jail string) error         // socket
    Reload(ctx, jail string) error           // exec; jail=="" → tümü
    AddIgnoreIP(ctx, jail, ipOrCIDR string) error
    DelIgnoreIP(ctx, jail, ipOrCIDR string) error
    LogTarget(ctx) (string, error)
    TestRegex(ctx, logOrLine, filterOrRegex string) (RegexResult, error) // exec fail2ban-regex
}
```
(`ctx` = `context.Context`; her çağrıda zaman aşımı, varsayılan 5 s.)

### 3.3 Sürüm uyumluluğu

| Sürüm | Dağıtım örneği | Not |
|---|---|---|
| 0.10.x | Ubuntu 18.04, Debian 10 | En eski hedef; `get <jail> banip` var mı DOĞRULA, yoksa `status <jail>` içindeki "Banned IP list" kullanılır |
| 0.11.x | Ubuntu 20.04/22.04, Debian 11 | `--with-time` DOĞRULA |
| 1.0.x / 1.1.x | Ubuntu 24.04+, Debian 12/13 | Ana hedef |

Açılışta `version` sorulur, `internal/f2b` içinde küçük bir yetenek tablosu (`caps.BanTime`, `caps.WithTime`) tutulur; desteklenmeyen özellik UI'da gri gösterilir. Bilinmeyen sürüm → uyarı + devam (`--skip-version-check` gerekmez).

---

## 4. Özellik listesi

| # | Özellik | MVP | Not |
|---|---|---|---|
| F1 | Jail listesi + durum (failed/banned sayaçları, aktif/pasif) | ✅ | Periyodik yenileme (varsayılan 2 s, `--refresh`) |
| F2 | Banlı IP listesi (jail başına ve "tümü" görünümü), ban zamanı/kalan süre | ✅ | Zaman yoksa "—" |
| F3 | Ban (IP gir, jail seç) / Unban (seçili, çoklu seçim, tüm jail'lerden) | ✅ | Onay iletişim kutusu |
| F4 | Jail start / stop / reload, global reload | ✅ | stop/reload onaylı |
| F5 | Jail config görüntüleme (etkin değerler socket'ten + ilgili dosyaların ham hali) | ✅ salt okunur | |
| F6 | Jail config düzenleme (`jail.d/lazyjail-<jail>.local`'a yazma, yedek, test, reload, geri alma) | v0.2 | §9 |
| F7 | Canlı log izleme (fail2ban.log; jail/IP'ye göre filtre, Ban/Unban vurgusu, duraklat) | ✅ | journald hedefi v0.2 |
| F8 | Filtre regex testi (`fail2ban-regex`; log dosyası veya tek satır × filtre adı veya regex) | ✅ | Sonuç özet + eşleşen satırlar |
| F9 | İstatistikler: sayaç özetleri | ✅ basit | Geçmiş grafikler (sqlite db, en çok banlanan IP, saat/gün dağılımı) v0.3 |
| F10 | Arama / filtre (`/`) tüm listelerde | ✅ | |
| F11 | ignoreip yönetimi | ✅ çalışma anı (socket) | Kalıcı (config'e yazma) v0.2, F6 ile |
| F12 | IP bilgisi (RDNS, whois, ülke) | v0.3 | Ağ çağrısı → isteğe bağlı |
| F13 | Çoklu sunucu (SSH üzerinden) | v1.x | Kapsam dışı (MVP) |
| F14 | Prometheus/JSON çıktı, `lazyjail status --json` CLI alt komutu | v0.3 | |

---

## 5. Klavye şeması, ekranlar, tema

### 5.1 Yerleşim (ana ekran)

```
┌ lazyjail ─ fail2ban 1.1.0 ─ host: pi5 ───────────────────── ⟳ 2s ─ root ┐
│ Jails (4)             │ sshd ─ banned 3 / total 41 ─ failed 2 / total 388│
│▶ sshd        ● 3  2   │ ┌ Banned IPs ────────────────────────────────────┐│
│  nginx-http  ● 0  0   │ │ IP              Banned at         Expires      ││
│  recidive    ● 1  0   │ │▶185.220.101.4   2026-11-21 19:02  in 8m        ││
│  postfix     ○ —  —   │ │ 45.95.147.12    2026-11-21 18:55  in 1m        ││
│                       │ │ 2001:db8::7     2026-11-21 18:40  expired?     ││
│                       │ └────────────────────────────────────────────────┘│
│                       │ ┌ Config (read-only) ────────────────────────────┐│
│                       │ │ bantime 10m  findtime 10m  maxretry 5          ││
│                       │ │ logpath /var/log/auth.log                      ││
│                       │ │ ignoreip 127.0.0.1/8 ::1                       ││
│                       │ └────────────────────────────────────────────────┘│
├───────────────────────┴──────────────────────────────────────────────────┤
│ 19:02:11 NOTICE [sshd] Ban 185.220.101.4                                  │
│ 19:01:58 INFO   [sshd] Found 185.220.101.4 - 2026-11-21 19:01:58          │
├───────────────────────────────────────────────────────────────────────────┤
│ b ban  u unban  s start/stop  r reload  l log  t regex  i ignoreip  ? yardım│
└───────────────────────────────────────────────────────────────────────────┘
```
● aktif, ○ yüklü değil/durdurulmuş. Dar terminalde (<100 sütun) panel sağda tek sütuna düşer; <60 sütunda "terminal çok dar" uyarısı.

### 5.2 Diğer ekranlar
- **Log (`l`)**: tam ekran tail; `f` jail filtresi, `/` ara, `space` duraklat, `G` sona git.
- **Regex testi (`t`)**: üstte iki giriş (log dosyası/satır, filtre adı/regex), `enter` çalıştır, altta özet (`Lines: N, Matched: M, Missed: K`) + eşleşen/kaçan satırlar sekmeleri.
- **Ban iletişim kutusu (`b`)**: IP girişi (anlık doğrulama), jail = seçili jail.
- **Onay kutusu**: `Unban 185.220.101.4 from sshd? [y/N]`.
- **ignoreip (`i`)**: liste + `a` ekle / `d` sil; başlıkta "çalışma anı — reload'da kaybolur" uyarısı (MVP).
- **Hata ekranı**: §6.2.

### 5.3 Kısayollar (vim + ok tuşları ikisi de)

| Tuş | Eylem | Kapsam |
|---|---|---|
| `↑↓` / `j k` | Gezin | liste |
| `←→` / `h l`, `tab` / `shift+tab` | Panel değiştir | global |
| `enter` | Aç / detay | liste |
| `/` | Ara/filtrele, `esc` temizle | liste |
| `space` | Çoklu seçim | IP listesi |
| `b` | Ban | jail |
| `u` | Unban (seçili/çoklu) | IP |
| `U` | Seçili IP'yi tüm jail'lerden unban | IP |
| `s` | Jail start/stop (duruma göre) | jail |
| `r` / `R` | Jail reload / global reload | jail / global |
| `l` | Log ekranı | global |
| `t` | Regex test ekranı | global |
| `i` | ignoreip | jail |
| `c` | Config ham dosya görüntüleyici | jail |
| `F5` / `ctrl+r` | Hemen yenile | global |
| `?` | Yardım (bağlama göre kısayol listesi, `bubbles/help`) | global |
| `q` / `ctrl+c` | Geri / çık | global |

Tüm eşlemeler `internal/ui/keys.go` içinde `bubbles/key` ile tek yerde; yardım ekranı buradan otomatik üretilir.

### 5.4 Tema
Lip Gloss ile `internal/ui/theme.go`: `default` (ANSI 16 renk uyumlu — her terminalde çalışır), `dark`, `light`. `NO_COLOR` ortam değişkeni ve `--no-color` desteklenir. Renk anlamları: banned=kırmızı, aktif=yeşil, durmuş=gri, uyarı=sarı.

---

## 6. Yetki ve hata durumları

### 6.1 Yetki
- Socket varsayılan `srwx------ root:root` → **root gerekir**. `fail2ban-client` de root ister; log dosyaları çoğunlukla `adm` grubu.
- Davranış: açılışta socket'e bağlan → `EACCES` ise:
  1. `--sudo` bayrağı verildiyse veya kullanıcı iletişim kutusunda "evet" derse: `syscall.Exec("/usr/bin/sudo", ["sudo","--", os.Executable(), args...])` ile kendini sudo altında yeniden çalıştır (TTY korunur, şifreyi sudo sorar).
  2. Aksi halde açıklayıcı hata ekranı: "Socket'e erişim yok. `sudo lazyjail` ile çalıştırın."
- Grup bazlı erişim (socket'i `chgrp fail2ban` yapan systemd drop-in) belgelerde **önerilmeyen seçenek** olarak anlatılır: socket erişimi = root eşdeğeri yetki (ban eylemleri root olarak iptables çalıştırır, `set <jail> action ... actionban` ile komut tanımlanabilir).
- `--socket`, `--client` (fail2ban-client yolu), `--regex` (fail2ban-regex yolu) bayrakları; varsayılanlar PATH + `/var/run/fail2ban/fail2ban.sock` (ayrıca `/run/fail2ban/…` denenir).
- Yazma işlemleri olmadan gezinmek için `--read-only` bayrağı (yazma tuşları devre dışı) — kazara ban/unban'e karşı.

### 6.2 Hata durumları

| Durum | Tespit | UI |
|---|---|---|
| fail2ban kurulu değil | `fail2ban-client` PATH'te yok ve socket yok | Tam ekran: kurulum ipucu (`apt install fail2ban`) |
| fail2ban kapalı | Socket dosyası yok / `ECONNREFUSED` | Tam ekran: "fail2ban çalışmıyor"; `r` tekrar dene; `S` → `systemctl start fail2ban` (onaylı, exec) |
| İzin yok | `EACCES` | §6.1 |
| Zaman aşımı | ctx deadline | Durum çubuğunda kırmızı uyarı, son iyi veri gösterilmeye devam |
| Komut hatası (`kod != 0`) | Socket yanıtı | Toast: istisna mesajı (ör. "Invalid IP address") |
| Bağlantı yenilemede koptu | — | Üst çubukta "bağlantı yok", otomatik geri bağlanma (üstel bekleme, en fazla 10 s) |
| Bilinmeyen sürüm | `version` | Tek seferlik uyarı |
| Log dosyası okunamıyor | open hatası | Log panelinde mesaj, uygulama çalışmaya devam |

---

## 7. Test stratejisi

1. **Birim — pickle kodek** (`internal/f2b/proto`): gerçek fail2ban yanıtlarının pickle baytları `testdata/*.pickle` olarak saklanır (Docker'daki gerçek sunucudan bir kez kaydedilir, her sürüm için) → çözücü testleri. Komut kodlama için gidiş-dönüş testi.
2. **Birim — sahte sunucu** (`internal/f2b/fake`): `net.Listen("unix", tmp)` üzerinde, komut → yanıt tablosuyla çalışan sahte fail2ban (jail durumu bellekte; ban/unban durumu değiştirir; hata/zaman aşımı enjekte edilebilir). `Backend` socket uygulamasının tüm metotları buna karşı test edilir. Exec kısmı için `--client`/`--regex` yolu testte sahte bir betiğe (testdata'daki shell script, sabit çıktı) yönlendirilir.
3. **UI** (`internal/ui`): Bubble Tea modelleri `Backend` mock'u ile; `Update` mesaj testleri + `charmbracelet/x/exp/teatest` ile golden çıktı (ana ekran, boş durum, hata ekranları). Terminal boyutu sabit (100×30).
4. **Entegrasyon — gerçek fail2ban (Docker)** (`test/integration`, build tag `integration`): `debian:12` (1.0.x), `ubuntu:22.04` (0.11.x), `ubuntu:24.04` (1.0.x/1.1.x) imajlarında fail2ban kurulur, sahte bir `sshd` jail'i (`backend = polling`, logpath `/tmp/auth.log`, `banaction = dummy`) — iptables gerektirmez. Test: log'a başarısız giriş satırları yaz → ban oluştu mu, unban, ignoreip ekle/sil, stop/start, reload, regex testi. CI'da matris olarak `make integration`.
5. **Lint**: `go vet`, `golangci-lint` (varsayılan set), `gofmt`.
6. Elle kabul: Pi 5 (arm64) üzerinde gerçek fail2ban ile kısa kontrol listesi (`docs/testing.md`).

---

## 8. Paketleme / dağıtım / CI

- **Derleme**: `CGO_ENABLED=0`, `-trimpath -ldflags "-s -w -X main.version=…"`. Hedefler: `linux/amd64`, `linux/arm64`, `linux/arm` (GOARM=7). (fail2ban yalnız Linux/BSD; macOS/Windows hedeflenmez, yalnız geliştirme için derlenebilir.)
- **GoReleaser** (`.goreleaser.yaml`): tag (`v*`) push'unda tar.gz arşivleri + `checksums.txt` + `nfpms` ile `.deb` (bağımlılık: `Recommends: fail2ban`) → GitHub Releases. İmza (cosign) v0.3.
- **install.sh**: mimariyi (`uname -m`) tespit eder, son sürümü GitHub API'den bulur, indirir, **checksum doğrular**, `/usr/local/bin/lazyjail`'e kurar. `curl | sh` yerine önce indirip okumayı README önerir.
- **go install**: `go install github.com/<owner>/lazyjail/cmd/lazyjail@latest`.
- **GitHub Actions**:
  - `ci.yml` (push/PR): gofmt kontrolü, `go vet`, golangci-lint, `go test ./...` (race), 3 hedef için derleme.
  - `integration.yml` (push main + PR, manuel): Docker matris entegrasyon testleri.
  - `release.yml` (tag `v*`): goreleaser.
- AUR/Homebrew/apt deposu: v1.x.

---

## 9. Güvenlik

| Risk | Önlem |
|---|---|
| Komut enjeksiyonu (exec) | Shell yok (`exec.Command` + argüman dizisi); IP `net/netip.ParseAddr`/`ParsePrefix` ile doğrulanır; jail adı yalnız sunucudan gelen jail listesinde varsa ve `^[A-Za-z0-9._-]+$`; filtre adı `filter.d` içindeki dosyalardan seçilir; serbest regex yalnız `fail2ban-regex`'e tek argüman olarak gider; `--` ile seçenek enjeksiyonu engellenir. |
| Socket'e kötü veri | Biz yalnız string listesi pickle'larız; gelen pickle Go'da çözülür (Go çözücü kod çalıştırmaz, Python `loads` riskini taşımaz). Yanıt boyutu sınırı (ör. 16 MB) ve zaman aşımı. |
| Root olarak çalışan TUI | Dosya yazma yok (MVP salt okunur config); log görüntüleyicide terminal kaçış dizileri temizlenir (log'a saldırganın yazdığı ANSI dizileri — kullanıcı adı alanı ile enjekte edilebilir — terminali manipüle etmesin). |
| Config düzenleme (v0.2) | Asla `jail.conf`/`filter.d/*.conf` düzenlenmez; yalnız `jail.d/lazyjail-<jail>.local` yazılır. Akış: mevcut dosyanın zaman damgalı yedeği (`/var/lib/lazyjail/backups/`) → geçici dosyaya yaz → `fail2ban-client -t` ile test → atomik `rename` → `reload <jail>` → başarısızsa yedeği geri koy ve tekrar reload. Dosya modu 0644 root. `$EDITOR` kullanımı v0.2'de seçenek (root olarak editör açmak = kullanıcının bilinçli tercihi). |
| Kendi IP'ni banlama | Ban iletişim kutusunda `SSH_CLIENT`/`SSH_CONNECTION` IP'si ile eşleşirse kırmızı uyarı + ikinci onay. |
| Kazara toplu unban | `U`/`--all` işlemleri ikinci onay ister; `--read-only` modu. |
| Tedarik zinciri | Az bağımlılık, `go.sum`, Dependabot, release checksum. |

---

## 10. MVP ve yol haritası

### 10.1 MVP (v0.1.0)
F1–F5, F7, F8, F9 (basit), F10, F11 (çalışma anı), yardım ekranı, tema (default + NO_COLOR), hata ekranları, `--sudo`/`--read-only`/`--socket`/`--refresh`/`--version`, CI, release (binary + .deb + install.sh), README + docs.

### 10.2 Sonrası
- **v0.2**: Config düzenleme (F6) + kalıcı ignoreip, journald log hedefi, dark/light tema seçimi, config dosyası (`~/.config/lazyjail/config.toml` — yalnız UI tercihleri).
- **v0.3**: İstatistik ekranı (fail2ban sqlite db salt okunur: en çok banlanan IP'ler, zaman dağılımı, sparkline), IP bilgisi (RDNS/whois, isteğe bağlı), `lazyjail status --json`, cosign imzalı release.
- **v1.0**: Kararlı kısayollar ve CLI, filtre düzenleyici + canlı regex testi, Homebrew/AUR.
- **v1.x**: SSH üzerinden çoklu sunucu (uzak socket'i `ssh -L` ile değil, uzakta `lazyjail --agent` ile).

### 10.3 MVP alt issue önerisi

> Güncel hali: [PLAN-MVP.md](PLAN-MVP.md) (TEK-448). Aşağıdaki tablo tarihsel kayıttır.

Sıra: **1 → (2 ∥ 3) → (4 ∥ 5)**. 2 ve 3, §3.2'deki `Backend` arayüzü sayesinde paralel yürür (3 mock ile çalışır).

| # | Başlık | Kapsam | Bitti ölçütü / doğrulama | Ajan | Bağımlılık |
|---|---|---|---|---|---|
| 1 | Repo iskeleti, CI, public repo | Public `lazyjail` reposu (MIT), `go mod`, §11 klasör yapısı, `cmd/lazyjail` (yalnız `--version`), `internal/f2b/backend.go` (arayüz + tipler, §3.2), Makefile, `.golangci.yml`, `ci.yml`, `.goreleaser.yaml` (snapshot), README + docs iskeleti | `make build test lint` yeşil; `goreleaser release --snapshot --clean` 3 arşiv + .deb üretir; GitHub Actions CI yeşil | CoderAgent | — |
| 2 | fail2ban socket istemcisi + sahte sunucu | `internal/f2b/proto` (pickle kodek, og-rek), `internal/f2b/socket` (Backend'in socket kısmı), exec kısmı (`fail2ban-client`, `fail2ban-regex`), yetenek tablosu, `internal/f2b/fake` | `go test ./internal/f2b/... -race` yeşil, kapsama ≥%80; sahte sunucuya karşı tüm Backend metotları; hata yanıtı (istisna) metne çevriliyor | ProAgent | 1 |
| 3 | TUI çekirdeği | Ana ekran (jail listesi, detay, banlı IP'ler, config paneli), ban/unban/start/stop/reload + onay kutuları, arama, yardım, tema, hata ekranları, `--sudo` yeniden çalıştırma, `--read-only` | Mock Backend ile `go test ./internal/ui/...` (teatest golden) yeşil; `go run ./cmd/lazyjail --demo` (mock backend ile) elle gezilebilir | CoderAgent | 1 (arayüz) |
| 4 | Log izleme, regex testi, ignoreip ekranları | Log ekranı (tail, filtre, ANSI temizleme), regex test ekranı, ignoreip ekranı; gerçek Backend'in UI'ya bağlanması | Birim testler + teatest; gerçek fail2ban'li makinede (Pi) `docs/testing.md` kontrol listesi | CoderAgent | 2, 3 |
| 5 | Docker entegrasyon testleri + release | `test/integration` (3 imaj matrisi), `integration.yml`, `release.yml`, `install.sh` (checksum doğrulamalı), pickle `testdata` kayıtları | `make integration` yerelde ve CI'da yeşil; `v0.1.0-rc1` tag'iyle release taslağı (yayın kullanıcı onayıyla) | CoderAgent | 2 |

Not: Varsayılan "bir issue = bir PR" ilkesine rağmen bölme gerekçesi: 2 ve 3 bağımsız doğrulanabilir (socket istemcisi tek başına test edilir, UI mock ile), 5 farklı altyapı (Docker/CI). Hepsi tek repoda; her biri ayrı PR.

---

## 11. Repo yapısı, ad, lisans, docs

**Ad: `lazyjail`** — kısa, lazygit/lazydocker ailesini çağrıştırır ("tembel ama hepsini yapan" TUI), "jail" fail2ban kavramı; arama sonucunda çakışma yok. Yedekler: `fail2tui`, `jailtui`. (`jailhouse` Siemens hipervizörüyle çakışır, önerilmez.)

**Lisans: MIT** — en basit, en yaygın; benzer araçlar (f2bs MIT/Apache, Offenders MIT) ile uyumlu. Apache-2.0 patent maddesi bu boyutta bir CLI için gereksiz. (Kullanıcı farklı isterse değişir; aşağıdaki "Kullanıcı kararı" bölümü.)

```
lazyjail/
├── cmd/lazyjail/main.go          # bayraklar, sudo yeniden çalıştırma, program başlatma
├── internal/
│   ├── f2b/
│   │   ├── backend.go            # Backend arayüzü + tipler
│   │   ├── proto/                # pickle kodek, çerçeveleme
│   │   ├── socket/               # socket Backend
│   │   ├── execbk/               # fail2ban-client / fail2ban-regex exec
│   │   ├── caps.go               # sürüm yetenek tablosu
│   │   ├── mock/                 # UI testleri ve --demo için bellek içi Backend
│   │   └── fake/                 # sahte unix socket sunucusu (testler)
│   ├── ui/                       # Bubble Tea modelleri: app, jails, bans, config, log, regex, ignoreip, dialogs, help, keys.go, theme.go
│   ├── logtail/                  # dosya tail + ANSI temizleme + satır ayrıştırma
│   └── validate/                 # IP/CIDR, jail adı doğrulama
├── test/integration/             # Docker tabanlı testler (build tag)
├── testdata/
├── scripts/install.sh
├── docs/
│   ├── README.md                 # indeks
│   ├── PLAN.md                   # bu belge
│   ├── architecture.md
│   ├── keybindings.md
│   ├── permissions.md            # root/sudo/socket
│   ├── testing.md
│   ├── releasing.md
│   └── DEĞİŞİKLİKLER.md
├── .github/workflows/{ci,integration,release}.yml
├── .goreleaser.yaml
├── .golangci.yml
├── Makefile                      # build, test, lint, integration, demo
├── LICENSE
├── README.md                     # GIF/ekran görüntüsü, kurulum, hızlı başlangıç, kısayollar, yetki, SSS
├── CONTRIBUTING.md
└── SECURITY.md
```

**README iskeleti:** başlık + tek cümle + demo GIF (vhs ile `docs/demo.tape`) → Özellikler → Kurulum (install.sh, .deb, go install, elle) → Kullanım (`sudo lazyjail`, bayraklar) → Kısayollar tablosu → Yetkiler → Uyumluluk (fail2ban sürümleri, mimariler) → Geliştirme (`make demo` mock ile root'suz) → Lisans.

---

## 12. Riskler ve açık sorular

| Risk | Etki | Azaltma |
|---|---|---|
| og-rek'in protokol 5 / fail2ban istisna nesnelerini çözememesi | Socket yolu çalışmaz | İş 2'nin ilk adımı: gerçek sunucudan kayıtlı pickle'larla spike. Başarısızsa `fail2ban-client` exec + çıktı parse yedeği (Backend arayüzü sayesinde UI etkilenmez) |
| 0.10'da komut farkları (`banip`, `--with-time`, `-t`) | Bazı özellikler eksik | Yetenek tablosu + entegrasyon matrisi; 0.10 "en iyi çaba" desteği |
| `fail2ban-client` exec yavaşlığı (reload/start) | Kullanıcı birkaç saniye bekler | Async komut + spinner; yalnız seyrek işlemler exec |
| Log hedefi journald olan sistemler | Canlı log boş | MVP'de anlaşılır mesaj; v0.2'de `journalctl -f -u fail2ban` exec |
| Root olarak TUI çalıştırma isteksizliği | Benimseme | `--read-only`, açık `docs/permissions.md`, `--demo` |

Açık sorular (uygulamayı engellemez, varsayılanlarla ilerlenebilir):
1. GitHub sahibi: kullanıcı hesabı `batv45` mi? (Varsayılan: evet.)
2. v0.1.0 release yayını (tag + GitHub Release) kullanıcı onayıyla mı yapılsın? (Varsayılan: rc taslağı, yayın onayla.)

## Kullanıcı kararı gerekiyor
- **Repo adı:** `lazyjail` (öneri) / `fail2tui` / `jailtui`. Öneri `lazyjail`; aksi belirtilmezse bununla ilerlenir.
- **Lisans:** MIT (öneri, en basit) / Apache-2.0 (patent koruması, daha uzun metin). Aksi belirtilmezse MIT.
