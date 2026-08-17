import os
import sys
import re
import shutil
import subprocess
import threading
import time
import tempfile

progress_pattern = re.compile(
    r"^\s*([\d,]+)\s+(\d+)%\s+([\d\.]+\w+/s)\s+[\d:]+\s+\(xfr#(\d+),\s+(?:to|ir|re)-chk=(\d+)/(\d+)\)"
)

def parse_line(line, job_id, progress_dict):
    match = progress_pattern.match(line)
    if match:
        percent = int(match.group(2))
        speed = match.group(3)
        xfer = int(match.group(4))
        total = int(match.group(6))
        
        progress_dict[job_id]["percent"] = percent
        progress_dict[job_id]["speed"] = speed
        progress_dict[job_id]["xfer"] = xfer
        progress_dict[job_id]["total"] = total
        progress_dict[job_id]["status"] = "Syncing"
    else:
        if not (line.startswith("sending ") or 
                line.startswith("receiving ") or 
                line.startswith("sent ") or 
                line.startswith("total size") or 
                line.startswith("speedup is") or
                line.startswith("building file list") or
                "bytes/sec" in line or
                "ir-chk" in line or
                "to-chk" in line):
            progress_dict[job_id]["current_file"] = line

def stdout_reader_thread(proc, job_id, progress_dict):
    buffer = b""
    while True:
        char = proc.stdout.read(1)
        if not char:
            break
        if char == b"\r" or char == b"\n":
            line = buffer.decode("utf-8", errors="ignore").strip()
            buffer = b""
            if line:
                parse_line(line, job_id, progress_dict)
        else:
            buffer += char
    
    progress_dict[job_id]["status"] = "Finished"
    progress_dict[job_id]["percent"] = 100
    progress_dict[job_id]["speed"] = "0 B/s"

def stderr_reader_thread(proc, job_id, errors_list):
    err = proc.stderr.read()
    if err:
        errors_list.append((job_id, err.decode("utf-8", errors="ignore")))

