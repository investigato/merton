# **Merton**

<div align=center>
  <img height="150" alt="merton-logo" src="https://github.com/investigato/merton/blob/main/assets/merton_small.png" />
</div>

## *What the shell?!*

---

## What Merton is

**Mini-disclaimer:** This tool is for authorized security testing and research only. Don't be a jerk. You know the drill. [Full Disclaimer](#disclaimer)

Merton is a Go-based WinRM client for when you've got creds and you'd like to do something useful with them.

It speaks both **CMD** (WinRS) and **PowerShell** (PSRP) because sometimes you need a shell and sometimes you need *a shell*, and those are different things on Windows.

Named after Merton McSnurtle, the world's fastest turtle because irony is a legitimate design philosophy.

---

## What Merton does

- **NTLM authentication** with full NTLM signing and sealing
- **Kerberos authentication** because sometimes NTLM is too loud
- **CMD shell** via WinRS for when you just need cmd.exe to cooperate
- **PowerShell shell** via PSRP for when you need a real runspace
- **Interactive prompt** with history, completion, and multiline paste support
- **Client-side CWD tracking** because WinRM's working directory support is a ghost story

---

## The shell

Merton drops you into an interactive prompt that tries not to embarrass itself:

- History that persists between sessions
- Tab completion
- Multiline paste that doesn't fire on every newline like it's personally offended
- `cd` that actually works, because someone had to fix that

If it breaks, it tells you why. Usually...

---

## Why not just use evil-winrm?

You can. Merton isn't trying to replace it.

Merton exists because sometimes you want a WinRM client that's a single static binary, doesn't need Ruby, and was written by someone who got annoyed enough to write their own, and isn't evil.

Also the mascot has goggles.

## Build it

```bash
go build -ldflags "-s -w -X main.version=0.0.1-percent-of-the-time-it-works-everytime" -trimpath ./cmd/merton
```

## Use it

```bash
merton -i <host> -u <username> -p <password> [flags]
```

### Flag it

| Flag | Default | Description |
| ---- | ------  | ----------- |
| `--host, -i` | | Target hostname or IP |
| `--port, -P` | 5985 | Port |
| `--username, -u` | | Username |
| `--password, -p` | | Password |
| `--hashes, -H` | | Hash |
| `--domain, -d` | | Domain |
| `--tls,-t` | false | Use TLS |
| `--insecure` | false | Skip TLS verification |
| `--target-spn` | | Set the Target Service Principal Name (SPN) |
| `--log-level` | | Log level (info,warn,error,debug) |
| `--kerberos, -k` | false | Kerberos authentication |
| `--krb5conf` | | Path to krb5.conf |
| `--krb5ccache` | | Path to ccache file |
| `--kdc-ip` | | KDC IP address |
| `--realm, -r` | | Kerberos realm |
| `--log-file` | | Path to logfile. |
| `--winrs` | false | Use WinRS (CMD) instead of PSRP (PowerShell) |
| `--enable-cbt` | false | Enabled Channel Binding Tokens |
| `--serveport` | 8080 | Port for upload/download HTTP server |
| `--version, -V` | | Get the app version |

### Shell Commands

| Command | Description |
|---------|-------------|
| `chsh` | Toggle between PowerShell (PSRP) and CMD (WinRS) |
| `upload <local> <remote>` | Upload a file to the target |
| `download <remote> <local>` | Download a file from the target |
| `serveport <port>` | Change the port used for file transfers |
| `exit` / `quit` | Close the session |

---

## Known Limitations

- WinRS `cd` is client-side tracked, the working directory is set at shell creation per command. Pipes and redirects require explicit `cmd /c`.
- Upload/download over WinRS is not yet supported.
- Kerberos keepalive not implemented so long sessions may timeout.

---

## AI Disclosure

Yup, I used a "you're a senior dev mentor and coach" prompted AI partner to talk architecture, rabbit holes, hunting buggies, and the occasional "no that's wrong and here's why." The latter worked both ways, C-Lo hallucinates like a boss sometimes. All code was written and understood by this actual human and I didn't give it write permissions on these files. The 3rd party libraries...that's on their devs. Crappy code here is investigato's fault.

---

## Credits

Born from a rabbit hole that turned into a genuine grudge against every existing WinRM client.

Merton is what happens when Go, offensive tooling, and a turtle with something to prove collide at 2am.

---

## Disclaimer

If you're using this against systems you don't own or don't have explicit written permission to test: don't.

If you're using this for authorized engagements, research, or CTFs: *carry on.*

Don't be evil. Don't get arrested. Do get shells.
