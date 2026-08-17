import os
import sys
import shutil
import subprocess
from xsync_core.tui import print_err, print_info

def check_dependencies():
    if not shutil.which("sshpass"):
        print_err("Khong tim thay sshpass. Cai dat: sudo apt install sshpass")
        sys.exit(1)
    if not shutil.which("rsync"):
        print_err("Khong tim thay rsync. Cai dat: sudo apt install rsync")
        sys.exit(1)

def get_ssh_control_path(host):
    return f"/tmp/rsync-ctrl-{host}"

def setup_ssh_master(host, password, control_path):
    if os.path.exists(control_path):
        try:
            os.remove(control_path)
        except OSError:
            pass
            
    cmd = [
        "ssh",
        "-o", "ControlMaster=yes",
        "-o", "ControlPersist=10m",
        "-o", "ControlPath=" + control_path,
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", "BatchMode=no",
        "-c", "aes128-gcm@openssh.com",
        "-o", "Compression=no",
        "-o", "IPQoS=throughput",
        "-Nf", host
    ]
    
    env = os.environ.copy()
    if password:
        cmd = ["sshpass", "-e"] + cmd
        env["SSHPASS"] = password
        
    res = subprocess.run(cmd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    if res.returncode != 0:
        err_msg = res.stderr.decode("utf-8", errors="ignore")
        return False, err_msg
    return True, ""

def cleanup_ssh_master(host, control_path):
    if os.path.exists(control_path):
        cmd = [
            "ssh",
            "-o", "ControlPath=" + control_path,
            "-O", "exit",
            host
        ]
        subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            os.remove(control_path)
        except OSError:
            pass

def run_dry_run(mode, local_dir, host, remote_dir, filter_file, delete):
    control_path = get_ssh_control_path(host)
    ssh_cmd = f"ssh -o ControlMaster=auto -o ControlPersist=10m -o ControlPath={control_path} -c aes128-gcm@openssh.com -o Compression=no -o IPQoS=throughput"
    
    cmd = [
        "rsync", "-avW", "--partial", "--info=progress2",
        "-e", ssh_cmd, "--dry-run"
    ]
    
    if filter_file:
        cmd.append(f"--filter=merge {filter_file}")
    if delete and mode == "push":
        cmd.append("--delete")
        
    if mode == "push":
        src = local_dir.rstrip("/") + "/"
        dest = f"{host}:{remote_dir.rstrip('/')}/"
    else:
        src = f"{host}:{remote_dir.rstrip('/')}/"
        dest = local_dir.rstrip("/") + "/"
        os.makedirs(local_dir, exist_ok=True)
        
    cmd.extend([src, dest])
    
    try:
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=1, text=True)
        for line in p.stdout:
            print("  " + line, end="")
        p.wait()
        return p.returncode == 0
    except KeyboardInterrupt:
        print("\n  [WARN] Dry-run bi dung boi nguoi dung.")
        return False
