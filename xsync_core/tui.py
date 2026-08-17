import os
import sys
import tempfile
import subprocess


C_RESET = '\033[0m'
C_INFO = '\033[36m'   # Cyan
C_OK = '\033[32m'     # Green
C_WARN = '\033[33m'   # Yellow
C_ERR = '\033[31m'    # Red
C_ACCENT = '\033[35m' # Magenta

def print_header(title):
    print(f"\n{C_ACCENT}=== {title} ==={C_RESET}")

def print_info(msg):
    print(f"  {C_INFO}[INFO]{C_RESET} {msg}")

def print_ok(msg):
    print(f"  {C_OK}[OK]{C_RESET}   {msg}")

def print_warn(msg):
    print(f"  {C_WARN}[WARN]{C_RESET} {msg}")

def print_err(msg):
    print(f"  {C_ERR}[ERR]{C_RESET}  {msg}")

def print_kv(key, val):
    print(f"  {C_INFO}{key:<20}{C_RESET} {val}")

def prompt_menu(title, options, default_idx=None):
    """
    Displays a numbered menu and prompts for selection.
    If default_idx is provided and the user presses Enter, returns default_idx.
    """
    print_header(title)
    for idx, opt in enumerate(options, 1):
        suffix = " (Mac dinh)" if default_idx is not None and idx - 1 == default_idx else ""
        print(f"  {idx}) {opt}{suffix}")
    
    hint = f" [Mac dinh: {default_idx + 1}]" if default_idx is not None else ""
    while True:
        try:
            choice = input(f"  Nhap lua chon (1-{len(options)}){hint}: ").strip()
            if not choice:
                if default_idx is not None:
                    return default_idx
                continue
            choice_idx = int(choice) - 1
            if 0 <= choice_idx < len(options):
                return choice_idx
            print_warn(f"Lua chon khong hop le. Nhap tu 1 den {len(options)}.")
        except ValueError:
            print_warn("Vui long nhap mot so.")
        except KeyboardInterrupt:
            print("\n  [INFO] Huy bo boi nguoi dung.")
            sys.exit(130)

def prompt_checklist(title, items):
    """
    Items is a list of tuples: (key, label, default_on_boolean)
    Returns list of selected keys.
    """
    selected = [item[2] for item in items]
    while True:
        print_header(title)
        for idx, item in enumerate(items, 1):
            status = "[X]" if selected[idx-1] else "[ ]"
            print(f"  {status} {idx}) {item[1]}")
        print("\n  Nhap so de chon/bo chon (vi du: '1 2'), hoac an Enter de xac nhan.")
        try:
            choice = input("  Lua chon: ").strip()
            if not choice:
                break
            parts = choice.split()
            for p in parts:
                if p.isdigit():
                    idx = int(p) - 1
                    if 0 <= idx < len(items):
                        selected[idx] = not selected[idx]
        except KeyboardInterrupt:
            print("\n  [INFO] Huy bo boi nguoi dung.")
            sys.exit(130)
    return [items[i][0] for i, sel in enumerate(selected) if sel]



def edit_text_in_editor(content, filename_suffix="_whitelist"):
    """
    Launches editor on a temp file with content and returns the updated content.
    """
    # Create temp file
    fd, temp_path = tempfile.mkstemp(suffix=filename_suffix, text=True)
    try:
        with os.fdopen(fd, 'w', encoding='utf-8') as f:
            f.write(content)
            
        # Find editor
        editor = os.environ.get("EDITOR", "")
        if not editor:
            # Check availability
            for candidate in ["nano", "vim", "vi"]:
                if subprocess.run(["which", candidate], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
                    editor = candidate
                    break
        if not editor:
            editor = "nano" # fallback
            
        # Run editor
        res = subprocess.run([editor, temp_path])
        if res.returncode != 0:
            print_err(f"Editor {editor} error. Dat lai mac dinh.")
            return content
            
        # Read back
        with open(temp_path, 'r', encoding='utf-8') as f:
            updated = f.read()
        return updated
    finally:
        if os.path.exists(temp_path):
            os.remove(temp_path)

def open_file_in_editor(file_path):
    """
    Opens a file directly using the system editor.
    """
    editor = os.environ.get("EDITOR", "")
    if not editor:
        for candidate in ["nano", "vim", "vi"]:
            if subprocess.run(["which", candidate], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
                editor = candidate
                break
    if not editor:
        editor = "nano"
    subprocess.run([editor, file_path])
