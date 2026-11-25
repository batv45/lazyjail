# Değişiklikler

Yeni maddeler en üstte. Biçim: tarih, issue no, ajan, özet.

## 2026-11-25 · TEK-449 · Coder

- fail2ban backend: `internal/f2b/proto` (pickle kodek, `og-rek`), `socket` (ban, unban, durum, config, ignoreip, stop), `execbk` (`start`, `reload`, `fail2ban-regex`), `real` (birleşik `Backend`), `caps.go`, `errors.go`, `internal/validate`.
- `internal/f2b/fake`: gerçek protokolü konuşan sahte sunucu (hata/gecikme/asılı sunucu enjeksiyonu).
- Entegrasyon testi: `test/integration` (`-tags integration`), `make integration BASE=...`, `.github/workflows/integration.yml` (ubuntu:22.04, ubuntu:24.04, debian:12).
- Gerçek sunucuda doğrulandı: `stop <jail>` jail'i siler, `start <jail>` geri getirmez; `StartJail` tam `reload`'a düşer (bkz. `docs/architecture.md`).
- `docs/architecture.md`, `docs/testing.md`; `scripts/record-pickles.py`.

## 2026-11-25 · TEK-451 · Coder

- Release hattı: `.goreleaser.yaml` (`draft: false`, `prerelease: auto`, sabit asset adları `lazyjail_linux_<arch>.tar.gz` / `lazyjail_<arch>.deb`, arşive LICENSE+README, `install.sh` release asset'i).
- `.github/workflows/release.yml` (`v*` tag'inde goreleaser); `ci.yml`'e `release-dry-run` işi (goreleaser check + snapshot, shellcheck, `install.sh` snapshot'a karşı). golangci-lint sürümü sabitlendi (v2.1.6).
- `scripts/install.sh`: mimari seçimi, SHA-256 doğrulama, `/usr/local/bin` kurulumu.
- README Kurulum bölümü; `docs/releasing.md`.
- `docs/DEĞİŞİKLİKLER.md` → `docs/DEGISIKLIKLER.md` (Türkçe karaktersiz ad); `make lint` artık `gofmt -s -l`.

## 2026-11-25 · TEK-450 · Coder

- `internal/ui`: Bubble Tea TUI çekirdeği. Ana ekran (jail listesi, banlı IP'ler, salt okunur config paneli, log şeridi yer tutucusu), ban/unban (çoklu seçim, `U` ile tüm jail'lerden, iki onay), jail start/stop/reload, global reload, `/` arama, `?` yardım, hata ekranları (`errors.Is` ile: kurulu değil, çalışmıyor, izin yok), dar terminal düzeni, periyodik yenileme.
- `internal/ui/keys.go` tek kısayol kaynağı; `theme.go` ANSI 16 renk + `NO_COLOR`/`--no-color`.
- `internal/f2b/errors.go`: ortak hata değerleri ve `CommandError` (PLAN-MVP §5, birebir).
- `internal/f2b/mock`: bellek içi Backend (`--demo` ve UI testleri).
- `cmd/lazyjail`: `--demo`, `--read-only`, `--sudo` (sudo ile yeniden çalıştırma), `--socket`, `--refresh`, `--no-color`, `--version`. Gerçek backend bağlanana kadar `--demo` dışı çalıştırma açıklayıcı mesajla çıkar (TEK-452).
- Ban kutusunda `SSH_CONNECTION` IP'si uyarısı (iki onay).
- Testler: `internal/ui` golden (100×30: ana, boş, dar, yardım, 3 hata ekranı) + akış testleri; `cmd` testleri.
- `make demo`; `docs/keybindings.md`, `docs/permissions.md`, `docs/img/demo.png`, `docs/img/demo-help.png`.
- Kısıt: `systemctl start fail2ban` kısayolu (`S`) ve kapsayıcı yeniden bağlanma üstel bekleme yok; teatest yerine doğrudan model sürülen golden testler kullanıldı.

## 2026-11-22 · TEK-192 · CoderAgent

- Public repo iskeleti: Go modülü (`github.com/batv45/lazyjail`), MIT lisansı.
- `cmd/lazyjail`: yalnızca `--version`. TUI henüz yok.
- `internal/f2b/backend.go`: fail2ban erişim arayüzü (`Backend`) ve tipleri, [PLAN.md §3.2](PLAN.md) sözleşmesine göre. Uygulama yok.
- Derleme ve kalite: `Makefile` (`build`, `test`, `test-race`, `vet`, `fmt`, `lint`), `.golangci.yml`.
- CI: `.github/workflows/ci.yml` (gofmt, vet, golangci-lint, race testi, üç hedef için derleme).
- Dağıtım: `.goreleaser.yaml` (linux amd64/arm64/armv7 tar.gz + .deb, `recommends: fail2ban`, release taslak). Sürüm etiketi ve yayın yok.
- Dokümanlar: `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `docs/PLAN.md`, `docs/README.md`.
