# Cơ chế hoạt động và kiến trúc dự án xsync

Tài liệu này giải thích chi tiết về kiến trúc tổng quan, vai trò từng module, luồng xử lý và các kỹ thuật tối ưu hóa trong công cụ `xsync`.

---

## 1. Tổng quan dự án

`xsync` là công cụ dòng lệnh (CLI / TUI) viết bằng Python, đóng vai trò như một lớp điều phối và tối ưu hóa cấp cao cho `rsync` và `OpenSSH`. 

Mục tiêu chính của công cụ là tự động hóa việc đồng bộ dữ liệu hai chiều giữa máy cục bộ (local) và máy chủ từ xa (remote server) với tốc độ cao, hỗ trợ lọc danh sách trắng (whitelist), tái sử dụng phiên kết nối SSH và phân luồng đồng bộ song song đa tiến trình (parallel multi-process rsync) kèm giao diện theo dõi thời gian thực.

---

## 2. Kiến trúc tổng thể và vai trò các module

Cấu trúc mã nguồn của dự án được tổ chức dạng module hóa:

```
xsync/
├── xsync                    # File thực thi chính (entry point & điều phối workflow)
├── xsync.ini                # Cấu hình profile SSH và đường dẫn thư mục remote
├── xsync.push.ini           # Danh sách trắng (whitelist) các thư mục/file khi push
├── xsync.pull.ini           # Danh sách trắng (whitelist) các thư mục/file khi pull
├── docs/
│   └── co_che_hoat_dong.md  # Tài liệu giải thích cơ chế hoạt động
└── xsync_core/
    ├── __init__.py
    ├── config.py            # Đọc ~/.ssh/config, tìm kiếm và quản lý xsync.ini
    ├── sync.py              # Khởi tạo SSH master socket và chạy dry-run
    ├── parallel.py          # Bộ điều phối đồng bộ song song đa luồng & live dashboard
    ├── path_utils.py        # Chuẩn hóa đường dẫn và sinh rsync merge filter
    └── tui.py               # Tiện ích giao diện terminal (menu, checklist, editor)
```

### Chi tiết các module:

*   **`xsync`**: Điểm nhập chính (entry point) của chương trình. Quản lý vòng đời chạy, xử lý tự động cài đặt/cập nhật vào `~/.local/bin`, điều hướng menu tương tác và tự động dọn dẹp tài nguyên khi thoát qua hook `atexit`.
*   **`xsync_core/config.py`**: Phân tích file `~/.ssh/config` để trích xuất danh sách SSH host, tìm kiếm file cấu hình gần nhất (`find_config_nearest`), đọc và ghi cấu hình profile máy chủ (`load_profiles`, `save_profile`).
*   **`xsync_core/path_utils.py`**: Chuẩn hóa đường dẫn tương đối/tuyệt đối từ input người dùng và chuyển đổi danh sách whitelist thành các quy tắc `rsync merge filter` (`build_include_filter`).
*   **`xsync_core/sync.py`**: Kiểm tra phụ thuộc hệ thống (`rsync`, `sshpass`), thiết lập kết nối SSH Master socket nền (`setup_ssh_master`), thực hiện chạy thử nghiệm (`run_dry_run`).
*   **`xsync_core/parallel.py`**: Trọng tâm xử lý hiệu năng cao: quét danh sách file cần truyền/xóa, chia nhỏ danh sách thành nhiều chunk, kích hoạt song song N tiến trình `rsync` và cập nhật giao diện tiến độ realtime trên terminal.
*   **`xsync_core/tui.py`**: Cung cấp các thành phần giao diện dòng lệnh: bảng màu ANSI, menu lựa chọn bằng số, checklist nhiều mục và tích hợp mở trình soạn thảo (`nano`, `vim`) khi cần nhập danh sách tệp.

---

## 3. Luồng hoạt động chi tiết (workflow)

Toàn bộ quy trình thực thi của công cụ diễn ra qua các bước tuần tự:

```mermaid
flowchart TD
    A["Khởi động xsync"] --> B["Kiểm tra cài đặt & phụ thuộc (rsync, sshpass)"]
    B --> C["Đọc ~/.ssh/config & chọn SSH host"]
    C --> D["Thiết lập kết nối SSH Master Control Socket"]
    D --> E["Chọn chế độ: Push / Pull & tùy chọn --delete"]
    E --> F["Phân tích danh sách whitelist (xsync.push.ini / xsync.pull.ini)"]
    F --> G["Chạy thử (Dry-Run) hiển thị danh sách thay đổi"]
    G --> H{"Người dùng xác nhận chạy thật?"}
    H -- Không --> I["Thoát & dọn dẹp"]
    H -- Có --> J["Phân chia file thành N chunks & chạy song song N luồng rsync"]
    J --> K["Hiển thị live dashboard tiến độ từng luồng"]
    K --> L["Hỏi lưu cấu hình & đóng kết nối SSH Master"]
```

### Bước 1: Khởi động và tự động cài đặt
*   Khi chạy lần đầu, script kiểm tra đường dẫn thực thi. Nếu chưa nằm trong `~/.local/share/xsync`, nó sẽ đề xuất cài đặt và tạo symlink vào `~/.local/bin/xsync`, đồng thời tự động cập nhật biến môi trường `PATH` trong file cấu hình shell (`.bashrc`, `.zshrc` hoặc `.profile`).

