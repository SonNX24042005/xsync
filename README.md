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

Các lệnh cài đặt trên tải binary từ **GitHub Releases mới nhất**, không nhất thiết là commit mới nhất trên `main`. Installer hiển thị phiên bản đã cài. Bản có sẵn trong `bin/` của thư mục hiện tại không được ưu tiên.

Push lên `main` tự động tăng phiên bản patch, ví dụ `v1.1.0` → `v1.1.1`, rồi build cho Linux/macOS/Windows trên amd64/arm64. Release chỉ được công bố và đặt làm mới nhất sau khi đủ cả 6 binary và tệp SHA-256. Trong lúc workflow đang chạy, installer tiếp tục dùng release hoàn chỉnh trước đó. Có thể tạo tag phiên bản riêng nếu cần tăng minor/major.

Hoặc build từ mã nguồn hiện tại:

```bash
go build -o bin/xsync ./cmd/xsync
./bin/xsync
```

Bản tự build từ mã nguồn hiển thị `dev` cùng mã commit; binary phát hành hiển thị phiên bản release.

Chế độ đồng bộ cần OpenSSH và rsync từ 3.1.3 ở cả local và remote. Git ở local hỗ trợ nhận diện đổi tên/di chuyển; installer thử cài Git nếu thiếu. Remote không cần Git. Nếu local chưa có Git, phiên vẫn đồng bộ bằng rsync và ghi cảnh báo rằng tối ưu đổi tên chưa hoạt động. Mật khẩu cần sshpass ở local; SSH key không cần sshpass. Giao diện web được nhúng trong binary, không cần Node.js khi sử dụng.

Khai báo alias máy chủ trong `~/.ssh/config`, ví dụ:

```sshconfig
Host my-server
    HostName 192.168.1.100
    User me
    IdentityFile ~/.ssh/id_ed25519
```

## Menu terminal

Chỉ cần chạy `xsync`, sau đó chọn **Phiên đồng bộ**, **Tạo phiên mới**, **Máy chủ**, **Mở giao diện web** hoặc **Cài đặt**. Menu dùng `↑`/`↓`, `Enter`, `/` để tìm, `PgUp`/`PgDn`, `Home`/`End` và `Esc` để quay lại. Danh sách phiên/máy chủ tự làm mới mỗi 2 giây; nhấn `r` để làm mới ngay.

- Phiên đồng bộ: xem chi tiết/trạng thái, bắt đầu/dừng, xem log, xem trước thay đổi, chọn lại dữ liệu khi phiên đã dừng.
- Tiến độ: web hiển thị thanh tiến độ trong danh sách và trang chi tiết. Terminal hiển thị tiến độ trong danh sách, màn hình chi tiết tự cập nhật mỗi giây và khi chạy trực tiếp. Có trạng thái kết nối, quét, xử lý đổi tên, truyền dữ liệu và hoàn tất. Phần trăm, tốc độ và thời gian còn lại do rsync ước tính; dung lượng là dữ liệu đã xử lý, bao gồm các block tái sử dụng, không phải số byte đi qua mạng. Chỉ báo 100% sau khi toàn bộ lượt đồng bộ và mốc Git đã lưu thành công. Lỗi giữ tiến độ dở dang; lượt thử lại bắt đầu lại phép đo. Worker cũ cần dừng và bắt đầu lại để nhận tính năng này.
- Máy chủ: hiển thị **Đã dùng key** và **Đã lưu mật khẩu**, kiểm tra SSH key, kiểm tra kết nối SSH/rsync, lưu/thay hoặc ngừng dùng mật khẩu.
- Tạo phiên: chọn server và xác thực → thư mục A/B → dữ liệu → xem trước/xác nhận. Có thể dùng mật khẩu đã lưu mà không nhập lại.
- Duyệt thư mục: `Enter` mở, `Backspace` lên thư mục cha, `/` tìm, `g` nhập đường dẫn, `r` tải lại, `Space` chọn, `Tab` hoàn tất chọn dữ liệu. Remote hỗ trợ `n` tạo thư mục.
- Chạy nền không tự khởi động tiếp tục hoạt động sau khi thoát menu. Mật khẩu không lưu được giữ trong tiến trình worker; sau khi worker dừng cần nhập lại.
- Bắt đầu đồng bộ: chọn một phiên → **Bắt đầu đồng bộ** → chạy nền, chạy nền và tự khởi động, hoặc chạy trong terminal. Chọn **Dừng đồng bộ** để dừng phiên và tắt tự khởi động.
- Cài đặt: đổi và lưu ngôn ngữ, xem nơi lưu dữ liệu hoặc thông tin phiên bản.

