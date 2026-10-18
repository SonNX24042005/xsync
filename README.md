# xsync

Đồng bộ liên tục từ thư mục local A đến `server:B/A` qua SSH và rsync. Quản lý cùng một kho phiên bằng menu terminal hoặc giao diện web.

## Cài đặt

Linux/macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.sh | bash
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.ps1 | iex
```

Hoặc build từ mã nguồn:

```bash
go build -o bin/xsync ./cmd/xsync
./bin/xsync
```

Chế độ đồng bộ cần OpenSSH và rsync từ 3.1.3 ở cả local và remote. Mật khẩu cần sshpass ở local; SSH key không cần sshpass. Giao diện web được nhúng trong binary, không cần Node.js khi sử dụng.

Khai báo alias máy chủ trong `~/.ssh/config`, ví dụ:

```sshconfig
Host my-server
    HostName 192.168.1.100
    User me
    IdentityFile ~/.ssh/id_ed25519
```

## Menu terminal

Chạy `xsync` để chọn **Phiên đồng bộ**, **Tạo phiên mới**, **Máy chủ** hoặc **Mở giao diện web**. Menu dùng `↑`/`↓`, `Enter`, `/` để tìm, `PgUp`/`PgDn`, `Home`/`End` và `Esc` để quay lại. Danh sách phiên/máy chủ tự làm mới mỗi 2 giây; nhấn `r` để làm mới ngay.

- Phiên đồng bộ: xem chi tiết/trạng thái, bắt đầu/dừng, xem log, xem trước thay đổi, chọn lại dữ liệu khi phiên đã dừng.
- Máy chủ: hiển thị **Đã dùng key** và **Đã lưu mật khẩu**, kiểm tra SSH key, kiểm tra kết nối SSH/rsync, lưu/thay hoặc ngừng dùng mật khẩu.
- Tạo phiên: chọn server và xác thực → thư mục A/B → dữ liệu → xem trước/xác nhận. Có thể dùng mật khẩu đã lưu mà không nhập lại.
- Duyệt thư mục: `Enter` mở, `Backspace` lên thư mục cha, `/` tìm, `g` nhập đường dẫn, `r` tải lại, `Space` chọn, `Tab` hoàn tất chọn dữ liệu. Remote hỗ trợ `n` tạo thư mục.
- Chạy nền không tự khởi động tiếp tục hoạt động sau khi thoát menu. Mật khẩu không lưu được giữ trong tiến trình worker; sau khi worker dừng cần nhập lại.

```bash
xsync session list
xsync session new
xsync session status <id>
xsync session background <id> # Chạy nền, không tự khởi động
xsync session start <id>      # Chạy nền và bật tự khởi động
xsync session stop <id>       # Dừng worker/dịch vụ
xsync session run <id>        # Chạy trong terminal
xsync session preview <id>
xsync session select <id>
xsync session logs <id>
xsync server list
xsync server check <host>
xsync server check-key <host>
xsync server password <host>  # Nhập mật khẩu ẩn
xsync server disable-password <host>
xsync config-path
```

## Giao diện web

Chạy `xsync ui`, sau đó mở **đầy đủ đường dẫn có `#token=...`** được in trong terminal. `xsync ui --port 0` chọn cổng trống; `--no-open` không tự mở trình duyệt. Mã truy cập thay đổi khi khởi động lại UI; tab/trình duyệt mới cần đường dẫn đầy đủ.

Giao diện chỉ lắng nghe trên loopback. Các danh sách phiên, máy chủ, thư mục và log cuộn trong khung riêng; màn hình hẹp có nút chuyển local/remote, máy chủ/xác thực và cấu hình/xem trước. Tạo phiên và chi tiết phiên dùng trang chính; tạo thư mục nhập ngay trong khung remote. Nhập mật khẩu tạm dùng hộp thoại.

Terminal và web đọc/ghi chung các phiên, mật khẩu và dấu key. Kiểm tra key chỉ xác thực SSH; kiểm tra kết nối báo riêng SSH và khả năng đồng bộ bằng rsync. Dấu key ghi nhận lịch sử thành công, không bảo đảm máy luôn kết nối được.

Các phiên tạm do UI khởi chạy tiếp tục đồng bộ khi đóng tab, nhưng dừng khi tiến trình UI dừng. Worker nền do terminal khởi chạy tiếp tục sau khi thoát terminal. Cả hai đều xuất hiện trong cùng danh sách phiên.

## Lưu trữ

| Dữ liệu | Đường dẫn |
|---|---|
| Cấu hình và phần dữ liệu được chọn | `~/.config/xsync/sessions/<id>/session.json` |
| Trạng thái, log và điều khiển worker | Cùng thư mục của phiên |
| Mật khẩu chung và lịch sử key | `~/.config/xsync/servers.json` |
| Ngôn ngữ | `~/.config/xsync/config.json` |
| Bản sao cấu hình cũ | `~/.config/xsync/migrations/<thời điểm>-<mã>/` |

Cấu hình xác thực của phiên đã tạo được giữ riêng; thay mật khẩu máy chủ áp dụng cho kết nối mới. Mật khẩu lưu dạng văn bản, không được trả về từ API đọc hay in trong list/status/log. Thư mục riêng dùng quyền `0700`, tệp dùng `0600` trên Unix. Ghi cấu hình máy chủ có khóa giữa các tiến trình để terminal và web không ghi đè thay đổi của nhau.

Luồng INI push/pull cũ đã được thay bằng phiên. Khi mở xsync tại một thư mục còn `xsync.ini`, `xsync.push.ini`, `xsync.pull.ini`, ứng dụng nhập mật khẩu chưa có vào kho chung, sao lưu và kiểm tra bản sao trước khi bỏ tệp cũ. Cấu hình ngôn ngữ INI cũng chuyển sang JSON. Không ghi đè mật khẩu đã cấu hình rõ ràng; không tự tạo hoặc chạy phiên từ whitelist cũ. Các phiên hiện có được giữ nguyên.

## Phạm vi đồng bộ và tự khởi động

Phiên phản chiếu cả thao tác xóa **trong phạm vi đã chọn**; dữ liệu remote ngoài phạm vi được giữ lại. Đích luôn là `B/A`. Lần đầu chạy ngay; thay đổi được gom khoảng 750 ms, đối soát mỗi 30 giây. Mỗi phiên chỉ chạy một lượt rsync tại một thời điểm.

Tự khởi động dùng systemd user + linger trên Linux, LaunchAgent trên macOS và Scheduled Task trên Windows. Key có passphrase cần SSH agent dùng được trong môi trường dịch vụ. Chạy trước đăng nhập trên Linux cần quyền bật linger.
