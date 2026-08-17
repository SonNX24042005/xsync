# Công cụ Đồng bộ Dữ liệu Rsync (Push & Pull)

Script bash này là một công cụ giúp tự động hoá việc đồng bộ thư mục/file hai chiều giữa máy cục bộ (Local) và máy chủ từ xa (Remote Server) thông qua giao thức SSH. Đồng thời, nó hỗ trợ tính năng chọn lọc danh sách trắng (whitelist) các thư mục và tập tin để quá trình tải lên hoặc tải xuống được hiệu quả và bảo mật nhất.

## 1. Yêu cầu cài đặt (Prerequisites)

Để đảm bảo script hoạt động hoàn hảo, hệ điều hành (Ubuntu/Debian hoặc hệ thống Linux tương tự) của bạn cần phải cài đặt sẵn `rsync` và `sshpass`. Bạn có thể kiểm tra hoặc cài đặt chúng bằng câu lệnh sau:

```bash
sudo apt update
sudo apt install rsync sshpass -y
```

## 2. Cấu hình ban đầu (.env_rsync)

Script lấy các thông tin đăng nhập và đường dẫn thư mục thông qua file cấu hình `.env_rsync`. 

Vui lòng tạo một định dạng file tên là `.env_rsync` tại chung thư mục chứa file `run.sh`. Bạn có thể sao chép các cấu trúc mẫu phía dưới và điền thông tin tương ứng cho hệ thống của bạn:

```sh
# ==========================================
# FILE CẤU HÌNH: .env_rsync
# ==========================================

# Mật khẩu để đăng nhập SSH vào server
SSH_PASSWORD="YOUR_SSH_PASSWORD"

# Đường dẫn thư mục gốc ở máy Local (khi đẩy dữ liệu - Push)
LOCAL_DIR="/path/to/your/local/directory"

# Tên người dùng SSH
REMOTE_USER="root"

# Địa chỉ IP hoặc Domain của Server
REMOTE_HOST="192.168.1.100"

# Cổng truy cập SSH (mặc định là 22 nếu không điền)
REMOTE_PORT="22"

# Đường dẫn thư mục đích trên Server (khi đẩy dữ liệu - Push)
REMOTE_DIR="/path/to/your/remote/directory"

# Đường dẫn thư mục nguồn trên Server (khi tải dữ liệu - Pull)
REMOTE_PULL_DIR="/path/to/your/remote/pull/directory"

# Đường dẫn thư mục nhận file ở máy cấu bộ (khi tải dữ liệu - Pull)
LOCAL_PULL_DIR="/path/to/your/local/pull/directory"
```

> **Lưu ý**: Hãy chắc chắn các giá trị được khai báo bên trong ngoặc kép `" "` không chứa khoảng trống thừa thãi hay ký tự đặc biệt vô nghĩa.

## 3. Cấu hình danh sách trắng (Whitelist)

Công cụ rsync này sử dụng hai tệp cấu hình riêng biệt để tuỳ chỉnh việc chia sẻ dữ liệu nào dành cho tải lên hay tải xuống nhằm tăng tốc độ cũng như bảo vệ hệ thống.

- **`.rsyncinclude.push`**: Danh sách thư mục/tập tin nằm trong `LOCAL_DIR` mà bạn muốn ĐẨY LÊN Server.
- **`.rsyncinclude.pull`**: Danh sách thư mục/tập tin nằm trong `REMOTE_PULL_DIR` mà bạn muốn TẢI VỀ Local.

**Cách viết đường dẫn:**
Đường dẫn phải bắt đầu tính từ bên trong không gian thư mục gốc. Bạn có thể sử dụng ký tự đại diện `**` hoặc `*` để chọn toàn bộ dữ liệu con bên trong. Từng thành phần phân cách bằng dòng mới, ví dụ cấu hình `push`:

```text
data/datasets/datasets_detect_pose/**
trainning/train_detect_pose/**
```

**Lưu ý:** Nếu tệp tin `include` tương ứng trống rỗng hoặc không tồn tại, script mặc định sẽ bỏ qua kiểm tra danh sách trắng và đồng bộ **TOÀN BỘ** mọi thứ nằm trong thư mục gốc.

## 4. Cách sử dụng script `run.sh`

**Bước 1:** Phân quyền thực thi chuẩn cho script trước khi tiến hành khởi chạy (chỉ cần làm một lần duy nhất).

```bash
chmod +x run.sh
```

**Bước 2:** Chạy lệnh bên dưới và làm theo chỉ dẫn lựa chọn ở trên màn hình terminal:

```bash
./run.sh
```

**Chi tiết menu điều hướng script cung cấp cho bạn các tuỳ chọn sau:**
1. **Chế độ đồng bộ:** Bạn cần điền phím tắt số `1` (Đẩy dữ liệu), `2` (Tải dữ liệu) hoặc `3` (Thoát).
2. **Kích hoạt đồng bộ xoá (Tuỳ chọn `--delete` - chỉ dành cho thao tác Push):** Khi chọn số `1` (Có), nếu một file đang tồn tại trên Server mà không nằm trong danh sách chuẩn truyền ở Local, nó sẽ bị ***xoá bỏ*** vĩnh viễn trên Server để đảm bảo cấu trúc nội dung Remote giống y hệt (mirror) thư mục Local. Để an toàn, hãy chọn `2` (Không).
3. **Dry-Run (Chạy Thử):** Script sẽ mặc định chạy giả lập tiến trình và liệt kê toàn bộ các thay đổi vào danh sách mà không tác động trực tiếp tới hệ thống. Việc này mang tính chất rà soát sai sót cực kỳ hoàn hảo.
4. **Xác nhận thực thi:** Sau khi quá trình Dry-run hoàn tất, script sẽ hỏi bạn có bước sang lệnh áp dụng thật (số `1`) hay thoát ngay lập tức để điều chỉnh lại (số `2`).

---
_Mẹo: Kết nối SSH của bạn đã được tối ưu hóa tính năng ControlMaster. Hệ thống sẽ duy trì phiên đăng nhập trong thời gian làm việc để tránh việc phải tạo lại xác thực gây chậm tiến độ._