### Bước 2: Lựa chọn SSH profile và kết nối SSH Master
*   Hàm `parse_ssh_hosts` đọc file `~/.ssh/config` để hiển thị menu các host đã cấu hình.
*   Sau khi chọn host, hàm `setup_ssh_master` khởi tạo một tiến trình SSH chạy nền mở **Unix Domain Socket** tại `/tmp/rsync-ctrl-<host>` với các tham số tối ưu:
    *   `ControlMaster=yes`, `ControlPersist=10m`: Duy trì kết nối socket sẵn sàng trong 10 phút.
    *   `-c aes128-gcm@openssh.com`: Sử dụng thuật toán mã hóa nhẹ, tận dụng tăng tốc phần cứng AES-NI.
    *   `Compression=no`: Tắt nén SSH để giảm tải CPU khi truyền dữ liệu dung lượng lớn.
    *   `IPQoS=throughput`: Thiết lập gói tin ưu tiên thông lượng tối đa.

### Bước 3: Phân tích đường dẫn và tạo bộ lọc danh sách trắng (whitelist)
*   Công cụ tìm kiếm và đọc file whitelist tương ứng (`xsync.push.ini` khi push hoặc `xsync.pull.ini` khi pull).
*   Nếu file chưa tồn tại, giao diện sẽ mở trình soạn thảo (`nano`/`vim`) để người dùng nhập đường dẫn.
*   Hàm `build_include_filter` sinh danh sách quy tắc filter chuẩn cho `rsync`:
    *   Mở đường dẫn cho tất cả thư mục cha: `+ parent/`, `+ parent/sub/`.
    *   Thêm quy tắc cho đối tượng đích: `+ target/` & `+ target/**` (nếu là thư mục) hoặc `+ target` (nếu là file).
    *   Thêm quy tắc loại trừ tất cả các file còn lại ở cuối: `- *`.

### Bước 4: Chạy thử nghiệm (dry-run)
*   Hàm `run_dry_run` thực thi lệnh `rsync` với cờ `--dry-run` thông qua SSH socket đã mở.
*   Toàn bộ danh sách tệp sẽ được thêm mới, cập nhật hoặc xóa (nếu chọn `--delete`) được hiển thị ra màn hình để người dùng kiểm tra trước khi áp dụng thật.

### Bước 5: Cơ chế đồng bộ song song (parallel sync engine)
1.  **Quét danh sách thay đổi:** Chạy rsync dry-run phân tích ngầm để thu thập danh sách file cần truyền (`files_to_transfer`) và file cần xóa (`files_to_delete`).
2.  **Xử lý xóa trước (pre-deletion):** Nếu tùy chọn `--delete` được kích hoạt:
    *   Chế độ push: Gom nhóm các file cần xóa (chunk 300 files) và thực thi lệnh `rm -rf` từ xa qua SSH channel một lần.
    *   Chế độ pull: Xóa trực tiếp trên máy cục bộ.
3.  **Chia chunk danh sách file:** Chia đều mảng `files_to_transfer` thành $N$ phần (mặc định 4 luồng/thread) và ghi tạm vào các file `_chunk_i.txt`.
4.  **Khởi chạy đa tiến trình song song:**
    *   Khởi tạo $N$ tiến trình `subprocess.Popen` chạy `rsync` với tham số `--files-from=_chunk_i.txt`.
    *   Tất cả $N$ tiến trình cùng chia sẻ một SSH control socket duy nhất, không tốn thời gian xác thực lại nhiều lần.
5.  **Live dashboard:** 
    *   Mỗi tiến trình được gắn với 2 background thread (`stdout_reader_thread` và `stderr_reader_thread`).
    *   Output `--info=progress2` được parse bằng regex để lấy phần trăm hoàn thành, tốc độ truyền tải và file đang đồng bộ.
    *   Sử dụng mã điều khiển con trỏ ANSI (`\033[NF`, `\033[K`) để vẽ lại thanh tiến trình (progress bar) của từng luồng theo thời gian thực trên terminal.

### Bước 6: Hoàn tất và dọn dẹp tài nguyên
*   Nếu người dùng có chỉnh sửa cấu hình trong phiên chạy, hệ thống sẽ hiện checklist hỏi có muốn lưu lại vào `xsync.ini`, `xsync.push.ini` hay `xsync.pull.ini` hay không.
*   Hàm `cleanup_ssh_master` đóng SSH socket bằng lệnh `ssh -O exit` và xóa socket file trong `/tmp`.

---

## 4. Tóm tắt các điểm tối ưu kỹ thuật nổi bật

1.  **SSH multiplexing:** Tận dụng `ControlMaster` của OpenSSH giúp giảm độ trễ bắt tay kết nối (TCP + SSH handshake) xuống bằng 0 cho tất cả các tiến trình rsync con.
2.  **Phân luồng đa tiến trình:** Giải quyết triệt để điểm nghẽn đơn luồng (single-thread bottleneck) của `rsync` khi chuyển hàng chục nghìn file hoặc các file lớn.
3.  **Bộ lọc danh sách trắng tự động:** Tự động giải quyết cây thư mục cha (`parent directory traversal`) để rsync không bỏ sót đường dẫn khi áp dụng whitelist.
4.  **An toàn dữ liệu:** Luôn có bước dry-run và xác nhận rõ ràng trước khi ghi đè hoặc xóa bất kỳ file nào.
