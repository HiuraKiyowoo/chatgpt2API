#!/usr/bin/env python3
"""Langkah 1: tulis config.yaml live (secrets acak) — dipisah biar gateway
bisa start dgn config ini sebelum seeding."""
import os, secrets

ROOT = "/root/chatgpt2API"
if os.path.exists(f"{ROOT}/config.yaml"):
    old = open(f"{ROOT}/config.yaml").read()
    if "jwtSecret:" in old and "CHANGE-ME" not in old:
        print("config.yaml sudah berisi secrets acak — tidak ditimpa")
        raise SystemExit(0)
cfg = f"""server:
  port: 8800
  frontendPath: ./frontend/dist
database:
  path: ./data/chatgpt2api.db
security:
  jwtSecret: "{secrets.token_hex(32)}"
  credentialEncryptionKey: "{secrets.token_hex(32)}"
admin:
  username: admin
  password: "{secrets.token_urlsafe(18)}"
solver:
  enabled: false
  url: http://127.0.0.1:7900
"""
open(f"{ROOT}/config.yaml", "w").write(cfg)
os.chmod(f"{ROOT}/config.yaml", 0o600)
print("config.yaml ditulis (secrets acak, 0600, gitignored)")
