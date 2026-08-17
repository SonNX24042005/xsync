import os

def find_config_nearest(filename, max_depth=8):
    """
    Walks down the directory structure from PWD to find the file.
    Returns the directory path containing the file, or None if not found.
    Searches depth-first and sorts by depth (shallowest first).
    """
    pwd = os.getcwd()
    matches = []
    for root, dirs, files in os.walk(pwd):
        depth = root[len(pwd):].count(os.sep)
        if depth > max_depth:
            continue
        if filename in files:
            matches.append((depth, root))
    if matches:
        matches.sort(key=lambda x: x[0])
        return matches[0][1]
    return None

import configparser

def parse_ssh_hosts():
    """
    Parses ~/.ssh/config and returns a list of configured Host names.
    Ignores wildcards (*, ?) and hosts containing 'github'.
    """
    ssh_config_path = os.path.expanduser("~/.ssh/config")
    hosts = []
    if not os.path.exists(ssh_config_path):
        return hosts
        
    try:
        with open(ssh_config_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                parts = line.split()
                if len(parts) >= 2 and parts[0].lower() == "host":
                    for host in parts[1:]:
                        if "*" in host or "?" in host:
                            continue
                        if "github" in host.lower():
                            continue
                        if host not in hosts:
                            hosts.append(host)
    except Exception:
        pass
    return sorted(hosts)

def load_profiles(env_file_path):
    """
    Loads all profiles from the INI file.
    Returns a dict: {profile_name: env_vars}
    """
    profiles = {}
    if not env_file_path or not os.path.exists(env_file_path):
        return profiles
        
    config = configparser.ConfigParser()
    try:
        config.read(env_file_path, encoding="utf-8")
        for section in config.sections():
            profiles[section] = {
                "SSH_PASSWORD": config.get(section, "SSH_PASSWORD", fallback=""),
                "REMOTE_DIR": config.get(section, "REMOTE_DIR", fallback="")
            }
    except Exception:
        pass
    return profiles

def save_profile(env_file_path, profile_name, env_vars):
    """
    Writes or updates a specific profile in the INI file.
    """
    config = configparser.ConfigParser()
    if os.path.exists(env_file_path):
        try:
            config.read(env_file_path, encoding="utf-8")
        except Exception:
            pass
            
    if not config.has_section(profile_name):
        config.add_section(profile_name)
        
    config.set(profile_name, "SSH_PASSWORD", env_vars.get("SSH_PASSWORD", ""))
    config.set(profile_name, "REMOTE_DIR", env_vars.get("REMOTE_DIR", ""))
    
    with open(env_file_path, "w", encoding="utf-8") as f:
        config.write(f)

def get_default_profile(env_file_path):
    if not env_file_path or not os.path.exists(env_file_path):
        return None
    config = configparser.ConfigParser()
    try:
        config.read(env_file_path, encoding="utf-8")
        if config.has_section("settings"):
            return config.get("settings", "default_profile", fallback=None)
    except Exception:
        pass
    return None

def set_default_profile(env_file_path, profile_name):
    config = configparser.ConfigParser()
    if os.path.exists(env_file_path):
        try:
            config.read(env_file_path, encoding="utf-8")
        except Exception:
            pass
    if not config.has_section("settings"):
        config.add_section("settings")
    config.set("settings", "default_profile", profile_name)
    try:
        with open(env_file_path, "w", encoding="utf-8") as f:
            config.write(f)
    except Exception:
        pass
