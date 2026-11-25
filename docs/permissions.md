# Yetki ve çalışma modları

- fail2ban socket'i varsayılan olarak yalnız root'a açıktır. Gerçek backend TEK-452 ile bağlanır; o zamana kadar yalnız `lazyjail --demo` çalışır.
- `--sudo`: root değilseniz uygulama `sudo -- lazyjail <aynı bayraklar>` ile kendini yeniden çalıştırır (`syscall.Exec`, TTY korunur, şifreyi sudo sorar). `--demo` ile birlikte gerekmez, yeniden çalıştırılmaz.
- `--read-only`: ban, unban, start/stop ve reload tuşları kapanır; kazara işleme karşı.
- `--demo`: sahte, bellek içi veri; root ve fail2ban gerekmez.
- Hata ekranları: kurulu değil / çalışmıyor / izin yok. `r` tekrar dener.
- `NO_COLOR` ya da `--no-color`: renksiz tema.
- Ban kutusunda girilen IP, `SSH_CONNECTION` içindeki istemci IP'si ise iki kez onay istenir.
