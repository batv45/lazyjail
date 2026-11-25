# Sürüm çıkarma

## Prova (her push/PR'da CI)

`ci.yml` içindeki `release-dry-run` işi: `goreleaser check`, `goreleaser release --snapshot --clean`,
`shellcheck scripts/install.sh`, sonra `dist/` yerel HTTP ile sunulur ve `install.sh`
`LAZYJAIL_BASE_URL=http://127.0.0.1:8000 INSTALL_DIR=...` ile snapshot asset'lerine karşı çalışır.

Yerelde (docker): `docker run --rm -v $PWD:/src -w /src --entrypoint sh goreleaser/goreleaser:latest -c 'git config --global --add safe.directory /src; goreleaser release --snapshot --clean'`

## Tag atma

1. Tüm işler main'de, CI yeşil. `docs/DEGISIKLIKLER.md`'ye sürüm maddesi eklenir.
2. `git tag -a vX.Y.Z -m "lazyjail vX.Y.Z" && git push origin vX.Y.Z`
3. `release.yml` çalışır ve yayınlanmış (draft olmayan) release üretir. `-rc` gibi ekli tag'ler pre-release olur
   ve "latest" sayılmaz.

## Asset'ler (sabit adlar)

`lazyjail_linux_{amd64,arm64,armv7}.tar.gz`, `lazyjail_{amd64,arm64,armv7}.deb`, `checksums.txt`, `install.sh`.
Böylece `https://github.com/batv45/lazyjail/releases/latest/download/<ad>` API'siz çalışır.

## Bozuk release

Tag silinmez/taşınmaz. Sorun düzeltilir ve bir sonraki yama sürümü (`v0.1.1`) atılır.