def run_parallel_sync(mode, local_dir, host, remote_dir, filter_file, delete, threads=4):
    ssh_control_path = f"/tmp/rsync-ctrl-{host}"
    ssh_cmd = f"ssh -o ControlMaster=auto -o ControlPersist=10m -o ControlPath={ssh_control_path} -c aes128-gcm@openssh.com -o Compression=no -o IPQoS=throughput"
    
    rsync_base = [
        "rsync",
        "-aW",
        "--partial",
        "-e", ssh_cmd
    ]

    # Source & Destination setup
    if mode == "push":
        src = local_dir.rstrip("/") + "/"
        dest = f"{host}:{remote_dir.rstrip('/')}/"
    else:
        src = f"{host}:{remote_dir.rstrip('/')}/"
        dest = local_dir.rstrip("/") + "/"

    print("  [INFO] Dang quet danh sach tep thay doi (Dry-run)...")
    dry_run_cmd = rsync_base + ["-v", "--dry-run"]
    if filter_file:
        dry_run_cmd.append(f"--filter=merge {filter_file}")
    if delete and mode == "push":
        dry_run_cmd.append("--delete")
    dry_run_cmd += [src, dest]

    proc = subprocess.Popen(dry_run_cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    stdout, stderr = proc.communicate()
    
    if proc.returncode != 0:
        print(f"  [ERR] Dry-run that bai: {stderr.decode('utf-8', errors='ignore')}")
        return False

    lines = stdout.decode("utf-8", errors="ignore").splitlines()
    files_to_transfer = []
    files_to_delete = []

    for line in lines:
        line = line.strip()
        if not line:
            continue
        if line.startswith("deleting "):
            files_to_delete.append(line[len("deleting "):])
        elif (line.startswith("sending incremental") or 
              line.startswith("receiving incremental") or 
              line.startswith("sent ") or 
              line.startswith("total size") or 
              line.startswith("speedup is") or 
              line.startswith("created directory ") or
              "bytes/sec" in line):
            continue
        elif line.endswith("/"):
            continue
        else:
            files_to_transfer.append(line)

    print(f"  [INFO] Tim thay {len(files_to_transfer)} tep can truyen tai, {len(files_to_delete)} tep can xoa.")

    # 2. PERFORM DELETIONS FIRST IF ENABLED
    if delete and files_to_delete:
        print("  [INFO] Dang xoa cac tep thua tren dich...")
        if mode == "pull":
            # Local deletions
            for f in files_to_delete:
                local_path = os.path.join(local_dir, f)
                if os.path.isdir(local_path):
                    shutil.rmtree(local_path, ignore_errors=True)
                elif os.path.exists(local_path):
                    os.remove(local_path)
        else:
            # Remote deletions via SSH channel
            chunk_size = 300
            for i in range(0, len(files_to_delete), chunk_size):
                chunk = files_to_delete[i:i+chunk_size]
                quoted_paths = " ".join(f"'{os.path.join(remote_dir, f)}'" for f in chunk)
                ssh_del_cmd = [
                    "ssh",
                    "-o", f"ControlPath={ssh_control_path}",
                    host,
                    f"rm -rf {quoted_paths}"
                ]
                subprocess.run(ssh_del_cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        print("  [OK] Da hoan tat xoa tep thua.")

    if not files_to_transfer:
        print("  [OK] Khong co tep nao can truyen tai. Dong bo hoan tat!")
        return True

    # 3. SPLIT FILES INTO CHUNKS
    num_threads = min(threads, len(files_to_transfer))
    chunks = [[] for _ in range(num_threads)]
    for idx, f in enumerate(files_to_transfer):
        chunks[idx % num_threads].append(f)

    chunk_files = []
    for i in range(num_threads):
        temp_f = tempfile.NamedTemporaryFile(mode="w", suffix=f"_chunk_{i}.txt", delete=False)
        for path in chunks[i]:
            temp_f.write(path + "\n")
        temp_f.close()
        chunk_files.append(temp_f.name)

    # 4. LAUNCH PARALLEL RSYNC PROCESSES
    print(f"  [INFO] Dang khoi chay {num_threads} luong truyen tai song song...")
    processes = []
    progress_dict = {}
    reader_threads = []
    errors_list = []

    for i in range(num_threads):
        progress_dict[i] = {
            "percent": 0,
            "speed": "0 B/s",
            "xfer": 0,
            "total": len(chunks[i]),
            "current_file": "Dang ket noi...",
            "status": "Waiting"
        }
        
        cmd = rsync_base + [
            "-v",
            "--info=progress2",
            f"--files-from={chunk_files[i]}",
            src,
            dest
        ]
        
        proc = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            bufsize=0
        )
        processes.append(proc)

        t_out = threading.Thread(target=stdout_reader_thread, args=(proc, i, progress_dict))
        t_out.daemon = True
        t_out.start()
        reader_threads.append(t_out)

        t_err = threading.Thread(target=stderr_reader_thread, args=(proc, i, errors_list))
        t_err.daemon = True
        t_err.start()
        reader_threads.append(t_err)

    # 5. LIVE PROGRESS DASHBOARD LOOP
    print("════════════════════════════════════════════════════════════════")
    print(" TIEN TRINH DONG BO PHAN LUONG (PARALLEL SYNC)")
    print("════════════════════════════════════════════════════════════════")
    for _ in range(num_threads):
        print()

    try:
        while True:
            all_finished = True
            for i in range(num_threads):
                if progress_dict[i]["status"] != "Finished":
                    all_finished = False
                    break

            sys.stdout.write(f"\033[{num_threads}F")
            
            for i in range(num_threads):
                job = progress_dict[i]
                percent = job["percent"]
                bar_width = 15
                filled = int(percent / 100 * bar_width)
                bar = "█" * filled + "░" * (bar_width - filled)
                
                file_info = job["current_file"]
                if len(file_info) > 30:
                    file_info = "..." + file_info[-27:]
                elif not file_info:
                    file_info = "-"
                
                if job["status"] == "Finished":
                    sys.stdout.write(f"\033[KLuong {i+1:02d}: [███████████████] 100% (Hoan thanh) - {job['total']}/{job['total']} tep\n")
                else:
                    sys.stdout.write(f"\033[KLuong {i+1:02d}: [{bar}] {percent:3d}% ({job['speed']}) - {file_info}\n")
            
            sys.stdout.flush()
            if all_finished:
                break
            time.sleep(0.2)
    except KeyboardInterrupt:
        print("\n  [WARN] Da dung truyen tai boi nguoi dung.")
        for proc in processes:
            if proc:
                proc.terminate()
        return False
    finally:
        for path in chunk_files:
            if path and os.path.exists(path):
                try:
                    os.remove(path)
                except OSError:
                    pass

    has_errors = False
    for i, proc in enumerate(processes):
        proc.wait()
        if proc.returncode != 0:
            has_errors = True

    if has_errors:
        print("════════════════════════════════════════════════════════════════")
        print("  [WARN] Co luong truyen tai that bai. Chi tiet loi:")
        for job_id, err_msg in errors_list:
            print(f"Luong {job_id+1}:")
            print(err_msg.strip())
        return False
    else:
        print("════════════════════════════════════════════════════════════════")
        print("  [OK] Dong bo tat ca cac luong hoan tat thanh cong!")
        return True