```bash
xsync
xsync --help
xsync --version
```

## Giao diện web

Chạy `xsync` → **Mở giao diện web**. Trong menu này có thể chọn cổng và bật/tắt tự mở trình duyệt, rồi chọn **Khởi chạy giao diện web**. Cổng `0` tự chọn cổng trống. Mở **đầy đủ đường dẫn có `#token=...`** được in trong terminal; mã truy cập thay đổi khi khởi động lại UI và tab/trình duyệt mới cần đường dẫn đầy đủ. Nhấn `Ctrl+C` để dừng giao diện web và trở về menu.

Giao diện chỉ lắng nghe trên loopback. Các danh sách phiên, máy chủ, thư mục và log cuộn trong khung riêng; màn hình hẹp có nút chuyển local/remote, máy chủ/xác thực và cấu hình/xem trước. Tạo phiên và chi tiết phiên dùng trang chính; tạo thư mục nhập ngay trong khung remote. Nhập mật khẩu tạm dùng hộp thoại.

Terminal và web đọc/ghi chung các phiên, mật khẩu và dấu key. Kiểm tra key chỉ xác thực SSH; kiểm tra kết nối báo riêng SSH và khả năng đồng bộ bằng rsync. Dấu key ghi nhận lịch sử thành công, không bảo đảm máy luôn kết nối được.

Các phiên tạm do UI khởi chạy tiếp tục đồng bộ khi đóng tab, nhưng dừng khi tiến trình UI dừng. Worker nền do terminal khởi chạy tiếp tục sau khi thoát terminal. Cả hai đều xuất hiện trong cùng danh sách phiên.

## Đổi tên và di chuyển

Sau lần đồng bộ thành công đầu tiên, mỗi phiên có kho Git riêng tại `~/.config/xsync/sessions/<id>/changes.git`. Kho này chỉ lưu cấu trúc đường dẫn và SHA-256 của nội dung, không sao chép nội dung file, không tạo commit trong dự án và không sửa index/branch của kho Git sẵn có. Không có thao tác push lên GitHub.

- Git so sánh mốc đã đồng bộ với cấu trúc hiện tại và nhận diện file đổi đường dẫn có nội dung trùng hoàn toàn. xsync kiểm tra SHA-256 của bản remote, rồi chuyển file ngay trên remote nếu đích chưa tồn tại và các thư mục cha không phải symlink. Rsync tiếp tục xử lý phần dữ liệu mới/sửa/xóa.
- Đổi tên hoặc di chuyển thư mục con bên trong phạm vi đã chọn tận dụng lại các file không đổi. File vừa di chuyển vừa sửa vẫn được rsync cập nhật theo cách thông thường.
- Nếu đổi tên/di chuyển chính thư mục hoặc file đã chọn riêng bên trong nguồn A, xsync tìm đường dẫn mới khi có đúng một kết quả khớp toàn bộ nội dung và đường dẫn đó chưa tồn tại ở mốc trước. Sau khi đồng bộ thành công, lựa chọn trong phiên được cập nhật; tên cũ được dọn trong phạm vi cũ. Không tự chiếm một đích remote đã tồn tại ngoài phạm vi đang quản lý.
- Trường hợp trùng nội dung gây mơ hồ, hoặc mục đã chọn biến mất đồng thời xuất hiện đường dẫn mới nhưng nội dung khác, phiên báo lỗi và giữ dữ liệu remote để bạn dừng phiên rồi chọn lại dữ liệu. Xóa hẳn mục đã chọn mà không có đường dẫn mới khả nghi vẫn được phản chiếu.
- Xem trước chỉ hiển thị các ứng viên đổi tên; không chuyển file remote, không sửa lựa chọn hay cập nhật mốc Git. Lỗi truyền tải giữ nguyên mốc trước và ghi thông tin thao tác đang chờ để có thể thử lại.

Cơ chế này không tự tìm lại chính thư mục nguồn A nếu A bị đổi tên hoặc chuyển ra khỏi đường dẫn cấu hình. Nó cũng không tự theo dõi mục được chuyển ra ngoài A. Đây là tối ưu truyền dữ liệu và nhận diện đường dẫn, không phải kho lịch sử/backup nội dung hay cơ chế khôi phục phiên bản như Git.

