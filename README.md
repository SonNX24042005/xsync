# Công cụ đồng bộ dữ liệu xsync (Push & Pull)

`xsync` là công cụ dòng lệnh (CLI / TUI) hiệu năng cao được viết bằng **Go (Golang)**, tự động hoá việc đồng bộ thư mục/file hai chiều giữa máy cục bộ (Local) và máy chủ từ xa (Remote Server) thông qua giao thức SSH và `rsync`.

### Tính năng nổi bật:
- **Đóng gói nhị phân độc lập (Single binary):** Không cần cài đặt môi trường Go hay Python để sử dụng.
- **Hỗ trợ đa nền tảng:** Chạy mượt mà trên Linux, macOS và Windows (PowerShell, Git Bash, WSL).
- **Đồng bộ song song (Parallel sync):** Phân luồng nhiều tiến trình `rsync` chạy đồng thời, tối ưu băng thông mạng.
- **SSH Master Socket:** Tái sử dụng phiên SSH đã xác thực, giảm tối đa độ trễ bắt tay kết nối.
- **Danh sách trắng (Whitelist):** Lọc chính xác các thư mục/tập tin cần đẩy lên (`xsync.push.ini`) hoặc tải về (`xsync.pull.ini`).
- **Hỗ trợ đa ngôn ngữ (i18n):** Lựa chọn tiếng Anh hoặc tiếng Việt ngay khi cài đặt, tự động lưu cấu hình và hỗ trợ cờ trợ giúp `-h` / `--help`.
- **Giao diện điều hướng hiện đại:** Điều hướng bằng phím mũi tên `↑` / `↓` mượt mà qua Bubble Tea.
- **Live Dashboard:** Theo dõi tiến độ, tốc độ truyền tải của từng luồng theo thời gian thực trên terminal.

---

## 1. Yêu cầu hệ thống (Prerequisites)

*   **Linux (Ubuntu / Debian):**
    ```bash
    sudo apt update && sudo apt install -y rsync sshpass
    ```
*   **macOS (Homebrew):**
    ```bash
    brew install rsync esolitos/ipa/sshpass
    ```
*   **Windows:**
    *   Cài đặt `rsync` qua **Scoop** (Khuyên dùng):
        ```powershell
        scoop install rsync
        ```
    *   Hoặc sử dụng sẵn bên trong **Git Bash** / **WSL (Windows Subsystem for Linux)**.

---

## 2. Cài đặt (Installation)

### Trên Linux & macOS:
Chạy một câu lệnh duy nhất dưới đây trên terminal:
```bash
curl -fsSL https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.sh | bash
```

### Trên Windows (PowerShell):
Mở PowerShell và chạy lệnh:
```powershell
irm https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.ps1 | iex
```

> **Lưu ý:** Script cài đặt hoàn toàn độc lập, không yêu cầu cài đặt Go compiler. Script cho phép lựa chọn ngôn ngữ (tiếng Anh hoặc tiếng Việt), tự lưu cấu hình vào `~/.config/xsync/config.ini`, tự cài đặt các phụ thuộc (`rsync`, `sshpass`), tải bản binary phù hợp từ GitHub Releases/kho lưu trữ và tự cấu hình biến môi trường `PATH`.

---

## 3. Cấu hình

### A. Cấu hình máy chủ SSH (`~/.ssh/config`)
`xsync` tự động nhận diện các host đã cấu hình trong file `~/.ssh/config`. Ví dụ:

```text
Host my-server
    HostName 192.168.1.100
    Port 22
    User root
```

### B. Cấu hình thư mục đồng bộ (`xsync.ini`)
Tự động được tạo mẫu khi gọi lệnh `xsync`, hoặc bạn có thể tự tạo tại thư mục làm việc:

```ini
[my-server]
ssh_password = YOUR_SSH_PASSWORD
remote_dir = /path/to/remote/directory

[settings]
default_profile = my-server
```

### C. Cấu hình danh sách trắng (Whitelist)
- **`xsync.push.ini`**: Danh sách thư mục/tập tin ở máy cục bộ muốn ĐẨY LÊN server.
- **`xsync.pull.ini`**: Danh sách thư mục/tập tin ở server muốn TẢI VỀ máy cục bộ.

Ví dụ nội dung file:
```text
data/datasets/
models/checkpoint.pt
scripts/train.py
```

