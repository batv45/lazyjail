# lazyjail: kalan MVP işleri, v0.1.0 release ve Ubuntu kurulum planı

Kaynak: TEK-448 (ana issue TEK-186). Yazan: Lead, 2026-11-25.
Temel: [PLAN.md](PLAN.md) (TEK-187 mimari planı) ve main `91e4997` (TEK-192 iskeleti).
Bu belge PLAN.md §10.3'ü güncel duruma göre yeniden yazar; çelişkide bu belge geçerlidir.

---

## 1. Amaç ve kapsam

- PLAN.md'deki MVP'yi (v0.1.0) bitirmek: fail2ban backend'i, TUI, log/regex/ignoreip ekranları.
- Tag ile tetiklenen CI'nin binary arşivlerini, `.deb` paketlerini, `checksums.txt` ve `install.sh`'ı
  **yayınlanmış** (draft olmayan) GitHub Release'e eklemesi.
- README'de Ubuntu için tek komutluk kurulum: latest release'i indirir, mimariyi seçer, checksum doğrular,
  `/usr/local/bin/lazyjail`'e koyar. Güncelleme, kaldırma ve `.deb` alternatifi.
- Demo modunun ekran görüntüsü (TEK-186'ya).
- Kapsam dışı (PLAN.md §10.2): config düzenleme, journald, istatistik ekranı, çoklu sunucu, imzalı release.

## 2. Mevcut durum (main `91e4997`)

| Var | Yok |
|---|---|
| `go.mod` (go 1.24, bağımlılık yok), `cmd/lazyjail` (yalnız `--version`) | Backend uygulaması (socket, exec, fake, mock) |
| `internal/f2b/backend.go`: `Backend` arayüzü ve tipleri (PLAN §3.2 ile birebir) | TUI (`internal/ui`), `logtail`, `validate` |
| `Makefile`, `.golangci.yml`, `ci.yml` (fmt, vet, lint, race test, 3 hedef derleme), CI yeşil | `release.yml`, `integration.yml`, `scripts/install.sh` |
| `.goreleaser.yaml` (3 arşiv + .deb + checksums, **`draft: true`**, sürümlü dosya adları) | Tag, release |
| README, CONTRIBUTING, SECURITY, `docs/PLAN.md`, `docs/README.md`, `docs/DEĞİŞİKLİKLER.md` | `architecture.md`, `keybindings.md`, `permissions.md`, `testing.md`, `releasing.md` |

## 3. TEK-187 ve TEK-192 incelemesinden çıkan düzeltmeler

Bulguların hiçbiri KRİTİK/ÖNEMLİ değil; ikisi de onaylandı. Aşağıdakiler işlere dağıtıldı:

| # | Bulgu | Nerede | İş |
|---|---|---|---|
| D1 | Release `draft: true`: "latest" indirme taslakta çalışmaz | `.goreleaser.yaml:56` | R |
| D2 | Arşiv/.deb adları sürüm içeriyor (`lazyjail_0.1.0_linux_amd64.tar.gz`): `releases/latest/download/<ad>` sabit adla çalışır | `.goreleaser.yaml:30,35` | R |
| D3 | PLAN §10.3 iş 5 "rc taslak, yayın onayla" diyor; kullanıcı artık yayınlanmış tag istiyor | `docs/PLAN.md` §10.3 | Bu belge |
| D4 | `golangci-lint-action` `version: latest`: yeni lint sürümü CI'yi habersiz kırabilir, sürüm sabitlenmeli | `.github/workflows/ci.yml:31` | R |
| D5 | `make lint` `gofmt -l`, CI `gofmt -s -l` kullanıyor; yerelde geçen CI'da kalabilir | `Makefile:26` | R |
| D6 | `docs/DEĞİŞİKLİKLER.md` adında Türkçe karakter var | `docs/` | R |
| D7 | UI'nın hata türlerini ayırt edeceği ortak hata değerleri arayüzde yok (izin yok / kapalı / desteklenmiyor) | `internal/f2b/` | §5 ortak sözleşme |
| D8 | PLAN'daki ajan adları (ProAgent, CoderAgent) eski; tüm işler Coder'a | `docs/PLAN.md` §10.3 | Bu belge |

## 4. Mimari kararlar (yeni)

1. **Sabit asset adları.** Arşivler `lazyjail_linux_{amd64,arm64,armv7}.tar.gz`, paketler
   `lazyjail_{amd64,arm64,armv7}.deb`. Böylece `https://github.com/batv45/lazyjail/releases/latest/download/<ad>`
   GitHub API'sine, `jq`'ya ve API hız sınırına gerek kalmadan çalışır. Sürüm, release'in kendisinde ve
   `lazyjail --version` çıktısında. Neden: en basit ve en sağlam "latest" indirme yolu.
2. **`install.sh` release asset'i.** Script repoda `scripts/install.sh`, goreleaser `release.extra_files` ile
   her release'e eklenir. Kurulum komutu main'deki değil, yayınlanmış sürümdeki scripti çalıştırır.
3. **Release yayınlanır:** `draft: false`, `prerelease: auto` (`-rc` gibi ekli tag'ler pre-release olur ve
   "latest" olmaz; bu yüzden ilk sürüm doğrudan `v0.1.0`).
4. **Tag'den önce prova.** Tag silmek/taşımak yasak (geçmiş yeniden yazma). Bu yüzden her PR/push'ta CI
   `goreleaser check` + `goreleaser release --snapshot --clean` ve `install.sh`'ı snapshot asset'lerine karşı
   çalıştırır. Tag sonrası release yine de bozulursa düzeltilip `v0.1.1` atılır.
5. **İlk sürüm: `v0.1.0`.** MVP (bu belgedeki A, B, C, R) main'e girip incelemeleri bittikten sonra atılır.
6. **Merge akışı:** Coder işini kendi dalında bitirir, yerel testler geçince main'e `--no-ff` merge edip push
   eder, sonra `in_review` yapar (TEK-192 ile aynı akış; sonraki aşama main'deki koda dayanır). Lead'in GERİ
   bulguları aynı dalda düzeltilip yeniden merge edilir. Tag yalnızca son aşamada, tüm incelemeler `done`
   olunca atılır; asıl kapı odur.

## 5. Ortak sözleşme (A ve B paralel başlamadan önce sabit)

`internal/f2b/backend.go` değişmez. Ek olarak `internal/f2b/errors.go` **tam olarak** şu içerikle eklenir.
A ve B'den hangisi önce ihtiyaç duyarsa birebir bu metni ekler (iki dalda aynı içerik git'te çakışmaz):

```go
package f2b

import "errors"

// Sentinel errors every Backend implementation wraps, so the UI can pick the right screen.
var (
	ErrNotInstalled = errors.New("fail2ban is not installed")
	ErrNotRunning   = errors.New("fail2ban server is not running")
	ErrPermission   = errors.New("permission denied for fail2ban socket")
	ErrUnsupported  = errors.New("operation not supported by this fail2ban version")
)

// CommandError is a non-zero reply from fail2ban; Msg is the exception text.
type CommandError struct {
	Msg string
}

func (e *CommandError) Error() string { return e.Msg }
```

Kurallar: uygulamalar bu hataları `fmt.Errorf("...: %w", ErrX)` ile sarar, UI `errors.Is` / `errors.As` ile
ayırt eder. Arayüz değişikliği gerekirse önce bu issue zincirinde (TEK-186) yazılır, tek başına değiştirilmez.

## 6. İş listesi (alt issue önerisi)

Sıra: **Aşama 1: A ∥ B ∥ R → Aşama 2: C → Aşama 3: T.** Hepsi Coder. Hepsi tek repo
(`https://github.com/batv45/lazyjail`), her iş kendi `agent/` dalında. Her işte ortak bitti ölçütü:
`make build test lint` yeşil, `docs/DEGISIKLIKLER.md`'ye madde (R merge olmadan önce eski ad), main'e merge,
rapor: dal, main SHA, komut çıktıları.

### A. fail2ban backend: socket + exec + sahte sunucu + entegrasyon testi (Aşama 1)

- **Kapsam:** PLAN §3, §7.1, §7.2, §7.4, §9.
  - `internal/f2b/proto`: pickle kodek (`kisielk/og-rek`), `<F2B_END_COMMAND>` çerçeveleme, 16 MB yanıt sınırı.
    **İlk adım spike:** Docker'daki gerçek fail2ban (ubuntu:24.04) yanıtlarını `testdata/*.pickle` olarak kaydet
    ve og-rek ile çöz (protokol 5, istisna nesneleri). Başarısızsa durup issue'ya yaz (yedek: exec + parse).
  - `internal/f2b/socket`: Backend'in socket kısmı; her komut için yeni bağlantı; varsayılan 5 s zaman aşımı;
    `/var/run/fail2ban/fail2ban.sock` ve `/run/fail2ban/fail2ban.sock`.
  - `internal/f2b/execbk`: `fail2ban-client` (`reload`, `start`) ve `fail2ban-regex`; shell yok, `--` ile.
  - Birleşik Backend (socket + exec) ve `caps.go` (sürüm yetenekleri; desteklenmeyen → `ErrUnsupported`).
  - `internal/validate`: IP/CIDR (`net/netip`), jail adı `^[A-Za-z0-9._-]+$`.
  - `internal/f2b/fake`: sahte unix socket sunucusu (bellek içi durum, hata/zaman aşımı enjeksiyonu).
  - `internal/f2b/errors.go` (§5).
  - `test/integration` (build tag `integration`) + `make integration` + `.github/workflows/integration.yml`
    (matris: ubuntu:22.04, ubuntu:24.04, debian:12; `banaction = dummy`, polling sshd jail).
  - `docs/architecture.md`, `docs/testing.md` (entegrasyon kısmı).
- **Bitti:** `go test -race ./internal/...` yeşil (CI'da; Pi'de race çalışmıyor, yerelde `go test`),
  `internal/f2b` kapsaması ≥ %80, tüm Backend metotları sahte sunucuya karşı test edildi, istisna yanıtı
  `CommandError` metnine çevriliyor, `make integration` en az ubuntu:24.04 için yerelde yeşil (Docker yoksa
  rapora yaz; CI linki).
- **Doğrulama:** `make test lint`, `go test -cover ./internal/f2b/...`, `make integration`.
- **İnceleme: var** (root olarak exec ve socket).

### B. TUI çekirdeği + demo modu + ekran görüntüsü (Aşama 1)

- **Kapsam:** PLAN §4 (F1-F5, F9 basit, F10), §5, §6.
  - `internal/f2b/mock`: bellek içi Backend (gerçekçi 4 jail, banlı IP'ler, zamanlar), `--demo` ve UI testleri için.
  - `internal/f2b/errors.go` (§5; A'dan önce gerekirse).
  - `internal/ui`: ana ekran (jail listesi, durum, banlı IP'ler, salt okunur config paneli, alt log şeridi yer
    tutucu), ban/unban (`u`, `U`, çoklu seçim)/start-stop/reload/global reload ve onay kutuları, `/` arama,
    `?` yardım (`keys.go` tek kaynak), `theme.go` (default + `NO_COLOR`/`--no-color`), hata ekranları (§6.2,
    `errors.Is` ile), dar terminal düzeni, periyodik yenileme (`--refresh`, varsayılan 2 s).
  - `cmd/lazyjail`: bayraklar `--demo`, `--read-only`, `--sudo`, `--socket`, `--refresh`, `--no-color`, `--version`;
    `--sudo` ile `syscall.Exec` yeniden çalıştırma. Gerçek backend bağlanana kadar `--demo` dışı çalıştırma
    açıklayıcı mesajla çıkar (C bağlar). `main_test.go` buna göre güncellenir.
  - Ban kutusunda `SSH_CONNECTION` IP'si uyarısı (PLAN §9).
  - `make demo` hedefi; `docs/keybindings.md`, `docs/permissions.md`.
  - **Ekran görüntüsü:** `lazyjail --demo` ana ekranı ve yardım ekranı PNG (vhs; yoksa `tmux capture-pane -e -p`
    + `charmbracelet/freeze`). Repoda `docs/img/demo.png`; rapora ek ve **TEK-186'ya ekli yorum** olarak.
- **Bitti:** `go test ./internal/ui/... ./cmd/...` yeşil (teatest golden, 100×30: ana ekran, boş durum,
  izin yok, fail2ban kapalı, yardım); `--read-only`'de yazma tuşları kapalı (test); ekran görüntüsü TEK-186'da.
- **Doğrulama:** `make test lint`, `go run ./cmd/lazyjail --demo` elle.
- **İnceleme: var** (sudo yeniden çalıştırma, yıkıcı işlemlerin onayı).

### R. Release hattı, install.sh, README kurulum, docs düzeltmeleri (Aşama 1)

- **Kapsam:** §4 kararları, §7 kurulum metni, §3 D1, D2, D4, D5, D6.
  - `.goreleaser.yaml`: `draft: false`, `prerelease: auto`; arşiv `name_template`
    `"{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}{{ if .Arm }}v{{ .Arm }}{{ end }}"`; nfpm `file_name_template`
    `"{{ .ProjectName }}_{{ .Arch }}{{ if .Arm }}v{{ .Arm }}{{ end }}"`; `release.extra_files: [{glob: scripts/install.sh}]`;
    arşive LICENSE ve README girsin.
  - `.github/workflows/release.yml`: `on.push.tags: ["v*"]`, `permissions: contents: write`, checkout
    `fetch-depth: 0`, setup-go (`go-version-file`), `go test ./...`, `goreleaser/goreleaser-action@v6`
    (`version: "~> v2"`, `args: release --clean`, `GITHUB_TOKEN`).
  - `ci.yml`: yeni `release-dry-run` işi: `goreleaser check`, `goreleaser release --snapshot --clean`,
    `shellcheck scripts/install.sh`, ardından `dist/`'i `python3 -m http.server` ile sunup
    `LAZYJAIL_BASE_URL=http://127.0.0.1:8000 INSTALL_DIR=$RUNNER_TEMP/bin sh scripts/install.sh` ve
    `$RUNNER_TEMP/bin/lazyjail --version`. golangci-lint sürümünü sabitle (D4).
  - `scripts/install.sh` (POSIX sh, `set -eu`, shellcheck temiz):
    - `uname -m`: `x86_64|amd64` → amd64, `aarch64|arm64` → arm64, `armv7l|armv8l` → armv7; diğerleri hata.
    - Ortam: `LAZYJAIL_VERSION` (boş → latest; `v0.1.0` → `releases/download/v0.1.0`), `LAZYJAIL_BASE_URL`
      (test için), `INSTALL_DIR` (varsayılan `/usr/local/bin`).
    - `curl -fsSL` ile arşiv + `checksums.txt` → `mktemp -d` (trap ile silinir) → arşivin satırını
      `sha256sum -c -` ile doğrula (eşleşmezse çık, hiçbir şey kurma) → `tar` ile yalnız `lazyjail`'i çıkar →
      `install -m 0755` → `lazyjail --version` yaz.
    - Yazma izni yoksa "sudo ile çalıştırın" mesajı; `curl`/`sha256sum`/`tar` yoksa açık hata.
  - `README.md`: Kurulum bölümü (§7 metni), durum notunun güncellenmesi, kısayollar tablosu yer tutucusu.
  - `docs/DEĞİŞİKLİKLER.md` → `docs/DEGISIKLIKLER.md` (`git mv`), README, CONTRIBUTING, `docs/README.md`,
    `docs/PLAN.md` ağacı bağlantıları; `Makefile` lint'te `gofmt -s -l` (D5). `docs/releasing.md`
    (tag atma, prova, bozuk release'te `v0.1.x` ile ilerleme).
- **Bitti:** yerelde `goreleaser release --snapshot --clean` 3 arşiv + 3 .deb + checksums + install.sh üretir,
  adlar §4.1'deki gibi; `shellcheck` temiz; yerelde snapshot'a karşı `install.sh` geçici dizine kurar ve bozuk
  checksum'da kurmadan çıkar (iki durum da çıktıyla rapora); `ubuntu:24.04` container'ında `.deb` kurulur
  (`apt install ./lazyjail_amd64.deb`, `lazyjail --version`). CI linki rapora.
- **Not:** A ve B de `docs/DEĞİŞİKLİKLER.md`'ye yazar; R ilk merge olursa diğerleri rebase'de yeni adı kullanır.
- **İnceleme: var** (internetten indirilip root ile çalışan script, release izinleri).

### C. Log, regex testi, ignoreip ekranları ve gerçek backend bağlantısı (Aşama 2; A ve B main'de)

- **Kapsam:** PLAN F7, F8, F11 (çalışma anı), §6.1, §9.
  - `internal/logtail`: dosya tail (rotasyonu izler), ANSI/kontrol karakteri temizleme, satır ayrıştırma
    (zaman, seviye, jail, Ban/Unban).
  - UI: log ekranı (`l`: `f` jail filtresi, `/`, `space` duraklat, `G`), ana ekrandaki log şeridi, regex ekranı
    (`t`; filtre adı `filter.d`'den seçim veya serbest regex), ignoreip ekranı (`i`; ekle/sil, "reload'da
    kaybolur" uyarısı). journald hedefinde açık mesaj.
  - `cmd/lazyjail`: demo dışında gerçek Backend (A) ile açılış; `EACCES` → `--sudo` önerisi/iletişim kutusu.
  - `README.md` kullanım + kısayollar, `docs/testing.md` elle kabul listesi.
- **Bitti:** birim + teatest yeşil; gerçek fail2ban'e karşı (A'nın Docker kurulumunda TUI'yi
  `docker exec -it` ile ya da fail2ban'li bir makinede) kontrol listesi: jail listesi, ban, unban, stop/start,
  reload, log akışı, regex testi, ignoreip ekle/sil. Çıktılar/ekran görüntüleri rapora.
- **İnceleme: var.**

### T. v0.1.0 tag ve kurulum doğrulaması (Aşama 3; A, B, R, C `done`)

- **Kapsam:** main'deki `docs/DEGISIKLIKLER.md`'ye v0.1.0 maddesi, `git tag -a v0.1.0 -m "lazyjail v0.1.0"`
  (main HEAD), `git push origin v0.1.0`. Release CI'sinin sonucunu **bu işin kabul ölçütü olarak** tek
  bloklayıcı çağrıyla bekle (`gh run watch <id> --exit-status`).
- **Bitti:** release yayınlanmış (draft değil, "Latest"); asset'ler: 3 tar.gz, 3 .deb, `checksums.txt`,
  `install.sh`; temiz `ubuntu:24.04` container'ında README'deki tek komut `lazyjail --version` → `lazyjail 0.1.0`;
  `.deb` yolu da aynı. Release linki rapora. Bozuksa tag'e dokunma, düzeltip `v0.1.1`.
- **İnceleme: yok** (tag izni kullanıcıdan alındı; yalnız doğrulama).

## 7. README kurulum metni (R işi birebir kullanır)

````markdown
## Kurulum (Ubuntu / Debian, amd64 · arm64 · armv7)

Son sürümü indirir, mimariyi seçer, SHA-256 doğrular ve `/usr/local/bin/lazyjail`'e kurar:

```sh
curl -fsSL https://github.com/batv45/lazyjail/releases/latest/download/install.sh | sudo sh
```

Scripti çalıştırmadan önce okumak isterseniz:

```sh
curl -fsSLO https://github.com/batv45/lazyjail/releases/latest/download/install.sh
less install.sh && sudo sh install.sh
```

Script kullanmadan, aynı işi elle:

```sh
case "$(uname -m)" in x86_64) A=amd64;; aarch64) A=arm64;; armv7l) A=armv7;; *) echo "desteklenmeyen mimari"; exit 1;; esac
cd "$(mktemp -d)" && U=https://github.com/batv45/lazyjail/releases/latest/download \
  && curl -fsSLO "$U/lazyjail_linux_$A.tar.gz" -O "$U/checksums.txt" \
  && sha256sum -c --ignore-missing checksums.txt \
  && tar xzf "lazyjail_linux_$A.tar.gz" lazyjail \
  && sudo install -m 0755 lazyjail /usr/local/bin/lazyjail && lazyjail --version
```

Belirli bir sürüm: `curl -fsSL https://github.com/batv45/lazyjail/releases/latest/download/install.sh | sudo LAZYJAIL_VERSION=v0.1.0 sh`

**Güncelleme:** kurulum komutunu yeniden çalıştırın.
**Kaldırma:** `sudo rm /usr/local/bin/lazyjail`

### .deb paketi

```sh
A=$(dpkg --print-architecture); [ "$A" = armhf ] && A=armv7
curl -fsSLO "https://github.com/batv45/lazyjail/releases/latest/download/lazyjail_$A.deb"
sudo apt install "./lazyjail_$A.deb"     # güncelleme: aynı komutlar
sudo apt remove lazyjail                 # kaldırma
```

`.deb` binary'yi `/usr/bin/lazyjail`'e koyar. İki yöntemden yalnız birini kullanın; ikisi birden kuruluysa
`/usr/local/bin` önce gelir.

### Çalıştırma

```sh
sudo lazyjail          # fail2ban socket'i root ister
lazyjail --demo        # root'suz, sahte verilerle deneme
```
````

## 8. Riskler ve açık sorular

| Risk | Etki | Azaltma |
|---|---|---|
| og-rek protokol 5 / istisna nesnelerini çözemez | Socket yolu çalışmaz | A'nın ilk adımı spike; başarısızsa exec + parse yedeği, B/C etkilenmez |
| Tag sonrası release CI bozulur | Asset yok, "latest" yanlış | CI'da her push'ta snapshot provası (R); bozulursa `v0.1.1` |
| Coder makinesinde Docker yok | Entegrasyon testi yerelde koşmaz | CI `integration.yml` sonucu; rapora yazılır |
| A ve B aynı anda `errors.go`, DEĞİŞİKLİKLER ekler | Merge çakışması | §5 birebir içerik; changelog çakışması elle, küçük |
| Repo Multica repo listesinde değil | `multica repo checkout` başarısız (bu işte görüldü) | Coder `git clone https://github.com/batv45/lazyjail` ile çalışır; ya da Müdür repoyu listeye ekletir |

## Kullanıcı kararı gerekiyor

- **Repo sahibi:** Repo `batv45/lazyjail` altında; çalışma alanı kuralı yeni repoları `TeknoLab45` altında
  istiyor. Kurulum URL'leri ve Go modül yolu (`github.com/batv45/lazyjail`) sahibe bağlı, bu yüzden karar
  v0.1.0'dan **önce** verilmeli. Seçenekler: (a) `batv45`'te kalsın, (b) v0.1.0'dan önce `TeknoLab45`'e taşınsın
  (taşımayı kullanıcı onaylar; modül yolu ve URL'ler R işinde değişir; GitHub eski URL'leri yönlendirir).
  **Öneri: (a)**, aksi söylenmezse böyle ilerlenir.
