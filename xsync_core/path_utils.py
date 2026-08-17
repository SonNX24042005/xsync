import os

def process_include_paths(raw_lines, mode, local_dir, remote_dir):
    """
    Cleans paths: strips absolute path prefixes matching either local_dir or remote_dir,
    removes comments, and appends a trailing slash for directories in push mode.
    """
    processed = []
    local_dir = os.path.normpath(local_dir) if local_dir else ""
    remote_dir = os.path.normpath(remote_dir) if remote_dir else ""
    
    for line in raw_lines:
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        # Strip quotes (terminal sometimes adds them when dragging files)
        if (line.startswith('"') and line.endswith('"')) or (line.startswith("'") and line.endswith("'")):
            line = line[1:-1].strip()
            
        norm_line = os.path.normpath(line)
        
        # Check remote_dir prefix
        if remote_dir and (norm_line.startswith(remote_dir + os.sep) or norm_line.startswith(remote_dir.replace("\\", "/") + "/")):
            rel = norm_line[len(remote_dir) + 1:]
        elif remote_dir and norm_line == remote_dir:
            rel = "."
        # Check local_dir prefix
        elif local_dir and (norm_line.startswith(local_dir + os.sep) or norm_line.startswith(local_dir.replace("\\", "/") + "/")):
            rel = norm_line[len(local_dir) + 1:]
        elif local_dir and norm_line == local_dir:
            rel = "."
        else:
            rel = line
            
        # Clean leading slashes or dots from relative path
        rel = rel.lstrip("/").lstrip("\\")
        
        # If it is a directory locally and does not end with /, append /
        # This only applies to push mode as we check local filesystems
        if mode == "push" and local_dir:
            full_path = os.path.join(local_dir, rel)
            if not rel.endswith("/") and os.path.isdir(full_path):
                rel = rel + "/"
                
        processed.append(rel)
    return processed

def build_include_filter(include_lines):
    """
    Constructs an rsync merge filter list from whitelist paths.
    For each path 'a/b/c':
      - adds '+ a/'
      - adds '+ a/b/'
      - adds '+ a/b/c' or '+ a/b/c/**' (if directory)
    Finally appends '- *' to exclude everything else.
    """
    filters = []
    roots = set()
    
    for line in include_lines:
        line = line.strip()
        if not line or line.startswith("#"):
            continue
            
        # Strip leading '+' or '-' prefix if user typed it
        if line.startswith("+ ") or line.startswith("- "):
            line = line[2:].strip()
            
        # Strip leading ./
        if line.startswith("./"):
            line = line[2:]
            
        # Normalize double slashes
        while "//" in line:
            line = line.replace("//", "/")
            
        if line.startswith("/"):
            # Ignore absolute paths
            continue
            
        is_dir = False
        if line.endswith("/"):
            is_dir = True
            line = line[:-1]
            
        if not line:
            continue
            
        parts = [p for p in line.split("/") if p]
        if not parts:
            continue
            
        # Add parent directories
        current = ""
        for p in parts[:-1]:
            current = f"{current}/{p}" if current else p
            filters.append(f"+ {current}/")
            
        # Add target path
        if is_dir:
            filters.append(f"+ {line}/")
            filters.append(f"+ {line}/**")
        else:
            filters.append(f"+ {line}")
            
        roots.add(parts[0])
        
    if not roots:
        return None
        
    # Deduplicate while preserving order
    seen = set()
    deduped_filters = []
    for f in filters:
        if f not in seen:
            seen.add(f)
            deduped_filters.append(f)
            
    deduped_filters.append("- *")
    return deduped_filters