---

## 4. Cách sử dụng

Sau khi cài đặt xong, bạn có thể gọi lệnh `xsync` tại bất kỳ thư mục nào:

```bash
# Khởi chạy giao diện tương tác
xsync

# Xem hướng dẫn trợ giúp theo ngôn ngữ đã lưu
xsync -h
xsync --help

# Chạy với ngôn ngữ khác tạm thời
xsync --lang en -h
xsync --lang vi

# Thay đổi ngôn ngữ mặc định toàn cục
xsync --set-lang en
xsync --set-lang vi
```

Giao diện tương tác sẽ hướng dẫn bạn từng bước:
1. **Chọn SSH host:** Dùng phím mũi tên `↑` / `↓` chọn máy chủ từ danh sách `~/.ssh/config`.
2. **Chế độ đồng bộ:** Đẩy dữ liệu (Push) hoặc Tải dữ liệu (Pull).
3. **Tùy chọn `--delete` (cho Push):** Xóa file thừa trên server nếu không có trong danh sách đẩy.
4. **Dry-Run (Chạy thử):** Quét và hiển thị trước danh sách file thay đổi kèm log chi tiết mà không ảnh hưởng dữ liệu.
5. **Xác nhận thực thi:** Chạy truyền tải song song đa luồng và hiển thị dashboard trực quan.

---

## 5. Giao diện web local

```bash
xsync ui
# Hoặc sau khi build trong dự án:
./bin/xsync ui
```

Ứng dụng mở trình duyệt tại `127.0.0.1:8787`, với giao diện chọn server, SSH key/mật khẩu, duyệt thư mục local/remote, chọn dữ liệu, xem trước và quản lý phiên. Giao diện được đóng gói trong binary, không cần cài Node.js để sử dụng.

- Nhấn **Tạo phiên mới** để chọn server → thư mục A/B → dữ liệu con → xác nhận.
- Tạo phiên và xem chi tiết dùng trang chính với chữ, nút và khung thư mục lớn. Nút **Tạo thư mục** mở ô nhập ngay trong trình duyệt thư mục remote; chỉ thao tác nhập mật khẩu tạm mới dùng hộp thoại.
- Trang **Máy chủ** hiển thị dấu **Đã dùng key** và **Đã lưu mật khẩu** cho từng host, đồng thời cho phép lưu/thay mật khẩu hoặc ngừng dùng mật khẩu cho kết nối mới. Mật khẩu từ `xsync.ini` và các phiên đã lưu cũng được nhận diện.
- Nút **Kiểm tra key** xác thực SSH riêng, không phụ thuộc phiên bản rsync trên server. Kết nối bằng key trong trình tạo phiên cũng ghi nhớ xác thực thành công, kể cả khi bước kiểm tra rsync sau đó thất bại. Dấu key thể hiện lịch sử thành công, không bảo đảm server luôn kết nối được.
- Nút **Kiểm tra kết nối** ưu tiên SSH key, rồi thử mật khẩu đã lưu nếu key không thành công. Kết quả báo riêng đăng nhập SSH và khả năng dùng rsync trên local/server; không tạo phiên hoặc đồng bộ dữ liệu. Cả hai nút kiểm tra đều có thể xác thực SSH khi local chưa có rsync.
- Cấu hình máy chủ lưu tại `~/.config/xsync/servers.json` (quyền `600`, thư mục `700` trên Unix). Mật khẩu không được mã hóa; API đọc chỉ trả trạng thái và nguồn lưu. Cấu hình trên trang này ưu tiên hơn cấu hình cũ; ngừng dùng mật khẩu bỏ qua nguồn cũ cho kết nối mới, không xóa mật khẩu trong các phiên hoặc tệp cũ. Các phiên đã tạo giữ cấu hình xác thực riêng.
- Trình duyệt thư mục có tìm kiếm tức thì, bộ nhớ đệm, nhập đường dẫn và tạo thư mục remote. Danh sách lớn chỉ vẽ các hàng đang được nhìn thấy.
- Các phiên hiển thị trạng thái, thời điểm đồng bộ và log; có nút bắt đầu/dừng.
- Bật **Tự chạy lại sau khi khởi động máy** để dùng dịch vụ hệ điều hành. Nếu bỏ chọn, phiên do tiến trình `xsync ui` quản lý; đóng tab vẫn đồng bộ, dừng tiến trình UI sẽ dừng các phiên tạm đó.
- Không lưu mật khẩu vẫn có thể chạy phiên tạm; tự khởi động cần SSH key hoặc mật khẩu đã lưu.