## Lưu trữ

| Dữ liệu | Đường dẫn |
|---|---|
| Cấu hình và phần dữ liệu được chọn | `~/.config/xsync/sessions/<id>/session.json` |
| Trạng thái, log và điều khiển worker | Cùng thư mục của phiên |
| Mốc đổi tên/di chuyển | `changes.git`, `changes.json`, `changes-pending.json` trong thư mục phiên |
| Mật khẩu chung và lịch sử key | `~/.config/xsync/servers.json` |
| Ngôn ngữ | `~/.config/xsync/config.json` |
| Bản sao cấu hình cũ | `~/.config/xsync/migrations/<thời điểm>-<mã>/` |

Cấu hình xác thực của phiên đã tạo được giữ riêng; thay mật khẩu máy chủ áp dụng cho kết nối mới. Mật khẩu lưu dạng văn bản, không được trả về từ API đọc hay in trong list/status/log. Thư mục riêng dùng quyền `0700`, tệp dùng `0600` trên Unix. Ghi cấu hình máy chủ có khóa giữa các tiến trình để terminal và web không ghi đè thay đổi của nhau.

Luồng INI push/pull cũ đã được thay bằng phiên. Khi mở xsync tại một thư mục còn `xsync.ini`, `xsync.push.ini`, `xsync.pull.ini`, ứng dụng nhập mật khẩu chưa có vào kho chung, sao lưu và kiểm tra bản sao trước khi bỏ tệp cũ. Cấu hình ngôn ngữ INI cũng chuyển sang JSON. Không ghi đè mật khẩu đã cấu hình rõ ràng; không tự tạo hoặc chạy phiên từ whitelist cũ. Các phiên hiện có được giữ nguyên.

## Phạm vi đồng bộ và tự khởi động

Mỗi phiên lưu một trong hai chế độ:

- **Chỉ đồng bộ mục đã chọn**: đồng bộ file đã chọn và toàn bộ nội dung của thư mục đã chọn, bao gồm file mới bên trong. File/thư mục mới ở ngoài các mục này không được đồng bộ. Đây là chế độ mặc định của các phiên cũ.
- **Đồng bộ tất cả, trừ mục đã chọn**: các mục được chọn là danh sách loại trừ theo đường dẫn tương đối trong nguồn A. Thư mục bị loại trừ bao gồm toàn bộ nội dung bên trong, kể cả file mới. Mọi thao tác thêm, sửa, xóa ở phần còn lại đều được đồng bộ. Danh sách loại trừ rỗng nghĩa là đồng bộ toàn bộ, kể cả mục mới. Đổi tên một mục ra ngoài đường dẫn bị loại trừ sẽ đưa nó vào phạm vi đồng bộ.

Phản chiếu thao tác xóa chỉ tác động đến phần đang được đồng bộ. Dữ liệu remote bị loại trừ hoặc ngoài danh sách bao gồm được giữ nguyên; chọn loại trừ một mục đã từng đồng bộ không xóa bản remote của mục đó.

Web cho chọn chế độ tại bước **Dữ liệu** khi tạo phiên và hiển thị phạm vi ở bước xác nhận/trang chi tiết. Trong TUI, chọn chế độ khi tạo phiên, hoặc dừng phiên → **Chọn lại dữ liệu** để đổi chế độ và danh sách của phiên hiện có. Khi đổi chế độ, giao diện xóa lựa chọn đang nhập để chọn lại theo ý nghĩa mới; cấu hình đã lưu chỉ thay đổi sau khi hoàn tất. Chế độ và danh sách lưu trong `session.json`; mốc Git tách biệt theo phạm vi, không nhận diện đổi tên xuyên qua mục loại trừ.

Đích luôn là `B/A`. Lần đầu chạy ngay; thay đổi được gom khoảng 750 ms, đối soát mỗi 30 giây. Mỗi phiên chỉ chạy một lượt rsync tại một thời điểm.

Tự khởi động dùng systemd user + linger trên Linux, LaunchAgent trên macOS và Scheduled Task trên Windows. Key có passphrase cần SSH agent dùng được trong môi trường dịch vụ. Chạy trước đăng nhập trên Linux cần quyền bật linger.
