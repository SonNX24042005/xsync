# Công cụ đồng bộ dữ liệu xsync (Push & Pull)

`xsync` là công cụ dòng lệnh (CLI / TUI) hiệu năng cao được viết bằng **Go (Golang)**, tự động hoá việc đồng bộ thư mục/file hai chiều giữa máy cục bộ (Local) và máy chủ từ xa (Remote Server) thông qua giao thức SSH và `rsync`.

Dự án hỗ trợ:
- **Single binary**: Đóng gói thành 1 file nhị phân độc lập duy nhất.
- **Đồng bộ song song (Parallel sync)**: Phân luồng nhiều tiến trình `rsync` truyền tải đồng thời, tối ưu băng thông.
- **SSH Master Socket**: Tái sử dụng phiên SSH đã xác thực, giảm tối đa độ trễ.
- **Whitelist**: Lọc danh sách trắng các thư mục và tập tin để tải lên (`xsync.push.ini`) hoặc tải xuống (`xsync.pull.ini`).
- **Live Dashboard**: Hiển thị thanh tiến trình trực quan theo thời gian thực trên terminal.

---

## 1. Yêu cầu hệ thống (Prerequisites)

Hệ thống Linux/macOS cần cài đặt sẵn `rsync` và `sshpass`:

```bash
# Ubuntu / Debian
sudo apt update
sudo apt install rsync sshpass -y
```

Để biên dịch mã nguồn Go:
- Cài đặt Go (phiên bản 1.22 trở lên).

---

## 2. Cài đặt (Installation)

### Cách 1: Cài đặt nhanh bằng một câu lệnh `curl` (Khuyên dùng)

Chỉ cần chạy lệnh sau trên terminal (tự động nhận diện hệ điều hành, tải binary hoặc tự build từ mã nguồn Go):

```bash
curl -fsSL https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.sh | bash
```

Hoặc nếu đã tải mã nguồn về máy:

```bash
./install.sh
```

### Cách 2: Biên dịch thủ công bằng Makefile

```bash
# Biên dịch file binary vào thư mục bin/
make build

# Cài đặt file binary xsync vào ~/.local/bin/
make install

# Chạy kiểm thử đơn vị
make test
```

### Cách 3: Biên dịch trực tiếp bằng Go

```bash
go build -ldflags="-s -w" -o bin/xsync ./cmd/xsync
```

---

## 3. Cấu hình

### A. Cấu hình máy chủ SSH (`~/.ssh/config`)
`xsync` tự động đọc danh sách host trong file `~/.ssh/config`. Ví dụ:

```text
Host my-server
    HostName 192.168.1.100
    Port 22
    User root
```

### B. Cấu hình thư mục đồng bộ (`xsync.ini`)
Tạo file `xsync.ini` tại thư mục làm việc:

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

Sau khi cài đặt, bạn chỉ cần gõ lệnh `xsync` tại bất kỳ thư mục dự án nào:

```bash
xsync
```

Hoặc chạy file nhị phân trực tiếp:

```bash
./bin/xsync
```

Menu điều hướng sẽ hướng dẫn chi tiết từng bước:
1. **Chọn SSH host**: Chọn từ danh sách host trong `~/.ssh/config`.
2. **Chế độ đồng bộ**: Đẩy dữ liệu (Push) hoặc Tải dữ liệu (Pull).
3. **Tùy chọn `--delete` (cho Push)**: Xóa file thừa trên server nếu không có trong danh sách đẩy.
4. **Dry-Run (Chạy thử)**: Quét và hiển thị trước danh sách file sẽ thay đổi.
5. **Xác nhận thực thi**: Chạy truyền tải song song đa luồng và hiển thị tiến trình realtime.