`xsync ui --port 0` chọn một cổng trống; `--no-open` chỉ in đường dẫn mà không mở trình duyệt. Khi mở lần đầu, dùng đường dẫn được lệnh in ra để xác thực giao diện. Máy chủ UI chỉ lắng nghe trên loopback và không cung cấp quyền truy cập qua mạng LAN.

## 6. Phiên đồng bộ thư mục liên tục

Chạy `xsync session new`, hoặc chọn **Phiên đồng bộ thư mục liên tục** trong menu `xsync`. Chế độ phiên cần rsync từ 3.1.3 ở cả local và server.

1. Chọn server từ `~/.ssh/config` (hỗ trợ cả `Include`).
2. Chọn SSH key/agent hoặc mật khẩu. Key không yêu cầu `sshpass`. Mật khẩu có thể được lưu trong cấu hình riêng của tài khoản để chạy nền; nếu không lưu thì phiên chạy trong terminal hiện tại.
3. Chọn thư mục local **A** và thư mục chứa **B** trên server. Đích đồng bộ luôn là **B/A**, ví dụ `/home/me/project` → `server:/srv/backups/project/`.
4. Chọn các file/thư mục con cần phản chiếu, hoặc chọn toàn bộ A. Các file mới trong thư mục đã chọn cũng được đồng bộ.
5. Xem trước thay đổi, rồi xác nhận lưu và chạy nền. **Thao tác xóa trong phần dữ liệu đã chọn ở local cũng được áp dụng lên server.** Dữ liệu ngoài phần đã chọn được giữ lại.

Trong giao diện duyệt thư mục: `↑`/`↓` di chuyển, `Enter` mở, `Backspace` lên thư mục cha, `/` tìm tức thì, `g` nhập đường dẫn, `r` tải lại, `q` hủy. Nhấn `Space` để chọn thư mục hiện tại trong bước chọn A/B. Khi chọn dữ liệu con, `Space` chọn/bỏ một mục, `a` chọn cả thư mục đang mở, `Tab` hoàn tất. Trên server có thêm `n` để tạo thư mục chứa mới. Danh sách remote được tải bất đồng bộ và lưu đệm khi quay lại.

```bash
xsync session list
xsync session status <id>
xsync session logs <id>
xsync session preview <id>
xsync session stop <id>    # Dừng dịch vụ và tắt tự khởi động
xsync session select <id>  # Chọn lại dữ liệu khi phiên đã dừng
xsync session start <id>   # Chạy nền và bật lại tự khởi động
xsync session run <id>     # Chạy trong terminal; Ctrl+C để dừng
```

Trên Linux, phiên dùng dịch vụ systemd của tài khoản và bật linger để tự chạy khi boot, kể cả trước khi đăng nhập. Máy cần có systemd user manager; nếu tài khoản không có quyền bật linger, lệnh sẽ báo cách khắc phục và giữ lại cấu hình phiên. Trên macOS/Windows, phiên dùng LaunchAgent/Scheduled Task để tự chạy khi đăng nhập tài khoản. SSH key có passphrase cần agent đã mở khóa; để chạy trước đăng nhập cần một phương thức xác thực dùng được không tương tác.

Cấu hình phiên nằm trong `~/.config/xsync/sessions/<id>/`. Mật khẩu được lưu dạng văn bản trong `session.json` với quyền `0600`, thư mục `0700` trên Unix; mật khẩu không được đặt trong dòng lệnh hay log. Xem [tài liệu phiên đồng bộ](docs/phien_dong_bo.md) để biết cơ chế theo dõi, phạm vi xóa và giới hạn nền tảng.

## 7. Tài liệu kỹ thuật

- [Các trường hợp xử lý ngoại lệ và lỗi trong xsync](docs/xu_ly_ngoai_le_va_loi.md)
- [Cơ chế hoạt động](docs/co_che_hoat_dong.md)
- [Phiên đồng bộ thư mục liên tục](docs/phien_dong_bo.md)
