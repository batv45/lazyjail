# Değişiklikler

Yeni maddeler en üstte. Biçim: tarih, issue no, ajan, özet.

## 2026-11-25 · TEK-448 · Lead

- `docs/PLAN-MVP.md`: kalan MVP işleri (A backend, B TUI, R release/kurulum, C log/regex/ignoreip, T v0.1.0 tag), release ve Ubuntu kurulum planı; TEK-187/TEK-192 inceleme bulguları.

## 2026-11-22 · TEK-192 · CoderAgent

- Public repo iskeleti: Go modülü (`github.com/batv45/lazyjail`), MIT lisansı.
- `cmd/lazyjail`: yalnızca `--version`. TUI henüz yok.
- `internal/f2b/backend.go`: fail2ban erişim arayüzü (`Backend`) ve tipleri, [PLAN.md §3.2](PLAN.md) sözleşmesine göre. Uygulama yok.
- Derleme ve kalite: `Makefile` (`build`, `test`, `test-race`, `vet`, `fmt`, `lint`), `.golangci.yml`.
- CI: `.github/workflows/ci.yml` (gofmt, vet, golangci-lint, race testi, üç hedef için derleme).
- Dağıtım: `.goreleaser.yaml` (linux amd64/arm64/armv7 tar.gz + .deb, `recommends: fail2ban`, release taslak). Sürüm etiketi ve yayın yok.
- Dokümanlar: `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `docs/PLAN.md`, `docs/README.md`.
