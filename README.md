# Công cụ đồng bộ dữ liệu xsync (Push & Pull)

`xsync` là công cụ dòng lệnh (CLI / TUI) hiệu năng cao được viết bằng **Go (Golang)**, tự động hoá việc đồng bộ thư mục/file hai chiều giữa máy cục bộ (Local) và máy chủ từ xa (Remote Server) thông qua giao thức SSH và `rsync`.

### Tính năng nổi bật:
- **Đóng gói nhị phân độc lập (Single binary):** Không cần cài đặt môi trường Go hay Python để sử dụng.
- **Hỗ trợ đa nền tảng:** Chạy mượt mà trên Linux, macOS và Windows (PowerShell, Git Bash, WSL).
- **Đồng bộ song song (Parallel sync):** Phân luồng nhiều tiến trình `rsync` chạy đồng thời, tối ưu băng thông mạng.
- **SSH Master Socket:** Tái sử dụng phiên SSH đã xác thực, giảm tối đa độ trễ bắt tay kết nối.
- **Danh sách trắng (Whitelist):** Lọc chính xác các thư mục/tập tin cần đẩy lên (`xsync.push.ini`) hoặc tải về (`xsync.pull.ini`).
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

> **Lưu ý:** Script cài đặt sẽ tự động nhận diện hệ điều hành và kiến trúc CPU, tải bản binary phù hợp, lưu vào thư mục hệ thống và tự cấu hình biến môi trường `PATH`.

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
xsync
```

Giao diện tương tác sẽ hướng dẫn bạn từng bước:
1. **Chọn SSH host:** Dùng phím mũi tên `↑` / `↓` chọn máy chủ từ danh sách `~/.ssh/config`.
2. **Chế độ đồng bộ:** Đẩy dữ liệu (Push) hoặc Tải dữ liệu (Pull).
3. **Tùy chọn `--delete` (cho Push):** Xóa file thừa trên server nếu không có trong danh sách đẩy.
4. **Dry-Run (Chạy thử):** Quét và hiển thị trước danh sách file thay đổi kèm log chi tiết mà không ảnh hưởng dữ liệu.
5. **Xác nhận thực thi:** Chạy truyền tải song song đa luồng và hiển thị dashboard trực quan.

---

## 5. Tài liệu kỹ thuật

- [Các trường hợp xử lý ngoại lệ và lỗi trong xsync](docs/xu_ly_ngoai_le_va_loi.md)