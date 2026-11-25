#!/bin/sh
# lazyjail kurulum scripti: son (ya da belirtilen) sürümü indirir, SHA-256 doğrular, kurar.
# Ortam: LAZYJAIL_VERSION (örn. v0.1.0; boş = latest), LAZYJAIL_BASE_URL (test), INSTALL_DIR.
set -eu

REPO="batv45/lazyjail"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${LAZYJAIL_VERSION:-}"

die() {
	echo "install.sh: $*" >&2
	exit 1
}

for tool in curl sha256sum tar install uname mktemp; do
	command -v "$tool" >/dev/null 2>&1 || die "'$tool' gerekli ama bulunamadı"
done

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
armv7l | armv8l) arch=armv7 ;;
*) die "desteklenmeyen mimari: $(uname -m)" ;;
esac

if [ -n "${LAZYJAIL_BASE_URL:-}" ]; then
	base="$LAZYJAIL_BASE_URL"
elif [ -z "$VERSION" ]; then
	base="https://github.com/$REPO/releases/latest/download"
else
	base="https://github.com/$REPO/releases/download/$VERSION"
fi

archive="lazyjail_linux_$arch.tar.gz"

if [ ! -d "$INSTALL_DIR" ] || [ ! -w "$INSTALL_DIR" ]; then
	die "$INSTALL_DIR yazılabilir değil; sudo ile çalıştırın (ör. ... | sudo sh)"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "İndiriliyor: $base/$archive"
curl -fsSL -o "$tmp/$archive" "$base/$archive" || die "arşiv indirilemedi"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "checksums.txt indirilemedi"

grep " $archive\$" "$tmp/checksums.txt" >"$tmp/check.txt" || die "checksums.txt içinde $archive yok"
(cd "$tmp" && sha256sum -c check.txt >/dev/null) || die "SHA-256 doğrulaması başarısız; kurulum iptal edildi"

tar -xzf "$tmp/$archive" -C "$tmp" lazyjail || die "arşiv açılamadı"
install -m 0755 "$tmp/lazyjail" "$INSTALL_DIR/lazyjail"

"$INSTALL_DIR/lazyjail" --version
