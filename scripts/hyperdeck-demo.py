#!/usr/bin/env python3
"""Drive the hyperdeck-adapter like a real HyperDeck controller would.

Connects over TCP, reads the connection banner, and sends a scripted sequence of
HyperDeck Ethernet Protocol commands (play / stop / relative goto / transport
info) with pauses so you can watch the locked-on player respond.

Usage: hyperdeck-demo.py [host:port]   (default 127.0.0.1:9993)
"""
import socket
import sys
import time

addr = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1:9993"
host, _, port = addr.partition(":")
sock = socket.create_connection((host, int(port or "9993")))
sock.settimeout(2)


def drain():
    out = b""
    try:
        while True:
            chunk = sock.recv(4096)
            if not chunk:
                break
            out += chunk
            if len(chunk) < 4096:
                break
    except socket.timeout:
        pass
    return out.decode(errors="replace").strip()


def cmd(line, wait=2.5, note=""):
    sock.sendall((line + "\r\n").encode())
    time.sleep(0.4)
    resp = drain()
    head = resp.splitlines()[0] if resp else "(no response)"
    print(f">> {line:24} | {head}   {note}")
    if "transport info" in line:
        for body in resp.splitlines()[1:]:
            if body:
                print(f"      {body}")
    time.sleep(wait)


print("<< banner >>\n" + drain() + "\n")

cmd("transport info", 1.0, "(initial modeled state)")
print("\n--- PLAY (player should start/resume playback) ---")
cmd("play", 3.0, "-> play key")
cmd("transport info", 1.0)
print("\n--- PLAY again (already playing: adapter suppresses, no double-toggle) ---")
cmd("play", 3.0, "-> nothing sent (idempotent)")
print("\n--- STOP ---")
cmd("stop", 3.0, "-> stop/pause key")
cmd("transport info", 1.0)
print("\n--- NEXT clip (relative goto +1) ---")
cmd("goto: clip id: +1", 3.0, "-> next key")
print("\n--- NEXT clip again ---")
cmd("goto: clip id: +1", 3.0, "-> next key")
print("\n--- PREVIOUS clip (relative goto -1) ---")
cmd("goto: clip id: -1", 3.0, "-> prev key")
cmd("transport info", 0.5)

sock.close()
print("\n[demo complete]")
