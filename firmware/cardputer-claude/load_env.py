Import("env")
import os, re

# Load firmware/.env and inject each KEY=VALUE as a -DENV_KEY build flag,
# so settings (WiFi, server IP) stay out of source and are set at upload time.
env_path = os.path.join(env["PROJECT_DIR"], ".env")
if not os.path.isfile(env_path):
    print("load_env: no .env found (using defaults in main.cpp)")
else:
    flags = []
    with open(env_path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, val = line.split("=", 1)
            key, val = key.strip(), val.strip()
            if re.fullmatch(r"-?\d+", val):        # numeric -> unquoted
                flags.append(f'-DENV_{key}={val}')
            else:                                    # string -> escaped quotes
                esc = val.replace('\\', '\\\\').replace('"', '\\"')
                flags.append(f'-DENV_{key}=\\"{esc}\\"')
    env.Append(BUILD_FLAGS=flags)
    print(f"load_env: injected {len(flags)} setting(s) from .env")
